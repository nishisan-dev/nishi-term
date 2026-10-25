package sshengine

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type fixture struct {
	config Config
	key    ssh.Signer
	auth   atomic.Int32
	resize chan [2]uint32
}

func signer(t *testing.T) ssh.Signer {
	t.Helper()
	_, k, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	s, e := ssh.NewSignerFromKey(k)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func server(t *testing.T, payload []byte, exit uint32) *fixture {
	t.Helper()
	f := &fixture{key: signer(t), resize: make(chan [2]uint32, 8)}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(port)
	trust := filepath.Join(t.TempDir(), "known_hosts")
	if e = os.WriteFile(trust, []byte(knownhosts.Line([]string{listener.Addr().String()}, f.key.PublicKey())+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	f.config = Config{Host: host, Port: p, User: "test", Password: "canary-secret", KnownHosts: trust, Cols: 80, Rows: 24}
	config := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		f.auth.Add(1)
		if string(p) != "canary-secret" {
			return nil, fmt.Errorf("auth denied")
		}
		return nil, nil
	}}
	config.AddHostKey(f.key)
	var mu sync.Mutex
	var conns []net.Conn
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, e := listener.Accept()
			if e != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				conn, channels, requests, e := ssh.NewServerConn(c, config)
				if e != nil {
					return
				}
				defer conn.Close()
				go func() {
					for r := range requests {
						_ = r.Reply(true, nil)
					}
				}()
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						_ = incoming.Reject(ssh.UnknownChannelType, "unsupported")
						continue
					}
					ch, reqs, e := incoming.Accept()
					if e != nil {
						return
					}
					go func() {
						defer ch.Close()
						for r := range reqs {
							switch r.Type {
							case "pty-req":
								_ = r.Reply(true, nil)
							case "window-change":
								var size struct{ Cols, Rows, Width, Height uint32 }
								_ = ssh.Unmarshal(r.Payload, &size)
								select {
								case f.resize <- [2]uint32{size.Cols, size.Rows}:
								default:
								}
								if r.WantReply {
									_ = r.Reply(true, nil)
								}
							case "shell":
								_ = r.Reply(true, nil)
								go func() {
									if payload == nil {
										_, _ = io.Copy(ch, ch)
									} else {
										_, _ = ch.Write(payload)
									}
									_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{exit}))
									_ = ch.Close()
								}()
							default:
								if r.WantReply {
									_ = r.Reply(false, nil)
								}
							}
						}
					}()
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	return f
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}
func manager(t *testing.T) *Manager { m := NewManager(); t.Cleanup(m.Shutdown); return m }
func start(t *testing.T, m *Manager, c Config) string {
	t.Helper()
	id, e := m.Start(c)
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func TestSSHInteractiveResizeAndClose(t *testing.T) {
	f := server(t, nil, 0)
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	if e := m.Resize(id, 120, 40); e != nil {
		t.Fatal(e)
	}
	select {
	case size := <-f.resize:
		if size != [2]uint32{120, 40} {
			t.Fatal(size)
		}
	case <-time.After(time.Second):
		t.Fatal("resize not received")
	}
	input := []byte("Olá 🌎\x03\x1b[31m")
	if e := m.Input(id, base64.StdEncoding.EncodeToString(input)); e != nil {
		t.Fatal(e)
	}
	var got []byte
	var ack uint64
	for len(got) < len(input) {
		fr, e := m.Poll(id, ack)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := base64.StdEncoding.DecodeString(fr.Data)
		got = append(got, b...)
		ack = fr.Sequence
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("echo mismatch: %q", got)
	}
	if e := m.Close(id); e != nil {
		t.Fatal(e)
	}
	if e := m.Close(id); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Poll(id, ack); e == nil {
		t.Fatal("closed session accepted")
	}
}
func TestHostTrustRejectsBeforeAuth(t *testing.T) {
	for _, mode := range []string{"unknown", "changed", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			f := server(t, nil, 0)
			m := manager(t)
			line := ""
			if mode == "changed" {
				line = knownhosts.Line([]string{net.JoinHostPort(f.config.Host, strconv.Itoa(f.config.Port))}, signer(t).PublicKey())
			}
			if mode == "revoked" {
				line = "@revoked " + knownhosts.Line([]string{net.JoinHostPort(f.config.Host, strconv.Itoa(f.config.Port))}, f.key.PublicKey())
			}
			if e := os.WriteFile(f.config.KnownHosts, []byte(line+"\n"), 0600); e != nil {
				t.Fatal(e)
			}
			id := start(t, m, f.config)
			s, _ := m.get(id)
			eventually(t, func() bool { state, _ := s.status(); return state == "failed" })
			if f.auth.Load() != 0 {
				t.Fatal("authentication occurred before trust")
			}
		})
	}
}
func TestBoundedOutputACKAndDrain(t *testing.T) {
	// Bigger than SSH's receive window, containing fragmented UTF-8 and terminal bytes.
	payload := bytes.Repeat([]byte("A🌎ç\x1b[0m\n"), 350000)
	f := server(t, payload, 0)
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { return len(s.out) == QueueSize })
	fr, e := m.Poll(id, 0)
	if e != nil || fr.Data == "" {
		t.Fatalf("first frame: %v", e)
	}
	retry, e := m.Poll(id, 0)
	if e != nil || retry.Data != fr.Data || retry.Sequence != fr.Sequence {
		t.Fatal("unacknowledged frame changed")
	}
	if _, e = m.Poll(id, fr.Sequence+1); e == nil {
		t.Fatal("future ACK accepted")
	}
	got, _ := base64.StdEncoding.DecodeString(fr.Data)
	ack := fr.Sequence
	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("drain timeout")
		}
		fr, e = m.Poll(id, ack)
		if e != nil {
			t.Fatal(e)
		}
		if fr.Data != "" {
			b, _ := base64.StdEncoding.DecodeString(fr.Data)
			if len(b) > ChunkSize {
				t.Fatal("oversized chunk")
			}
			got = append(got, b...)
			ack = fr.Sequence
			continue
		}
		if fr.State == "closed" {
			break
		}
		if fr.State == "failed" {
			t.Fatal(fr.Error)
		}
	}
	if !bytes.Equal(payload, got) {
		t.Fatalf("output lost: want %d got %d", len(payload), len(got))
	}
}
func TestNonzeroExitPreservesOutput(t *testing.T) {
	f := server(t, []byte("final message"), 7)
	m := manager(t)
	id := start(t, m, f.config)
	var ack uint64
	var got []byte
	for i := 0; i < 30; i++ {
		fr, e := m.Poll(id, ack)
		if e != nil {
			t.Fatal(e)
		}
		if fr.Data != "" {
			b, _ := base64.StdEncoding.DecodeString(fr.Data)
			got = append(got, b...)
			ack = fr.Sequence
			continue
		}
		if fr.State == "failed" {
			if string(got) != "final message" {
				t.Fatal("lost final output")
			}
			return
		}
	}
	t.Fatal("missing failure state")
}
func TestAuthenticationErrorDoesNotExposePassword(t *testing.T) {
	f := server(t, nil, 0)
	m := manager(t)
	f.config.Password = "sensitive-password"
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "failed" })
	_, message := s.status()
	if bytes.Contains([]byte(message), []byte(f.config.Password)) {
		t.Fatal("secret leaked")
	}
}
func TestCancelBlockedHandshake(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, e := listener.Accept()
		if e == nil {
			accepted <- c
		}
	}()
	path := filepath.Join(t.TempDir(), "known_hosts")
	_ = os.WriteFile(path, nil, 0600)
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(port)
	m := manager(t)
	id := start(t, m, Config{Host: host, Port: p, User: "test", KnownHosts: path, Cols: 80, Rows: 24})
	var c net.Conn
	select {
	case c = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("no TCP connection")
	}
	defer c.Close()
	s, _ := m.get(id)
	_ = m.Close(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "failed" })
}
func TestInputLimitsAndSessionCap(t *testing.T) {
	f := server(t, nil, 0)
	m := manager(t)
	var ids []string
	for i := 0; i < MaxSessions; i++ {
		ids = append(ids, start(t, m, f.config))
	}
	if _, e := m.Start(f.config); e == nil {
		t.Fatal("session cap not enforced")
	}
	if e := m.Input(ids[0], base64.StdEncoding.EncodeToString(make([]byte, InputSize+1))); e == nil {
		t.Fatal("oversized input accepted")
	}
	if e := m.Resize(ids[0], 0, 24); e == nil {
		t.Fatal("invalid dimensions accepted")
	}
	m.Shutdown()
	if _, e := m.Start(f.config); e == nil {
		t.Fatal("start after shutdown accepted")
	}
}
func TestInputBackpressureAndConcurrentPoll(t *testing.T) {
	m := manager(t)
	s := &session{state: "open", input: make(chan []byte, QueueSize), out: make(chan []byte, QueueSize)}
	// A bounded input queue with a deliberately paused writer.
	s.ctx, s.cancel = context.WithCancel(context.Background())
	m.sessions["paused"] = s
	for i := 0; i < QueueSize; i++ {
		if e := m.Input("paused", "YQ=="); e != nil {
			t.Fatal(e)
		}
	}
	if e := m.Input("paused", "YQ=="); e == nil {
		t.Fatal("full queue accepted input")
	}
	s.pollMu.Lock()
	if _, e := m.Poll("paused", 0); e == nil {
		t.Fatal("concurrent poll accepted")
	}
	s.pollMu.Unlock()
}

func TestRepeatedSessionCleanup(t *testing.T) {
	f := server(t, nil, 0)
	m := manager(t)
	for i := 0; i < 100; i++ {
		id := start(t, m, f.config)
		s, _ := m.get(id)
		eventually(t, func() bool { state, _ := s.status(); return state == "open" })
		_ = m.Close(id)
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
			t.Fatalf("session %d leaked workers", i)
		}
	}
	if len(m.sessions) != 0 {
		t.Fatal("sessions retained")
	}
}
func TestCloseWhileOutputBackpressured(t *testing.T) {
	f := server(t, bytes.Repeat([]byte("x"), 4*1024*1024), 0)
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { return len(s.out) == QueueSize })
	_ = m.Close(id)
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked readers did not stop")
	}
}
