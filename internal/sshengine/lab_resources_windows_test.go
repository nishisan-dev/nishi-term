package sshengine

import (
	"syscall"
	"unsafe"
)

func labProcessResources() (uint64, uint64) {
	type counters struct {
		CB                                                                                                     uint32
		Faults                                                                                                 uint32
		Peak, WorkingSet, QuotaPeakPaged, QuotaPaged, QuotaPeakNonpaged, QuotaNonpaged, Pagefile, PeakPagefile uintptr
	}
	var c counters
	c.CB = uint32(unsafe.Sizeof(c))
	var handles uint32
	proc := ^uintptr(0)
	syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo").Call(proc, uintptr(unsafe.Pointer(&c)), uintptr(c.CB))
	syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessHandleCount").Call(proc, uintptr(unsafe.Pointer(&handles)))
	return uint64(c.WorkingSet), uint64(handles)
}
