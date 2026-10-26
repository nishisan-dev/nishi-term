package sshengine

import (
	"io"
	"nishiterm/internal/observability"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const metricsFixture = `NISHI_STAT
cpu 100 0 100 800 0 0 0 0 0 0
cpu0 1 0 1 1 0 0 0 0
NISHI_MEM
MemTotal: 1000 kB
MemAvailable: 400 kB
NISHI_NET
 eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0
NISHI_UP
100.0 50.0
NISHI_DISK
Filesystem 1024-blocks Used Available Capacity Mounted on
/dev/test 10000 2500 7500 25% /
NISHI_END
`

func serveMetricsFixture(ch ssh.Channel, r *ssh.Request, mode string) {
	var request struct{ Command string }
	_ = ssh.Unmarshal(r.Payload, &request)
	if request.Command != observability.Command {
		_ = r.Reply(false, nil)
		return
	}
	if mode == "metrics:stall" {
		_, _ = io.Copy(io.Discard, ch)
		return
	}
	_ = r.Reply(true, nil)
	switch mode {
	case "metrics:oversize":
		_, _ = io.WriteString(ch, strings.Repeat("x", observability.MaxBytes+1))
	case "metrics:invalid":
		_, _ = io.WriteString(ch, "remote text is not metrics")
	default:
		_, _ = io.WriteString(ch, metricsFixture)
	}
	_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
}
func TestMetricsIsolationAndLimits(t *testing.T) {
	for _, mode := range []string{"ok", "invalid", "oversize", "stall", "open-stall"} {
		t.Run(mode, func(t *testing.T) {
			f := serverWithFiles(t, nil, 0, "metrics:"+mode)
			m := manager(t)
			id := start(t, m, f.config)
			s, _ := m.get(id)
			eventually(t, func() bool { state, _ := s.status(); return state == "open" })
			completed := make(chan error, 1)
			go func() {
				sample, e := m.collectMetrics(id, 300*time.Millisecond)
				if mode == "ok" && sample.MemoryTotal != 1024000 {
					t.Error("bad sample")
				}
				completed <- e
			}()
			eventually(t, func() bool { return f.metricsStarted.Load() })
			if mode == "stall" {
				if _, e := m.ListFiles(id, "."); e != nil {
					t.Fatal("metrics blocked SFTP", e)
				}
			}
			if mode == "stall" || mode == "open-stall" {
				if _, e := m.Metrics(id); e == nil {
					t.Fatal("overlapping collection accepted")
				}
			}
			var ack uint64
			if e := echoOnce(m, id, &ack, "during-metrics"); e != nil {
				t.Fatal(e)
			}
			select {
			case e := <-completed:
				if (mode == "ok") != (e == nil) {
					t.Fatal("unexpected result", e)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("collection stuck")
			}
			if e := echoOnce(m, id, &ack, "after-metrics"); e != nil {
				t.Fatal(e)
			}
			if mode == "open-stall" {
				for i := 0; i < 10; i++ {
					if _, e := m.Metrics(id); e == nil {
						t.Fatal("new workers during blocked open")
					}
				}
			}
			m.Close(id)
			eventually(t, func() bool {
				if s.metricsMu.TryLock() {
					s.metricsMu.Unlock()
					return true
				}
				return false
			})
		})
	}
}
func TestMetricsCancel(t *testing.T) {
	f := serverWithFiles(t, nil, 0, "metrics:stall")
	m := manager(t)
	id := start(t, m, f.config)
	s, _ := m.get(id)
	eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	done := make(chan error, 1)
	go func() { _, e := m.Metrics(id); done <- e }()
	eventually(t, func() bool { return f.metricsStarted.Load() })
	m.CancelMetrics(id)
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancelled collection succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock")
	}
	var ack uint64
	if e := echoOnce(m, id, &ack, "after-cancel"); e != nil {
		t.Fatal(e)
	}
}
func TestMetricsCommandLocalLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux collector, Windows transport covered separately")
	}
	// Local read-only fixture only: never SSH to user hosts.
	data, e := exec.Command("sh", "-c", observability.Command).Output()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = observability.Parse(data); e != nil {
		t.Fatal(e)
	}
}

func TestMetricsSixteenSessionsRemainIndependent(t *testing.T) {
	f := serverWithFiles(t, nil, 0, "metrics:stall")
	m := manager(t)
	ids := make([]string, MaxSessions)
	sessions := make([]*session, MaxSessions)
	for i := range ids {
		ids[i] = start(t, m, f.config)
		sessions[i], _ = m.get(ids[i])
		s := sessions[i]
		eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	}
	done := make(chan error, MaxSessions)
	for _, id := range ids {
		go func(id string) { _, e := m.collectMetrics(id, 200*time.Millisecond); done <- e }(id)
	}
	for _, id := range ids {
		var ack uint64
		if e := echoOnce(m, id, &ack, "metrics-load"); e != nil {
			t.Fatal(e)
		}
	}
	for range ids {
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("stalled collection succeeded")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("metrics load stuck")
		}
	}
	if m.ActiveCount() != MaxSessions {
		t.Fatal("metrics affected session capacity")
	}
	for _, id := range ids {
		m.Close(id)
	}
	for _, s := range sessions {
		eventually(t, func() bool {
			if s.metricsMu.TryLock() {
				s.metricsMu.Unlock()
				return true
			}
			return false
		})
	}
}
