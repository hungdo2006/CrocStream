package transport

import (
	"context"
	"io"
	"net"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func lanTestOptions(base Options) Options {
	opts := base
	opts.EnableLAN = true
	opts.LANDiscoverTimeout = 200 * time.Millisecond
	opts.lanListenAddr = "127.0.0.1:0"
	opts.lanUDPAddr = "127.0.0.1:0"
	return opts
}

func TestLANFirstWinsOverRelayAndTransfersEncryptedData(t *testing.T) {
	base := startRelay(t)
	hostOpts := lanTestOptions(base)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l, err := Host(ctx, hostOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if l.lan == nil {
		t.Fatal("expected LAN acceptor to start")
	}

	viewerOpts := lanTestOptions(base)
	viewerOpts.lanProbeTargets = []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1), Port: l.lan.udpPort}}

	var hostAuth, joinAuth atomic.Bool
	type answer struct {
		s   *Session
		err error
	}
	accepted := make(chan answer, 1)
	go func() {
		s, e := l.AcceptWithProgress(ctx, func() { hostAuth.Store(true) })
		accepted <- answer{s, e}
	}()

	v, err := JoinWithProgress(ctx, l.Invitation(), viewerOpts, func() { joinAuth.Store(true) })
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	h := <-accepted
	if h.err != nil {
		t.Fatal(h.err)
	}
	defer h.s.Close()

	if v.Route() != RouteLAN || h.s.Route() != RouteLAN {
		t.Fatalf("expected LAN route on both sides, got host=%q viewer=%q", h.s.Route(), v.Route())
	}
	if !hostAuth.Load() || !joinAuth.Load() {
		t.Fatal("expected authenticating callback on both sides")
	}

	// Verify duplex encrypted transfer over the winning LAN session.
	msg := []byte("hello-over-lan-pake-aead")
	written := make(chan error, 1)
	go func() {
		_, e := v.Write(msg)
		written <- e
	}()
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(h.s, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(msg) {
		t.Fatalf("payload mismatch: %q", string(got))
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}

	reply := []byte("ack-from-host")
	go func() {
		_, e := h.s.Write(reply)
		written <- e
	}()
	gotReply := make([]byte, len(reply))
	if _, err := io.ReadFull(v, gotReply); err != nil {
		t.Fatal(err)
	}
	if string(gotReply) != string(reply) {
		t.Fatalf("reply mismatch: %q", string(gotReply))
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
}

func TestLANFallbackToRelayWhenPeerNotOnLAN(t *testing.T) {
	base := startRelay(t)
	hostOpts := lanTestOptions(base)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l, err := Host(ctx, hostOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Pick a guaranteed unused UDP port so viewer's LAN probe times out cleanly
	// and falls back to the relay.
	unused, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadPort := unused.LocalAddr().(*net.UDPAddr).Port
	_ = unused.Close()

	viewerOpts := lanTestOptions(base)
	viewerOpts.LANDiscoverTimeout = 100 * time.Millisecond
	viewerOpts.lanProbeTargets = []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1), Port: deadPort}}

	type answer struct {
		s   *Session
		err error
	}
	accepted := make(chan answer, 1)
	go func() {
		s, e := l.Accept(ctx)
		accepted <- answer{s, e}
	}()

	v, err := Join(ctx, l.Invitation(), viewerOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	h := <-accepted
	if h.err != nil {
		t.Fatal(h.err)
	}
	defer h.s.Close()

	if v.Route() != RouteRelay || h.s.Route() != RouteRelay {
		t.Fatalf("expected relay fallback on both sides, got host=%q viewer=%q", h.s.Route(), v.Route())
	}

	msg := []byte("fallback-to-relay")
	done := make(chan error, 1)
	go func() {
		_, e := h.s.Write(msg)
		done <- e
	}()
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(v, got); err != nil || string(got) != string(msg) {
		t.Fatalf("relay transfer failed: %q, err=%v", string(got), err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestLANRejectsMalformedUDPAndUnrelatedTCPScanners(t *testing.T) {
	base := startRelay(t)
	hostOpts := lanTestOptions(base)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l, err := Host(ctx, hostOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Send malformed UDP queries and a random TCP scanner connection before the real viewer.
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	dst := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: l.lan.udpPort}
	for _, bad := range [][]byte{
		[]byte("short"),
		make([]byte, lanQuerySize),
		append([]byte("CSL1"), make([]byte, lanQuerySize-4)...),
	} {
		_, _ = pc.WriteTo(bad, dst)
	}
	_ = pc.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	buf := make([]byte, 64)
	if n, _, err := pc.ReadFrom(buf); err == nil {
		t.Fatalf("host unexpectedly replied to malformed UDP probe (%d bytes)", n)
	}

	type answer struct {
		s   *Session
		err error
	}
	accepted := make(chan answer, 1)
	go func() {
		s, e := l.Accept(ctx)
		accepted <- answer{s, e}
	}()

	// Unrelated scanner connects to Host's TCP port with wrong tag.
	scanner, err := net.DialTimeout("tcp4", l.lan.tcpLn.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = scanner.Write([]byte("GET / HTTP/1.1\r\n\r\n!!"))
	_ = scanner.Close()

	// Legitimate viewer must still connect over LAN cleanly.
	viewerOpts := lanTestOptions(base)
	viewerOpts.lanProbeTargets = []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1), Port: l.lan.udpPort}}
	v, err := Join(ctx, l.Invitation(), viewerOpts)
	if err != nil {
		t.Fatalf("legitimate viewer failed after scanner noise: %v", err)
	}
	defer v.Close()
	h := <-accepted
	if h.err != nil {
		t.Fatalf("host accept failed after scanner noise: %v", h.err)
	}
	defer h.s.Close()
	if v.Route() != RouteLAN || h.s.Route() != RouteLAN {
		t.Fatalf("expected LAN route after scanner noise, got host=%q viewer=%q", h.s.Route(), v.Route())
	}
}

func TestLANWrongPAKECodeFailsBothPeersWithoutBypass(t *testing.T) {
	base := startRelay(t)
	hostOpts := lanTestOptions(base)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l, err := Host(ctx, hostOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	done := make(chan error, 1)
	go func() {
		_, e := l.Accept(ctx)
		done <- e
	}()

	code := l.Invitation()
	at := strings.LastIndex(code, "-")
	wrong := code[:at+1] + "acid"
	if wrong == code {
		wrong = code[:at+1] + "acorn"
	}

	viewerOpts := lanTestOptions(base)
	viewerOpts.lanProbeTargets = []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1), Port: l.lan.udpPort}}
	if s, err := Join(ctx, wrong, viewerOpts); err == nil {
		s.Close()
		t.Fatal("wrong PAKE code accepted over LAN")
	}
	if err := <-done; err == nil {
		t.Fatal("host accepted wrong PAKE code over LAN")
	}
}

func TestLANRaceCancellationHasNoGoroutineLeak(t *testing.T) {
	base := startRelay(t)
	startG := runtime.NumGoroutine()
	for i := 0; i < 15; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		l, err := Host(ctx, lanTestOptions(base))
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, e := l.Accept(ctx)
			done <- e
		}()
		cancel()
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("expected canceled accept")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("LAN race accept hung on cancel")
		}
		_ = l.Close()
	}
	// Allow upstream test relay's 1s waiting-room ticker to finish.
	time.Sleep(1300 * time.Millisecond)
	if got := runtime.NumGoroutine(); got > startG+8 {
		t.Fatalf("goroutine growth after LAN race cancels: start=%d end=%d", startG, got)
	}
}
