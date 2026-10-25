package vault

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const master = "a-long-master-canary-2026"

func credential() Credential {
	return Credential{Host: "fixture.invalid", Port: 22, User: "tester", Method: "password", Password: "credential-canary-never-plaintext"}
}
func TestVaultLifecycleAndEncryption(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.json")
	s := New(p)
	defer s.Lock()
	if _, e := s.Get("a"); e != ErrLocked {
		t.Fatal("locked vault readable")
	}
	if e := s.Create(master); e != nil {
		t.Fatal(e)
	}
	if e := s.Save("a", credential()); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(p)
	for _, secret := range []string{master, credential().Password, credential().Host} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("plaintext in vault")
		}
	}
	s.Lock()
	if _, e := s.Get("a"); e != ErrLocked {
		t.Fatal("lock ineffective")
	}
	fresh := New(p)
	defer fresh.Lock()
	if st, e := fresh.Status(); e != nil || !st.Exists || !st.Locked {
		t.Fatal("restart not locked")
	}
	if e := fresh.Unlock("wrong-password"); e != ErrUnlock {
		t.Fatal("wrong password accepted")
	}
	fresh.retry = time.Time{}
	if e := fresh.Unlock(master); e != nil {
		t.Fatal(e)
	}
	got, e := fresh.Get("a")
	if e != nil || got != credential() {
		t.Fatal("roundtrip failed")
	}
	if e := fresh.Save("a", credential()); e != nil {
		t.Fatal(e)
	}
	again, _ := os.ReadFile(p)
	if bytes.Equal(raw, again) {
		t.Fatal("nonce reused")
	}
	if e := fresh.Delete("a"); e != nil {
		t.Fatal(e)
	}
	if _, e := fresh.Get("a"); e == nil {
		t.Fatal("delete failed")
	}
	fresh.expires = time.Now().Add(-time.Second).UnixNano()
	if _, e := fresh.Get("a"); e != ErrLocked {
		t.Fatal("idle timeout failed")
	}
}
func TestVaultTamperAndConflictingWriter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "vault.json")
	a := New(p)
	defer a.Lock()
	if e := a.Create(master); e != nil {
		t.Fatal(e)
	}
	b := New(p)
	defer b.Lock()
	if e := b.Unlock(master); e != nil {
		t.Fatal(e)
	}
	if e := a.Save("a", credential()); e != nil {
		t.Fatal(e)
	}
	if e := b.Save("b", credential()); e != ErrFile {
		t.Fatal("stale writer overwrote vault")
	}
	original, _ := os.ReadFile(p)
	os.WriteFile(p+".lock", nil, 0600)
	if e := a.Save("b", credential()); e == nil {
		t.Fatal("lock ignored")
	}
	os.Remove(p + ".lock")
	after, _ := os.ReadFile(p)
	if !bytes.Equal(original, after) {
		t.Fatal("failed write changed vault")
	}
	var env envelope
	if json.Unmarshal(original, &env) != nil {
		t.Fatal("invalid fixture")
	}
	env.Data[len(env.Data)-1] ^= 1
	bad, _ := json.Marshal(env)
	os.WriteFile(p, bad, 0600)
	a.Lock()
	if e := a.Unlock(master); e != ErrUnlock {
		t.Fatal("tampered ciphertext accepted")
	}
	if e := a.Create(master); e == nil {
		t.Fatal("existing vault overwritten")
	}
	after, _ = os.ReadFile(p)
	if !bytes.Equal(after, bad) {
		t.Fatal("corrupt vault lost")
	}
}
func TestVaultInvalidFormatsFailBeforeKDF(t *testing.T) {
	for _, data := range []string{`{}`, `{"version":99,"salt":"","data":""}`, `{"version":1,"salt":"AA==","data":"AA=="}`, `not-json`} {
		p := filepath.Join(t.TempDir(), "vault.json")
		os.WriteFile(p, []byte(data), 0600)
		s := New(p)
		if e := s.Unlock(master); e != ErrUnlock {
			t.Fatal("invalid vault accepted")
		}
	}
}
