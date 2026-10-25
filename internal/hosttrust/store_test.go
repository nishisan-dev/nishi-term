package hosttrust

import (
	"crypto/ed25519"
	"crypto/rand"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"testing"
)

func key(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	k, e := ssh.NewPublicKey(pub)
	if e != nil {
		t.Fatal(e)
	}
	return k
}
func TestTrustStore(t *testing.T) {
	p := filepath.Join(t.TempDir(), "trust.json")
	s := New(p)
	k := key(t)
	endpoint := Endpoint("EXAMPLE.invalid", 2222)
	if ok, e := s.Check(endpoint, k); ok || e != nil {
		t.Fatal("expected unknown")
	}
	if e := s.Accept(endpoint, k); e != nil {
		t.Fatal(e)
	}
	if ok, e := New(p).Check(endpoint, k); !ok || e != nil {
		t.Fatal("not persisted")
	}
	before, _ := os.ReadFile(p)
	if e := s.Accept(endpoint, key(t)); e != ErrChanged {
		t.Fatal("replacement allowed")
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("changed file")
	}
	os.WriteFile(p+".lock", nil, 0600)
	if e := s.Accept(Endpoint("other.invalid", 22), k); e == nil {
		t.Fatal("competing writer allowed")
	}
	os.Remove(p + ".lock")
	os.WriteFile(p, []byte("invalid"), 0600)
	if e := s.Accept(endpoint, k); e == nil {
		t.Fatal("corrupt store overwritten")
	}
	after, _ = os.ReadFile(p)
	if string(after) != "invalid" {
		t.Fatal("corrupt file lost")
	}
}
