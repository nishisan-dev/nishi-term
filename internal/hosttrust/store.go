// Package hosttrust stores explicitly accepted server identities, never user credentials.
package hosttrust

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var ErrChanged = errors.New("identidade do servidor mudou: conexão bloqueada; verifique a mudança com o administrador")
var ErrStore = errors.New("não foi possível ler ou salvar identidades confiáveis; o arquivo existente foi preservado")

type Store struct {
	mu   sync.Mutex
	path string
}
type document struct {
	Version int               `json:"version"`
	Hosts   map[string]string `json:"hosts"`
}

func New(path string) *Store { return &Store{path: path} }
func Endpoint(host string, port int) string {
	return net.JoinHostPort(strings.ToLower(strings.TrimSpace(host)), strconv.Itoa(port))
}
func (s *Store) read() (document, error) {
	d := document{Version: 1, Hosts: map[string]string{}}
	f, e := os.Open(s.path)
	if os.IsNotExist(e) {
		return d, nil
	}
	if e != nil {
		return d, ErrStore
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > 2<<20 {
		return d, ErrStore
	}
	dec := json.NewDecoder(io.LimitReader(f, (2<<20)+1))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil || d.Version != 1 || d.Hosts == nil || len(d.Hosts) > 1000 {
		return d, ErrStore
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return d, ErrStore
	}
	for host, k := range d.Hosts {
		if len(host) > 300 {
			return d, ErrStore
		}
		b, e := base64.StdEncoding.DecodeString(k)
		if e != nil {
			return d, ErrStore
		}
		if _, e = ssh.ParsePublicKey(b); e != nil {
			return d, ErrStore
		}
	}
	return d, nil
}
func match(d document, endpoint string, key ssh.PublicKey) (bool, error) {
	old, ok := d.Hosts[endpoint]
	if !ok {
		return false, nil
	}
	if old != base64.StdEncoding.EncodeToString(key.Marshal()) {
		return false, ErrChanged
	}
	return true, nil
}
func (s *Store) Check(endpoint string, key ssh.PublicKey) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read()
	if e != nil {
		return false, e
	}
	return match(d, endpoint, key)
}
func (s *Store) Accept(endpoint string, key ssh.PublicKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(endpoint) > 300 {
		return ErrStore
	}
	if os.MkdirAll(filepath.Dir(s.path), 0700) != nil {
		return ErrStore
	}
	// Cross-process exclusion: stale locks fail closed; never overwrite a competing decision.
	lock, e := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrStore
	}
	lock.Close()
	defer os.Remove(s.path + ".lock")
	d, e := s.read()
	if e != nil {
		return e
	}
	ok, e := match(d, endpoint, key)
	if e != nil || ok {
		return e
	}
	if len(d.Hosts) >= 1000 {
		return ErrStore
	}
	d.Hosts[endpoint] = base64.StdEncoding.EncodeToString(key.Marshal())
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil || len(b) > 2<<20 {
		return ErrStore
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".trust-*")
	if e != nil {
		return ErrStore
	}
	defer os.Remove(f.Name())
	_, e = io.Copy(f, bytes.NewReader(b))
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return ErrStore
	}
	if os.Rename(f.Name(), s.path) != nil {
		return ErrStore
	}
	return nil
}
