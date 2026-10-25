// Package sshengine owns SSH transports and bounded terminal sessions, without UI dependencies.
package sshengine

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	ChunkSize   = 16 * 1024
	QueueSize   = 8
	InputSize   = 4 * 1024
	MaxSessions = 4
)

var ErrClosed = errors.New("sessão inexistente ou encerrada")

type Config struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	KnownHosts string `json:"knownHosts"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
}
type Frame struct {
	Sequence uint64 `json:"sequence"`
	Data     string `json:"data"`
	State    string `json:"state"`
	Error    string `json:"error"`
}
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
	stopped  bool
}
type session struct {
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex
	state, publicError string
	transport          net.Conn
	remote             *ssh.Session
	out                chan []byte
	input              chan []byte
	pollMu             sync.Mutex
	resizeMu           sync.Mutex
	done               chan struct{}
	sequence           uint64
	pending            []byte
}

func NewManager() *Manager           { return &Manager{sessions: make(map[string]*session)} }
func dimensions(cols, rows int) bool { return cols > 0 && cols <= 1000 && rows > 0 && rows <= 1000 }
func (m *Manager) Start(c Config) (string, error) {
	if len(c.Host) > 253 || len(c.User) > 256 || len(c.Password) > 65536 || len(c.KnownHosts) > 4096 || strings.TrimSpace(c.Host) == "" || strings.ContainsAny(c.Host, "\x00\r\n") || c.Port < 1 || c.Port > 65535 || c.User == "" || c.KnownHosts == "" || !dimensions(c.Cols, c.Rows) {
		return "", errors.New("parâmetros de conexão inválidos")
	}
	info, err := os.Stat(c.KnownHosts)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 8*1024*1024 {
		return "", errors.New("known_hosts deve ser um arquivo regular local de até 8 MiB")
	}
	// Build trust policy before any network operation. Do not return paths or parser details to UI.
	verify, err := knownhosts.New(c.KnownHosts)
	if err != nil {
		return "", errors.New("não foi possível ler known_hosts")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return "", ErrClosed
	}
	if len(m.sessions) >= MaxSessions {
		return "", errors.New("limite de sessões atingido")
	}
	id := rand.Text()
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{done: make(chan struct{}), ctx: ctx, cancel: cancel, state: "connecting", out: make(chan []byte, QueueSize), input: make(chan []byte, QueueSize)}
	m.sessions[id] = s
	go s.run(c, verify)
	return id, nil
}
func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrClosed
	}
	return s, nil
}
func (s *session) status() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.publicError
}
func (s *session) fail(message string) {
	s.mu.Lock()
	s.publicError = message
	s.state = "failed"
	s.mu.Unlock()
}
func (s *session) stop() {
	s.cancel()
	s.mu.Lock()
	conn := s.transport
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		s.stop()
	}
	return nil
}
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.stopped = true
	all := m.sessions
	m.sessions = make(map[string]*session)
	m.mu.Unlock()
	for _, s := range all {
		s.stop()
	}
}
func (m *Manager) Poll(id string, ack uint64) (Frame, error) {
	s, err := m.get(id)
	if err != nil {
		return Frame{}, err
	}
	if !s.pollMu.TryLock() {
		return Frame{}, errors.New("poll já em andamento")
	}
	defer s.pollMu.Unlock()
	if ack > s.sequence || (s.sequence > 0 && ack < s.sequence-1) {
		return Frame{}, errors.New("ACK inválido")
	}
	if s.pending != nil && ack == s.sequence {
		s.pending = nil
	}
	if s.pending == nil {
		if ack != s.sequence {
			return Frame{}, errors.New("ACK inválido")
		}
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case b, ok := <-s.out:
			if ok {
				s.pending = b
				s.sequence++
			} else {
				state, msg := s.status()
				if state == "draining" {
					state = "closed"
					if msg != "" {
						state = "failed"
					}
				}
				return Frame{Sequence: s.sequence, State: state, Error: msg}, nil
			}
		case <-s.ctx.Done():
			return Frame{}, ErrClosed
		case <-timer.C:
		}
	}
	state, msg := s.status()
	// Do not expose terminal completion until the final output frame has been acknowledged.
	if s.pending != nil {
		return Frame{Sequence: s.sequence, Data: base64.StdEncoding.EncodeToString(s.pending), State: state, Error: msg}, nil
	}
	return Frame{Sequence: s.sequence, State: state, Error: msg}, nil
}
func (m *Manager) Input(id, encoded string) error {
	if len(encoded) > base64.StdEncoding.EncodedLen(InputSize) {
		return errors.New("entrada muito grande")
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(b) == 0 || len(b) > InputSize {
		return errors.New("entrada inválida")
	}
	s, err := m.get(id)
	if err != nil {
		return err
	}
	state, _ := s.status()
	if state != "open" {
		return ErrClosed
	}
	select {
	case <-s.ctx.Done():
		return ErrClosed
	default:
	}
	select {
	case s.input <- b:
		return nil
	default:
		return errors.New("entrada congestionada; sessão deve ser encerrada")
	}
}
func (m *Manager) Resize(id string, cols, rows int) error {
	if !dimensions(cols, rows) {
		return errors.New("dimensões inválidas")
	}
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if !s.resizeMu.TryLock() {
		return errors.New("resize já em andamento")
	}
	defer s.resizeMu.Unlock()
	s.mu.Lock()
	remote, state := s.remote, s.state
	s.mu.Unlock()
	if remote == nil || state != "open" {
		return ErrClosed
	}
	if err := remote.WindowChange(rows, cols); err != nil {
		return errors.New("falha ao redimensionar terminal")
	}
	return nil
}
func (s *session) run(c Config, verify ssh.HostKeyCallback) {
	defer close(s.done)
	defer close(s.out)
	deadline := time.Now().Add(15 * time.Second)
	dialCtx, cancel := context.WithDeadline(s.ctx, deadline)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)))
	if err != nil {
		s.fail("falha ao conectar ao servidor SSH")
		return
	}
	defer conn.Close()
	s.mu.Lock()
	s.transport = conn
	s.mu.Unlock()
	stopWatch := context.AfterFunc(s.ctx, func() { _ = conn.Close() })
	defer stopWatch()
	_ = conn.SetDeadline(deadline)
	config := &ssh.ClientConfig{User: c.User, Auth: []ssh.AuthMethod{ssh.Password(c.Password)}, HostKeyCallback: verify}
	c.Password = ""
	cc, chans, reqs, err := ssh.NewClientConn(conn, net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), config)
	config.Auth = nil
	if err != nil {
		s.fail("handshake SSH recusado: verifique identidade do host, credenciais e rede")
		return
	}
	client := ssh.NewClient(cc, chans, reqs)
	defer client.Close()
	remote, err := client.NewSession()
	if err != nil {
		s.fail("não foi possível abrir canal SSH")
		return
	}
	defer remote.Close()
	stdin, err := remote.StdinPipe()
	if err != nil {
		s.fail("falha ao abrir entrada SSH")
		return
	}
	stdout, err := remote.StdoutPipe()
	if err != nil {
		s.fail("falha ao abrir saída SSH")
		return
	}
	stderr, err := remote.StderrPipe()
	if err != nil {
		s.fail("falha ao abrir saída SSH")
		return
	}
	if err = remote.RequestPty("xterm-256color", c.Rows, c.Cols, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}); err != nil {
		s.fail("servidor recusou PTY")
		return
	}
	if err = remote.Shell(); err != nil {
		s.fail("servidor recusou shell")
		return
	}
	_ = conn.SetDeadline(time.Time{})
	s.mu.Lock()
	s.remote = remote
	s.state = "open"
	s.mu.Unlock()
	workersCtx, stopWorkers := context.WithCancel(s.ctx)
	defer stopWorkers()
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for {
			select {
			case <-workersCtx.Done():
				return
			case b := <-s.input:
				if _, e := stdin.Write(b); e != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workersCtx.Done():
				return
			case <-ticker.C:
				timeout := time.AfterFunc(5*time.Second, func() { _ = conn.Close() })
				_, _, e := client.SendRequest("keepalive@openssh.com", true, nil)
				timeout.Stop()
				if e != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	var readers sync.WaitGroup
	read := func(r io.Reader) {
		defer readers.Done()
		for {
			buf := make([]byte, ChunkSize)
			n, e := r.Read(buf)
			if n > 0 {
				select {
				case s.out <- buf[:n]:
				case <-s.ctx.Done():
					return
				}
			}
			if e != nil {
				return
			}
		}
	}
	readers.Add(2)
	go read(stdout)
	go read(stderr)
	err = remote.Wait()
	// Readers may still have final bytes. Keep session alive until those bytes enter the bounded queue.
	stopWorkers()
	_ = client.Close()
	workers.Wait()
	s.mu.Lock()
	s.state = "draining"
	if err != nil {
		s.publicError = "sessão encerrada com erro ou conexão perdida"
	}
	s.mu.Unlock()
	readers.Wait()
}
