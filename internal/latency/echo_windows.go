package latency

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"golang.org/x/sys/windows"
	"net"
	"runtime"
	"strconv"
	"time"
	"unsafe"
)

var helper = windows.NewLazySystemDLL("iphlpapi.dll")
var create4 = helper.NewProc("IcmpCreateFile")
var create6 = helper.NewProc("Icmp6CreateFile")
var closeICMP = helper.NewProc("IcmpCloseHandle")
var send4 = helper.NewProc("IcmpSendEcho")
var send6 = helper.NewProc("Icmp6SendEcho2")

type sockaddr6 struct {
	Family, Port uint16
	Flow         uint32
	Address      [16]byte
	Scope        uint32
}

func echo(ctx context.Context, address net.IPAddr) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(budget)
	}
	timeout := time.Until(deadline).Milliseconds()
	if timeout < 1 {
		return 0, context.DeadlineExceeded
	}
	ipv4 := address.IP.To4()
	create := create4
	if ipv4 == nil {
		create = create6
	}
	handle, _, err := create.Call()
	if handle == 0 || handle == ^uintptr(0) {
		return 0, err
	}
	defer closeICMP.Call(handle)
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return 0, e
	}
	// Aligned, bounded buffer; only documented fixed reply-header fields are read.
	var storage [128]uint64
	reply := unsafe.Slice((*byte)(unsafe.Pointer(&storage[0])), 1024)
	var count uintptr
	statusOffset, rttOffset := 4, 8
	if ipv4 != nil {
		count, _, err = send4.Call(handle, uintptr(binary.LittleEndian.Uint32(ipv4)), uintptr(unsafe.Pointer(&nonce[0])), uintptr(len(nonce)), 0, uintptr(unsafe.Pointer(&storage[0])), uintptr(len(reply)), uintptr(timeout))
	} else {
		src, dst := sockaddr6{Family: windows.AF_INET6}, sockaddr6{Family: windows.AF_INET6}
		copy(dst.Address[:], address.IP.To16())
		if address.Zone != "" {
			if n, e := strconv.ParseUint(address.Zone, 10, 32); e == nil {
				dst.Scope = uint32(n)
			} else if device, e := net.InterfaceByName(address.Zone); e == nil {
				dst.Scope = uint32(device.Index)
			} else {
				return 0, e
			}
		}
		count, _, err = send6.Call(handle, 0, 0, 0, uintptr(unsafe.Pointer(&src)), uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&nonce[0])), uintptr(len(nonce)), 0, uintptr(unsafe.Pointer(&storage[0])), uintptr(len(reply)), uintptr(timeout))
		runtime.KeepAlive(src)
		runtime.KeepAlive(dst)
		statusOffset, rttOffset = 28, 32
	}
	runtime.KeepAlive(nonce)
	runtime.KeepAlive(storage)
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if count == 0 {
		return 0, errors.New("no echo reply")
	}
	if binary.LittleEndian.Uint32(reply[statusOffset:]) != 0 {
		return 0, errors.New("no echo reply")
	}
	return time.Duration(binary.LittleEndian.Uint32(reply[rttOffset:])) * time.Millisecond, nil
}
