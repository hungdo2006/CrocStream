package transport

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/schollz/croc/v11/src/crypt"
	"github.com/schollz/croc/v11/src/tcp"
)

func TestRecordsRejectTamperingReplayAndReflection(t *testing.T) {
	key := bytes.Repeat([]byte{42}, 32)
	a, _ := directional(key, 0)
	other, _ := directional(key, 1)
	p, err := seal(a, 0, dataKind, 0, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := seal(a, 0, dataKind, 0, []byte("payload"))
	if bytes.Equal(p[:12], second[:12]) {
		t.Fatal("nonce reused")
	}
	kind, data, err := open(a, 0, 0, p)
	if err != nil || kind != dataKind || string(data) != "payload" {
		t.Fatal("roundtrip", err)
	}
	for _, check := range []func() error{
		func() error { _, _, e := open(a, 0, 1, p); return e },     // replay
		func() error { _, _, e := open(a, 1, 0, p); return e },     // role
		func() error { _, _, e := open(other, 1, 0, p); return e }, // reflection
		func() error { b := append([]byte(nil), p...); b[len(b)-1] ^= 1; _, _, e := open(a, 0, 0, b); return e },
		func() error { _, _, e := open(a, 0, 0, p[:len(p)-1]); return e },
		func() error { _, _, e := open(a, 0, 0, append(p, 0)); return e },
	} {
		if check() == nil {
			t.Fatal("accepted invalid record")
		}
	}
	// Authenticated, structurally invalid messages from a malicious peer.
	for _, mode := range []string{"empty", "kind", "length", "magic", "seq"} {
		b := make([]byte, 18)
		copy(b, "CSR1")
		b[5] = dataKind
		switch mode {
		case "kind":
			b[5] = 99
		case "length":
			binary.BigEndian.PutUint32(b[14:], maxData+1)
		case "magic":
			b[0] = 0
		case "seq":
			binary.BigEndian.PutUint64(b[6:], 2)
		}
		p, _ := crypt.EncryptAEAD(b, a)
		if _, _, err := open(a, 0, 0, p); err == nil {
			t.Fatal(mode)
		}
	}
	if _, err := seal(a, 0, dataKind, maxRecords-1, []byte{1}); err == nil {
		t.Fatal("nonce budget")
	}
	p, err = seal(a, 0, closeKind, maxRecords-1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = open(a, 0, maxRecords-1, p); err != nil {
		t.Fatal(err)
	}
	if _, err = seal(a, 0, closeKind, maxRecords, nil); err == nil {
		t.Fatal("overflow")
	}
}

func testOwner(c net.Conn) *owner {
	ctx, cancel := context.WithCancel(context.Background())
	o := &owner{conn: c, cancel: cancel}
	o.stop = context.AfterFunc(ctx, func() { c.Close() })
	return o
}
func pair(t *testing.T) (*Session, *Session) {
	t.Helper()
	a, b := net.Pipe()
	oa, ob := testOwner(a), testOwner(b)
	t.Cleanup(oa.abort)
	t.Cleanup(ob.abort)
	key := bytes.Repeat([]byte{7}, 32)
	sa, err := newSession(oa, key, 0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := newSession(ob, key, 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return sa, sb
}

func TestDuplexTinyReadsLargeWrites(t *testing.T) {
	a, b := pair(t)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for _, entry := range []struct {
		s     *Session
		value byte
	}{{a, 3}, {b, 9}} {
		wg.Add(2)
		go func(s *Session, v byte) {
			defer wg.Done()
			data := bytes.Repeat([]byte{v}, 2*maxData+17)
			n, e := s.Write(data)
			if e == nil && n != len(data) {
				e = io.ErrShortWrite
			}
			errs <- e
		}(entry.s, entry.value)
		go func(s *Session, v byte) {
			defer wg.Done()
			if v == 3 {
				v = 9
			} else {
				v = 3
			}
			for i := 0; i < 2*maxData+17; i++ {
				var p [1]byte
				_, e := io.ReadFull(s, p[:])
				if e != nil {
					errs <- e
					return
				}
				if p[0] != v {
					errs <- errors.New("wrong byte")
					return
				}
			}
			errs <- nil
		}(entry.s, entry.value)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
}

func TestCloseUnblocksBlockedWriteAndRead(t *testing.T) {
	for _, mode := range []string{"read", "write"} {
		t.Run(mode, func(t *testing.T) {
			a, _ := pair(t)
			started := make(chan struct{})
			finished := make(chan error, 1)
			go func() {
				close(started)
				if mode == "write" {
					_, e := a.Write(make([]byte, maxData))
					finished <- e
				} else {
					_, e := a.Read(make([]byte, 1))
					finished <- e
				}
			}()
			<-started
			start := time.Now()
			_ = a.Close()
			_ = a.Close()
			select {
			case e := <-finished:
				if e == nil {
					t.Fatal("expected failure")
				}
			case <-time.After(time.Second):
				t.Fatal("blocked I/O")
			}
			if time.Since(start) > time.Second {
				t.Fatal("slow close")
			}
		})
	}
}

func TestAuthenticatedCloseAndUnexpectedEOF(t *testing.T) {
	a, b := pair(t)
	go a.Close()
	if _, err := b.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	c, d := pair(t)
	c.own.abort()
	if _, err := d.Read(make([]byte, 1)); err == nil || errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestFramingBoundsAndTruncation(t *testing.T) {
	valid := append([]byte("croc"), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(valid[4:], 3)
	valid = append(valid, 1, 2, 3)
	for cut := 0; cut < len(valid); cut++ {
		a, b := net.Pipe()
		go func() { defer b.Close(); _ = writeAll(b, valid[:cut]) }()
		_, err := readFrame(a, time.Now().Add(time.Second), 8)
		a.Close()
		if err == nil {
			t.Fatalf("accepted cut %d", cut)
		}
	}
	for _, n := range []uint32{9, ^uint32(0)} {
		a, b := net.Pipe()
		go func() {
			defer b.Close()
			h := append([]byte("croc"), 0, 0, 0, 0)
			binary.LittleEndian.PutUint32(h[4:], n)
			_ = writeAll(b, h)
		}()
		_, err := readFrame(a, time.Now().Add(time.Second), 8)
		a.Close()
		if !errors.Is(err, ErrProtocol) {
			t.Fatal(err)
		}
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(b []byte) (int, error) {
	if len(b) > 3 {
		b = b[:3]
	}
	return w.Buffer.Write(b)
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }
func TestPartialWrites(t *testing.T) {
	var w shortWriter
	if err := writeAll(&w, []byte("12345678")); err != nil || w.String() != "12345678" {
		t.Fatal(err)
	}
	if err := writeAll(zeroWriter{}, []byte{1}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

func TestHandshakeAndWrongPassword(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		a, b := net.Pipe()
		oa, ob := testOwner(a), testOwner(b)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		opts := Options{HandshakeTimeout: time.Second, WaitTimeout: time.Second, IOTimeout: time.Second}
		done := make(chan error, 1)
		pw := "test"
		if wrong {
			pw = "different"
		}
		go func() {
			_, err := handshake(ctx, ob, opts, pw, "room", 1, nil)
			if err != nil {
				ob.abort()
			}
			done <- err
		}()
		_, err := handshake(ctx, oa, opts, "test", "room", 0, nil)
		if wrong && err == nil {
			t.Fatal("accepted wrong password")
		}
		if !wrong && err != nil {
			t.Fatal(err)
		}
		if peerErr := <-done; wrong == (peerErr == nil) {
			t.Fatal("unexpected peer result")
		}
		oa.abort()
		ob.abort()
		cancel()
	}
}

func TestRelayPasswordNotRetainedAfterAdmission(t *testing.T) {
	opts := startRelay(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	l, err := Host(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if l.opts.RelayPassword != "" {
		t.Fatal("listener retained relay password")
	}
	l.Close()
}

func TestHandshakeProgressOccursOnlyAtPeerAuthentication(t *testing.T) {
	opts := startRelay(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, err := Host(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var hostAuth, joinAuth atomic.Bool
	type accepted struct {
		s   *Session
		err error
	}
	done := make(chan accepted, 1)
	go func() { s, e := l.AcceptWithProgress(ctx, func() { hostAuth.Store(true) }); done <- accepted{s, e} }()
	joined, err := JoinWithProgress(ctx, l.Invitation(), opts, func() { joinAuth.Store(true) })
	if err != nil {
		t.Fatal(err)
	}
	host := <-done
	if host.err != nil {
		t.Fatal(host.err)
	}
	if !hostAuth.Load() || !joinAuth.Load() {
		t.Fatal("authenticated transition missing")
	}
	joined.Close()
	host.s.Close()
}

func startRelay(t *testing.T) Options {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- tcp.RunWithListener(ln, "test-relay-password", tcp.WithCtx(ctx), tcp.WithLogLevel("error"),
			// Only the private test relay needs a higher admission allowance for 100 cycles.
			tcp.WithAdmissionLimits(1000, 1000, time.Minute))
	}()
	t.Cleanup(func() {
		cancel()
		ln.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("relay cleanup timeout")
		}
	})
	return Options{RelayAddress: ln.Addr().String(), RelayPassword: "test-relay-password", HandshakeTimeout: 2 * time.Second, WaitTimeout: 2 * time.Second, IOTimeout: 2 * time.Second}
}

func TestRelay100Lifecycles(t *testing.T) {
	opts := startRelay(t)
	startG := runtime.NumGoroutine()
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		l, err := Host(ctx, opts)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		type answer struct {
			s   *Session
			err error
		}
		accepted := make(chan answer, 1)
		go func() { s, e := l.Accept(ctx); accepted <- answer{s, e} }()
		v, err := Join(ctx, l.Invitation(), opts)
		if err != nil {
			cancel()
			l.Close()
			t.Fatal(err)
		}
		h := <-accepted
		if h.err != nil {
			t.Fatal(h.err)
		}
		written := make(chan error, 1)
		go func() { _, e := v.Write([]byte("duplex")); written <- e }()
		b := make([]byte, 6)
		_, err = io.ReadFull(h.s, b)
		if err != nil || string(b) != "duplex" {
			t.Fatal(err)
		}
		if e := <-written; e != nil {
			t.Fatal(e)
		}
		// Context cancellation closes both directions, including blocked reads.
		done := make(chan error, 1)
		go func() { _, e := v.Read(b); done <- e }()
		cancel()
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("missing cancel error")
			}
		case <-time.After(time.Second):
			t.Fatal("cancel hung")
		}
		_ = l.Close()
		v.Close()
		h.s.Close()
		if l.Invitation() != "" {
			t.Fatal("stale invitation")
		}
		if _, e := l.Accept(ctx); e == nil {
			t.Fatal("second Accept")
		}
	}
	// Allow upstream relay workers' bounded waiting-room ticker to finish.
	time.Sleep(1200 * time.Millisecond)
	if got := runtime.NumGoroutine(); got > startG+12 {
		t.Fatalf("goroutine growth start=%d end=%d", startG, got)
	} else {
		t.Logf("100 cycles; goroutines start=%d end=%d (private relay still active)", startG, got)
	}
}

func TestRelayAcceptCancellationAndWrongCode(t *testing.T) {
	opts := startRelay(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, err := Host(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	wait, cancelWait := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, e := l.Accept(wait); done <- e }()
	cancelWait()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("expected canceled accept")
		}
	case <-time.After(time.Second):
		t.Fatal("accept hung")
	}
	l2, err := Host(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	go func() { _, e := l2.Accept(ctx); done <- e }()
	code := l2.Invitation()
	// Replace final PAKE word without changing first two room words.
	at := strings.LastIndex(code, "-")
	code = code[:at+1] + "acid"
	if code == l2.Invitation() {
		code = code[:at+1] + "acorn"
	}
	_, err = Join(ctx, code, opts)
	if err == nil {
		t.Fatal("wrong code accepted")
	}
	if e := <-done; e == nil {
		t.Fatal("wrong code accepted by host")
	}
}

func FuzzRecord(f *testing.F) {
	a, _ := directional(bytes.Repeat([]byte{1}, 32), 0)
	valid, _ := seal(a, 0, dataKind, 0, []byte{1})
	f.Add(valid)
	f.Fuzz(func(t *testing.T, b []byte) { _, _, _ = open(a, 0, 0, b) })
}

func FuzzAuthenticatedPlaintext(f *testing.F) {
	a, _ := directional(bytes.Repeat([]byte{1}, 32), 0)
	f.Add([]byte("CSR1"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 18+maxData {
			return
		}
		p, err := crypt.EncryptAEAD(b, a)
		if err != nil {
			t.Fatal(err)
		}
		_, _, _ = open(a, 0, 0, p)
	})
}

func TestConcurrentWritersAreSerialized(t *testing.T) {
	a, b := pair(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(v byte) { defer wg.Done(); _, e := a.Write(bytes.Repeat([]byte{v}, 1024)); errs <- e }(byte(i))
	}
	seen := map[byte]bool{}
	for i := 0; i < 8; i++ {
		buf := make([]byte, 1024)
		if _, err := io.ReadFull(b, buf); err != nil {
			t.Fatal(err)
		}
		if seen[buf[0]] || !bytes.Equal(buf, bytes.Repeat(buf[:1], 1024)) {
			t.Fatal("interleaved writer")
		}
		seen[buf[0]] = true
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHandshakeKeepaliveCannotExtendDeadline(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if writeFrame(b, time.Now().Add(time.Second), []byte{1}) != nil {
				return
			}
		}
	}()
	start := time.Now()
	if _, err := recvHS(a, start.Add(50*time.Millisecond), 1, true); err == nil {
		t.Fatal("keepalive extended handshake")
	}
	if time.Since(start) > time.Second {
		t.Fatal("deadline missed")
	}
	a.Close()
	b.Close()
	<-done
}

func TestCrossSessionCiphertextAndMalformedHandshake(t *testing.T) {
	a, _ := directional(bytes.Repeat([]byte{1}, 32), 0)
	b, _ := directional(bytes.Repeat([]byte{2}, 32), 0)
	p, _ := seal(a, 0, dataKind, 0, []byte("one"))
	if _, _, err := open(b, 0, 0, p); err == nil {
		t.Fatal("session isolation failed")
	}
	for _, bad := range [][]byte{nil, []byte("CSH2"), append([]byte("CSH1"), 99)} {
		l, r := net.Pipe()
		go func() { defer r.Close(); _ = writeFrame(r, time.Now().Add(time.Second), bad) }()
		if _, err := recvHS(l, time.Now().Add(time.Second), 1, false); err == nil {
			t.Fatal("invalid handshake")
		}
		l.Close()
	}
}

func TestZeroLengthAndTerminalAfterCorruption(t *testing.T) {
	a, b := pair(t)
	if n, e := a.Write(nil); n != 0 || e != nil {
		t.Fatal(n, e)
	}
	if n, e := b.Read(nil); n != 0 || e != nil {
		t.Fatal(n, e)
	}
	go func() {
		payload, _ := seal(a.send, 0, dataKind, 0, []byte("secret"))
		payload[len(payload)-1] ^= 1
		_ = writeFrame(a.own.conn, time.Now().Add(time.Second), payload)
	}()
	buf := bytes.Repeat([]byte{0xaa}, 6)
	if n, e := b.Read(buf); n != 0 || e == nil {
		t.Fatal("bad record returned data")
	}
	if !bytes.Equal(buf, bytes.Repeat([]byte{0xaa}, 6)) {
		t.Fatal("caller buffer mutated")
	}
	if _, e := b.Write([]byte("after failure")); e == nil {
		t.Fatal("session continued after auth error")
	}
}

func TestCancellationWhileWaitingForPeer(t *testing.T) {
	opts := startRelay(t)
	ctx, cancel := context.WithCancel(context.Background())
	l, err := Host(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	invite := l.Invitation()
	l.Close()
	// Join a now empty room with a timeout; server keepalives cannot extend it.
	cancel()
	joinCtx, done := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer done()
	start := time.Now()
	s, err := Join(joinCtx, invite, opts)
	if err == nil {
		if s != nil {
			s.Close()
		}
		t.Fatal("joined absent peer")
	}
	if time.Since(start) > time.Second {
		t.Fatal("join cancellation blocked")
	}
}

func TestIdleReadTimeoutAndDuplicateHandshake(t *testing.T) {
	a, _ := pair(t)
	a.timeout = 50 * time.Millisecond
	start := time.Now()
	if _, err := a.Read(make([]byte, 1)); err == nil {
		t.Fatal("idle read never timed out")
	}
	if time.Since(start) > time.Second {
		t.Fatal("idle read too slow")
	}
	l, r := net.Pipe()
	defer l.Close()
	defer r.Close()
	go func() { _ = sendHS(r, time.Now().Add(time.Second), 1, []byte{0, 1, 0}) }()
	if _, err := recvHS(l, time.Now().Add(time.Second), 3, false); err == nil {
		t.Fatal("duplicate INIT accepted in CONFIRM phase")
	}
}

func FuzzHandshakeBody(f *testing.F) {
	b, _ := packPAKE([]byte{1, 2, 3}, make([]byte, 32))
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) { _, _, _ = unpackPAKE(b, 32) })
}
