//go:build !linux && !windows

package latency

import (
	"context"
	"errors"
	"net"
	"time"
)

func echo(context.Context, net.IPAddr) (time.Duration, error) {
	return 0, errors.New("ICMP unavailable")
}
