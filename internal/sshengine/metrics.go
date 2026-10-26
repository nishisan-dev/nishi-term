package sshengine

import (
	"bytes"
	"context"
	"errors"
	"io"
	"nishiterm/internal/observability"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var errMetrics = errors.New("sem atualização: coleta indisponível, cancelada ou tempo esgotado")

const metricsTimeout = 3 * time.Second

type boundedMetrics struct{ bytes.Buffer }

func (w *boundedMetrics) Write(p []byte) (int, error) {
	if len(p) > observability.MaxBytes-w.Len() {
		return 0, errors.New("metrics response exceeds budget")
	}
	return w.Buffer.Write(p)
}

// Metrics keeps at most one operation per session. A channel-open request cannot
// be cancelled by x/crypto/ssh: retain its single worker until it resolves or the
// SSH transport closes. Never close the terminal transport for a metrics timeout.
func (m *Manager) Metrics(id string) (observability.Sample, error) {
	return m.collectMetrics(id, metricsTimeout)
}
func (m *Manager) collectMetrics(id string, timeout time.Duration) (observability.Sample, error) {
	s, err := m.get(id)
	if err != nil {
		return observability.Sample{}, err
	}
	if !s.metricsMu.TryLock() {
		return observability.Sample{}, errors.New("aguarde a coleta anterior")
	}
	s.mu.Lock()
	client := s.client
	if s.state != "open" || client == nil || s.ctx.Err() != nil {
		s.mu.Unlock()
		s.metricsMu.Unlock()
		return observability.Sample{}, ErrClosed
	}
	if time.Since(s.metricsStarted) < 4*time.Second {
		s.mu.Unlock()
		s.metricsMu.Unlock()
		return observability.Sample{}, errors.New("aguarde o próximo intervalo de coleta")
	}
	s.metricsStarted = time.Now()
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	s.metricsCancel = cancel
	s.mu.Unlock()
	type result struct {
		sample observability.Sample
		err    error
	}
	done := make(chan result, 1)
	go func() {
		defer s.metricsMu.Unlock()
		defer func() { cancel(); s.mu.Lock(); s.metricsCancel = nil; s.mu.Unlock() }()
		var channelMu sync.Mutex
		var remote *ssh.Session
		stop := context.AfterFunc(ctx, func() {
			channelMu.Lock()
			defer channelMu.Unlock()
			if remote != nil {
				_ = remote.Close()
			}
		})
		defer stop()
		opened, e := client.NewSession()
		channelMu.Lock()
		remote = opened
		channelMu.Unlock()
		if e != nil {
			done <- result{err: errMetrics}
			return
		}
		defer opened.Close()
		if ctx.Err() != nil {
			done <- result{err: errMetrics}
			return
		}
		output := &boundedMetrics{}
		opened.Stdout = output
		opened.Stderr = io.Discard
		if e = opened.Run(observability.Command); e != nil || ctx.Err() != nil {
			done <- result{err: errMetrics}
			return
		}
		raw, e := observability.Parse(output.Bytes())
		if e != nil {
			done <- result{err: e}
			return
		}
		now := time.Now()
		sample := observability.Calculate(raw, s.metricsPrevious, now.Sub(s.metricsMeasured).Seconds(), now.UnixMilli())
		s.metricsPrevious = &raw
		s.metricsMeasured = now
		done <- result{sample: sample}
	}()
	select {
	case r := <-done:
		return r.sample, r.err
	case <-ctx.Done():
		return observability.Sample{}, errMetrics
	}
}
func (m *Manager) CancelMetrics(id string) {
	s, e := m.get(id)
	if e != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.metricsCancel != nil {
		s.metricsCancel()
	}
}
