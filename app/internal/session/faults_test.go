package session

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/delta"
	"crocstream.local/app/internal/protocol"
)

func writeConfig(t *testing.T, w *protocol.Writer, cfg protocol.Config) {
	t.Helper()
	b, err := protocol.EncodeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WritePacket(controlPacket(protocol.SessionConfig, b[:])); err != nil {
		t.Fatal(err)
	}
}

type packetSink func(protocol.Packet) error

func (f packetSink) WritePacket(p protocol.Packet) error { return f(p) }

func TestCorruptDeltaRetainsPixelsAndRequestsBoundedRecovery(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	metrics := NewMetrics()
	var mu sync.Mutex
	images := map[uint32][]byte{}
	viewer := NewPeer(b, false, metrics, Hooks{Reference: func(id uint32, rgba []byte) { mu.Lock(); images[id] = bytes.Clone(rgba); mu.Unlock() }})
	done := make(chan error, 1)
	go func() { done <- viewer.Run(ctx, protocol.Config{}, nil) }()
	w, _ := protocol.NewWriter(a, protocol.FromBroadcaster)
	r, _ := protocol.NewReader(a, protocol.FromViewer)
	cfg := protocol.DefaultConfig()
	cfg.Width, cfg.Height = 640, 360
	writeConfig(t, w, cfg)
	encoder, err := delta.NewEncoder(cfg, 4)
	if err != nil {
		t.Fatal(err)
	}
	var rawPacket protocol.Packet
	sink := packetSink(func(p protocol.Packet) error { rawPacket = p; return nil })
	if _, err = encoder.Emit(ctx, sink, testFrame(0).RGBA, 0); err != nil {
		t.Fatal(err)
	}
	if err = w.WritePacket(rawPacket); err != nil {
		t.Fatal(err)
	}
	if _, err = encoder.Emit(ctx, sink, testFrame(1).RGBA, 0); err != nil {
		t.Fatal(err)
	}
	if rawPacket.Header.Type != protocol.Delta {
		t.Fatal("expected delta")
	}
	rawPacket.Payload[len(rawPacket.Payload)-1] ^= 1 // Corrupt compressed content checksum.
	if err = w.WritePacket(rawPacket); err != nil {
		t.Fatal(err)
	}
	packet, err := r.ReadPacket()
	if err != nil || packet.Header.Type != protocol.RequestKeyframe {
		t.Fatal("no immediate recovery request", err)
	}
	// A burst of invalid-reference deltas must not create a PLI storm.
	for i := 0; i < 10; i++ {
		rawPacket.Header.FrameID++
		if err = w.WritePacket(rawPacket); err != nil {
			t.Fatal(err)
		}
	}
	encoder.RequestKeyframe()
	if _, err = encoder.Emit(ctx, sink, testFrame(9).RGBA, 0); err != nil {
		t.Fatal(err)
	}
	rawPacket.Header.FrameID = 13 // Independent KEY resync across the gap.
	if err = w.WritePacket(rawPacket); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { c, _ := metrics.Read(); return c.LastID == 13 })
	if err = w.WritePacket(controlPacket(protocol.End, nil)); err != nil {
		t.Fatal(err)
	}
	// Drain final END. No video is sent on the reverse direction.
	packet, err = r.ReadPacket()
	if err != nil || packet.Header.Type != protocol.End {
		t.Fatal("missing viewer END", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("viewer did not exit")
	}
	count, window := metrics.Read()
	if count.Received != 13 || count.Decoded != 2 || count.PLIRequested != 1 || count.Recoveries != 1 || window.Received.Frames != 13 {
		t.Fatal("corrupt/discarded packet accounting", count)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(images) != 2 || !bytes.Equal(images[1], testFrame(0).RGBA) || !bytes.Equal(images[13], testFrame(9).RGBA) {
		t.Fatal("corruption mutated last good output")
	}
}

func TestMalformedMediaDisconnectsAndNoReferenceLeaksAcrossSessions(t *testing.T) {
	for _, wire := range [][]byte{
		{0xC5, 0x77, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0xC5, byte(protocol.Keyframe), 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x80, 0, 1},
		{0xC5},
	} {
		a, b := net.Pipe()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		p := NewPeer(b, false, NewMetrics(), Hooks{})
		done := make(chan error, 1)
		go func() { done <- p.Run(ctx, protocol.Config{}, nil) }()
		_, _ = a.Write(wire)
		a.Close()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("malformed frame accepted")
			}
		case <-ctx.Done():
			t.Fatal("malformed input hung")
		}
		cancel()
		b.Close()
	}
}

type instrumentedConn struct {
	io.ReadWriteCloser
	busy        atomic.Int32
	interleaved atomic.Bool
	written     atomic.Int64
	blockAfter  int64
	entered     chan struct{}
	once        sync.Once
	done        chan struct{}
	closeOnce   sync.Once
}

func (c *instrumentedConn) Write(b []byte) (int, error) {
	if c.busy.Add(1) != 1 {
		c.interleaved.Store(true)
	}
	defer c.busy.Add(-1)
	if c.written.Load() >= c.blockAfter {
		c.once.Do(func() { close(c.entered) })
		<-c.done
		return 0, io.ErrClosedPipe
	}
	n, err := c.ReadWriteCloser.Write(b)
	c.written.Add(int64(n))
	return n, err
}
func (c *instrumentedConn) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	return c.ReadWriteCloser.Close()
}
func TestRemoteEndUnblocksInFlightWriteAndPreservesSingleWriter(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	c := &instrumentedConn{ReadWriteCloser: a, blockAfter: protocol.HeaderSize + protocol.ConfigSize,
		entered: make(chan struct{}), done: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	frames := make(chan capture.Frame, 1)
	frames <- testFrame(1)
	p := NewPeer(c, true, NewMetrics(), Hooks{})
	done := make(chan error, 1)
	cfg := protocol.DefaultConfig()
	cfg.Width, cfg.Height = 640, 360
	go func() { done <- p.Run(ctx, cfg, frames) }()
	r, _ := protocol.NewReader(b, protocol.FromBroadcaster)
	if packet, err := r.ReadPacket(); err != nil || packet.Header.Type != protocol.SessionConfig {
		t.Fatal(err)
	}
	select {
	case <-c.entered:
	case <-ctx.Done():
		t.Fatal("video write did not block")
	}
	w, _ := protocol.NewWriter(b, protocol.FromViewer)
	if err := w.WritePacket(controlPacket(protocol.End, nil)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("remote END did not finish cleanly", err)
		}
	case <-ctx.Done():
		t.Fatal("remote END did not cancel blocked video")
	}
	if c.interleaved.Load() {
		t.Fatal("more than one writer used the stream")
	}
}

type failingReaderConn struct {
	readErr      error
	closeStarted chan struct{}
	allowClose   chan struct{}
	writeStarted chan struct{}
	closeCount   atomic.Int32
	writeOnce    sync.Once
}

func (c *failingReaderConn) Read([]byte) (int, error) { return 0, c.readErr }
func (c *failingReaderConn) Write(p []byte) (int, error) {
	c.writeOnce.Do(func() { close(c.writeStarted) })
	return len(p), nil
}
func (c *failingReaderConn) Close() error {
	if c.closeCount.Add(1) == 1 {
		close(c.closeStarted)
		<-c.allowClose
	}
	return nil
}

func TestPeerWaitsForReceiveCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn := &failingReaderConn{readErr: errors.New("receive failed"), closeStarted: make(chan struct{}),
		allowClose: make(chan struct{}), writeStarted: make(chan struct{})}
	release := sync.OnceFunc(func() { close(conn.allowClose) })
	defer release()
	p := NewPeer(conn, false, NewMetrics(), Hooks{State: func(State) {
		// Ensure the receive worker starts transport cleanup before the writer exits.
		select {
		case <-conn.closeStarted:
		case <-ctx.Done():
		}
	}})
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, protocol.Config{}, nil) }()
	select {
	case <-conn.writeStarted:
	case <-ctx.Done():
		t.Fatal("writer did not attempt END")
	}
	select {
	case err := <-done:
		t.Fatalf("Run returned before transport cleanup finished: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		if !errors.Is(err, conn.readErr) || conn.closeCount.Load() != 1 {
			t.Fatalf("Run returned %v, Close called %d times", err, conn.closeCount.Load())
		}
	case <-ctx.Done():
		t.Fatal("Run did not join the receiver")
	}
}

func TestSlowUIStillDecodesAllDeltas(t *testing.T) {
	a, b := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer a.Close()
	defer b.Close()
	frames := make(chan capture.Frame, 1)
	images := make(chan capture.Frame, 1)
	metrics := NewMetrics()
	viewer := NewPeer(b, false, metrics, Hooks{Image: func(f capture.Frame) { latest(images, f) }})
	host := NewPeer(a, true, NewMetrics(), Hooks{})
	done := make(chan error, 2)
	cfg := protocol.DefaultConfig()
	cfg.Width, cfg.Height = 640, 360
	go func() { done <- viewer.Run(ctx, protocol.Config{}, nil) }()
	go func() { done <- host.Run(ctx, cfg, frames) }()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; i < 50; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("sender timed out")
		case <-ticker.C:
		}
		latest(frames, testFrame(i)) // Source admission may drop only before encoding.
	}
	close(frames)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("decode stalled")
		}
	}
	count, _ := metrics.Read()
	hostCount, _ := host.metrics.Read()
	if count.Decoded != hostCount.Emitted || count.Decoded == 0 || len(images) != 1 || count.LastID != hostCount.LastID {
		t.Fatal("UI drop also dropped codec state", count)
	}
}
