package sshengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

func waitFinished(t *testing.T, s *session) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not release workers")
	}
}
func TestSixteenConcurrentAdmission(t *testing.T) {
	f := server(t, nil, 0)
	m := manager(t)
	var wg sync.WaitGroup
	ids := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if id, e := m.Start(f.config); e == nil {
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	if len(ids) != 16 {
		t.Fatalf("accepted %d simultaneous starts, want 16", len(ids))
	}
	for id := range ids {
		m.Close(id)
	}
}
func TestSixteenConnectingAndEndedCapacity(t *testing.T) {
	t.Run("pending consent occupies slots", func(t *testing.T) {
		f := server(t, nil, 0)
		c := f.config
		c.KnownHosts = ""
		m := NewManagerWithTrust(filepath.Join(t.TempDir(), "trust.json"))
		defer m.Shutdown()
		ids := make([]string, 16)
		for i := range ids {
			ids[i] = start(t, m, c)
		}
		for _, id := range ids {
			s, _ := m.get(id)
			eventually(t, func() bool { return s.challenge() != nil })
		}
		if _, e := m.Start(c); e == nil {
			t.Fatal("17th pending handshake accepted")
		}
		cancelled, _ := m.get(ids[0])
		m.Close(ids[0])
		waitFinished(t, cancelled)
		start(t, m, c)
		if f.auth.Load() != 0 {
			t.Fatal("credentials sent without trust")
		}
	})
	t.Run("completed and failed records do not occupy slots", func(t *testing.T) {
		finished := server(t, []byte("final-output"), 0)
		live := server(t, nil, 0)
		m := manager(t)
		var old []string
		for i := 0; i < 16; i++ {
			id := start(t, m, finished.config)
			old = append(old, id)
			s, _ := m.get(id)
			waitFinished(t, s)
		}
		for i := 0; i < 16; i++ {
			c := live.config
			c.Password = "incorrect"
			id := start(t, m, c)
			s, _ := m.get(id)
			waitFinished(t, s)
		}
		for i := 0; i < 16; i++ {
			start(t, m, live.config)
		}
		if _, e := m.Start(live.config); e == nil {
			t.Fatal("17th live session accepted")
		}
		for _, id := range old {
			frame, e := m.Poll(id, 0)
			if e != nil {
				t.Fatal(e)
			}
			data, e := base64.StdEncoding.DecodeString(frame.Data)
			if e != nil || string(data) != "final-output" {
				t.Fatal("retained final output lost")
			}
			m.Close(id)
		}
	})
}

func TestSixteenLoadIntegrity(t *testing.T) {
	payload := bytes.Repeat([]byte{'x'}, 4<<20)
	expected := sha256.Sum256(payload)
	f := server(t, payload, 0)
	m := manager(t)
	ids := make([]string, 16)
	sessions := make([]*session, 16)
	begin := time.Now()
	for i := range ids {
		ids[i] = start(t, m, f.config)
		sessions[i], _ = m.get(ids[i])
	}
	for _, s := range sessions {
		eventually(t, func() bool { return len(s.out) == QueueSize })
	}
	if _, e := m.Start(f.config); e == nil {
		t.Fatal("17th loaded session accepted")
	}
	time.Sleep(time.Second)
	middle := labSnapshot()
	time.Sleep(2 * time.Second)
	paused := labSnapshot()
	for _, s := range sessions {
		if len(s.out) != QueueSize {
			t.Fatal("paused queue changed size")
		}
	}
	if delta(middle.Heap, paused.Heap) > 32<<20 {
		t.Fatal("heap failed to stabilize during paused consumption")
	}
	var wg sync.WaitGroup
	errors := make(chan error, 16)
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			hash := sha256.New()
			var ack uint64
			total := 0
			deadline := time.Now().Add(90 * time.Second)
			for time.Now().Before(deadline) {
				frame, e := m.Poll(id, ack)
				if e != nil {
					errors <- e
					return
				}
				if frame.Data != "" {
					b, e := base64.StdEncoding.DecodeString(frame.Data)
					if e != nil {
						errors <- e
						return
					}
					hash.Write(b)
					total += len(b)
					ack = frame.Sequence
				}
				if frame.State == "failed" {
					errors <- fmt.Errorf("load session failed")
					return
				}
				if frame.State == "closed" {
					if total != len(payload) || !bytes.Equal(hash.Sum(nil), expected[:]) {
						errors <- fmt.Errorf("integrity mismatch")
					}
					return
				}
			}
			errors <- fmt.Errorf("load drain timed out")
		}(id)
	}
	wg.Wait()
	close(errors)
	for e := range errors {
		t.Error(e)
	}
	for i, id := range ids {
		m.Close(id)
		waitFinished(t, sessions[i])
	}
	labRecord(t, "sixteenLoad", map[string]any{"sessions": 16, "bytes": 16 * len(payload), "pauseMs": 3000, "queueBlocksPerSession": QueueSize, "middle": middle, "paused": paused, "elapsedMs": time.Since(begin).Milliseconds(), "sha256PerSession": fmt.Sprintf("%x", expected)})
}
func echoOnce(m *Manager, id string, ack *uint64, marker string) error {
	if e := m.Input(id, base64.StdEncoding.EncodeToString([]byte(marker))); e != nil {
		return e
	}
	var got []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f, e := m.Poll(id, *ack)
		if e != nil {
			return e
		}
		if f.Data != "" {
			b, e := base64.StdEncoding.DecodeString(f.Data)
			if e != nil {
				return e
			}
			got = append(got, b...)
			*ack = f.Sequence
			if len(got) >= len(marker) {
				if string(got) != marker {
					return fmt.Errorf("session output mixed or corrupted")
				}
				return nil
			}
		}
		if f.State == "failed" || f.State == "closed" {
			return fmt.Errorf("session ended unexpectedly")
		}
	}
	return fmt.Errorf("echo timeout")
}
func TestSixteenIsolation(t *testing.T) {
	flood := server(t, bytes.Repeat([]byte{'f'}, 4<<20), 0)
	echo := server(t, nil, 0)
	m := manager(t)
	blocked := start(t, m, flood.config)
	slow, _ := m.get(blocked)
	eventually(t, func() bool { return len(slow.out) == QueueSize })
	ids := make([]string, 15)
	for i := range ids {
		ids[i] = start(t, m, echo.config)
		s, _ := m.get(ids[i])
		eventually(t, func() bool { state, _ := s.status(); return state == "open" })
	}
	if _, e := m.Start(echo.config); e == nil {
		t.Fatal("17th accepted")
	}
	samples := make(chan time.Duration, 300)
	errs := make(chan error, 15)
	acks := make([]uint64, 15)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			for n := 0; n < 20; n++ {
				begin := time.Now()
				if e := echoOnce(m, id, &acks[i], fmt.Sprintf("session-%d-message-%d\n", i, n)); e != nil {
					errs <- e
					return
				}
				samples <- time.Since(begin)
			}
		}(i, id)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	close(samples)
	var ms []float64
	for d := range samples {
		ms = append(ms, d.Seconds()*1000)
	}
	if len(ms) != 300 {
		t.Fatal("missing interactive responses")
	}
	sort.Float64s(ms)
	begin := time.Now()
	m.Close(blocked)
	waitFinished(t, slow)
	closeMS := time.Since(begin).Seconds() * 1000
	replacement := start(t, m, echo.config)
	defer m.Close(replacement)
	for i, id := range ids {
		if e := echoOnce(m, id, &acks[i], "still-alive\n"); e != nil {
			t.Fatal(e)
		}
	}
	if ms[284] > 1000 {
		t.Fatal("loopback p95 exceeded 1s CI tolerance")
	}
	labRecord(t, "sixteenIsolation", map[string]any{"blockedSessions": 1, "interactiveSessions": 15, "samples": len(ms), "p95Ms": ms[284], "maxMs": ms[len(ms)-1], "blockedCloseMs": closeMS, "replacementAccepted": true, "remainingSessionsRespond": true})
}
func TestSixteenCleanup(t *testing.T) {
	before := labSnapshot()
	t.Run("four waves", func(t *testing.T) {
		f := server(t, nil, 0)
		m := manager(t)
		for wave := 0; wave < 4; wave++ {
			ids := make([]string, 16)
			sessions := make([]*session, 16)
			for i := range ids {
				ids[i] = start(t, m, f.config)
				sessions[i], _ = m.get(ids[i])
			}
			for _, s := range sessions {
				eventually(t, func() bool { state, _ := s.status(); return state == "open" })
			}
			for i, id := range ids {
				m.Close(id)
				waitFinished(t, sessions[i])
			}
			if len(m.sessions) != 0 {
				t.Fatal("records leaked after close")
			}
		}
	})
	time.Sleep(100 * time.Millisecond)
	after := labSnapshot()
	if after.Goroutines > before.Goroutines+4 || after.Handles > before.Handles+8 {
		t.Fatalf("resources not released: before=%+v after=%+v", before, after)
	}
	labRecord(t, "sixteenCleanup", map[string]any{"waves": 4, "connections": 64, "before": before, "after": after})
}
