// Package credentials resolves vault entries only inside the backend.
package credentials

import (
	"errors"
	"nishiterm/internal/profiles"
	"nishiterm/internal/sshengine"
	"nishiterm/internal/vault"
)

type Service struct {
	Profiles *profiles.Store
	Vault    *vault.Store
}

func (s *Service) profile(id string) (profiles.Profile, error) {
	items, e := s.Profiles.List()
	if e != nil {
		return profiles.Profile{}, e
	}
	for _, p := range items {
		if p.ID == id {
			return p, nil
		}
	}
	return profiles.Profile{}, errors.New("conexão não encontrada")
}
func (s *Service) Save(id, method, keyPath, password, passphrase string) error {
	p, e := s.profile(id)
	if e != nil {
		return e
	}
	return s.Vault.Save(id, vault.Credential{Host: p.Host, Port: p.Port, User: p.User, Method: method, KeyPath: keyPath, Password: password, Passphrase: passphrase})
}
func (s *Service) Resolve(c sshengine.Config) (sshengine.Config, error) {
	if c.SavedCredential == "" {
		if c.AuthMethod == "vault" {
			return c, errors.New("selecione uma conexão salva")
		}
		return c, nil
	}
	if c.AuthMethod != "vault" || c.Password != "" || c.Passphrase != "" || c.PrivateKeyPath != "" {
		return c, errors.New("credenciais conflitantes")
	}
	p, e := s.profile(c.SavedCredential)
	if e != nil {
		return c, e
	}
	if c.Host != p.Host || c.Port != p.Port || c.User != p.User || c.KnownHosts != p.KnownHosts {
		return c, errors.New("perfil alterado: feche e reabra a aba")
	}
	secret, e := s.Vault.Get(p.ID)
	if e != nil {
		return c, e
	}
	if secret.Host != p.Host || secret.Port != p.Port || secret.User != p.User {
		return c, errors.New("destino alterado: salve uma nova credencial para este perfil")
	}
	c.SavedCredential = ""
	c.AuthMethod = secret.Method
	c.PrivateKeyPath = secret.KeyPath
	c.Password = secret.Password
	c.Passphrase = secret.Passphrase
	return c, nil
}

func (s *Service) SaveFor(id string, c sshengine.Config) error {
	p, e := s.profile(id)
	if e != nil {
		return e
	}
	if p.Host != c.Host || p.Port != c.Port || p.User != c.User || p.KnownHosts != c.KnownHosts {
		return errors.New("perfil alterado: feche e reabra a aba")
	}
	return s.Save(id, c.AuthMethod, c.PrivateKeyPath, c.Password, c.Passphrase)
}
