package latency

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"net"
	"time"
)

func echo(ctx context.Context, address net.IPAddr) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	network, bind, protocol := "udp4", "0.0.0.0", 1
	var request, reply icmp.Type = ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	if address.IP.To4() == nil {
		network, bind, protocol = "udp6", "::", 58
		request, reply = ipv6.ICMPTypeEchoRequest, ipv6.ICMPTypeEchoReply
	}
	conn, err := icmp.ListenPacket(network, bind)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(budget)
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return 0, err
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return 0, err
	}
	message := icmp.Message{Type: request, Body: &icmp.Echo{ID: 1, Seq: 1, Data: nonce}}
	packet, err := message.Marshal(nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	if _, err = conn.WriteTo(packet, &net.UDPAddr{IP: address.IP, Zone: address.Zone}); err != nil {
		return 0, err
	}
	var buffer [512]byte
	for {
		n, peer, e := conn.ReadFrom(buffer[:])
		if e != nil {
			return 0, e
		}
		source, ok := peer.(*net.UDPAddr)
		if !ok || !source.IP.Equal(address.IP) {
			continue
		}
		response, e := icmp.ParseMessage(protocol, buffer[:n])
		if e != nil || response.Type != reply || response.Code != 0 {
			continue
		}
		body, ok := response.Body.(*icmp.Echo)
		if ok && body.Seq == 1 && bytes.Equal(body.Data, nonce) {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			return time.Since(start), nil
		}
		if ctx.Err() != nil {
			return 0, errors.New("cancelled")
		}
	}
}
