package sshengine

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestPrivateKeyAuthentication(t *testing.T) {
	for _, mode := range []string{"plain", "encrypted", "wrong-passphrase", "wrong-key", "unknown-host", "changed-host", "revoked-host"} {
		t.Run(mode, func(t *testing.T) {
			_, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			pub, err := ssh.NewPublicKey(key.Public())
			if err != nil {
				t.Fatal(err)
			}
			f := server(t, nil, 0, pub)
			if mode == "wrong-key" {
				_, key, _ = ed25519.GenerateKey(rand.Reader)
			}
			var block *pem.Block
			pass := ""
			if mode == "encrypted" || mode == "wrong-passphrase" {
				pass = "ephemeral-key-canary"
				block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "test", []byte(pass))
			} else {
				block, err = ssh.MarshalPrivateKey(key, "test")
			}
			if err != nil {
				t.Fatal(err)
			}
			keyPath := filepath.Join(t.TempDir(), "id_test")
			if err = os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "wrong-passphrase" {
				pass = "wrong-key-canary"
			}
			if strings.HasSuffix(mode, "host") {
				trust, _ := os.ReadFile(f.config.KnownHosts)
				switch mode {
				case "unknown-host":
					trust = nil
				case "changed-host":
					trust = []byte(knownhosts.Line([]string{net.JoinHostPort(f.config.Host, strconv.Itoa(f.config.Port))}, signer(t).PublicKey()) + "\n")
				case "revoked-host":
					trust = append([]byte("@revoked "), trust...)
				}
				if err = os.WriteFile(f.config.KnownHosts, trust, 0600); err != nil {
					t.Fatal(err)
				}
			}
			c := f.config
			c.Password = ""
			c.AuthMethod = "key"
			c.PrivateKeyPath = keyPath
			c.Passphrase = pass
			m := manager(t)
			id := start(t, m, c)
			s, _ := m.get(id)
			want := "failed"
			if mode == "plain" || mode == "encrypted" {
				want = "open"
			}
			eventually(t, func() bool { state, _ := s.status(); return state == want })
			_, public := s.status()
			if strings.Contains(public, pass) && pass != "" {
				t.Fatal("secret in error")
			}
			if strings.HasSuffix(mode, "host") && f.auth.Load() != 0 {
				t.Fatal("key offered before host verification")
			}
			if want == "open" && f.auth.Load() == 0 {
				t.Fatal("no key authentication")
			}
			_ = m.Close(id)
		})
	}
}

func TestPrivateKeyValidation(t *testing.T) {
	for _, mode := range []string{"missing", "directory", "oversize", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "key")
			switch mode {
			case "directory":
				if err := os.Mkdir(p, 0700); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				if err := os.WriteFile(p, make([]byte, maxPrivateKey+1), 0600); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				if err := os.WriteFile(p, []byte("not-a-key-canary"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := authentication(Config{AuthMethod: "key", PrivateKeyPath: p}); err != errPrivateKey {
				t.Fatalf("expected generic key error, got %v", err)
			}
		})
	}
	f := server(t, nil, 0)
	for _, c := range []Config{
		{AuthMethod: "agent"}, {AuthMethod: "key", PrivateKeyPath: "key", Password: "secret"},
		{AuthMethod: "key"}, {AuthMethod: "password", Passphrase: "secret"},
	} {
		c.Host = f.config.Host
		c.Port = f.config.Port
		c.User = f.config.User
		c.KnownHosts = f.config.KnownHosts
		c.Cols = 80
		c.Rows = 24
		if _, err := manager(t).Start(c); err == nil {
			t.Fatal("ambiguous/invalid credentials accepted")
		}
	}
}

func TestOpenSSHWorkFactorBound(t *testing.T) {
	pack := func(b []byte) []byte { return append(binary.BigEndian.AppendUint32(nil, uint32(len(b))), b...) }
	for _, rounds := range []uint32{0, 16, 64, 65, 0xffffffff} {
		data := []byte("openssh-key-v1\x00")
		data = append(data, pack([]byte("aes256-ctr"))...)
		data = append(data, pack([]byte("bcrypt"))...)
		opts := append(pack([]byte("salt")), binary.BigEndian.AppendUint32(nil, rounds)...)
		data = append(data, pack(opts)...)
		got := boundedKeyWork(pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: data}))
		if got != (rounds > 0 && rounds <= 64) {
			t.Fatalf("rounds %d incorrectly accepted/rejected", rounds)
		}
	}
}
