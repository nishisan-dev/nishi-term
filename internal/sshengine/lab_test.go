package sshengine

// Opt-in tests against the disposable OpenSSH fixture. Credentials are read from
// a private JSON file and never included in test output. No production targets.
import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh/knownhosts"
)

func labConfig(t *testing.T) Config {
	t.Helper()
	path := os.Getenv("NISHITERM_LAB_CONFIG")
	if path == "" {
		t.Skip("opt-in disposable OpenSSH lab: NISHITERM_LAB_CONFIG is unset")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal("cannot read private lab config")
	}
	var c Config
	if json.Unmarshal(raw, &c) != nil {
		t.Fatal("invalid private lab config")
	}
	c.Cols, c.Rows = 100, 30
	return c
}

type labStream struct {
	t        *testing.T
	m        *Manager
	id       string
	ack      uint64
	stash    []byte
	deadline time.Time
}

func labOpen(t *testing.T, m *Manager, c Config) *labStream {
	t.Helper()
	p := &labStream{t: t, m: m, id: start(t, m, c), deadline: time.Now().Add(3 * time.Minute)}
	t.Cleanup(func() { _ = m.Close(p.id) })
	s, _ := m.get(p.id)
	deadline := time.Now().Add(20 * time.Second)
	for {
		state, msg := s.status()
		if state == "open" {
			break
		}
		if state == "failed" {
			t.Fatal(msg)
		}
		if time.Now().After(deadline) {
			t.Fatal("open timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	p.send("stty -echo -onlcr; printf 'LAB_%s\\n' READY\n")
	p.until("LAB_READY\n")
	return p
}
func (p *labStream) send(s string) {
	p.t.Helper()
	if e := p.m.Input(p.id, base64.StdEncoding.EncodeToString([]byte(s))); e != nil {
		p.t.Fatal(e)
	}
}
func (p *labStream) Read(b []byte) (int, error) {
	for len(p.stash) == 0 {
		if time.Now().After(p.deadline) {
			return 0, fmt.Errorf("lab read deadline")
		}
		f, e := p.m.Poll(p.id, p.ack)
		if e != nil {
			return 0, e
		}
		if f.Data != "" {
			p.stash, e = base64.StdEncoding.DecodeString(f.Data)
			if e != nil {
				return 0, e
			}
			p.ack = f.Sequence
		} else if f.State == "closed" || f.State == "failed" {
			return 0, io.EOF
		}
	}
	n := copy(b, p.stash)
	p.stash = p.stash[n:]
	return n, nil
}
func (p *labStream) until(marker string) {
	p.t.Helper()
	var tail []byte
	var b [1]byte
	for {
		if _, e := io.ReadFull(p, b[:]); e != nil {
			p.t.Fatalf("marker missing: %v", e)
		}
		tail = append(tail, b[0])
		if len(tail) > len(marker) {
			tail = tail[1:]
		}
		if string(tail) == marker {
			return
		}
	}
}
func labRecord(t *testing.T, name string, v any) {
	b, _ := json.Marshal(v)
	t.Logf("MEASUREMENT %s %s", name, b)
}

type labMemory struct {
	Heap       uint64 `json:"heapBytes"`
	RSS        uint64 `json:"rssBytes"`
	Handles    uint64 `json:"handlesOrFDs"`
	Goroutines int    `json:"goroutines"`
}

func labSnapshot() labMemory {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	rss, h := labProcessResources()
	return labMemory{m.HeapAlloc, rss, h, runtime.NumGoroutine()}
}
func delta(a, b uint64) int64 { return int64(b) - int64(a) }
func labClose(t *testing.T, m *Manager, id string) time.Duration {
	t.Helper()
	s, e := m.get(id)
	if e != nil {
		t.Fatal(e)
	}
	begin := time.Now()
	_ = m.Close(id)
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("workers survived close deadline")
	}
	return time.Since(begin)
}

func TestLabOutput64MiBPaused30Seconds(t *testing.T) {
	c := labConfig(t)
	m := manager(t)
	p := labOpen(t, m, c)
	p.send("printf 'BEGIN_%s\\n' LOAD; python3 -c \"import os; b=b'x'*65536; [os.write(1,b) for _ in range(1024)]\"; printf 'END_%s\\n' LOAD\n")
	p.until("BEGIN_LOAD\n")
	s, _ := m.get(p.id)
	eventually(t, func() bool { return len(s.out) == QueueSize })
	begin := time.Now()
	before := labSnapshot()
	time.Sleep(15 * time.Second)
	middle := labSnapshot()
	time.Sleep(15 * time.Second)
	after := labSnapshot()
	labRecord(t, "pausedOutput", map[string]any{"pauseMs": time.Since(begin).Milliseconds(), "queueBlocks": len(s.out), "before": before, "middle": middle, "after": after, "heapGrowth": delta(before.Heap, after.Heap), "rssGrowth": delta(before.RSS, after.RSS)})
	if delta(before.Heap, after.Heap) > 8<<20 || delta(before.RSS, after.RSS) > 32<<20 {
		t.Fatal("paused memory exceeded lab gate")
	}
	received := sha256.New()
	n, e := io.CopyN(received, p, 64<<20)
	if e != nil {
		t.Fatal(e)
	}
	expected := sha256.New()
	block := bytes.Repeat([]byte{'x'}, 65536)
	for i := 0; i < 1024; i++ {
		_, _ = expected.Write(block)
	}
	if !bytes.Equal(received.Sum(nil), expected.Sum(nil)) {
		t.Fatal("output corruption")
	}
	p.until("END_LOAD\n")
	labRecord(t, "outputIntegrity", map[string]any{"bytes": n, "sha256": fmt.Sprintf("%x", received.Sum(nil)), "closeMs": labClose(t, m, p.id).Seconds() * 1000})
}
func TestLabFourSessionsIsolation(t *testing.T) {
	c := labConfig(t)
	m := manager(t)
	ps := make([]*labStream, 4)
	for i := range ps {
		ps[i] = labOpen(t, m, c)
	}
	if _, e := m.Start(c); e == nil {
		t.Fatal("fifth session accepted")
	}
	ps[0].send("python3 -c \"import os; b=b'x'*65536; exec('while True: os.write(1,b)')\"\n")
	slow, _ := m.get(ps[0].id)
	eventually(t, func() bool { return len(slow.out) == QueueSize })
	var wg sync.WaitGroup
	times := make(chan time.Duration, 60)
	for i := 1; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := ps[i]
			for j := 0; j < 20; j++ {
				begin := time.Now()
				marker := fmt.Sprintf("SESSION_%d_%d", i, j)
				p.send(fmt.Sprintf("printf '%s\\n'\n", marker))
				p.until(marker + "\n")
				times <- time.Since(begin)
			}
		}(i)
	}
	wg.Wait()
	close(times)
	var ms []float64
	for d := range times {
		ms = append(ms, d.Seconds()*1000)
	}
	sort.Float64s(ms)
	closeMS := labClose(t, m, ps[0].id).Seconds() * 1000
	for i := 1; i < 4; i++ {
		ps[i].send("printf 'STILL_%s\\n' OPEN\n")
		ps[i].until("STILL_OPEN\n")
	}
	labRecord(t, "fourSessions", map[string]any{"interactiveSamples": len(ms), "roundTripP95Ms": ms[56], "roundTripMaxMs": ms[len(ms)-1], "blockedCloseMs": closeMS, "remainingSessionsRespond": true})
	if ms[56] > 250 {
		t.Error("LAN interactive p95 exceeded 250ms")
	}
}
func TestLab100CyclesCleanup(t *testing.T) {
	c := labConfig(t)
	m := manager(t)
	// Warm caches and SSH code paths before the baseline.
	p := labOpen(t, m, c)
	labClose(t, m, p.id)
	time.Sleep(200 * time.Millisecond)
	before := labSnapshot()
	var maxClose float64
	for i := 0; i < 100; i++ {
		p = labOpen(t, m, c)
		p.send("printf 'CYCLE_%s\\n' OK\n")
		p.until("CYCLE_OK\n")
		d := labClose(t, m, p.id).Seconds() * 1000
		if d > maxClose {
			maxClose = d
		}
	}
	time.Sleep(300 * time.Millisecond)
	after := labSnapshot()
	labRecord(t, "cycles", map[string]any{"count": 100, "before": before, "after": after, "maxCloseMs": maxClose, "remainingSessions": len(m.sessions)})
	if len(m.sessions) != 0 || after.Goroutines > before.Goroutines+3 || after.Handles > before.Handles+8 || delta(before.Heap, after.Heap) > 8<<20 {
		t.Fatal("cleanup baseline not restored")
	}
}

// This userspace relay drops traffic without changing any host firewall rules.
func labRelay(t *testing.T, c Config) (Config, *atomic.Bool) {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	drop := &atomic.Bool{}
	target := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	var mu sync.Mutex
	var sockets []net.Conn
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			a, e := l.Accept()
			if e != nil {
				return
			}
			b, e := net.DialTimeout("tcp", target, 5*time.Second)
			if e != nil {
				_ = a.Close()
				continue
			}
			mu.Lock()
			sockets = append(sockets, a, b)
			mu.Unlock()
			wg.Add(2)
			forward := func(dst, src net.Conn) {
				defer wg.Done()
				defer dst.Close()
				buf := make([]byte, 32768)
				for {
					n, e := src.Read(buf)
					if n > 0 && !drop.Load() {
						if _, w := dst.Write(buf[:n]); w != nil {
							return
						}
					}
					if e != nil {
						return
					}
				}
			}
			go forward(a, b)
			go forward(b, a)
		}
	}()
	t.Cleanup(func() {
		_ = l.Close()
		mu.Lock()
		for _, s := range sockets {
			_ = s.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	raw, e := os.ReadFile(c.KnownHosts)
	if e != nil {
		t.Fatal(e)
	}
	var trust strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && !strings.HasPrefix(fields[0], "#") {
			fmt.Fprintf(&trust, "%s %s %s\n", knownhosts.Normalize(l.Addr().String()), fields[1], fields[2])
		}
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	if e = os.WriteFile(path, []byte(trust.String()), 0600); e != nil {
		t.Fatal(e)
	}
	c.Host, c.Port = "127.0.0.1", l.Addr().(*net.TCPAddr).Port
	c.KnownHosts = path
	return c, drop
}
func TestLabNetworkBlackhole(t *testing.T) {
	c, drop := labRelay(t, labConfig(t))
	m := manager(t)
	p := labOpen(t, m, c)
	s, _ := m.get(p.id)
	begin := time.Now()
	drop.Store(true)
	select {
	case <-s.done:
	case <-time.After(16 * time.Second):
		t.Fatal("blackhole detection exceeded 16s tolerance")
	}
	state, msg := s.status()
	labRecord(t, "blackhole", map[string]any{"detectedMs": time.Since(begin).Milliseconds(), "state": state, "hasError": msg != ""})
	if msg == "" {
		t.Fatal("network loss not reported")
	}
}
func TestLabCancelAndTimeoutHandshake(t *testing.T) {
	c := labConfig(t)
	for _, cancel := range []bool{true, false} {
		t.Run(fmt.Sprintf("cancel=%v", cancel), func(t *testing.T) {
			l, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			accepted := make(chan net.Conn, 1)
			go func() {
				a, e := l.Accept()
				if e == nil {
					accepted <- a
				}
			}()
			c.Host, c.Port = "127.0.0.1", l.Addr().(*net.TCPAddr).Port
			m := manager(t)
			begin := time.Now()
			id := start(t, m, c)
			s, _ := m.get(id)
			select {
			case a := <-accepted:
				defer a.Close()
			case <-time.After(2 * time.Second):
				t.Fatal("missing TCP connection")
			}
			if cancel {
				ms := labClose(t, m, id).Seconds() * 1000
				labRecord(t, "cancelHandshake", map[string]any{"closeMs": ms})
			} else {
				select {
				case <-s.done:
				case <-time.After(16 * time.Second):
					t.Fatal("handshake timeout exceeded")
				}
				labRecord(t, "handshakeTimeout", map[string]any{"elapsedMs": time.Since(begin).Milliseconds()})
			}
		})
	}
}

func TestLabCtrlCLatency(t *testing.T) {
	c := labConfig(t)
	m := manager(t)
	p := labOpen(t, m, c)
	var samples []float64
	for i := 0; i < 30; i++ {
		marker := fmt.Sprintf("RUN_%d", i)
		p.send(fmt.Sprintf("printf '%s\\n'; sleep 60\n", marker))
		p.until(marker + "\n")
		time.Sleep(50 * time.Millisecond)
		begin := time.Now()
		p.send("\x03")
		time.Sleep(20 * time.Millisecond) // Allow the PTY interrupt to flush input before the probe command.
		p.send(fmt.Sprintf("printf 'STOP_%d\\n'\n", i))
		p.until(fmt.Sprintf("STOP_%d\n", i))
		samples = append(samples, time.Since(begin).Seconds()*1000)
	}
	sort.Float64s(samples)
	labRecord(t, "ctrlC", map[string]any{"samples": len(samples), "p95Ms": samples[28], "maxMs": samples[29]})
	if samples[28] > 250 {
		t.Error("Ctrl-C LAN p95 exceeded 250ms")
	}
}
