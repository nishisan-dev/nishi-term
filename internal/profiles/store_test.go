package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func valid() Profile {
	return Profile{Name: "Laboratório", Group: "Local", Host: "127.0.0.1", Port: 22, User: "nishi", KnownHosts: "/home/test/.ssh/known_hosts"}
}
func TestMetadataRoundTripAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "profiles.json")
	s := New(path)
	p := valid()
	items, e := s.Save(p)
	if e != nil {
		t.Fatal(e)
	}
	if len(items) != 1 || items[0].ID == "" {
		t.Fatal("missing generated ID")
	}
	p = items[0]
	p.Name = "Renomeado"
	p.Group = ""
	if _, e = s.Save(p); e != nil {
		t.Fatal(e)
	}
	loaded, e := New(path).List()
	if e != nil || len(loaded) != 1 || loaded[0] != p {
		t.Fatal("round trip failed")
	}
	if _, e = s.Delete(p.ID); e != nil {
		t.Fatal(e)
	}
	loaded, e = s.List()
	if e != nil || len(loaded) != 0 {
		t.Fatal("delete not persisted")
	}
}
func TestCorruptOrFutureFileIsPreserved(t *testing.T) {
	for _, raw := range []string{`{broken`, `{"version":2,"profiles":[]}`, `{"version":1,"profiles":[],"password":"canary"}`, `{"version":1,"profiles":[]} {}`} {
		t.Run(raw, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.json")
			if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
				t.Fatal(e)
			}
			s := New(path)
			if _, e := s.List(); e == nil {
				t.Fatal("bad document accepted")
			}
			if _, e := s.Save(valid()); e == nil {
				t.Fatal("bad document overwritten")
			}
			data, _ := os.ReadFile(path)
			if string(data) != raw {
				t.Fatal("original file changed")
			}
		})
	}
}
func TestMetadataAllowlistAndConcurrency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s := New(path)
	// Wails ignores unknown DTO fields; the store's explicit type cannot serialize them.
	raw := `{"name":"test","host":"localhost","port":22,"user":"nishi","knownHosts":"/trusted","password":"canary-secret","passphrase":"canary-secret"}`
	var p Profile
	if json.Unmarshal([]byte(raw), &p) != nil {
		t.Fatal("decode")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Save(p); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	items, e := s.List()
	if e != nil || len(items) != 12 {
		t.Fatal("lost concurrent writes")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "canary-secret") || strings.Contains(string(data), "password") {
		t.Fatal("secret field persisted")
	}
}
func TestRejectInvalidProfileWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s := New(path)
	p := valid()
	p.Port = 0
	if _, e := s.Save(p); e == nil {
		t.Fatal("invalid port accepted")
	}
	p = valid()
	p.ID = "nonexistent"
	if _, e := s.Save(p); e == nil {
		t.Fatal("unknown ID accepted")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("invalid save created file")
	}
}

func TestAuthenticationPreferences(t *testing.T) {
	for _, method := range []string{"", "password", "vault", "key"} {
		t.Run(method, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.json")
			s := New(path)
			p := valid()
			p.AuthMethod = method
			if method == "key" {
				p.PrivateKeyPath = "/home/test/.ssh/id_ed25519"
			}
			saved, err := s.Save(p)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := New(path).List()
			if err != nil || loaded[0] != saved[0] {
				t.Fatal("preference lost", err)
			}
		})
	}
	for _, p := range []Profile{
		func() Profile { p := valid(); p.AuthMethod = "unknown"; return p }(),
		func() Profile { p := valid(); p.AuthMethod = "key"; return p }(),
		func() Profile { p := valid(); p.PrivateKeyPath = "/unexpected"; return p }(),
		func() Profile { p := valid(); p.AuthMethod = "key"; p.PrivateKeyPath = "bad\npath"; return p }(),
	} {
		if _, err := New(filepath.Join(t.TempDir(), "profiles.json")).Save(p); err == nil {
			t.Fatal("invalid preference accepted")
		}
	}
}
