package credentials

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"golang.org/x/crypto/ssh"
	"nishiterm/internal/profiles"
	"nishiterm/internal/vault"
	"sort"
	"strings"
)

type KeyInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
}
type Coverage struct {
	Profile profiles.Profile `json:"profile"`
	Ready   bool             `json:"ready"`
	Source  string           `json:"source"`
}

func groupID(group string) string {
	sum := sha256.Sum256([]byte(group))
	return "group-" + hex.EncodeToString(sum[:28])
}
func (s *Service) credential(p profiles.Profile) (vault.Credential, error) {
	entries, e := s.Vault.Snapshot()
	if e != nil {
		return vault.Credential{}, e
	}
	if _, ok := entries[p.ID]; ok {
		return s.Vault.Get(p.ID)
	}
	if c, ok := entries[groupID(p.Group)]; ok && c.Method == "group-key" && c.Group == p.Group {
		return s.Vault.Get(groupID(p.Group))
	}
	return vault.Credential{}, errors.New("nenhuma credencial salva para esta conexão ou grupo")
}
func keyInfo(id string, c vault.Credential) (KeyInfo, error) {
	if c.Method != "identity" {
		return KeyInfo{}, errors.New("chave não encontrada")
	}
	signer, e := ssh.ParsePrivateKey([]byte(c.PrivateKey))
	if e != nil {
		return KeyInfo{}, errors.New("chave inválida no cofre")
	}
	return KeyInfo{ID: id, Label: c.Label, PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + " NishiTerm", Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}, nil
}
func (s *Service) GenerateKey(label string) (KeyInfo, error) {
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 100 || strings.ContainsAny(label, "\x00\r\n") {
		return KeyInfo{}, errors.New("nome da chave inválido")
	}
	if _, e := s.Vault.Snapshot(); e != nil {
		return KeyInfo{}, e
	}
	_, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return KeyInfo{}, e
	}
	defer clear(private)
	block, e := ssh.MarshalPrivateKey(private, "")
	if e != nil {
		return KeyInfo{}, e
	}
	defer clear(block.Bytes)
	encoded := pem.EncodeToMemory(block)
	defer clear(encoded)
	c := vault.Credential{Method: "identity", Label: label, PrivateKey: string(encoded)}
	id := "key-" + rand.Text()
	if e = s.Vault.Save(id, c); e != nil {
		return KeyInfo{}, e
	}
	return keyInfo(id, c)
}
func (s *Service) Keys() ([]KeyInfo, error) {
	entries, e := s.Vault.Snapshot()
	if e != nil {
		return nil, e
	}
	result := []KeyInfo{}
	for id, c := range entries {
		if c.Method == "identity" {
			k, e := keyInfo(id, c)
			if e != nil {
				return nil, e
			}
			result = append(result, k)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Label < result[j].Label })
	return result, nil
}
func (s *Service) PublicKey(id string) (KeyInfo, error) {
	c, e := s.Vault.Get(id)
	if e != nil {
		return KeyInfo{}, e
	}
	return keyInfo(id, c)
}
func (s *Service) AssignKey(keyID, profileID, group string, toGroup bool) error {
	if _, e := s.PublicKey(keyID); e != nil {
		return e
	}
	if toGroup {
		items, e := s.Profiles.List()
		if e != nil {
			return e
		}
		found := false
		for _, p := range items {
			if p.Group == group {
				found = true
			}
		}
		if !found {
			return errors.New("grupo não encontrado")
		}
		return s.Vault.Save(groupID(group), vault.Credential{Method: "group-key", Group: group, KeyID: keyID})
	}
	p, e := s.profile(profileID)
	if e != nil {
		return e
	}
	return s.Vault.Save(p.ID, vault.Credential{Host: p.Host, Port: p.Port, User: p.User, Method: "managed-key", KeyID: keyID})
}
func (s *Service) Coverage(group string) ([]Coverage, error) {
	entries, e := s.Vault.Snapshot()
	if e != nil {
		return nil, e
	}
	items, e := s.Profiles.List()
	if e != nil {
		return nil, e
	}
	result := []Coverage{}
	for _, p := range items {
		if p.Group != group {
			continue
		}
		c, ok := entries[p.ID]
		source := "host"
		if !ok {
			c, ok = entries[groupID(group)]
			source = "grupo"
			ok = ok && c.Method == "group-key" && c.Group == group
		}
		ready := ok && (source == "grupo" || (c.Host == p.Host && c.Port == p.Port && c.User == p.User))
		if c.Method == "managed-key" || c.Method == "group-key" {
			key, exists := entries[c.KeyID]
			ready = ready && exists && key.Method == "identity"
		}
		if c.Method != "password" && c.Method != "key" && c.Method != "managed-key" && c.Method != "group-key" {
			ready = false
		}
		result = append(result, Coverage{Profile: p, Ready: ready, Source: source})
	}
	return result, nil
}
func (s *Service) Profile(id string) (profiles.Profile, error) { return s.profile(id) }

// AssignedKey returns public metadata only. Reading a profile does not mutate its
// encrypted host association or inherited group default.
func (s *Service) AssignedKey(profileID string) (*KeyInfo, error) {
	p, e := s.profile(profileID)
	if e != nil {
		return nil, e
	}
	entries, e := s.Vault.Snapshot()
	if e != nil {
		return nil, e
	}
	c, ok := entries[p.ID]
	if !ok {
		c, ok = entries[groupID(p.Group)]
		ok = ok && c.Method == "group-key" && c.Group == p.Group
	}
	if !ok {
		return nil, nil
	}
	if c.Method != "group-key" && (c.Host != p.Host || c.Port != p.Port || c.User != p.User) {
		return nil, errors.New("destino alterado: associe uma chave novamente para este perfil")
	}
	if c.Method != "managed-key" && c.Method != "group-key" {
		return nil, nil
	}
	key, e := s.PublicKey(c.KeyID)
	if e != nil {
		return nil, e
	}
	return &key, nil
}
