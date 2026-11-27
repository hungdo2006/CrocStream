package delta

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	"crocstream.local/app/internal/protocol"
	"crocstream.local/app/internal/telemetry"
)

var bg = context.Background()

func configuration(small bool) protocol.Config {
	c := protocol.DefaultConfig()
	if small {
		c.Width, c.Height = 640, 360
	}
	return c
}

func solid(c protocol.Config, v byte) []byte {
	b := make([]byte, int(c.Width)*int(c.Height)*4)
	for i := 0; i < len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = v, v, v, 255
	}
	return b
}

func encoder(t testing.TB, c protocol.Config, tau uint8) *Encoder {
	t.Helper()
	e, err := NewEncoder(c, tau)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func decoder(t testing.TB, c protocol.Config) *Decoder {
	t.Helper()
	d, err := NewDecoder(c)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type packetWriterFunc func(protocol.Packet) error

func (f packetWriterFunc) WritePacket(p protocol.Packet) error { return f(p) }

func emit(t testing.TB, e *Encoder, rgba []byte) (protocol.Packet, Measurement) {
	t.Helper()
	var packet protocol.Packet
	m, err := e.Emit(bg, packetWriterFunc(func(p protocol.Packet) error {
		packet = protocol.Packet{Header: p.Header, Payload: bytes.Clone(p.Payload)}
		return nil
	}), rgba, 123456)
	if err != nil {
		t.Fatal(err)
	}
	return packet, m
}

func packet(t testing.TB, typ protocol.Type, id uint32, raw []byte) protocol.Packet {
	t.Helper()
	payload, err := compress(bg, raw)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.Packet{Header: protocol.Header{Type: typ, FrameID: id, PayloadLength: uint32(len(payload))}, Payload: payload}
}

func noise(c protocol.Config, seed int64) []byte {
	b := solid(c, 0)
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < len(b); i += 4 {
		b[i], b[i+1], b[i+2] = byte(rng.Intn(256)), byte(rng.Intn(256)), byte(rng.Intn(256))
	}
	return b
}

// Slow independent oracle: aggregate differences by pixel coordinates, compare
// floating-point MAE, then traverse pixels again. It shares no tile helpers.
func oracle(c protocol.Config, old, current []byte, tau uint8, force bool) ([]byte, protocol.Type) {
	if old == nil || force {
		return bytes.Clone(current), protocol.Keyframe
	}
	w, h := int(c.Width), int(c.Height)
	cols, rows := (w+31)/32, (h+31)/32
	sums, counts := make([]int, cols*rows), make([]int, cols*rows)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tileID, p := y/32*cols+x/32, (y*w+x)*4
			counts[tileID]++
			for ch := 0; ch < 3; ch++ {
				a, b := int(current[p+ch]), int(old[p+ch])
				if a > b {
					sums[tileID] += a - b
				} else {
					sums[tileID] += b - a
				}
			}
		}
	}
	dirty := make([]bool, len(sums))
	n := 0
	for i := range dirty {
		dirty[i] = float64(sums[i])/float64(3*counts[i]) > float64(tau)
		if dirty[i] {
			n++
		}
	}
	if float64(n)/float64(len(dirty)) > 0.65 {
		return bytes.Clone(current), protocol.Keyframe
	}
	next := bytes.Clone(old)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if dirty[y/32*cols+x/32] {
				p := (y*w + x) * 4
				copy(next[p:p+4], current[p:p+4])
			}
		}
	}
	return next, protocol.Delta
}

func pattern(c protocol.Config, tick int) []byte {
	b := solid(c, 0)
	w, h := int(c.Width), int(c.Height)
	switch {
	case tick == 0: // Black and one dirty pixel in the subsequent sample.
	case tick == 1:
		b[0] = 255
	case tick == 2:
		return solid(c, 255)
	case tick == 3: // Color bars.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				b[(y*w+x)*4+(x/(w/3+1))%3] = 255
			}
		}
	case tick == 4 || tick%47 == 0:
		return noise(c, int64(tick))
	default: // Deterministic pseudo-text on a static desktop plus moving box.
		for y := 24; y < min(h, 180); y++ {
			for x := 20; x < min(w, 500); x++ {
				if y%13 < 7 && x%9 < 5 {
					p := (y*w + x) * 4
					b[p], b[p+1], b[p+2] = 180, 180, 180
				}
			}
		}
		x0, y0 := tick*3%(w-48), tick*2%(h-24)
		for y := y0; y < y0+24; y++ {
			for x := x0; x < x0+48; x++ {
				p := (y*w + x) * 4
				b[p], b[p+1], b[p+2] = byte(tick), 220, 80
			}
		}
	}
	return b
}

func TestCanonical201Frames(t *testing.T) {
	for _, small := range []bool{true, false} {
		for _, tau := range []uint8{0, 4} {
			c := configuration(small)
			t.Run(string(rune('0'+tau))+"/"+map[bool]string{true: "640x360", false: "1280x720"}[small], func(t *testing.T) {
				e, d := encoder(t, c, tau), decoder(t, c)
				var expected []byte
				for tick := 0; tick <= 200; tick++ {
					current := pattern(c, tick)
					force := tick == 19 || tick == 113
					if force {
						e.RequestKeyframe()
					}
					want, kind := oracle(c, expected, current, tau, force || tick%int(c.KeyInterval) == 0)
					p, m := emit(t, e, current)
					if p.Header.Type != kind || p.Header.FrameID != uint32(tick+1) {
						t.Fatalf("tick %d: unexpected kind/ID", tick)
					}
					if _, err := d.Decode(bg, p); err != nil {
						t.Fatal(tick, err)
					}
					sender, id := e.Snapshot()
					viewer, vid, valid := d.Snapshot()
					if id != vid || !valid || !bytes.Equal(sender, viewer) || !bytes.Equal(sender, want) {
						t.Fatalf("canonical/oracle mismatch tick %d", tick)
					}
					if tau == 0 && !bytes.Equal(viewer, current) {
						t.Fatalf("tau0 lost pixels at %d", tick)
					}
					baseline, err := compress(bg, current)
					if err != nil || m.BaselineBytes != protocol.HeaderSize+len(baseline) ||
						m.PacketBytes != protocol.HeaderSize+len(p.Payload) || m.BaselineTime > m.EncodeTime {
						t.Fatal("unfair measurements", err)
					}
					expected = want
				}
			})
		}
	}
}

func TestSubthresholdAccumulatesAgainstTransmittedReference(t *testing.T) {
	c := configuration(true)
	e, d := encoder(t, c, 4), decoder(t, c)
	current := solid(c, 0)
	p, _ := emit(t, e, current)
	if _, err := d.Decode(bg, p); err != nil {
		t.Fatal(err)
	}
	for step := 1; step <= 12; step++ {
		// Each pixel changes by just 1 per capture; cumulative tile MAE reaches 5.
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				pos := (y*int(c.Width) + x) * 4
				current[pos], current[pos+1], current[pos+2] = byte(step), byte(step), byte(step)
			}
		}
		// Also track one slowly changing pixel that alone stays below tau.
		current[40*4] = byte(step)
		p, _ = emit(t, e, current)
		if _, err := d.Decode(bg, p); err != nil {
			t.Fatal(err)
		}
		out, _, _ := d.Snapshot()
		if out[0] != byte(step/5*5) || out[40*4] != 0 {
			t.Fatal("reference drift or >= threshold", step)
		}
		sent, _ := e.Snapshot()
		if !bytes.Equal(out, sent) {
			t.Fatal("peer drift")
		}
	}
}

func TestDirtyRatioAndSchedule(t *testing.T) {
	c := configuration(true)
	for _, changed := range []int{156, 157} { // Exactly 65% vs just above.
		e := encoder(t, c, 0)
		emit(t, e, solid(c, 0))
		current := solid(c, 0)
		for index := 0; index < changed; index++ {
			// One RGB pixel per tile is enough when tau=0.
			x, y := index%20*32, index/20*32
			current[(y*640+x)*4] = 1
		}
		p, m := emit(t, e, current)
		want := protocol.Delta
		if changed == 157 {
			want = protocol.Keyframe
		}
		if p.Header.Type != want || m.DirtyTiles != changed {
			t.Fatal("65% boundary", changed, m)
		}
	}
	for _, interval := range []uint16{1, 30, 120} {
		c.KeyInterval = interval
		e := encoder(t, c, 4)
		current := solid(c, 0)
		for id := 1; id <= int(interval)+2; id++ {
			pli := id == 3
			if pli {
				e.RequestKeyframe()
			}
			p, _ := emit(t, e, current)
			key := id == 1 || (id-1)%int(interval) == 0 || pli
			if (p.Header.Type == protocol.Keyframe) != key {
				t.Fatal("schedule shifted by PLI", interval, id)
			}
		}
	}
}

func TestRandomPairsAgainstOracle(t *testing.T) {
	c := configuration(true)
	for _, tau := range []uint8{0, 4} {
		e, d := encoder(t, c, tau), decoder(t, c)
		current := noise(c, 99)
		p, _ := emit(t, e, current)
		if _, err := d.Decode(bg, p); err != nil {
			t.Fatal(err)
		}
		expected := bytes.Clone(current)
		rng := rand.New(rand.NewSource(2026))
		for i := 0; i < 12; i++ {
			for j := 0; j < 3000; j++ {
				pixel := rng.Intn(len(current)/4) * 4
				current[pixel] = byte(rng.Intn(256))
			}
			want, kind := oracle(c, expected, current, tau, false)
			p, _ = emit(t, e, current)
			if p.Header.Type != kind {
				t.Fatal("oracle kind")
			}
			if _, err := d.Decode(bg, p); err != nil {
				t.Fatal(err)
			}
			got, _, _ := d.Snapshot()
			if !bytes.Equal(got, want) {
				t.Fatal("random oracle mismatch")
			}
			expected = want
		}
	}
}

func TestWriterCommitFailureAndCancellation(t *testing.T) {
	c := configuration(true)
	current := solid(c, 1)
	e := encoder(t, c, 4)
	boom := errors.New("partial write")
	_, err := e.Emit(bg, packetWriterFunc(func(p protocol.Packet) error {
		if b, id := e.Snapshot(); b != nil || id != 0 {
			t.Fatal("committed before writer acceptance")
		}
		return boom
	}), current, 0)
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if b, id := e.Snapshot(); b != nil || id != 0 {
		t.Fatal("failed write committed")
	}
	if _, err = e.Emit(bg, packetWriterFunc(func(protocol.Packet) error { t.Fatal("retried"); return nil }), current, 0); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	e = encoder(t, c, 4)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err = e.Emit(ctx, nil, current, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = e.Emit(bg, nil, current[:10], 0); !errors.Is(err, ErrPixels) {
		t.Fatal(err)
	}
	bad := bytes.Clone(current)
	bad[3] = 0
	if _, err = e.Emit(bg, nil, bad, 0); !errors.Is(err, ErrPixels) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(bg)
	_, err = e.Emit(ctx, packetWriterFunc(func(protocol.Packet) error { cancel(); return nil }), current, 0)
	if err != nil {
		t.Fatal(err)
	}
	current[0] = 255
	got, id := e.Snapshot()
	if id != 1 || got[0] != 1 {
		t.Fatal("write acceptance/input ownership")
	}
	got[0] = 200
	if other, _ := e.Snapshot(); other[0] != 1 {
		t.Fatal("snapshot aliases encoder")
	}
	e.last = ^uint32(0)
	if _, err = e.Emit(bg, nil, solid(c, 0), 0); !errors.Is(err, protocol.ErrFrameIDExhausted) {
		t.Fatal(err)
	}
}

func TestDecoderRecoveryAndAtomicity(t *testing.T) {
	c := configuration(true)
	d := decoder(t, c)
	key := packet(t, protocol.Keyframe, 1, solid(c, 42))
	if _, err := d.Decode(bg, key); err != nil {
		t.Fatal(err)
	}
	old, id, _ := d.Snapshot()
	malformed := []byte{0, 1, 0, 0, 255} // Valid envelope, truncated tile.
	bad := packet(t, protocol.Delta, 2, malformed)
	if _, err := d.Decode(bg, bad); !errors.Is(err, ErrRecord) {
		t.Fatal(err)
	}
	got, newID, valid := d.Snapshot()
	if valid || newID != id || !bytes.Equal(got, old) {
		t.Fatal("corrupt delta changed canonical bytes")
	}
	bad = packet(t, protocol.Delta, 3, []byte{0, 0})
	if _, err := d.Decode(bg, bad); !errors.Is(err, protocol.ErrNeedKeyframe) {
		t.Fatal(err)
	}
	for _, id := range []uint32{1, 2, 3} {
		key.Header.FrameID = id
		if _, err := d.Decode(bg, key); !errors.Is(err, protocol.ErrNeedKeyframe) {
			t.Fatal("replay", err)
		}
	}
	key.Header.FrameID = 8
	if _, err := d.Decode(bg, key); err != nil {
		t.Fatal(err)
	}
	got, id, valid = d.Snapshot()
	if !valid || id != 8 || !bytes.Equal(got, old) {
		t.Fatal("KEY gap did not recover")
	}
	got[0] = 0
	if copy, _, _ := d.Snapshot(); copy[0] != 42 {
		t.Fatal("snapshot aliases decoder")
	}
	// A gap invalidated by the protocol parser must also invalidate codec state.
	d.Invalidate()
	bad.Header.FrameID = 9
	if _, err := d.Decode(bg, bad); !errors.Is(err, protocol.ErrNeedKeyframe) {
		t.Fatal(err)
	}
	key.Header.FrameID = ^uint32(0)
	if _, err := d.Decode(bg, key); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Decode(bg, key); !errors.Is(err, protocol.ErrFrameIDExhausted) {
		t.Fatal(err)
	}
}

func TestInitialDecodeFailureAndInputValidation(t *testing.T) {
	c := configuration(true)
	d := decoder(t, c)
	key := packet(t, protocol.Keyframe, 1, solid(c, 0))
	bad := key
	bad.Payload = bytes.Clone(key.Payload)
	bad.Payload[len(bad.Payload)-1] ^= 1
	if _, err := d.Decode(bg, bad); err == nil {
		t.Fatal("bad checksum")
	}
	if _, err := d.Decode(bg, key); !errors.Is(err, protocol.ErrNeedKeyframe) {
		t.Fatal("failed KEY reused ID", err)
	}
	key.Header.FrameID = 2
	if _, err := d.Decode(bg, key); err != nil {
		t.Fatal(err)
	}
	for _, p := range []protocol.Packet{
		{Header: protocol.Header{Type: protocol.Ping, PayloadLength: 8}, Payload: make([]byte, 8)},
		{Header: protocol.Header{Type: protocol.Keyframe, FrameID: 3, PayloadLength: protocol.MaxPayload + 1}},
		{Header: protocol.Header{Type: protocol.Delta, FrameID: 0, PayloadLength: 1}, Payload: []byte{0}},
	} {
		if _, err := d.Decode(bg, p); err == nil {
			t.Fatal("accepted wrong packet")
		}
	}
	for _, raw := range [][]byte{solid(c, 0)[:40], append(solid(c, 0), 0)} {
		d = decoder(t, c)
		if _, err := d.Decode(bg, packet(t, protocol.Keyframe, 1, raw)); err == nil {
			t.Fatal("bad key raw length")
		}
	}
	raw := solid(c, 0)
	raw[3] = 0
	if _, err := decoder(t, c).Decode(bg, packet(t, protocol.Keyframe, 1, raw)); !errors.Is(err, ErrPixels) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := d.Decode(ctx, key); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	c.Width = 0
	if _, err := NewDecoder(c); err == nil {
		t.Fatal("config")
	}
	if _, err := NewEncoder(c, 4); err == nil {
		t.Fatal("config")
	}
}

func TestProtocolCodecTelemetryIntegration(t *testing.T) {
	c := configuration(true)
	now := time.Unix(1000, 0)
	meter := telemetry.New(func() time.Time { return now })
	var stream bytes.Buffer
	w, err := protocol.NewWriter(&stream, protocol.FromBroadcaster)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := protocol.EncodeConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.WritePacket(protocol.Packet{Header: protocol.Header{Type: protocol.SessionConfig, PayloadLength: 11}, Payload: cfg[:]}); err != nil {
		t.Fatal(err)
	}
	e := encoder(t, c, 4)
	totalActual, totalBaseline := 0, 0
	for i := 0; i < 5; i++ {
		if err := meter.Record(telemetry.Event{Kind: telemetry.Captured}); err != nil {
			t.Fatal(err)
		}
		measurement, err := e.Emit(bg, w, solid(c, byte(i)), 0)
		if err != nil {
			t.Fatal(err)
		}
		totalActual += measurement.PacketBytes
		totalBaseline += measurement.BaselineBytes
		err = meter.Record(telemetry.Event{Kind: telemetry.Emitted, VideoType: measurement.Kind, Bytes: measurement.PacketBytes,
			BaselineBytes: measurement.BaselineBytes, CodecTime: measurement.EncodeTime, BaselineTime: measurement.BaselineTime})
		if err != nil {
			t.Fatal(err)
		}
	}
	if stream.Len() != protocol.HeaderSize+protocol.ConfigSize+totalActual {
		t.Fatal("wire byte accounting")
	}
	r, err := protocol.NewReader(&stream, protocol.FromBroadcaster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ReadPacket(); err != nil {
		t.Fatal(err)
	}
	d := decoder(t, c)
	for i := 0; i < 5; i++ {
		p, err := r.ReadPacket()
		if err != nil {
			t.Fatal(err)
		}
		if err := meter.Record(telemetry.Event{Kind: telemetry.Received, VideoType: p.Header.Type, Bytes: protocol.HeaderSize + len(p.Payload)}); err != nil {
			t.Fatal(err)
		}
		measurement, decodeErr := d.Decode(bg, p)
		if err := r.ResolveVideo(decodeErr == nil); err != nil {
			t.Fatal(err)
		}
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if err := meter.Record(telemetry.Event{Kind: telemetry.Decoded, CodecTime: measurement.DecodeTime}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.ReadPacket(); err != io.EOF {
		t.Fatal(err)
	}
	s, err := meter.Snapshot()
	if err != nil || s.EmittedFPS != 5 || s.DecodedFPS != 5 || s.RenderedFPS != 0 ||
		s.Sent.Bytes != uint64(totalActual) || s.BaselineBytes != uint64(totalBaseline) || !s.SavingsKnown {
		t.Fatal("measurement", err, s)
	}
	sent, _ := e.Snapshot()
	received, _, _ := d.Snapshot()
	if !bytes.Equal(sent, received) {
		t.Fatal("G1/G2 integration drift")
	}
}

func TestRecordFixturesAndEdges(t *testing.T) {
	for _, small := range []bool{true, false} {
		c := configuration(small)
		limits, _ := c.Limits()
		index := limits.TileCount - 1
		height := int(c.Height) % 32
		raw := make([]byte, 4+32*height*4)
		binary.BigEndian.PutUint16(raw, 1)
		binary.BigEndian.PutUint16(raw[2:], uint16(index))
		for i := 4; i < len(raw); i += 4 {
			raw[i], raw[i+1], raw[i+2], raw[i+3] = 11, 22, 33, 255
		}
		tiles, err := DecodeDelta(bg, c, raw)
		if err != nil || len(tiles) != 1 || int(tiles[0].Index) != index {
			t.Fatal(err)
		}
		d := decoder(t, c)
		if _, err = d.Decode(bg, packet(t, protocol.Keyframe, 1, solid(c, 0))); err != nil {
			t.Fatal(err)
		}
		if _, err = d.Decode(bg, packet(t, protocol.Delta, 2, raw)); err != nil {
			t.Fatal(err)
		}
		got, _, _ := d.Snapshot()
		for y := 0; y < int(c.Height); y++ {
			for x := 0; x < int(c.Width); x++ {
				p := (y*int(c.Width) + x) * 4
				want := byte(0)
				if y >= int(c.Height)-height && x >= int(c.Width)-32 {
					want = 11
				}
				if got[p] != want {
					t.Fatal("rightmost/bottom partial geometry")
				}
			}
		}
		// Maximum dirty count is valid on the wire even though encoder uses KEY.
		indices := make([]int, limits.TileCount)
		for i := range indices {
			indices[i] = i
		}
		all := encodeRecord(c, solid(c, 99), indices)
		if len(all) != limits.DeltaBytes {
			t.Fatal("max decompressed budget")
		}
		if _, err = d.Decode(bg, packet(t, protocol.Delta, 3, all)); err != nil {
			t.Fatal(err)
		}
		got, _, _ = d.Snapshot()
		if !bytes.Equal(got, solid(c, 99)) {
			t.Fatal("max record roundtrip")
		}
		empty, err := DecodeDelta(bg, c, []byte{0, 0})
		if err != nil || len(empty) != 0 {
			t.Fatal(err)
		}
	}
	// Verify unrolled opaque checks handle tail lengths (1..5 pixels) accurately.
	for pixels := 1; pixels <= 5; pixels++ {
		buf := make([]byte, pixels*4)
		for i := 3; i < len(buf); i += 4 {
			buf[i] = 255
		}
		if !opaque(buf) {
			t.Fatalf("expected opaque for %d pixels", pixels)
		}
		for bad := 0; bad < pixels; bad++ {
			buf[bad*4+3] = 254
			if opaque(buf) {
				t.Fatalf("accepted non-opaque pixel %d/%d", bad, pixels)
			}
			buf[bad*4+3] = 255
		}
	}
}

func BenchmarkEmitAndDecode720pDelta(b *testing.B) {
	c := configuration(false)
	c.KeyInterval = 120
	e := encoder(b, c, DefaultTau)
	d := decoder(b, c)
	base := pattern(c, 5)
	next := pattern(c, 6)
	var pkt protocol.Packet
	write := packetWriterFunc(func(p protocol.Packet) error {
		pkt = p
		return nil
	})
	if _, err := e.Emit(bg, write, base, 1); err != nil {
		b.Fatal(err)
	}
	if _, err := d.Decode(bg, pkt); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := next
		if i%2 == 1 {
			src = base
		}
		if _, err := e.Emit(bg, write, src, int64(i+2)); err != nil {
			b.Fatal(err)
		}
		if _, err := d.Decode(bg, pkt); err != nil {
			b.Fatal(err)
		}
	}
}
