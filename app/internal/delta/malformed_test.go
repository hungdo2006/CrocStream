package delta

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"crocstream.local/app/internal/protocol"
	"github.com/pierrec/lz4/v4"
)

func libraryFrame(t testing.TB, raw []byte, options ...lz4.Option) []byte {
	t.Helper()
	var b bytes.Buffer
	w := lz4.NewWriter(&b)
	if err := w.Apply(options...); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestLZ4SingleFrameAndBombBounds(t *testing.T) {
	raw := bytes.Repeat([]byte("abcdef"), 1000)
	// Independently exercise all standard block sizes, optional size/checksums.
	for _, size := range []lz4.BlockSize{lz4.Block64Kb, lz4.Block256Kb, lz4.Block1Mb, lz4.Block4Mb} {
		for _, checksum := range []bool{true, false} {
			data := libraryFrame(t, raw, lz4.BlockSizeOption(size), lz4.ChecksumOption(checksum), lz4.BlockChecksumOption(checksum))
			got, err := decompress(bg, data, len(raw))
			if err != nil || !bytes.Equal(got, raw) {
				t.Fatal(size, checksum, err)
			}
		}
	}
	valid := libraryFrame(t, raw, lz4.BlockSizeOption(lz4.Block64Kb), lz4.ChecksumOption(true), lz4.BlockChecksumOption(true), lz4.SizeOption(uint64(len(raw))))
	for i := 0; i < len(valid); i++ {
		if _, err := decompress(bg, valid[:i], len(raw)); err == nil {
			t.Fatal("accepted truncated frame", i)
		}
	}
	empty := libraryFrame(t, nil)
	for _, bad := range [][]byte{
		nil, []byte{1, 2, 3, 4}, append(bytes.Clone(valid), 0),
		append(bytes.Clone(valid), valid...), append(bytes.Clone(valid), empty...),
		append(empty, valid...), libraryFrame(t, raw, lz4.LegacyOption(true)),
		{0x50, 0x2A, 0x4D, 0x18, 0, 0, 0, 0},
	} {
		if _, err := decompress(bg, bad, len(raw)); err == nil {
			t.Fatal("accepted trailing/legacy/skippable data")
		}
	}
	for _, offset := range []int{4, 5, 14, 20, len(valid) - 1} {
		bad := bytes.Clone(valid)
		bad[offset] ^= 0x80
		if _, err := decompress(bg, bad, len(raw)); err == nil {
			t.Fatal("accepted corrupted frame", offset)
		}
	}
	oversize := bytes.Clone(valid)
	binary.LittleEndian.PutUint64(oversize[6:14], ^uint64(0))
	if err := singleFrame(bg, oversize, len(raw)); err == nil {
		t.Fatal("unbounded content size")
	}
	for _, limit := range []int{-1, 0, 5, len(raw) - 1, protocol.MaxPayload} {
		if _, err := decompress(bg, valid, limit); err == nil {
			t.Fatal("invalid/too small output limit", limit)
		}
	}
	// No content-size hint: must stop decompression on overshoot, not ReadAll.
	bomb := libraryFrame(t, bytes.Repeat([]byte{0}, 2*1024*1024), lz4.BlockSizeOption(lz4.Block64Kb))
	if _, err := decompress(bg, bomb, 1024); !errors.Is(err, ErrCompressed) {
		t.Fatal("compression bomb", err)
	}
	// Frame metadata cannot demand an allocation from an arbitrary block length.
	block := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(block[15:19], 0x7FFFFFFF)
	if err := singleFrame(bg, block, len(raw)); err == nil {
		t.Fatal("oversize compressed block")
	}
	block = bytes.Clone(valid)
	binary.LittleEndian.PutUint32(block[15:19], 0x80000000)
	if err := singleFrame(bg, block, len(raw)); err == nil {
		t.Fatal("zero-length raw block")
	}
	if _, err := decompress(bg, make([]byte, protocol.MaxPayload+1), 10); err == nil {
		t.Fatal("oversize packet")
	}
}

// Adjust a test frame descriptor checksum using the library's independently
// implemented header validator; never copy its internal checksum implementation.
func fixHeader(t testing.TB, b []byte, checksumOffset int) {
	t.Helper()
	for n := 0; n < 256; n++ {
		b[checksumOffset] = byte(n)
		if valid, err := lz4.ValidFrameHeader(b[:checksumOffset+1]); err == nil && valid {
			return
		}
	}
	t.Fatal("could not construct test descriptor")
}

func TestDependentBlocksAndFalseContentSize(t *testing.T) {
	raw := bytes.Repeat([]byte("abc"), 50000)
	data := libraryFrame(t, raw, lz4.BlockSizeOption(lz4.Block64Kb), lz4.SizeOption(uint64(len(raw))))
	data[4] &^= 0x20 // Independent encoding remains a valid dependent-block stream.
	fixHeader(t, data, 14)
	got, err := decompress(bg, data, len(raw))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("dependent frame", err)
	}
	// Correct header checksum but lying, smaller content size must fail at EOF.
	data = libraryFrame(t, raw, lz4.SizeOption(uint64(len(raw))))
	binary.LittleEndian.PutUint64(data[6:14], uint64(len(raw)-1))
	fixHeader(t, data, 14)
	if _, err := decompress(bg, data, len(raw)); err == nil {
		t.Fatal("content size mismatch")
	}
}

func TestMalformedDeltaDoesNotMutatePixels(t *testing.T) {
	c := configuration(true)
	pixels := solid(c, 23)
	one := encodeRecord(c, pixels, []int{0})
	two := encodeRecord(c, pixels, []int{0, 1})
	tests := map[string][]byte{
		"nil": nil, "shortCount": {0}, "extraEmpty": {0, 0, 1}, "countOverflow": {0xFF, 0xFF},
		"truncatedIndex": {0, 1, 0}, "outOfRange": {0, 1, 0, 240},
		"truncatedTile": one[:len(one)-1], "extraTile": append(bytes.Clone(one), 0),
	}
	duplicate := bytes.Clone(two)
	copy(duplicate[4+32*32*4:6+32*32*4], []byte{0, 0})
	tests["duplicate"] = duplicate
	unsorted := bytes.Clone(two)
	copy(unsorted[2:4], []byte{0, 2})
	tests["unsorted"] = unsorted
	alpha := bytes.Clone(one)
	alpha[7] = 0
	tests["alpha"] = alpha
	// First valid tile followed by malformed second tile proves all-or-nothing.
	partial := bytes.Clone(two)
	partial[len(partial)-1] = 0
	tests["lateAlpha"] = partial
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDelta(bg, c, raw); err == nil {
				t.Fatal("accepted malformed record")
			}
			d := decoder(t, c)
			if _, err := d.Decode(bg, packet(t, protocol.Keyframe, 1, solid(c, 0))); err != nil {
				t.Fatal(err)
			}
			before, id, _ := d.Snapshot()
			if _, err := d.Decode(bg, packet(t, protocol.Delta, 2, raw)); err == nil {
				t.Fatal("accepted compressed malformed record")
			}
			after, newID, valid := d.Snapshot()
			if valid || newID != id || !bytes.Equal(before, after) {
				t.Fatal("partial mutation on malformed delta")
			}
		})
	}
	limits, _ := c.Limits()
	if _, err := DecodeDelta(bg, c, make([]byte, limits.DeltaBytes+1)); err == nil {
		t.Fatal("oversize raw record")
	}
	c.Height = 1
	if _, err := DecodeDelta(bg, c, []byte{0, 0}); err == nil {
		t.Fatal("unsupported config")
	}
}

// A deterministic context trips after bounded work has begun, exercising
// cancellation paths without timing-sensitive sleeps or extra goroutines.
type budgetContext struct {
	context.Context
	remaining int
}

func (c *budgetContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestCancellationDuringCodecWork(t *testing.T) {
	c := configuration(true)
	current := solid(c, 7)
	for _, budget := range []int{2, 6, 16} {
		ctx := &budgetContext{Context: bg, remaining: budget}
		e := encoder(t, c, 4)
		_, err := e.Emit(ctx, packetWriterFunc(func(protocol.Packet) error { t.Fatal("wrote after cancellation"); return nil }), current, 0)
		if !errors.Is(err, context.Canceled) {
			t.Fatal("encoder cancel", budget, err)
		}
		if b, id := e.Snapshot(); b != nil || id != 0 {
			t.Fatal("cancel committed")
		}
	}
	d := decoder(t, c)
	key := packet(t, protocol.Keyframe, 1, current)
	if _, err := d.Decode(bg, key); err != nil {
		t.Fatal(err)
	}
	before, id, _ := d.Snapshot()
	key.Header.FrameID = 2
	ctx := &budgetContext{Context: bg, remaining: 20}
	if _, err := d.Decode(ctx, key); !errors.Is(err, context.Canceled) {
		t.Fatal("decoder cancel", err)
	}
	after, newID, valid := d.Snapshot()
	if valid || id != newID || !bytes.Equal(before, after) {
		t.Fatal("cancel changed reference")
	}
	cancelled, cancel := context.WithCancel(bg)
	cancel()
	if _, err := DecodeDelta(cancelled, c, []byte{0, 0}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestLibraryInteroperabilityAndDeterminism(t *testing.T) {
	c := configuration(true)
	raw := noise(c, 12)
	a, err := compress(bg, raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := compress(bg, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("nondeterministic encoding")
	}
	r := lz4.NewReader(bytes.NewReader(a))
	out, err := io.ReadAll(io.LimitReader(r, int64(len(raw))+1))
	r.Reset(nil)
	if err != nil || !bytes.Equal(out, raw) {
		t.Fatal("standard LZ4 reader cannot decode output", err)
	}
}

func BenchmarkCodec(b *testing.B) {
	for _, small := range []bool{true, false} {
		c := configuration(small)
		res := map[bool]string{true: "640x360", false: "1280x720"}[small]
		for _, scenario := range []string{"static", "moving", "noise"} {
			b.Run(res+"/"+scenario, func(b *testing.B) {
				e, d := encoder(b, c, 4), decoder(b, c)
				a, z := solid(c, 0), solid(c, 0)
				if scenario == "moving" {
					a = pattern(c, 10)
					z = pattern(c, 11)
				}
				if scenario == "noise" {
					a = noise(c, 10)
					z = noise(c, 11)
				}
				var actual, baseline int64
				w := packetWriterFunc(func(p protocol.Packet) error { _, err := d.Decode(bg, p); return err })
				b.ReportAllocs()
				b.SetBytes(int64(len(a)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					current := a
					if i%2 == 1 {
						current = z
					}
					m, err := e.Emit(bg, w, current, int64(i))
					if err != nil {
						b.Fatal(err)
					}
					actual += int64(m.PacketBytes)
					baseline += int64(m.BaselineBytes)
				}
				b.StopTimer()
				b.ReportMetric(float64(actual)/float64(b.N), "video-B/frame")
				b.ReportMetric(float64(baseline)/float64(b.N), "baseline-B/frame")
				if baseline != 0 {
					b.ReportMetric(100*(1-float64(actual)/float64(baseline)), "savings-%")
				}
			})
		}
	}
}
