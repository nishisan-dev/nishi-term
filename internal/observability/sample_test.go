package observability

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func snapshot(cpu, up string) []byte {
	return []byte(fmt.Sprintf(`NISHI_STAT
cpu %s
cpu0 1 0 1 1 0 0 0 0 0 0
NISHI_MEM
MemTotal: 1000 kB
MemAvailable: 400 kB
NISHI_NET
Inter-| Receive | Transmit
 eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0
 lo: 999999 0 0 0 0 0 0 0 999999 0 0 0 0 0 0 0
NISHI_UP
%s 10.00
NISHI_DISK
Filesystem 1024-blocks Used Available Capacity Mounted on
/dev/test 10000 2500 7500 25%% /
NISHI_END
`, cpu, up))
}
func TestParseAndRates(t *testing.T) {
	first, e := Parse(snapshot("100 0 100 800 0 0 0 0 50 0", "100.0"))
	if e != nil {
		t.Fatal(e)
	}
	if first.Total != 1000 || first.Idle != 800 || first.MemoryUsed != 600*1024 || first.DiskUsed != 2500*1024 || len(first.Network) != 1 {
		t.Fatal(first)
	}
	next, e := Parse(snapshot("150 0 100 850 0 0 0 0 50 0", "105.0"))
	if e != nil {
		t.Fatal(e)
	}
	next.Network["eth0"] = Counters{1500, 3000}
	initial := Calculate(first, nil, 0, 1)
	if initial.CPU != nil || initial.Receive != nil {
		t.Fatal("invented initial rate")
	}
	s := Calculate(next, &first, 5, 2)
	if s.CPU == nil || math.Abs(*s.CPU-50) > 0.001 || *s.Receive != 100 || *s.Transmit != 200 {
		t.Fatal(s)
	}
	if Calculate(next, &first, 20, 2).CPU != nil {
		t.Fatal("long gap must reset")
	}
	next.Network["eth0"] = Counters{1, 2}
	if Calculate(next, &first, 5, 2).Receive != nil {
		t.Fatal("reset must not spike")
	}
	next.Uptime = 2
	if Calculate(next, &first, 5, 2).CPU != nil {
		t.Fatal("reboot must reset")
	}
}
func TestRejectIncompleteMalformedAndOversized(t *testing.T) {
	good := string(snapshot("100 0 100 800 0 0 0 0 0 0", "100"))
	for _, bad := range []string{strings.Repeat("x", MaxBytes+1), strings.ReplaceAll(good, "NISHI_END", "missing"), strings.ReplaceAll(good, "400 kB", "NaN kB"), strings.ReplaceAll(good, "400 kB", "2000 kB"), strings.ReplaceAll(good, "100 10.00", "NaN 10.00"), strings.ReplaceAll(good, "2500 7500", "99999 7500"), strings.ReplaceAll(good, "100 0 100 800", "18446744073709551615 0 100 800")} {
		if _, e := Parse([]byte(bad)); e == nil {
			t.Fatal("malformed accepted")
		}
	}
}
