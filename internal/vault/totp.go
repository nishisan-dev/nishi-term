package vault

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var ErrFactor = errors.New("código 2FA inválido, expirado ou já utilizado; use um código novo ou de recuperação")
var aadV2 = []byte("NishiTerm vault v2;argon2id:t3:m65536:p1;aes256gcm;totp")

type twoFactor struct {
	Secret   []byte   `json:"secret"`
	LastStep int64    `json:"lastStep"`
	Recovery []string `json:"recovery"`
}
type payloadV2 struct {
	Entries map[string]Credential `json:"entries"`
	Factor  *twoFactor            `json:"factor"`
}
type enrollment struct {
	secret  []byte
	token   string
	expires time.Time
}
type Enrollment struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	Token  string `json:"token"`
}

func associated(version int) []byte {
	if version == 2 {
		return aadV2
	}
	return aad
}
func validFactor(f *twoFactor) bool {
	if f == nil || len(f.Secret) != 20 || f.LastStep < -1 || len(f.Recovery) > 10 {
		return false
	}
	for _, hash := range f.Recovery {
		b, e := hex.DecodeString(hash)
		if e != nil || len(b) != 32 {
			return false
		}
	}
	return true
}

// hotp implements RFC 4226; TOTP uses floor(unix time / 30) as its counter.
func hotp(secret []byte, counter int64, digits int) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(counter))
	mac := hmac.New(sha1.New, secret)
	mac.Write(b[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	modulus := uint32(1000000)
	if digits == 8 {
		modulus = 100000000
	}
	return fmt.Sprintf("%0*d", digits, value%modulus)
}
func normalizeRecovery(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(code), "-", ""), " ", ""))
}
func recoveryHash(code string) string {
	h := sha256.Sum256([]byte("NishiTerm recovery v1:" + normalizeRecovery(code)))
	return hex.EncodeToString(h[:])
}
func consumeFactor(f *twoFactor, code string, now time.Time) (*twoFactor, error) {
	if !validFactor(f) || len(code) > 64 {
		return nil, ErrFactor
	}
	next := &twoFactor{Secret: append([]byte(nil), f.Secret...), LastStep: f.LastStep, Recovery: append([]string(nil), f.Recovery...)}
	code = strings.TrimSpace(code)
	if len(code) == 6 {
		step := now.Unix() / 30
		matched := int64(-1)
		for delta := int64(-1); delta <= 1; delta++ {
			candidate := step + delta
			if candidate > f.LastStep && candidate >= 0 && subtle.ConstantTimeCompare([]byte(hotp(f.Secret, candidate, 6)), []byte(code)) == 1 {
				matched = candidate
			}
		}
		if matched >= 0 {
			next.LastStep = matched
			return next, nil
		}
	} else if normalized := normalizeRecovery(code); len(normalized) == 32 {
		if b, e := hex.DecodeString(normalized); e == nil && len(b) == 16 {
			hash := recoveryHash(code)
			for i, want := range f.Recovery {
				if subtle.ConstantTimeCompare([]byte(hash), []byte(want)) == 1 {
					next.Recovery = append(next.Recovery[:i], next.Recovery[i+1:]...)
					return next, nil
				}
			}
		}
	}
	clear(next.Secret)
	return nil, ErrFactor
}
func (s *Store) reauthenticate(master string) error {
	if e := s.current(); e != nil {
		return e
	}
	if time.Now().Before(s.retry) {
		return errors.New("aguarde um segundo antes de tentar novamente")
	}
	s.retry = time.Now().Add(time.Second)
	if len(master) > 1024 {
		return ErrUnlock
	}
	candidate := derive(master, s.salt)
	defer clear(candidate)
	if subtle.ConstantTimeCompare(candidate, s.key) != 1 {
		return ErrUnlock
	}
	return nil
}
func (s *Store) BeginTwoFactor(master string) (Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.reauthenticate(master); e != nil {
		return Enrollment{}, e
	}
	if s.factor != nil {
		return Enrollment{}, errors.New("2FA já está ativado")
	}
	if s.pending != nil {
		clear(s.pending.secret)
		s.pending = nil
	}
	secret := make([]byte, 20)
	if _, e := rand.Read(secret); e != nil {
		return Enrollment{}, ErrFile
	}
	token := rand.Text()
	s.pending = &enrollment{secret: secret, token: token, expires: time.Now().Add(5 * time.Minute)}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	// An opaque per-enrollment label distinguishes vaults without disclosing account/host details.
	label := "NishiTerm:Cofre-" + token[:6]
	uri := "otpauth://totp/" + url.PathEscape(label) + "?" + url.Values{"secret": {encoded}, "issuer": {"NishiTerm"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}.Encode()
	s.retry = time.Time{}
	s.touch()
	return Enrollment{Secret: encoded, URI: uri, Token: token}, nil
}
func (s *Store) CancelTwoFactorEnrollment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil {
		clear(s.pending.secret)
		s.pending = nil
	}
}
func (s *Store) ConfirmTwoFactor(token, code string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.current(); e != nil {
		return nil, e
	}
	if s.factor != nil || s.pending == nil || s.pending.token != token || time.Now().After(s.pending.expires) {
		return nil, errors.New("configuração 2FA expirada; comece novamente")
	}
	if time.Now().Before(s.retry) {
		return nil, errors.New("aguarde um segundo antes de tentar novamente")
	}
	s.retry = time.Now().Add(time.Second)
	next, e := consumeFactor(&twoFactor{Secret: s.pending.secret, LastStep: -1}, code, time.Now())
	if e != nil {
		return nil, e
	}
	codes := make([]string, 10)
	for i := range codes {
		var b [16]byte
		if _, e = rand.Read(b[:]); e != nil {
			clear(next.Secret)
			return nil, ErrFile
		}
		raw := strings.ToUpper(hex.EncodeToString(b[:]))
		codes[i] = raw[:8] + "-" + raw[8:16] + "-" + raw[16:24] + "-" + raw[24:]
		next.Recovery = append(next.Recovery, recoveryHash(codes[i]))
	}
	s.factor = next
	if e = s.write(s.entries, false); e != nil {
		clear(next.Secret)
		s.factor = nil
		return nil, e
	}
	clear(s.pending.secret)
	s.pending = nil
	s.retry = time.Time{}
	s.touch()
	return codes, nil
}
func (s *Store) DisableTwoFactor(master, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.reauthenticate(master); e != nil {
		return e
	}
	if s.factor == nil {
		return errors.New("2FA não está ativado")
	}
	used, e := consumeFactor(s.factor, code, time.Now())
	if e != nil {
		// A recovery-authenticated session may use its consumed code to remove a
		// lost authenticator, even when that was the last available recovery code.
		if len(code) > 64 {
			return e
		}
		proof := recoveryHash(code)
		if s.recoveryProof == "" || !time.Now().Before(s.recoveryUntil) || subtle.ConstantTimeCompare([]byte(proof), []byte(s.recoveryProof)) != 1 {
			return e
		}
	} else {
		clear(used.Secret)
	}
	previous := s.factor
	s.factor = nil
	if e = s.write(s.entries, false); e != nil {
		if s.key != nil {
			s.factor = previous
		}
		return e
	}
	clear(previous.Secret)
	s.recoveryProof = ""
	s.recoveryUntil = time.Time{}
	s.retry = time.Time{}
	s.touch()
	return nil
}
