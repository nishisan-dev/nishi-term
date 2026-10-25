// Package vault protects credentials at rest with explicit master-password unlock.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/argon2"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrLocked = errors.New("cofre bloqueado: desbloqueie com sua senha mestra")
var ErrFile = errors.New("cofre indisponível ou alterado; arquivo preservado, desbloqueie novamente")
var ErrUnlock = errors.New("não foi possível desbloquear: senha incorreta ou cofre inválido")

const maxSize = 2 << 20
const idle = 5 * time.Minute

var aad = []byte("NishiTerm vault v1;argon2id:t3:m65536:p1;aes256gcm")

type Credential struct {
	KeyID      string `json:"keyID,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Label      string `json:"label,omitempty"`
	Group      string `json:"group,omitempty"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Method     string `json:"method"`
	KeyPath    string `json:"keyPath"`
	Password   string `json:"password"`
	Passphrase string `json:"passphrase"`
}
type envelope struct {
	Version int    `json:"version"`
	Salt    []byte `json:"salt"`
	Data    []byte `json:"data"`
}
type Status struct {
	Exists    bool `json:"exists"`
	Locked    bool `json:"locked"`
	TwoFactor bool `json:"twoFactor"`
}
type Store struct {
	recoveryProof string
	recoveryUntil time.Time
	factor        *twoFactor
	pending       *enrollment
	mu            sync.Mutex
	path          string
	key, salt     []byte
	entries       map[string]Credential
	digest        [32]byte
	expires       int64
	retry         time.Time
	timer         *time.Timer
	epoch         uint64
}

func New(path string) *Store { return &Store{path: path} }
func (s *Store) lock() {
	s.recoveryProof = ""
	s.recoveryUntil = time.Time{}
	if s.factor != nil {
		clear(s.factor.Secret)
		s.factor = nil
	}
	if s.pending != nil {
		clear(s.pending.secret)
		s.pending = nil
	}
	s.epoch++
	if s.timer != nil {
		s.timer.Stop()
	}
	clear(s.key)
	s.key = nil
	s.salt = nil
	clear(s.entries)
	s.entries = nil
	s.expires = 0
}
func (s *Store) Lock() { s.mu.Lock(); defer s.mu.Unlock(); s.lock() }
func (s *Store) expire() {
	if s.key != nil && time.Now().UnixNano() >= s.expires {
		s.lock()
	}
}
func (s *Store) touch() {
	s.epoch++
	generation := s.epoch
	s.expires = time.Now().Add(idle).UnixNano()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(idle, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if generation == s.epoch {
			s.lock()
		}
	})
}
func (s *Store) Status() (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	st, e := os.Stat(s.path)
	if os.IsNotExist(e) {
		s.lock()
		return Status{Locked: true}, nil
	}
	if e != nil || !st.Mode().IsRegular() {
		return Status{Exists: true, Locked: true}, ErrFile
	}
	raw, e := read(s.path)
	if e != nil {
		return Status{Exists: true, Locked: true}, e
	}
	var env envelope
	if decode(raw, &env) != nil || (env.Version != 1 && env.Version != 2) {
		return Status{Exists: true, Locked: true}, ErrFile
	}
	return Status{Exists: true, Locked: s.key == nil, TwoFactor: env.Version == 2}, nil
}
func read(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, ErrFile
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > maxSize {
		return nil, ErrFile
	}
	b, e := io.ReadAll(io.LimitReader(f, maxSize+1))
	if e != nil || len(b) > maxSize {
		return nil, ErrFile
	}
	return b, nil
}
func decode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrFile
	}
	var x any
	if d.Decode(&x) != io.EOF {
		return ErrFile
	}
	return nil
}
func derive(master string, salt []byte) []byte {
	p := []byte(master)
	defer clear(p)
	return argon2.IDKey(p, salt, 3, 64*1024, 1, 32)
}
func aead(key []byte) (cipher.AEAD, error) {
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCMWithRandomNonce(b)
}
func valid(c Credential) bool {
	if len(c.KeyID) > 64 || len(c.Label) > 100 || len(c.Group) > 100 || len(c.PrivateKey) > 4096 {
		return false
	}
	switch c.Method {
	case "identity":
		return c.PrivateKey != "" && c.Label != "" && c.KeyID == "" && c.Host == "" && c.Port == 0 && c.User == "" && c.Group == "" && c.Password == "" && c.Passphrase == "" && c.KeyPath == ""
	case "group-key":
		return c.KeyID != "" && c.PrivateKey == "" && c.Label == "" && c.Host == "" && c.Port == 0 && c.User == "" && c.Password == "" && c.Passphrase == "" && c.KeyPath == ""
	}
	if c.PrivateKey != "" || c.Label != "" || c.Group != "" {
		return false
	}
	return c.Host != "" && len(c.Host) <= 253 && c.User != "" && len(c.User) <= 256 && c.Port > 0 && c.Port <= 65535 && len(c.Password) <= 65536 && len(c.Passphrase) <= 65536 && len(c.KeyPath) <= 4096 && ((c.Method == "password" && c.KeyID == "" && c.KeyPath == "" && c.Passphrase == "") ||
		(c.Method == "key" && c.KeyID == "" && c.KeyPath != "" && c.Password == "") ||
		(c.Method == "managed-key" && c.KeyID != "" && c.KeyPath == "" && c.Password == "" && c.Passphrase == ""))
}

// Snapshot is backend-only; metadata APIs must never expose Credential values.
func (s *Store) Snapshot() (map[string]Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.current(); e != nil {
		return nil, e
	}
	result := make(map[string]Credential, len(s.entries))
	for k, v := range s.entries {
		result[k] = v
	}
	return result, nil
}

func (s *Store) Create(master string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(master) < 12 || len(master) > 1024 {
		return errors.New("use uma senha mestra de 12 a 1024 bytes")
	}
	if _, e := os.Stat(s.path); !os.IsNotExist(e) {
		return errors.New("cofre já existe ou não pode ser criado")
	}
	s.lock()
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return ErrFile
	}
	s.key = derive(master, salt)
	s.salt = salt
	s.entries = map[string]Credential{}
	if e := s.write(s.entries, true); e != nil {
		s.lock()
		return e
	}
	s.touch()
	return nil
}
func (s *Store) Unlock(master string, codes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.retry) {
		return errors.New("aguarde um segundo antes de tentar novamente")
	}
	s.lock()
	s.retry = time.Now().Add(time.Second)
	if len(master) > 1024 {
		return ErrUnlock
	}
	raw, e := read(s.path)
	if e != nil {
		return ErrUnlock
	}
	var env envelope
	if decode(raw, &env) != nil || (env.Version != 1 && env.Version != 2) || len(env.Salt) != 16 || len(env.Data) < 28 {
		return ErrUnlock
	}
	key := derive(master, env.Salt)
	a, e := aead(key)
	if e != nil {
		clear(key)
		return ErrUnlock
	}
	plain, e := a.Open(nil, nil, env.Data, associated(env.Version))
	if e != nil {
		clear(key)
		return ErrUnlock
	}
	defer clear(plain)
	var entries map[string]Credential
	var factor *twoFactor
	var parseErr error
	if env.Version == 2 {
		var payload payloadV2
		parseErr = decode(plain, &payload)
		entries = payload.Entries
		factor = payload.Factor
		if !validFactor(factor) {
			parseErr = ErrUnlock
		}
	} else {
		parseErr = decode(plain, &entries)
	}
	if parseErr != nil || entries == nil || len(entries) > 200 {
		clear(key)
		return ErrUnlock
	}
	for id, c := range entries {
		if id == "" || len(id) > 64 || !valid(c) {
			clear(key)
			clear(entries)
			return ErrUnlock
		}
	}
	if factor != nil {
		code := ""
		if len(codes) == 1 {
			code = codes[0]
		}
		updated, err := consumeFactor(factor, code, time.Now())
		clear(factor.Secret)
		if err != nil {
			clear(key)
			clear(entries)
			return ErrFactor
		}
		factor = updated
	}
	s.key = key
	s.factor = factor
	s.salt = env.Salt
	s.entries = entries
	s.digest = sha256.Sum256(raw)
	if factor != nil {
		if e = s.write(entries, false); e != nil {
			s.lock()
			return e
		}
	}
	if factor != nil && len(codes) == 1 && len(normalizeRecovery(codes[0])) == 32 {
		s.recoveryProof = recoveryHash(codes[0])
		s.recoveryUntil = time.Now().Add(5 * time.Minute)
	}
	s.retry = time.Time{}
	s.touch()
	return nil
}
func (s *Store) current() error {
	s.expire()
	if s.key == nil {
		return ErrLocked
	}
	raw, e := read(s.path)
	if e != nil || sha256.Sum256(raw) != s.digest {
		s.lock()
		return ErrFile
	}
	return nil
}

// Get is backend-only: App never binds this method to the renderer.
func (s *Store) Get(id string) (Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.current(); e != nil {
		return Credential{}, e
	}
	c, ok := s.entries[id]
	if !ok {
		return c, errors.New("nenhuma credencial salva para esta conexão")
	}
	s.touch()
	return c, nil
}
func (s *Store) Save(id string, c Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.current(); e != nil {
		return e
	}
	if id == "" || len(id) > 64 || !valid(c) {
		return errors.New("credencial inválida")
	}
	next := make(map[string]Credential, len(s.entries)+1)
	for k, v := range s.entries {
		next[k] = v
	}
	next[id] = c
	if len(next) > 200 {
		return errors.New("limite de credenciais atingido")
	}
	if e := s.write(next, false); e != nil {
		clear(next)
		return e
	}
	clear(s.entries)
	s.entries = next
	s.touch()
	return nil
}
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.current(); e != nil {
		return e
	}
	next := make(map[string]Credential, len(s.entries))
	for k, v := range s.entries {
		if k != id {
			next[k] = v
		}
	}
	if e := s.write(next, false); e != nil {
		clear(next)
		return e
	}
	clear(s.entries)
	s.entries = next
	s.touch()
	return nil
}
func (s *Store) write(entries map[string]Credential, create bool) error {
	if os.MkdirAll(filepath.Dir(s.path), 0700) != nil {
		return ErrFile
	}
	lock, e := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrFile
	}
	lock.Close()
	defer os.Remove(s.path + ".lock")
	if create {
		if _, e = os.Stat(s.path); !os.IsNotExist(e) {
			return ErrFile
		}
	} else {
		raw, e := read(s.path)
		if e != nil || sha256.Sum256(raw) != s.digest {
			s.lock()
			return ErrFile
		}
	}
	version := 1
	var payload any = entries
	if s.factor != nil {
		version = 2
		payload = payloadV2{Entries: entries, Factor: s.factor}
	}
	plain, e := json.Marshal(payload)
	if e != nil {
		return ErrFile
	}
	defer clear(plain)
	a, e := aead(s.key)
	if e != nil {
		return ErrFile
	}
	raw, e := json.Marshal(envelope{Version: version, Salt: s.salt, Data: a.Seal(nil, nil, plain, associated(version))})
	if e != nil || len(raw) > maxSize {
		return ErrFile
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".vault-*")
	if e != nil {
		return ErrFile
	}
	defer os.Remove(f.Name())
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return ErrFile
	}
	if os.Rename(f.Name(), s.path) != nil {
		return ErrFile
	}
	s.digest = sha256.Sum256(raw)
	return nil
}
