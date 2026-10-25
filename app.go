package main

import (
	"context"
	"errors"
	"nishiterm/internal/credentials"
	"nishiterm/internal/profiles"
	"nishiterm/internal/sshengine"
	"nishiterm/internal/vault"
	"os"
	"path/filepath"
)

type App struct {
	vault       *vault.Store
	credentials *credentials.Service
	engine      *sshengine.Manager
	profiles    *profiles.Store
}

func NewApp() *App {
	a := &App{engine: sshengine.NewManager()}
	if dir, e := os.UserConfigDir(); e == nil {
		a.engine = sshengine.NewManagerWithTrust(filepath.Join(dir, "NishiTerm", "trusted-hosts.json"))
		a.profiles = profiles.New(filepath.Join(dir, "NishiTerm", "profiles.json"))
		a.vault = vault.New(filepath.Join(dir, "NishiTerm", "vault.json"))
		a.credentials = &credentials.Service{Profiles: a.profiles, Vault: a.vault}
	}
	return a
}
func (a *App) ListProfiles() ([]profiles.Profile, error) {
	if a.profiles == nil {
		return nil, errors.New("diretório de perfis indisponível")
	}
	return a.profiles.List()
}
func (a *App) SaveProfile(p profiles.Profile) ([]profiles.Profile, error) {
	if a.profiles == nil {
		return nil, errors.New("diretório de perfis indisponível")
	}
	return a.profiles.Save(p)
}
func (a *App) DeleteProfile(id string) ([]profiles.Profile, error) {
	if a.profiles == nil {
		return nil, errors.New("diretório de perfis indisponível")
	}
	return a.profiles.Delete(id)
}

func (a *App) shutdown(context.Context) {
	a.engine.Shutdown()
	if a.vault != nil {
		a.vault.Lock()
	}
}
func (a *App) Start(c sshengine.Config) (string, error) {
	if a.credentials == nil {
		return "", errors.New("diretório de usuário indisponível")
	}
	resolved, e := a.credentials.Resolve(c)
	if e != nil {
		return "", e
	}
	return a.engine.Start(resolved)
}
func (a *App) Poll(id string, ack uint64) (sshengine.Frame, error) { return a.engine.Poll(id, ack) }
func (a *App) Input(id, data string) error                         { return a.engine.Input(id, data) }
func (a *App) Resize(id string, cols, rows int) error              { return a.engine.Resize(id, cols, rows) }
func (a *App) Close(id string) error                               { return a.engine.Close(id) }

func (a *App) ConfirmHost(id, token string) error { return a.engine.ConfirmHost(id, token) }

func (a *App) VaultStatus() (vault.Status, error) {
	if a.vault == nil {
		return vault.Status{Locked: true}, errors.New("cofre indisponível")
	}
	return a.vault.Status()
}
func (a *App) CreateVault(master string) error {
	if a.vault == nil {
		return errors.New("cofre indisponível")
	}
	return a.vault.Create(master)
}
func (a *App) UnlockVault(master string) error {
	if a.vault == nil {
		return errors.New("cofre indisponível")
	}
	return a.vault.Unlock(master)
}
func (a *App) LockVault() {
	if a.vault != nil {
		a.vault.Lock()
	}
}
func (a *App) SaveCredential(id string, c sshengine.Config) error {
	if a.credentials == nil {
		return errors.New("cofre indisponível")
	}
	return a.credentials.SaveFor(id, c)
}
func (a *App) DeleteCredential(id string) error {
	if a.vault == nil {
		return errors.New("cofre indisponível")
	}
	return a.vault.Delete(id)
}

func (a *App) SessionUsage() int { return a.engine.ActiveCount() }

func (a *App) ListFiles(id, path string) (sshengine.Directory, error) {
	return a.engine.ListFiles(id, path)
}
func (a *App) CancelFiles(id string) { a.engine.CancelFiles(id) }
