package credentials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nishiterm/internal/profiles"
	"nishiterm/internal/sshengine"
	"nishiterm/internal/vault"
)

func TestManagedKeysAndGroupCoverage(t *testing.T) {
	dir := t.TempDir()
	v := vault.New(filepath.Join(dir, "vault.json"))
	defer v.Lock()
	s := &Service{Vault: v, Profiles: profiles.New(filepath.Join(dir, "profiles.json"))}
	if _, e := s.GenerateKey("locked"); e == nil {
		t.Fatal("generated while locked")
	}
	if e := v.Create("master-key-test-canary"); e != nil {
		t.Fatal(e)
	}
	for _, host := range []string{"one.invalid", "two.invalid"} {
		if _, e := s.Profiles.Save(profiles.Profile{Name: host, Group: "lab", Host: host, Port: 22, User: "tester"}); e != nil {
			t.Fatal(e)
		}
	}
	ps, _ := s.Profiles.List()
	key, e := s.GenerateKey("shared")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(key.PublicKey, "ssh-ed25519 ") || !strings.HasPrefix(key.Fingerprint, "SHA256:") {
		t.Fatal("invalid public metadata")
	}
	metadata, _ := json.Marshal(key)
	if strings.Contains(string(metadata), "PRIVATE") {
		t.Fatal("private key in DTO")
	}
	if e = s.AssignKey(key.ID, "", "lab", true); e != nil {
		t.Fatal(e)
	}
	coverage, e := s.Coverage("lab")
	if e != nil || len(coverage) != 2 || !coverage[0].Ready || !coverage[1].Ready {
		t.Fatal("group coverage")
	}
	if e = s.Save(ps[0].ID, "password", "", "password-canary", ""); e != nil {
		t.Fatal(e)
	}
	config := func(p profiles.Profile) sshengine.Config {
		return sshengine.Config{SavedCredential: p.ID, Host: p.Host, Port: p.Port, User: p.User, KnownHosts: p.KnownHosts, AuthMethod: "vault"}
	}
	first, e := s.Resolve(config(ps[0]))
	if e != nil || first.AuthMethod != "password" || first.Password != "password-canary" {
		t.Fatal("host override lost")
	}
	second, e := s.Resolve(config(ps[1]))
	if e != nil || second.AuthMethod != "key" || second.PrivateKeyPath != "" {
		t.Fatal("group key not resolved")
	}
	encoded, _ := json.Marshal(second)
	if strings.Contains(string(encoded), "PRIVATE KEY") || strings.Contains(string(encoded), "keyData") {
		t.Fatal("key bytes cross JSON")
	}
	p := ps[0]
	p.Host = "changed.invalid"
	s.Profiles.Save(p)
	cov, _ := s.Coverage("lab")
	if cov[0].Ready {
		t.Fatal("stale host credential inherited silently")
	}
	if _, e = s.Resolve(config(p)); e == nil {
		t.Fatal("stale credential accepted")
	}
	if e = s.AssignKey(key.ID, p.ID, "", false); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Resolve(config(p)); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "vault.json"))
	if strings.Contains(string(raw), "PRIVATE KEY") || strings.Contains(string(raw), "password-canary") || strings.Contains(string(raw), "shared") {
		t.Fatal("plaintext in vault")
	}
	v.Lock()
	if _, e = s.Coverage("lab"); e == nil {
		t.Fatal("coverage while locked")
	}
	if _, e = s.PublicKey(key.ID); e == nil {
		t.Fatal("key while locked")
	}
	if e = v.Unlock("master-key-test-canary"); e != nil {
		t.Fatal(e)
	}
	keys, e := s.Keys()
	if e != nil || len(keys) != 1 || keys[0].PublicKey != key.PublicKey {
		t.Fatal("key not preserved across unlock")
	}
	p.Group = "other"
	s.Profiles.Save(p)
	// A host association remains bound to that host even when its group changes.
	if _, e = s.Resolve(config(p)); e != nil {
		t.Fatal(e)
	}
}

func TestAssignedKeyMetadataPreservesHostAndGroupAssociations(t *testing.T) {
	dir := t.TempDir()
	v := vault.New(filepath.Join(dir, "vault.json"))
	defer v.Lock()
	s := &Service{Vault: v, Profiles: profiles.New(filepath.Join(dir, "profiles.json"))}
	if e := v.Create("assigned-key-metadata-fixture"); e != nil {
		t.Fatal(e)
	}
	ps, e := s.Profiles.Save(profiles.Profile{Name: "fixture", Group: "lab", Host: "fixture.invalid", Port: 22, User: "tester"})
	if e != nil {
		t.Fatal(e)
	}
	p := ps[0]
	if key, e := s.AssignedKey(p.ID); e != nil || key != nil {
		t.Fatal("unexpected association", e)
	}
	key, e := s.GenerateKey("group fixture")
	if e != nil {
		t.Fatal(e)
	}
	if e := s.AssignKey(key.ID, "", "lab", true); e != nil {
		t.Fatal(e)
	}
	before, _ := v.Snapshot()
	assigned, e := s.AssignedKey(p.ID)
	if e != nil || assigned == nil || assigned.ID != key.ID || assigned.Fingerprint != key.Fingerprint {
		t.Fatal("inherited public metadata unavailable", e)
	}
	after, _ := v.Snapshot()
	if len(before) != len(after) {
		t.Fatal("inspection wrote a host association")
	}
	if _, ok := after[p.ID]; ok {
		t.Fatal("inspection overrode group default")
	}
	if e := s.Save(p.ID, "password", "", "fixture-secret", ""); e != nil {
		t.Fatal(e)
	}
	if assigned, e := s.AssignedKey(p.ID); e != nil || assigned != nil {
		t.Fatal("password host association fell back to group key", e)
	}
	if e := s.AssignKey(key.ID, p.ID, "", false); e != nil {
		t.Fatal(e)
	}
	p.Host = "changed.invalid"
	if _, e := s.Profiles.Save(p); e != nil {
		t.Fatal(e)
	}
	if _, e := s.AssignedKey(p.ID); e == nil {
		t.Fatal("stale host binding accepted")
	}
	v.Lock()
	if _, e := s.AssignedKey(p.ID); e == nil {
		t.Fatal("metadata read while locked")
	}
}
