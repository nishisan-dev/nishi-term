package vault

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRFC6238SHA1Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, v := range []struct {
		unix int64
		code string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"}} {
		if got := hotp(secret, v.unix/30, 8); got != v.code {
			t.Fatalf("vector %d: %s", v.unix, got)
		}
	}
}
func TestTOTPWindowAndReplay(t *testing.T) {
	f := &twoFactor{Secret: []byte("12345678901234567890"), LastStep: -1}
	now := time.Unix(1234567890, 0)
	step := now.Unix() / 30
	for _, delta := range []int64{-1, 0, 1} {
		next, e := consumeFactor(f, hotp(f.Secret, step+delta, 6), now)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = consumeFactor(next, hotp(f.Secret, step+delta, 6), now); e == nil {
			t.Fatal("replay accepted")
		}
	}
	if _, e := consumeFactor(f, hotp(f.Secret, step-3, 6), now); e == nil {
		t.Fatal("expired code accepted")
	}
	if _, e := consumeFactor(f, "abcdef", now); e == nil {
		t.Fatal("invalid code accepted")
	}
}
func TestTwoFactorLifecycleRecoveryAndTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	s := New(path)
	defer s.Lock()
	master := "master-password-2fa-test"
	if e := s.Create(master); e != nil {
		t.Fatal(e)
	}
	if e := s.Save("host", Credential{Host: "test.invalid", Port: 22, User: "tester", Method: "password", Password: "never-plaintext"}); e != nil {
		t.Fatal(e)
	}
	setup, e := s.BeginTwoFactor(master)
	if e != nil {
		t.Fatal(e)
	}
	secret, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(setup.URI, "otpauth://totp/") {
		t.Fatal("invalid enrollment URI")
	}
	if status, _ := s.Status(); status.TwoFactor {
		t.Fatal("enabled without confirmation")
	}
	if _, e = s.ConfirmTwoFactor("wrong-token", "123456"); e == nil {
		t.Fatal("wrong enrollment accepted")
	}
	if _, e = s.ConfirmTwoFactor(setup.Token, "abcdef"); e == nil {
		t.Fatal("wrong OTP enabled 2FA")
	}
	s.retry = time.Time{}
	code := hotp(secret, time.Now().Unix()/30, 6)
	codes, e := s.ConfirmTwoFactor(setup.Token, code)
	if e != nil || len(codes) != 10 {
		t.Fatal("enrollment failed", e)
	}
	if status, _ := s.Status(); !status.TwoFactor {
		t.Fatal("status missing factor")
	}
	original, _ := os.ReadFile(path)
	if strings.Contains(string(original), setup.Secret) || strings.Contains(string(original), codes[0]) || strings.Contains(string(original), "never-plaintext") {
		t.Fatal("plaintext persisted")
	}
	s.Lock()
	if e = s.Unlock(master); !errors.Is(e, ErrFactor) {
		t.Fatal("master alone unlocked", e)
	}
	if _, e = s.Get("host"); !errors.Is(e, ErrLocked) {
		t.Fatal("secret exposed before MFA")
	}
	s.retry = time.Time{}
	if e = s.Unlock(master, code); !errors.Is(e, ErrFactor) {
		t.Fatal("enrollment OTP reused", e)
	}
	s.retry = time.Time{}
	// Consumption must be durable before granting access.
	if e = os.WriteFile(path+".lock", nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.Unlock(master, codes[0]); e == nil {
		t.Fatal("unlock succeeded without durable consumption")
	}
	os.Remove(path + ".lock")
	s.retry = time.Time{}
	if e = s.Unlock(master, codes[0]); e != nil {
		t.Fatal(e)
	}
	if c, e := s.Get("host"); e != nil || c.Password != "never-plaintext" {
		t.Fatal("credential lost")
	}
	s.Lock()
	other := New(path)
	defer other.Lock()
	if e = other.Unlock(master, codes[0]); !errors.Is(e, ErrFactor) {
		t.Fatal("recovery code replay after restart", e)
	}
	other.retry = time.Time{}
	if e = other.Unlock(master, codes[1]); e != nil {
		t.Fatal(e)
	}
	if e = other.DisableTwoFactor("wrong-master", codes[2]); e == nil {
		t.Fatal("disabled with wrong master")
	}
	other.retry = time.Time{}
	other.recoveryUntil = time.Now().Add(-time.Second)
	if e = other.DisableTwoFactor(master, codes[1]); !errors.Is(e, ErrFactor) {
		t.Fatal("expired recovery-session proof accepted", e)
	}
	other.retry = time.Time{}
	other.recoveryUntil = time.Now().Add(time.Minute)
	if e = other.DisableTwoFactor(master, codes[1]); e != nil {
		t.Fatal(e)
	}
	other.Lock()
	if e = other.Unlock(master); e != nil {
		t.Fatal("legacy unlock after disable", e)
	}
	// An attacker cannot downgrade a v2 envelope to bypass MFA.
	var env envelope
	json.Unmarshal(original, &env)
	env.Version = 1
	tampered, _ := json.Marshal(env)
	os.WriteFile(path, tampered, 0600)
	other.Lock()
	other.retry = time.Time{}
	if e = other.Unlock(master); e == nil {
		t.Fatal("downgrade accepted")
	}
}
func TestEnrollmentCancellationAndExpiry(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "vault.json"))
	defer s.Lock()
	master := "master-password-expiry"
	if _, e := s.BeginTwoFactor(master); !errors.Is(e, ErrLocked) {
		t.Fatal("enrollment while locked")
	}
	s.Create(master)
	setup, e := s.BeginTwoFactor(master)
	if e != nil {
		t.Fatal(e)
	}
	secret := s.pending.secret
	s.CancelTwoFactorEnrollment()
	for _, b := range secret {
		if b != 0 {
			t.Fatal("pending seed not cleared")
		}
	}
	if _, e = s.ConfirmTwoFactor(setup.Token, "123456"); e == nil {
		t.Fatal("cancelled enrollment accepted")
	}
	s.BeginTwoFactor(master)
	s.pending.expires = time.Now().Add(-time.Minute)
	if _, e = s.ConfirmTwoFactor(s.pending.token, hotp(s.pending.secret, time.Now().Unix()/30, 6)); e == nil {
		t.Fatal("expired enrollment accepted")
	}
	s.Lock()
	if s.pending != nil {
		t.Fatal("enrollment survived lock")
	}
}
