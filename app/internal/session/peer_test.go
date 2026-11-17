package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/protocol"
	"crocstream.local/app/internal/transport"
	"github.com/schollz/croc/v11/src/tcp"
)

func testFrame(tick int) capture.Frame {
	b := make([]byte, 640*360*4)
	for i := 3; i < len(b); i += 4 {
		b[i] = 255
	}
	// Whole tile slowly changes to exercise canonical threshold accumulation.
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			p := (y*640 + x) * 4
			b[p] = byte(tick)
			b[p+1] = byte(tick)
			b[p+2] = byte(tick)
		}
	}
	return capture.Frame{RGBA: b, Width: 640, Height: 360, At: time.Now()}
}
func relay(t *testing.T) transport.Options {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- tcp.RunWithListener(ln, "private-test-password", tcp.WithCtx(ctx), tcp.WithLogLevel("error"), tcp.WithAdmissionLimits(1000, 1000, time.Minute))
	}()
	t.Cleanup(func() {
		cancel()
		ln.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("relay leaked")
		}
	})
	return transport.Options{Address: ln.Addr().String(), Password: "private-test-password"}
}
func pairWithRelay(t *testing.T, ctx context.Context) (io.ReadWriteCloser, io.ReadWriteCloser) {
	t.Helper()
	options := relay(t)
	factory := transport.Croc{}
	l, err := factory.Host(ctx, options, nil)
	if err != nil {
		t.Fatal("host", err)
	}
	t.Cleanup(func() { l.Close() })
	type accepted struct {
		conn io.ReadWriteCloser
		err  error
	}
	result := make(chan accepted, 1)
	go func() { c, e := l.Accept(ctx); result <- accepted{c, e} }()
	viewer, err := factory.Join(ctx, l.Invitation(), options, nil)
	if err != nil {
		t.Fatal("join failed")
	}
	host := <-result
	if host.err != nil {
		t.Fatal("accept failed")
	}
	t.Cleanup(func() { host.conn.Close(); viewer.Close() })
	return host.conn, viewer
}

func TestSecureMediaRoundtripRecoveryAndControls(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a, b := pairWithRelay(t, ctx)
	var mu sync.Mutex
	sent, received := map[uint32][32]byte{}, map[uint32][32]byte{}
	am, bm := NewMetrics(), NewMetrics()
	host := NewPeer(a, true, am, Hooks{Reference: func(id uint32, image []byte) { mu.Lock(); sent[id] = sha256.Sum256(image); mu.Unlock() }})
	viewer := NewPeer(b, false, bm, Hooks{Reference: func(id uint32, image []byte) { mu.Lock(); received[id] = sha256.Sum256(image); mu.Unlock() }})
	frames := make(chan capture.Frame, 1)
	done := make(chan error, 2)
	cfg := protocol.DefaultConfig()
	cfg.Width, cfg.Height, cfg.KeyInterval = 640, 360, 120
	go func() { done <- host.Run(ctx, cfg, frames) }()
	go func() { done <- viewer.Run(ctx, protocol.Config{}, nil) }()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("deadline")
		case <-ticker.C:
		}
		if i == 20 {
			viewer.RequestRecovery()
		}
		select {
		case frames <- testFrame(i):
		case <-ctx.Done():
			t.Fatal("blocked admission")
		}
	}
	// A marker PING after final video is not required for this test; wait for
	// decoder progress via bounded observation rather than guessing completion.
	wait := time.NewTicker(10 * time.Millisecond)
	defer wait.Stop()
	for {
		count, _ := bm.Read()
		if count.LastID == 100 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("last frame not decoded", count)
		case <-wait.C:
		}
	}
	close(frames)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("peer leaked")
		}
	}
	ac, _ := am.Read()
	bc, _ := bm.Read()
	if ac.Emitted != 100 || bc.Decoded == 0 || ac.PLIAccepted == 0 || bc.PLIRequested == 0 || bc.Recoveries == 0 || ac.Pongs == 0 || bc.Pongs == 0 || !bc.Ended {
		t.Fatal("missing actual media/control evidence", ac, bc)
	}
	mu.Lock()
	defer mu.Unlock()
	for id, hash := range received {
		if sent[id] != hash {
			t.Fatal("canonical mismatch", id)
		}
	}
}

func TestCancellationUnblocksSilentPeer(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	peer := NewPeer(a, true, NewMetrics(), Hooks{})
	go func() { done <- peer.Run(ctx, protocol.DefaultConfig(), make(chan capture.Frame)) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked CONFIG writer did not cancel")
	}
}

func TestPLIFloodAndPongValidation(t *testing.T) {
	p := NewPeer(nil, true, NewMetrics(), Hooks{})
	var last time.Time
	for i := 0; i < 1000; i++ {
		if err := p.control(controlPacket(protocol.RequestKeyframe, nil), &last); err != nil {
			t.Fatal(err)
		}
	}
	count, _ := p.metrics.Read()
	if count.PLIAccepted != 1 {
		t.Fatal("PLI not rate limited", count)
	}
	packet, err := p.pingPacket()
	if err != nil {
		t.Fatal(err)
	}
	wrong := bytes.Clone(packet.Payload)
	wrong[0] ^= 1
	_ = p.control(controlPacket(protocol.Pong, wrong), &last)
	count, _ = p.metrics.Read()
	if count.RTTKnown {
		t.Fatal("unmatched pong measured")
	}
	_ = p.control(controlPacket(protocol.Pong, packet.Payload), &last)
	count, _ = p.metrics.Read()
	if !count.RTTKnown || count.Pongs != 1 {
		t.Fatal("matching pong not measured")
	}
	for i := 0; i < 8; i++ {
		if err := p.control(controlPacket(protocol.Ping, make([]byte, 8)), &last); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.control(controlPacket(protocol.Ping, make([]byte, 8)), &last); !errors.Is(err, ErrControlFlood) {
		t.Fatal("unbounded controls")
	}
}
