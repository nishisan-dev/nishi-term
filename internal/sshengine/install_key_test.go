package sshengine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestVaultKeyAuthenticationAndPrivateJSON(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	pub, _ := ssh.NewPublicKey(private.Public())
	block, _ := ssh.MarshalPrivateKey(private, "")
	f := server(t, nil, 0, pub)
	m := manager(t)
	c := f.config
	c.Password = ""
	c.AuthMethod = "key"
	data := pem.EncodeToMemory(block)
	c = WithPrivateKey(c, data)
	raw, e := json.Marshal(c)
	if e != nil || strings.Contains(string(raw), "PRIVATE KEY") || strings.Contains(string(raw), "keyData") {
		t.Fatal("secret exposed")
	}
	id := start(t, m, c)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	for _, b := range data {
		if b != 0 {
			t.Fatal("temporary key bytes not cleared")
		}
	}
	var injected Config
	json.Unmarshal([]byte(`{"authMethod":"key","keyData":"secret"}`), &injected)
	if len(injected.keyData) != 0 {
		t.Fatal("renderer supplied private material")
	}
	var ack uint64
	if e := echoOnce(m, id, &ack, "vault-key-login"); e != nil {
		t.Fatal(e)
	}
	if e := m.InstallPublicKey(id, "wrong.invalid", 22, "test", string(ssh.MarshalAuthorizedKey(pub))); e == nil {
		t.Fatal("wrong endpoint accepted")
	}
}
func TestInstallCommandPreservesExistingKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell script executed on Linux; Windows builds use remote shell")
	}
	public := string(ssh.MarshalAuthorizedKey(signer(t).PublicKey()))
	command, e := installKeyCommand(strings.TrimSpace(public) + " comment; touch BAD")
	if e != nil {
		t.Fatal(e)
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	os.Mkdir(dir, 0700)
	file := filepath.Join(dir, "authorized_keys")
	original := "# existing settings\nssh-ed25519 ORIGINAL other\n"
	os.WriteFile(file, []byte(original), 0600)
	for i := 0; i < 2; i++ {
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = home
		cmd.Env = append(os.Environ(), "HOME="+home)
		if e = cmd.Run(); e != nil {
			t.Fatal(e)
		}
	}
	data, _ := os.ReadFile(file)
	blob := strings.Fields(public)[1]
	if !strings.HasPrefix(string(data), original) || strings.Count(string(data), blob) != 1 {
		t.Fatal("existing keys lost or duplicate added")
	}
	if _, e = os.Stat(filepath.Join(home, "BAD")); !os.IsNotExist(e) {
		t.Fatal("comment interpreted")
	}
	restricted := "command=\"echo restricted\" " + strings.TrimSpace(public) + "\n"
	os.WriteFile(file, []byte(restricted), 0600)
	restrictedCommand := exec.Command("sh", "-c", command)
	restrictedCommand.Dir = home
	restrictedCommand.Env = append(os.Environ(), "HOME="+home)
	if e = restrictedCommand.Run(); e == nil {
		t.Fatal("restricted key was treated as unrestricted")
	}
	unchanged, _ := os.ReadFile(file)
	if string(unchanged) != restricted {
		t.Fatal("restricted key modified")
	}
	os.Remove(file)
	outside := filepath.Join(home, "outside")
	os.WriteFile(outside, []byte("unchanged"), 0600)
	os.Symlink(outside, file)
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "HOME="+home)
	if e = cmd.Run(); e == nil {
		t.Fatal("symlink accepted")
	}
	data, _ = os.ReadFile(outside)
	if string(data) != "unchanged" {
		t.Fatal("symlink target modified")
	}
	if _, e = installKeyCommand("command=\"evil\" " + public); e == nil {
		t.Fatal("options accepted")
	}
}

func TestInstallOverVerifiedSSH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture remote POSIX shell runs on Linux")
	}
	home := t.TempDir()
	f := serverWithFiles(t, nil, 0, home)
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	public := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer(t).PublicKey())))
	for i := 0; i < 2; i++ {
		if e := m.InstallPublicKey(id, f.config.Host, f.config.Port, f.config.User, public); e != nil {
			t.Fatal(e)
		}
	}
	contents, e := os.ReadFile(filepath.Join(home, ".ssh", "authorized_keys"))
	if e != nil || strings.Count(string(contents), public) != 1 {
		t.Fatal("remote key missing or duplicate")
	}
	var ack uint64
	if e := echoOnce(m, id, &ack, "after-key-install"); e != nil {
		t.Fatal(e)
	}
}
