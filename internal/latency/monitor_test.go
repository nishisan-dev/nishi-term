package latency

import (
	"context"
	"errors"
	"fmt"
	"net"
	"nishiterm/internal/profiles"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestSamplesLifecycle(t *testing.T) {
	items := []profiles.Profile{{Host: "one"}, {Host: "one"}, {Host: "two"}}
	m := New(func() ([]profiles.Profile, error) { return items, nil })
	calls := atomic.Int32{}
	m.probe = func(context.Context, string) (Sample, error) { calls.Add(1); return Sample{Milliseconds: 12}, nil }
	m.cycle(context.Background())
	if calls.Load() != 2 || len(m.Snapshot()) != 2 {
		t.Fatal("must deduplicate hosts")
	}
	snapshot := m.Snapshot()
	delete(snapshot, "one")
	if len(m.Snapshot()) != 2 {
		t.Fatal("snapshot must be isolated")
	}
	items = items[:1]
	m.probe = func(context.Context, string) (Sample, error) { return Sample{}, errors.New("blocked") }
	m.cycle(context.Background())
	if len(m.Snapshot()) != 0 {
		t.Fatal("failure and removed hosts must disappear")
	}
	m.samples["stale"] = Sample{MeasuredAt: time.Now().Add(-Lifetime - time.Second).UnixMilli()}
	if len(m.Snapshot()) != 0 {
		t.Fatal("stale sample visible")
	}
	m.source = func() ([]profiles.Profile, error) { return nil, errors.New("read failure") }
	m.cycle(context.Background())
	if len(m.samples) != 0 {
		t.Fatal("source failure must clear samples")
	}
}
func TestBoundedWorkersAndCancellation(t *testing.T) {
	items := make([]profiles.Profile, 200)
	for i := range items {
		items[i].Host = fmt.Sprint(i)
	}
	m := New(func() ([]profiles.Profile, error) { return items, nil })
	entered := make(chan struct{}, 200)
	var active, peak atomic.Int32
	m.probe = func(ctx context.Context, _ string) (Sample, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		<-ctx.Done()
		return Sample{}, ctx.Err()
	}
	m.Start(context.Background())
	for i := 0; i < concurrency; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("workers did not start")
		}
	}
	m.Close()
	m.Close()
	if peak.Load() > concurrency || active.Load() != 0 || len(m.Snapshot()) != 0 {
		t.Fatal("worker/cancellation bound violated")
	}
}
func TestEchoLoopback(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("unsupported platform")
	}
	for _, host := range []string{"127.0.0.1", "::1"} {
		t.Run(host, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			elapsed, err := echo(ctx, net.IPAddr{IP: net.ParseIP(host)})
			if err != nil {
				t.Fatal(err)
			}
			if elapsed < 0 || elapsed > time.Second {
				t.Fatal("invalid RTT", elapsed)
			}
		})
	}
}
func TestProbeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := probe(ctx, "127.0.0.1"); err == nil {
		t.Fatal("cancelled probe succeeded")
	}
}
