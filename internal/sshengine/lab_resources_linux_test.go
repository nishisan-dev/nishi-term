package sshengine

import (
	"os"
	"strconv"
	"strings"
)

func labProcessResources() (uint64, uint64) {
	b, _ := os.ReadFile("/proc/self/statm")
	f := strings.Fields(string(b))
	var rss uint64
	if len(f) > 1 {
		rss, _ = strconv.ParseUint(f[1], 10, 64)
		rss *= uint64(os.Getpagesize())
	}
	fds, _ := os.ReadDir("/proc/self/fd")
	return rss, uint64(len(fds))
}
