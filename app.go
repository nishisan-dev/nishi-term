package main

import (
	"context"
	"errors"
	"nishiterm/internal/profiles"
	"nishiterm/internal/sshengine"
	"os"
	"path/filepath"
)

type App struct {
	engine   *sshengine.Manager
	profiles *profiles.Store
}

func NewApp() *App {
	a := &App{engine: sshengine.NewManager()}
	if dir, e := os.UserConfigDir(); e == nil {
		a.profiles = profiles.New(filepath.Join(dir, "NishiTerm", "profiles.json"))
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

func (a *App) shutdown(context.Context)                            { a.engine.Shutdown() }
func (a *App) Start(c sshengine.Config) (string, error)            { return a.engine.Start(c) }
func (a *App) Poll(id string, ack uint64) (sshengine.Frame, error) { return a.engine.Poll(id, ack) }
func (a *App) Input(id, data string) error                         { return a.engine.Input(id, data) }
func (a *App) Resize(id string, cols, rows int) error              { return a.engine.Resize(id, cols, rows) }
func (a *App) Close(id string) error                               { return a.engine.Close(id) }
