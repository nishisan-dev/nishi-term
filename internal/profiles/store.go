// Package profiles persists connection metadata only. It never accepts credentials.
package profiles

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const MaxProfiles = 200
const maxFileSize = 2 << 20

type Profile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Group      string `json:"group"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	KnownHosts string `json:"knownHosts"`
}
type document struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}
type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store { return &Store{path: path} }

var errRead = errors.New("não foi possível ler os perfis; o arquivo existente foi preservado")

func validate(p Profile) error {
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 100 || len(p.Group) > 100 || strings.TrimSpace(p.Host) == "" || len(p.Host) > 253 || p.Port < 1 || p.Port > 65535 || strings.TrimSpace(p.User) == "" || len(p.User) > 256 || len(p.KnownHosts) > 4096 || len(p.ID) > 64 {
		return errors.New("perfil inválido: confira nome, endereço, porta, usuário e known_hosts")
	}
	for _, v := range []string{p.ID, p.Name, p.Group, p.Host, p.User, p.KnownHosts} {
		if strings.ContainsAny(v, "\x00\r\n") {
			return errors.New("perfil contém caracteres inválidos")
		}
	}
	return nil
}
func (s *Store) read() ([]Profile, error) {
	f, e := os.Open(s.path)
	if os.IsNotExist(e) {
		return []Profile{}, nil
	}
	if e != nil {
		return nil, errRead
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > maxFileSize {
		return nil, errRead
	}
	decoder := json.NewDecoder(io.LimitReader(f, maxFileSize+1))
	decoder.DisallowUnknownFields()
	var doc document
	if decoder.Decode(&doc) != nil || doc.Version != 1 || len(doc.Profiles) > MaxProfiles {
		return nil, errRead
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errRead
	}
	seen := map[string]bool{}
	for _, p := range doc.Profiles {
		if p.ID == "" || seen[p.ID] || validate(p) != nil {
			return nil, errRead
		}
		seen[p.ID] = true
	}
	if doc.Profiles == nil {
		doc.Profiles = []Profile{}
	}
	return doc.Profiles, nil
}
func (s *Store) List() ([]Profile, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.read() }
func (s *Store) Save(p Profile) ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Name = strings.TrimSpace(p.Name)
	p.Group = strings.TrimSpace(p.Group)
	p.Host = strings.TrimSpace(p.Host)
	p.User = strings.TrimSpace(p.User)
	if e := validate(p); e != nil {
		return nil, e
	}
	items, e := s.read()
	if e != nil {
		return nil, e
	}
	if p.ID == "" {
		if len(items) >= MaxProfiles {
			return nil, errors.New("limite de 200 perfis atingido")
		}
		p.ID = rand.Text()
		items = append(items, p)
	} else {
		found := false
		for i := range items {
			if items[i].ID == p.ID {
				items[i] = p
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("perfil não encontrado; recarregue a lista")
		}
	}
	if e = s.write(items); e != nil {
		return nil, e
	}
	return items, nil
}
func (s *Store) Delete(id string) ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.read()
	if e != nil {
		return nil, e
	}
	found := false
	for i := range items {
		if items[i].ID == id {
			items = append(items[:i], items[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("perfil não encontrado; recarregue a lista")
	}
	if e = s.write(items); e != nil {
		return nil, e
	}
	return items, nil
}
func (s *Store) write(items []Profile) error {
	failure := errors.New("não foi possível salvar os perfis; tente novamente")
	data, e := json.MarshalIndent(document{Version: 1, Profiles: items}, "", "  ")
	if e != nil || len(data) > maxFileSize {
		return failure
	}
	dir := filepath.Dir(s.path)
	if os.MkdirAll(dir, 0700) != nil {
		return failure
	}
	f, e := os.CreateTemp(dir, ".profiles-*")
	if e != nil {
		return failure
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return failure
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return failure
	}
	if f.Close() != nil {
		return failure
	}
	if os.Rename(temp, s.path) != nil {
		return failure
	}
	return nil
}
