package credentials

import (
	"nishiterm/internal/profiles"
	"nishiterm/internal/sshengine"
	"nishiterm/internal/vault"
	"path/filepath"
	"testing"
)

func TestSavedCredentialIsBoundToProfileDestination(t *testing.T) {
	dir := t.TempDir()
	s := &Service{Profiles: profiles.New(filepath.Join(dir, "profiles.json")), Vault: vault.New(filepath.Join(dir, "vault.json"))}
	defer s.Vault.Lock()
	items, e := s.Profiles.Save(profiles.Profile{Name: "fixture", Host: "one.invalid", Port: 22, User: "tester"})
	if e != nil {
		t.Fatal(e)
	}
	p := items[0]
	if e = s.Vault.Create("master-password-canary"); e != nil {
		t.Fatal(e)
	}
	c := sshengine.Config{Host: p.Host, Port: p.Port, User: p.User, AuthMethod: "password", Password: "secret"}
	if e = s.SaveFor(p.ID, c); e != nil {
		t.Fatal(e)
	}
	c.Password = ""
	c.AuthMethod = "vault"
	c.SavedCredential = p.ID
	resolved, e := s.Resolve(c)
	if e != nil || resolved.Password != "secret" || resolved.SavedCredential != "" {
		t.Fatal("resolution failed")
	}
	p.Host = "other.invalid"
	if _, e = s.Profiles.Save(p); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Resolve(c); e == nil {
		t.Fatal("stale tab accepted")
	}
	c.Host = p.Host
	if _, e = s.Resolve(c); e == nil {
		t.Fatal("credential redirected to another host")
	}
	c.Host = "one.invalid"
	c.AuthMethod = "password"
	c.Password = "newsecret"
	if e = s.SaveFor(p.ID, c); e == nil {
		t.Fatal("stale tab saved credential to changed profile")
	}
}
