package protocol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func configPacket(t testing.TB) Packet {
	t.Helper()
	b, err := EncodeConfig(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return Packet{Header{Type: SessionConfig, PayloadLength: ConfigSize}, b[:]}
}

// Synthetic bytes exercise framing only, not successful LZ4 decompression.
func video(typ Type, id uint32) Packet {
	return Packet{Header{typ, id, 123456789, 5}, []byte{1, 2, 3, 4, 5}}
}

func control(typ Type) Packet {
	p := Packet{Header: Header{Type: typ}}
	if typ == Ping || typ == Pong {
		p.Payload = []byte{0, 0xFF, 2, 3, 4, 5, 6, 7}
		p.Header.PayloadLength = 8
	}
	return p
}

func wire(t testing.TB, packets ...Packet) []byte {
	t.Helper()
	var data []byte
	for _, p := range packets {
		b, err := EncodeHeader(p.Header)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, b[:]...)
		data = append(data, p.Payload...)
	}
	return data
}

func reader(t testing.TB, input io.Reader, d Direction) *Reader {
	t.Helper()
	r, err := NewReader(input, d)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func writer(t testing.TB, output io.Writer, d Direction) *Writer {
	t.Helper()
	w, err := NewWriter(output, d)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func readOK(t testing.TB, r *Reader) Packet {
	t.Helper()
	p, err := r.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func resolve(t testing.TB, r *Reader, valid bool) {
	t.Helper()
	if err := r.ResolveVideo(valid); err != nil {
		t.Fatal(err)
	}
}

func TestDecodePacket(t *testing.T) {
	for _, p := range []Packet{configPacket(t), video(Keyframe, 1), video(Delta, 2), control(Ping), control(Pong), control(End), control(RequestKeyframe)} {
		d := FromBroadcaster
		if p.Header.Type == RequestKeyframe {
			d = FromViewer
		}
		b := wire(t, p)
		got, err := DecodePacket(b, d)
		if err != nil || got.Header != p.Header || !bytes.Equal(got.Payload, p.Payload) {
			t.Fatal(err)
		}
		for n := 0; n < len(b); n++ {
			if _, err := DecodePacket(b[:n], d); err == nil {
				t.Fatalf("accepted prefix %d of type %x", n, p.Header.Type)
			}
		}
		if _, err := DecodePacket(append(b, 0), d); !errors.Is(err, ErrLength) {
			t.Fatal(err)
		}
	}
}

type tinyReader struct {
	r io.Reader
	n int
}

func (r *tinyReader) Read(b []byte) (int, error) {
	r.n = r.n%3 + 1
	return r.r.Read(b[:min(len(b), r.n)])
}

type tinyWriter struct {
	w io.Writer
	n int
}

func (w *tinyWriter) Write(b []byte) (int, error) {
	w.n = w.n%3 + 1
	return w.w.Write(b[:min(len(b), w.n)])
}

func TestFragmentedPipeRoundtrip(t *testing.T) {
	for _, d := range []Direction{FromBroadcaster, FromViewer} {
		packets := []Packet{control(RequestKeyframe), control(Ping), control(Pong), control(End)}
		if d == FromBroadcaster {
			packets = []Packet{configPacket(t), video(Keyframe, 1), control(Ping), video(Delta, 2), video(Keyframe, 9), video(Delta, 10), control(Pong), control(End)}
		}
		pr, pw := io.Pipe()
		t.Cleanup(func() { pr.Close(); pw.Close() })
		w := writer(t, &tinyWriter{w: pw}, d)
		r := reader(t, &tinyReader{r: pr}, d)
		done := make(chan error, 1)
		go func() {
			defer pw.Close()
			for _, p := range packets {
				if err := w.WritePacket(p); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
		for _, want := range packets {
			got := readOK(t, r)
			if got.Header != want.Header || !bytes.Equal(got.Payload, want.Payload) {
				t.Fatal("fragmented roundtrip mismatch")
			}
			if got.Header.Type.video() {
				resolve(t, r, true) // Mock codec acceptance, not an LZ4 test.
			}
		}
		if _, err := r.ReadPacket(); err != io.EOF {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if err := w.WritePacket(control(Ping)); !errors.Is(err, ErrEnded) {
			t.Fatal(err)
		}
	}
}

func TestEveryTruncationIsTerminal(t *testing.T) {
	cfg := wire(t, configPacket(t))
	for _, packet := range []Packet{configPacket(t), video(Keyframe, 1), control(Ping), control(End)} {
		b := wire(t, packet)
		for n := 0; n < len(b); n++ {
			prefix := []byte(nil)
			if packet.Header.Type != SessionConfig {
				prefix = bytes.Clone(cfg)
			}
			r := reader(t, bytes.NewReader(append(prefix, b[:n]...)), FromBroadcaster)
			if prefix != nil {
				readOK(t, r)
			}
			_, err := r.ReadPacket()
			want := io.ErrUnexpectedEOF
			if n == 0 {
				want = io.EOF
			}
			if !errors.Is(err, want) {
				t.Fatalf("type %x prefix %d: %v", packet.Header.Type, n, err)
			}
			if _, again := r.ReadPacket(); again != err {
				t.Fatal("read resumed after terminal error")
			}
		}
	}
}

type observedReader struct {
	r          *bytes.Reader
	maxRequest int
}

func (r *observedReader) Read(b []byte) (int, error) {
	r.maxRequest = max(r.maxRequest, len(b))
	return r.r.Read(b)
}

func TestHeaderRejectedBeforePayloadRead(t *testing.T) {
	cfg := wire(t, configPacket(t))
	tests := []struct {
		name   string
		d      Direction
		prefix []byte
		p      Packet
		want   error
	}{
		{"beforeConfig", FromBroadcaster, nil, video(Keyframe, 1), ErrConfigRequired},
		{"pingBeforeConfig", FromBroadcaster, nil, control(Ping), ErrConfigRequired},
		{"wrongDirection", FromViewer, nil, video(Keyframe, 1), ErrDirection},
		{"pliDirection", FromBroadcaster, cfg, control(RequestKeyframe), ErrDirection},
		{"duplicateConfig", FromBroadcaster, cfg, configPacket(t), ErrDuplicateConfig},
		{"firstDelta", FromBroadcaster, cfg, video(Delta, 1), ErrSequence},
		{"firstKeyID2", FromBroadcaster, cfg, video(Keyframe, 2), ErrSequence},
		{"controlID", FromViewer, nil, Packet{Header: Header{Type: End, FrameID: 1}}, ErrHeader},
		{"controlTimestamp", FromViewer, nil, Packet{Header: Header{Type: End, TimestampMs: -1}}, ErrHeader},
		{"pliPayload", FromViewer, nil, Packet{Header: Header{Type: RequestKeyframe, PayloadLength: 1}}, ErrLength},
		{"configLength", FromBroadcaster, nil, Packet{Header: Header{Type: SessionConfig, PayloadLength: 12}}, ErrLength},
		{"pingLength", FromViewer, nil, Packet{Header: Header{Type: Ping, PayloadLength: 9}}, ErrLength},
		{"emptyVideo", FromBroadcaster, cfg, Packet{Header: Header{Type: Keyframe, FrameID: 1}}, ErrLength},
		{"zeroVideoID", FromBroadcaster, cfg, video(Keyframe, 0), ErrHeader},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := append(bytes.Clone(tt.prefix), wire(t, tt.p)...)
			b = append(b, wire(t, configPacket(t))...) // Must not scan into this.
			observed := &observedReader{r: bytes.NewReader(b)}
			r := reader(t, observed, tt.d)
			if len(tt.prefix) != 0 {
				readOK(t, r)
			}
			_, err := r.ReadPacket()
			if !errors.Is(err, tt.want) {
				t.Fatal(err)
			}
			consumed := len(b) - observed.r.Len()
			if consumed != len(tt.prefix)+HeaderSize || observed.maxRequest > HeaderSize {
				t.Fatal("read payload before rejecting header/state")
			}
			if _, again := r.ReadPacket(); again != err {
				t.Fatal("terminal parser resumed")
			}
		})
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[0] = 0 },
		func(b []byte) { b[1] = 7 },
		func(b []byte) { b[14], b[15], b[16], b[17] = 0, 0x80, 0, 1 },
		func(b []byte) { b[14], b[15], b[16], b[17] = 0xFF, 0xFF, 0xFF, 0xFF },
	} {
		b := wire(t, configPacket(t))
		mutate(b)
		o := &observedReader{r: bytes.NewReader(b)}
		r := reader(t, o, FromBroadcaster)
		if _, err := r.ReadPacket(); err == nil {
			t.Fatal("accepted malformed envelope")
		}
		if len(b)-o.r.Len() != HeaderSize || o.maxRequest > HeaderSize {
			t.Fatal("read beyond malformed envelope")
		}
	}
}

func TestConfigurationRejectedAndImmutable(t *testing.T) {
	p := configPacket(t)
	p.Payload[0] = 2
	r := reader(t, bytes.NewReader(wire(t, p, configPacket(t))), FromBroadcaster)
	if _, err := r.ReadPacket(); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
	if _, ok := r.Config(); ok {
		t.Fatal("stored unsupported config")
	}
	r = reader(t, bytes.NewReader(wire(t, configPacket(t))), FromBroadcaster)
	p = readOK(t, r)
	p.Payload[0] = 99
	c, ok := r.Config()
	if !ok || c != DefaultConfig() {
		t.Fatal("CONFIG aliases caller buffer")
	}
}

func TestVideoCommitAndRecovery(t *testing.T) {
	packets := []Packet{
		configPacket(t), video(Keyframe, 1), video(Delta, 2),
		video(Delta, 3), video(Keyframe, 5), video(Delta, 7),
		video(Delta, 6), video(Keyframe, 5), video(Keyframe, 10),
		video(Delta, 11), control(End),
	}
	r := reader(t, bytes.NewReader(wire(t, packets...)), FromBroadcaster)
	readOK(t, r)
	readOK(t, r) // KEY1 pending.
	if r.state.lastVideo != 0 {
		t.Fatal("ID committed before codec validation")
	}
	if _, err := r.ReadPacket(); !errors.Is(err, ErrPendingVideo) {
		t.Fatal(err)
	}
	resolve(t, r, true)
	if err := r.ResolveVideo(true); !errors.Is(err, ErrNoPendingVideo) {
		t.Fatal(err)
	}
	readOK(t, r) // DELTA2 body rejected by a mock codec.
	resolve(t, r, false)
	if r.state.lastVideo != 1 || r.state.reference {
		t.Fatal("failed video mutated last good ID/reference")
	}
	if _, err := r.ReadPacket(); !errors.Is(err, ErrNeedKeyframe) { // DELTA3
		t.Fatal(err)
	}
	readOK(t, r) // KEY5 can jump forward.
	resolve(t, r, true)
	for i := 0; i < 3; i++ { // Gap, sequential DELTA while invalid, replayed KEY.
		if _, err := r.ReadPacket(); !errors.Is(err, ErrNeedKeyframe) {
			t.Fatal(i, err)
		}
		if r.state.lastVideo != 5 || r.state.reference {
			t.Fatal("rejected packet changed reference progress")
		}
	}
	readOK(t, r) // KEY10
	resolve(t, r, true)
	readOK(t, r) // DELTA11
	resolve(t, r, true)
	if r.state.lastVideo != 11 {
		t.Fatal("did not recover")
	}
	readOK(t, r)
	if _, err := r.ReadPacket(); err != io.EOF {
		t.Fatal(err)
	}
}

func TestRejectedInitialKeyAndExhaustion(t *testing.T) {
	maxID := ^uint32(0)
	r := reader(t, bytes.NewReader(wire(t, configPacket(t), video(Keyframe, 1), video(Keyframe, 2), video(Keyframe, maxID), video(Keyframe, 1))), FromBroadcaster)
	readOK(t, r)
	readOK(t, r)
	resolve(t, r, false)
	readOK(t, r) // First corrupt KEY must not trap receiver waiting for id1.
	resolve(t, r, true)
	readOK(t, r)
	resolve(t, r, true)
	if _, err := r.ReadPacket(); !errors.Is(err, ErrFrameIDExhausted) {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := writer(t, &out, FromBroadcaster)
	for _, p := range []Packet{configPacket(t), video(Keyframe, 1), video(Keyframe, maxID)} {
		if err := w.WritePacket(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []Packet{video(Delta, 0), video(Keyframe, 1)} {
		if err := w.WritePacket(p); err == nil {
			t.Fatal("writer wrapped frame IDs")
		}
	}
	if err := w.WritePacket(control(End)); err != nil {
		t.Fatal(err)
	}
	// Reconnect requires fresh CONFIG and KEY1.
	fresh := reader(t, bytes.NewReader(wire(t, video(Keyframe, 1))), FromBroadcaster)
	if _, err := fresh.ReadPacket(); !errors.Is(err, ErrConfigRequired) {
		t.Fatal(err)
	}
}

func TestReplayAfterRejectedVideo(t *testing.T) {
	r := reader(t, bytes.NewReader(wire(t,
		configPacket(t), video(Keyframe, 1), video(Keyframe, 1),
		video(Keyframe, 4), video(Keyframe, 4), video(Keyframe, 3),
		video(Keyframe, 5), video(Delta, 7), video(Keyframe, 6),
		video(Keyframe, 8),
	)), FromBroadcaster)
	readOK(t, r)
	readOK(t, r)
	resolve(t, r, false) // Corrupt initial KEY still consumes wire id1.
	if _, err := r.ReadPacket(); !errors.Is(err, ErrNeedKeyframe) {
		t.Fatal("replayed corrupt initial KEY accepted", err)
	}
	readOK(t, r)
	resolve(t, r, false) // Corrupt KEY4 must not make id4 or id3 reusable.
	for i := 0; i < 2; i++ {
		if _, err := r.ReadPacket(); !errors.Is(err, ErrNeedKeyframe) {
			t.Fatal("replayed/out-of-order rejected KEY accepted", err)
		}
	}
	if r.state.lastVideo != 0 || r.state.lastSeen != 4 {
		t.Fatal("wire and canonical progress were conflated")
	}
	readOK(t, r)
	resolve(t, r, true)
	for i := 0; i < 2; i++ { // Gap DELTA7 consumes id7; subsequent KEY6 is stale.
		if _, err := r.ReadPacket(); !errors.Is(err, ErrNeedKeyframe) {
			t.Fatal("non-increasing wire ID accepted", err)
		}
	}
	if r.state.lastVideo != 5 || r.state.lastSeen != 7 {
		t.Fatal("discard changed canonical ID or lost wire high-water mark")
	}
	readOK(t, r)
	resolve(t, r, true)
	if r.state.lastVideo != 8 || !r.state.reference {
		t.Fatal("fresh KEY did not recover")
	}
}

func TestRecoveryDrainBoundedAndTruncation(t *testing.T) {
	for _, size := range []int{0, 4095, 4096, 4097, 9000} {
		p := video(Delta, 3)
		p.Header.PayloadLength = 9000
		p.Payload = make([]byte, size)
		b := wire(t, configPacket(t), video(Keyframe, 1), p)
		if size == 9000 {
			b = append(b, wire(t, video(Keyframe, 4))...)
		}
		o := &observedReader{r: bytes.NewReader(b)}
		r := reader(t, o, FromBroadcaster)
		readOK(t, r)
		readOK(t, r)
		resolve(t, r, true)
		_, err := r.ReadPacket()
		want := io.ErrUnexpectedEOF
		if size == 9000 {
			want = ErrNeedKeyframe
		}
		if !errors.Is(err, want) || o.maxRequest > 4096 {
			t.Fatal(size, err, o.maxRequest)
		}
		if size == 9000 {
			readOK(t, r)
			resolve(t, r, true)
		} else if _, again := r.ReadPacket(); again != err {
			t.Fatal("truncated discard did not terminate")
		}
	}
}

func TestMaxPayloadAndBufferOwnership(t *testing.T) {
	p := video(Keyframe, 1)
	p.Header.PayloadLength = MaxPayload
	p.Payload = bytes.Repeat([]byte{0xA7}, MaxPayload)
	r := reader(t, bytes.NewReader(wire(t, configPacket(t), p, video(Delta, 2))), FromBroadcaster)
	readOK(t, r)
	got := readOK(t, r)
	if len(got.Payload) != MaxPayload || !bytes.Equal(got.Payload, p.Payload) {
		t.Fatal("maximum bounded payload differs")
	}
	resolve(t, r, true)
	second := readOK(t, r)
	second.Payload[0] = 0
	if got.Payload[0] != 0xA7 {
		t.Fatal("reader reused returned payload")
	}
	resolve(t, r, true)
}

func TestWriterValidationDoesNotWrite(t *testing.T) {
	var out bytes.Buffer
	w := writer(t, &out, FromBroadcaster)
	if err := w.WritePacket(video(Keyframe, 1)); !errors.Is(err, ErrConfigRequired) || out.Len() != 0 {
		t.Fatal(err)
	}
	if err := w.WritePacket(configPacket(t)); err != nil {
		t.Fatal(err)
	}
	before := out.Len()
	badConfig := configPacket(t)
	badConfig.Payload[0] = 2
	badLength := video(Keyframe, 1)
	badLength.Header.PayloadLength++
	for _, p := range []Packet{configPacket(t), badConfig, badLength, video(Delta, 1), video(Keyframe, 2), control(RequestKeyframe)} {
		if err := w.WritePacket(p); err == nil || out.Len() != before {
			t.Fatal("invalid packet wrote bytes", err)
		}
	}
	if err := w.WritePacket(video(Keyframe, 1)); err != nil {
		t.Fatal(err)
	}
	before = out.Len()
	for _, p := range []Packet{video(Delta, 3), video(Keyframe, 1), video(Delta, 1)} {
		if err := w.WritePacket(p); !errors.Is(err, ErrNeedKeyframe) || out.Len() != before {
			t.Fatal(err)
		}
	}
	if err := w.WritePacket(video(Delta, 2)); err != nil {
		t.Fatal("validation failure changed writer state", err)
	}
}

type failingWriter struct {
	remaining int
	written   int
	err       error
}

func (w *failingWriter) Write(b []byte) (int, error) {
	n := min(len(b), w.remaining)
	w.remaining -= n
	w.written += n
	if n == len(b) {
		return n, nil
	}
	return n, w.err
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(b []byte) (int, error) { return f(b) }

func TestWriterIOFailureIsTerminal(t *testing.T) {
	boom := errors.New("injected I/O failure")
	for _, n := range []int{0, 1, HeaderSize - 1, HeaderSize, HeaderSize + 5} {
		output := &failingWriter{remaining: n, err: boom}
		w := writer(t, output, FromBroadcaster)
		err := w.WritePacket(configPacket(t))
		if !errors.Is(err, boom) || w.state.configured {
			t.Fatal(n, err)
		}
		if again := w.WritePacket(configPacket(t)); again != err || output.written != n {
			t.Fatal("retried failed write")
		}
	}
	for _, n := range []int{-1, 0, HeaderSize + 1} {
		w := writer(t, writerFunc(func([]byte) (int, error) { return n, nil }), FromViewer)
		if err := w.WritePacket(control(Ping)); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal(err)
		}
	}
	output := &failingWriter{remaining: HeaderSize + ConfigSize + HeaderSize + 2, err: boom}
	w := writer(t, output, FromBroadcaster)
	if err := w.WritePacket(configPacket(t)); err != nil {
		t.Fatal(err)
	}
	if err := w.WritePacket(video(Keyframe, 1)); !errors.Is(err, boom) || w.state.lastVideo != 0 {
		t.Fatal("partially written video committed", err)
	}
}

func TestEndBeforeConfigAndNoReadAfterEnd(t *testing.T) {
	for _, d := range []Direction{FromBroadcaster, FromViewer} {
		var out bytes.Buffer
		w := writer(t, &out, d)
		if err := w.WritePacket(control(End)); err != nil {
			t.Fatal(err)
		}
		out.Write(wire(t, control(Ping)))
		r := reader(t, &out, d)
		if p := readOK(t, r); p.Header.Type != End {
			t.Fatal("missing END")
		}
		left := out.Len()
		if _, err := r.ReadPacket(); err != io.EOF || out.Len() != left {
			t.Fatal("read after END", err)
		}
	}
}

func TestInvalidDirection(t *testing.T) {
	for _, d := range []Direction{0, 3, 255} {
		if _, err := NewReader(bytes.NewReader(nil), d); !errors.Is(err, ErrDirection) {
			t.Fatal(err)
		}
		if _, err := NewWriter(io.Discard, d); !errors.Is(err, ErrDirection) {
			t.Fatal(err)
		}
	}
}

func TestSessionCloseCancelsBlockedIO(t *testing.T) {
	for _, writing := range []bool{false, true} {
		pr, pw := io.Pipe()
		t.Cleanup(func() { pr.Close(); pw.Close() })
		ctx, cancel := context.WithCancel(context.Background())
		stop := context.AfterFunc(ctx, func() {
			pr.CloseWithError(ctx.Err())
			pw.CloseWithError(ctx.Err())
		})
		t.Cleanup(func() { cancel(); stop() })
		done := make(chan error, 1)
		entered := make(chan struct{})
		r := reader(t, readerFunc(func(b []byte) (int, error) {
			close(entered)
			return pr.Read(b)
		}), FromViewer)
		w := writer(t, writerFunc(func(b []byte) (int, error) {
			close(entered)
			return pw.Write(b)
		}), FromViewer)
		go func() {
			if writing {
				done <- w.WritePacket(control(Ping))
			} else {
				_, err := r.ReadPacket()
				done <- err
			}
		}()
		select {
		case <-entered: // I/O was entered with no peer to complete it.
			cancel()
		case <-time.After(3 * time.Second):
			t.Fatal("I/O was not entered")
		}
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("cancelled I/O succeeded")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("session-owned close did not unblock I/O")
		}
	}
}

func TestPipeSenderTerminatesDuringPayload(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pr.Close(); pw.Close() })
	p := control(Ping)
	data := wire(t, p)
	done := make(chan error, 1)
	go func() {
		_, err := pw.Write(data[:HeaderSize+4])
		pw.Close()
		done <- err
	}()
	r := reader(t, pr, FromViewer)
	if _, err := r.ReadPacket(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("sender termination accepted as complete packet", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
