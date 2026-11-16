// Package delta implements deterministic, lossless KEY and thresholded tile
// DELTA encoding. Encoder/Decoder are single-owner synchronous objects. They
// own their reference buffers, expose copies, create no goroutines, and accept
// context cancellation during bounded CPU work.
package delta

import (
	"bytes"
	"context"
	"errors"
	"time"

	"crocstream.local/app/internal/protocol"
)

const DefaultTau uint8 = 4

var (
	ErrClosed = errors.New("delta: encoder terminated after writer failure")
	ErrVideo  = errors.New("delta: expected video packet")
)

// PacketWriter must synchronously finish writing a packet or return an error.
// It must not retain/mutate Payload. The session owns serialization, CONFIG,
// cancellation of blocked I/O and termination after any writer failure.
type PacketWriter interface {
	WritePacket(protocol.Packet) error
}

// Measurement contains observed application bytes and elapsed codec time.
// Durations are wall/monotonic execution time, not process CPU measurements.
// EncodeTime includes baseline compression; baseline includes the 18-byte header.
type Measurement struct {
	Kind            protocol.Type
	FrameID         uint32
	PacketBytes     int
	BaselineBytes   int
	RawBytes        int
	DirtyTiles      int
	DirtyTilesKnown bool
	EncodeTime      time.Duration
	BaselineTime    time.Duration
}

type Encoder struct {
	config    protocol.Config
	limits    protocol.Limits
	tau       uint8
	reference []byte
	last      uint32
	forceKey  bool
	failed    bool
}

func NewEncoder(c protocol.Config, tau uint8) (*Encoder, error) {
	limits, err := c.Limits()
	if err != nil {
		return nil, err
	}
	return &Encoder{config: c, limits: limits, tau: tau}, nil
}

// RequestKeyframe marks the next emitted sample as KEY. Rate limiting is owned
// by the session. It does not send or copy a stale reference immediately.
func (e *Encoder) RequestKeyframe() { e.forceKey = true }

func (e *Encoder) Snapshot() ([]byte, uint32) { return bytes.Clone(e.reference), e.last }

// Emit encodes one already-normalized capture, writes synchronously, then commits
// canonical state. Input is borrowed during the call. A canceled/invalid capture
// before writing consumes no ID; after a successful write state must commit even
// if cancellation races with completion. A writer error permanently poisons the
// encoder because peer progress is uncertain.
func (e *Encoder) Emit(ctx context.Context, w PacketWriter, rgba []byte, timestampMs int64) (Measurement, error) {
	if e.failed {
		return Measurement{}, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return Measurement{}, err
	}
	start := time.Now()
	if len(rgba) != e.limits.RawBytes || !opaque(rgba) {
		return Measurement{}, ErrPixels
	}
	id, err := protocol.NextFrameID(e.last)
	if err != nil {
		return Measurement{}, err
	}
	current := bytes.Clone(rgba)
	baselineStart := time.Now()
	baseline, err := compress(ctx, current)
	if err != nil {
		return Measurement{}, err
	}
	m := Measurement{Kind: protocol.Keyframe, FrameID: id, RawBytes: len(current),
		BaselineBytes: protocol.HeaderSize + len(baseline), BaselineTime: time.Since(baselineStart)}
	payload, next := baseline, current
	key := id == 1 || (id-1)%uint32(e.config.KeyInterval) == 0 || e.forceKey
	if !key {
		indices, err := dirtyTiles(ctx, e.config, current, e.reference, e.limits.TileCount, e.tau)
		if err != nil {
			return Measurement{}, err
		}
		m.DirtyTiles = len(indices)
		m.DirtyTilesKnown = true
		// Strictly >65%; integer comparison avoids floating-point boundary drift.
		if len(indices)*100 <= e.limits.TileCount*65 {
			raw := encodeRecord(e.config, current, indices)
			payload, err = compress(ctx, raw)
			if err != nil {
				return Measurement{}, err
			}
			m.Kind, m.RawBytes = protocol.Delta, len(raw)
			next = bytes.Clone(e.reference)
			pos := 2
			for _, index := range indices {
				g := tile(e.config, index)
				n := g.w * g.h * 4
				pos += 2
				copyTile(next, raw[pos:pos+n], e.config, index, false)
				pos += n
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return Measurement{}, err
	}
	m.PacketBytes, m.EncodeTime = protocol.HeaderSize+len(payload), time.Since(start)
	p := protocol.Packet{Header: protocol.Header{Type: m.Kind, FrameID: id,
		TimestampMs: timestampMs, PayloadLength: uint32(len(payload))}, Payload: payload}
	if err := w.WritePacket(p); err != nil {
		e.failed = true
		return Measurement{}, err
	}
	e.reference, e.last, e.forceKey = next, id, false
	return m, nil
}

type DecodeMeasurement struct {
	Kind        protocol.Type
	FrameID     uint32
	PacketBytes int
	DecodeTime  time.Duration
}

type Decoder struct {
	config    protocol.Config
	limits    protocol.Limits
	reference []byte
	last      uint32
	seen      uint32
	valid     bool
}

func NewDecoder(c protocol.Config) (*Decoder, error) {
	limits, err := c.Limits()
	if err != nil {
		return nil, err
	}
	return &Decoder{config: c, limits: limits}, nil
}

// Snapshot returns an owned copy of the last good image, its ID and whether it
// may receive deltas. During recovery old pixels remain available for display.
func (d *Decoder) Snapshot() ([]byte, uint32, bool) {
	return bytes.Clone(d.reference), d.last, d.valid
}

// Invalidate must also be called when a protocol.Reader discards a bad sequence
// before invoking Decode. It preserves display pixels and forces KEY recovery.
func (d *Decoder) Invalidate() { d.valid = false }

// Decode independently validates packet/config/order, decompresses under a cap,
// then stages and atomically replaces the reference. The caller passes its
// success/failure to protocol.Reader.ResolveVideo. Payload is borrowed only for
// this call; controls must be handled by the session instead.
func (d *Decoder) Decode(ctx context.Context, p protocol.Packet) (DecodeMeasurement, error) {
	if err := ctx.Err(); err != nil {
		return DecodeMeasurement{}, err
	}
	start := time.Now()
	fail := func(err error) (DecodeMeasurement, error) {
		d.valid = false
		return DecodeMeasurement{}, err
	}
	if err := p.Validate(protocol.FromBroadcaster); err != nil {
		return fail(err)
	}
	h := p.Header
	if h.Type != protocol.Keyframe && h.Type != protocol.Delta {
		return fail(ErrVideo)
	}
	if d.seen == 0 && (h.Type != protocol.Keyframe || h.FrameID != 1) {
		return fail(protocol.ErrSequence)
	}
	if d.seen == ^uint32(0) {
		return fail(protocol.ErrFrameIDExhausted)
	}
	if h.FrameID <= d.seen {
		return fail(protocol.ErrNeedKeyframe)
	}
	d.seen = h.FrameID
	if h.Type == protocol.Delta && (!d.valid || h.FrameID != d.last+1) {
		return fail(protocol.ErrNeedKeyframe)
	}
	capacity := d.limits.RawBytes
	if h.Type == protocol.Delta {
		capacity = d.limits.DeltaBytes
	}
	raw, err := decompress(ctx, p.Payload, capacity)
	if err != nil {
		return fail(err)
	}
	next := raw
	if h.Type == protocol.Keyframe {
		if len(raw) != d.limits.RawBytes || !opaque(raw) {
			return fail(ErrPixels)
		}
	} else {
		tiles, err := DecodeDelta(ctx, d.config, raw)
		if err != nil {
			return fail(err)
		}
		next = bytes.Clone(d.reference)
		for _, t := range tiles {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			copyTile(next, t.Pixels, d.config, int(t.Index), false)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	d.reference, d.last, d.valid = next, h.FrameID, true
	return DecodeMeasurement{h.Type, h.FrameID, protocol.HeaderSize + len(p.Payload), time.Since(start)}, nil
}
