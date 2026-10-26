// Package observability parses bounded Linux snapshots; it has no SSH or UI dependencies.
package observability

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

const MaxBytes = 128 << 10

// Command is constant: no renderer, profile or remote text is interpolated.
const Command = `LC_ALL=C; export LC_ALL
[ "$(uname -s)" = Linux ] || exit 40
printf 'NISHI_STAT\n'; cat /proc/stat || exit 41
printf 'NISHI_MEM\n'; cat /proc/meminfo || exit 41
printf 'NISHI_NET\n'; cat /proc/net/dev || exit 41
printf 'NISHI_UP\n'; cat /proc/uptime || exit 41
printf 'NISHI_DISK\n'; df -Pk / || exit 41
printf 'NISHI_END\n'
`

type Counters struct{ RX, TX uint64 }
type Raw struct {
	Total, Idle             uint64
	Cores                   int
	MemoryTotal, MemoryUsed uint64
	DiskTotal, DiskUsed     uint64
	Uptime                  float64
	Network                 map[string]Counters
}
type Sample struct {
	MeasuredAt  int64    `json:"measuredAt"`
	CPU         *float64 `json:"cpu"`
	Cores       int      `json:"cores"`
	MemoryTotal uint64   `json:"memoryTotal"`
	MemoryUsed  uint64   `json:"memoryUsed"`
	DiskTotal   uint64   `json:"diskTotal"`
	DiskUsed    uint64   `json:"diskUsed"`
	Uptime      float64  `json:"uptime"`
	Receive     *float64 `json:"receive"`
	Transmit    *float64 `json:"transmit"`
}

var ErrUnsupported = errors.New("métricas indisponíveis: requer Linux com /proc e df acessíveis")

func number(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) }
func Parse(data []byte) (Raw, error) {
	r := Raw{Network: map[string]Counters{}}
	if len(data) > MaxBytes {
		return r, ErrUnsupported
	}
	sections := map[string][]string{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "NISHI_") {
			if _, exists := sections[line]; exists {
				return r, ErrUnsupported
			}
			section = line
			sections[section] = nil
			continue
		}
		if strings.TrimSpace(line) != "" {
			sections[section] = append(sections[section], line)
		}
	}
	for _, name := range []string{"NISHI_STAT", "NISHI_MEM", "NISHI_NET", "NISHI_UP", "NISHI_DISK", "NISHI_END"} {
		if _, ok := sections[name]; !ok {
			return r, ErrUnsupported
		}
	}
	cpuFound := false
	for _, line := range sections["NISHI_STAT"] {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if f[0] == "cpu" {
			if cpuFound || len(f) < 9 {
				return r, ErrUnsupported
			}
			cpuFound = true
			for i := 1; i <= 8; i++ {
				n, e := number(f[i])
				if e != nil || math.MaxUint64-r.Total < n {
					return r, ErrUnsupported
				}
				r.Total += n
				if i == 4 || i == 5 {
					r.Idle += n
				}
			}
		} else if strings.HasPrefix(f[0], "cpu") {
			if _, e := number(strings.TrimPrefix(f[0], "cpu")); e == nil {
				r.Cores++
			}
		}
	}
	mem := map[string]uint64{}
	for _, line := range sections["NISHI_MEM"] {
		f := strings.Fields(line)
		if len(f) >= 3 && (f[0] == "MemTotal:" || f[0] == "MemAvailable:") {
			n, e := number(f[1])
			if e != nil || f[2] != "kB" || n > math.MaxUint64/1024 {
				return r, ErrUnsupported
			}
			mem[f[0]] = n * 1024
		}
	}
	available, ok := mem["MemAvailable:"]
	r.MemoryTotal = mem["MemTotal:"]
	if !ok || r.MemoryTotal == 0 || available > r.MemoryTotal {
		return r, ErrUnsupported
	}
	r.MemoryUsed = r.MemoryTotal - available
	for _, line := range sections["NISHI_NET"] {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "lo" {
			continue
		}
		f := strings.Fields(rest)
		if len(f) != 16 || len(name) > 64 || len(r.Network) >= 256 {
			return r, ErrUnsupported
		}
		rx, e := number(f[0])
		tx, e2 := number(f[8])
		if e != nil || e2 != nil {
			return r, ErrUnsupported
		}
		r.Network[name] = Counters{rx, tx}
	}
	up := strings.Fields(strings.Join(sections["NISHI_UP"], " "))
	if len(up) != 2 {
		return r, ErrUnsupported
	}
	var e error
	r.Uptime, e = strconv.ParseFloat(up[0], 64)
	if e != nil || math.IsNaN(r.Uptime) || math.IsInf(r.Uptime, 0) || r.Uptime < 0 {
		return r, ErrUnsupported
	}
	disk := sections["NISHI_DISK"]
	if len(disk) != 2 {
		return r, ErrUnsupported
	}
	f := strings.Fields(disk[1])
	if len(f) != 6 || f[5] != "/" {
		return r, ErrUnsupported
	}
	total, e := number(f[1])
	used, e2 := number(f[2])
	if e != nil || e2 != nil || total == 0 || used > total || total > math.MaxUint64/1024 {
		return r, ErrUnsupported
	}
	r.DiskTotal = total * 1024
	r.DiskUsed = used * 1024
	if !cpuFound || r.Cores < 1 || r.Idle > r.Total {
		return r, ErrUnsupported
	}
	return r, nil
}

// Rates are absent on initial sample, reset, changed interfaces, or long gaps.
func Calculate(r Raw, previous *Raw, elapsed float64, measuredAt int64) Sample {
	s := Sample{MeasuredAt: measuredAt, Cores: r.Cores, MemoryTotal: r.MemoryTotal, MemoryUsed: r.MemoryUsed, DiskTotal: r.DiskTotal, DiskUsed: r.DiskUsed, Uptime: r.Uptime}
	if previous == nil || elapsed <= 0 || elapsed > 15 || r.Uptime <= previous.Uptime {
		return s
	}
	if r.Cores == previous.Cores && r.Total > previous.Total && r.Idle >= previous.Idle {
		total, idle := r.Total-previous.Total, r.Idle-previous.Idle
		if idle <= total {
			v := 100 * (1 - float64(idle)/float64(total))
			s.CPU = &v
		}
	}
	if len(r.Network) != len(previous.Network) {
		return s
	}
	rx, tx := float64(0), float64(0)
	for name, c := range r.Network {
		old, ok := previous.Network[name]
		if !ok || c.RX < old.RX || c.TX < old.TX {
			return s
		}
		rx += float64(c.RX-old.RX) / elapsed
		tx += float64(c.TX-old.TX) / elapsed
	}
	s.Receive = &rx
	s.Transmit = &tx
	return s
}
