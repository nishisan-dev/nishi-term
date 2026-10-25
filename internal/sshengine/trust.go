package sshengine

import (
	"crypto/rand"
	"errors"
	"golang.org/x/crypto/ssh"
	"net"
	"nishiterm/internal/hosttrust"
	"time"
)

type HostChallenge struct {
	Token       string `json:"token"`
	Endpoint    string `json:"endpoint"`
	Algorithm   string `json:"algorithm"`
	Fingerprint string `json:"fingerprint"`
}

func NewManagerWithTrust(path string) *Manager {
	m := NewManager()
	m.trust = hosttrust.New(path)
	return m
}
func (s *session) challenge() *HostChallenge {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trustChallenge == nil {
		return nil
	}
	c := *s.trustChallenge
	return &c
}
func (m *Manager) ConfirmHost(id, token string) error {
	s, e := m.get(id)
	if e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trustChallenge == nil || s.trustChallenge.Token != token || s.ctx.Err() != nil {
		return errors.New("confirmação de identidade expirada")
	}
	select {
	case s.trustDecision <- token:
		return nil
	default:
		return errors.New("confirmação já enviada")
	}
}
func (s *session) managedVerifier(store *hosttrust.Store, c Config, conn net.Conn) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		endpoint := hosttrust.Endpoint(c.Host, c.Port)
		ok, e := store.Check(endpoint, key)
		if e != nil {
			s.fail(e.Error())
			return e
		}
		if ok {
			return nil
		}
		challenge := &HostChallenge{Token: rand.Text(), Endpoint: endpoint, Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)}
		s.mu.Lock()
		s.trustChallenge = challenge
		s.trustDecision = make(chan string, 1)
		decision := s.trustDecision
		s.mu.Unlock()
		defer func() { s.mu.Lock(); s.trustChallenge = nil; s.mu.Unlock() }()
		_ = conn.SetDeadline(time.Now().Add(120 * time.Second))
		timer := time.NewTimer(120 * time.Second)
		defer timer.Stop()
		select {
		case <-s.ctx.Done():
			return ErrClosed
		case <-timer.C:
			s.fail("confirmação de identidade expirou")
			return errors.New("trust timeout")
		case token := <-decision:
			if token != challenge.Token || s.ctx.Err() != nil {
				return ErrClosed
			}
			if e = store.Accept(endpoint, key); e != nil {
				s.fail(e.Error())
				return e
			}
			_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
			return nil
		}
	}
}
