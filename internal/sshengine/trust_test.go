package sshengine

import (
	"nishiterm/internal/hosttrust"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagedTrustConfirmationAndReconnect(t *testing.T) {
	f := server(t, nil, 0)
	c := f.config
	c.KnownHosts = ""
	path := filepath.Join(t.TempDir(), "trust.json")
	m := NewManagerWithTrust(path)
	defer m.Shutdown()
	id := start(t, m, c)
	s, _ := m.get(id)
	eventually(t, func() bool { return s.challenge() != nil })
	if f.auth.Load() != 0 {
		t.Fatal("authentication before consent")
	}
	challenge := s.challenge()
	if m.ConfirmHost(id, "wrong-token") == nil {
		t.Fatal("invalid consent accepted")
	}
	if e := m.ConfirmHost(id, challenge.Token); e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	if f.auth.Load() == 0 {
		t.Fatal("no login after consent")
	}
	m.Close(id)
	// Fresh manager proves persistence, not just in-memory approval.
	next := NewManagerWithTrust(path)
	defer next.Shutdown()
	id = start(t, next, c)
	s, _ = next.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	if s.challenge() != nil {
		t.Fatal("trusted host challenged again")
	}
}
func TestManagedTrustRejectCloseAndChanged(t *testing.T) {
	for _, mode := range []string{"cancel", "changed", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			f := server(t, nil, 0)
			c := f.config
			c.KnownHosts = ""
			path := filepath.Join(t.TempDir(), "trust.json")
			if mode == "changed" {
				if e := hosttrust.New(path).Accept(hosttrust.Endpoint(c.Host, c.Port), signer(t).PublicKey()); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "corrupt" {
				os.WriteFile(path, []byte("broken"), 0600)
			}
			m := NewManagerWithTrust(path)
			defer m.Shutdown()
			id := start(t, m, c)
			s, _ := m.get(id)
			if mode == "cancel" {
				eventually(t, func() bool { return s.challenge() != nil })
				token := s.challenge().Token
				m.Close(id)
				if m.ConfirmHost(id, token) == nil {
					t.Fatal("late consent accepted")
				}
				select {
				case <-s.done:
				case <-time.After(time.Second):
					t.Fatal("cancel did not release handshake")
				}
				if _, e := os.Stat(path); !os.IsNotExist(e) {
					t.Fatal("cancel saved trust")
				}
			} else {
				eventually(t, func() bool { state, _ := s.status(); return state == "failed" })
				if s.challenge() != nil {
					t.Fatal("blocked host offered approval")
				}
			}
			if f.auth.Load() != 0 {
				t.Fatal("authentication happened before trust")
			}
		})
	}
}
