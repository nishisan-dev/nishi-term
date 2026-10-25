// Package latency reports optional ICMP RTT, never host availability or SSH state.
package latency

import (
	"context"
	"errors"
	"net"
	"nishiterm/internal/profiles"
	"sync"
	"time"
)

const Interval = 10 * time.Second
const Lifetime = 20 * time.Second
const concurrency = 32
const budget = time.Second

type Sample struct {
	Milliseconds float64 `json:"milliseconds"`
	Address      string  `json:"address"`
	MeasuredAt   int64   `json:"measuredAt"`
}
type probeFunc func(context.Context, string) (Sample, error)
type Monitor struct {
	source  func() ([]profiles.Profile, error)
	probe   probeFunc
	mu      sync.Mutex
	samples map[string]Sample
	cancel  context.CancelFunc
	done    chan struct{}
}

func New(source func() ([]profiles.Profile, error)) *Monitor {
	return &Monitor{source: source, probe: probe, samples: map[string]Sample{}}
}
func (m *Monitor) Start(parent context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.done = make(chan struct{})
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(Interval)
		defer ticker.Stop()
		for {
			m.cycle(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (m *Monitor) Close() {
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
func (m *Monitor) Snapshot() map[string]Sample {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := map[string]Sample{}
	cutoff := time.Now().Add(-Lifetime).UnixMilli()
	for host, s := range m.samples {
		if s.MeasuredAt >= cutoff {
			result[host] = s
		}
	}
	return result
}
func (m *Monitor) cycle(ctx context.Context) {
	items, err := m.source()
	if err != nil {
		m.mu.Lock()
		clear(m.samples)
		m.mu.Unlock()
		return
	}
	hosts := map[string]bool{}
	for _, p := range items {
		if len(hosts) >= profiles.MaxProfiles {
			break
		}
		hosts[p.Host] = true
	}
	m.mu.Lock()
	for host := range m.samples {
		if !hosts[host] {
			delete(m.samples, host)
		}
	}
	m.mu.Unlock()
	jobs := make(chan string, len(hosts))
	for host := range hosts {
		jobs <- host
	}
	close(jobs)
	var workers sync.WaitGroup
	for i := 0; i < concurrency && i < len(hosts); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for host := range jobs {
				if ctx.Err() != nil {
					return
				}
				attempt, cancel := context.WithTimeout(ctx, budget)
				sample, e := m.probe(attempt, host)
				cancel()
				m.mu.Lock()
				if e == nil && ctx.Err() == nil {
					sample.MeasuredAt = time.Now().UnixMilli()
					m.samples[host] = sample
				} else {
					delete(m.samples, host)
				}
				m.mu.Unlock()
			}
		}()
	}
	workers.Wait()
}
func probe(ctx context.Context, host string) (Sample, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return Sample{}, errors.New("no address")
	}
	address := addresses[0]
	for _, a := range addresses {
		if a.IP.To4() != nil {
			address = a
			break
		}
	}
	if address.IP.IsUnspecified() || address.IP.IsMulticast() || address.IP.Equal(net.IPv4bcast) {
		return Sample{}, errors.New("not a unicast target")
	}
	elapsed, err := echo(ctx, address)
	if err != nil {
		return Sample{}, err
	}
	return Sample{Milliseconds: float64(elapsed) / float64(time.Millisecond), Address: address.String()}, nil
}
