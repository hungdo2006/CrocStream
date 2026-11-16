package protocol

import (
	"errors"
	"fmt"
	"io"
)

type streamState struct {
	configured bool
	config     Config
	lastSeen   uint32
	lastVideo  uint32
	reference  bool
}

func (s *streamState) check(h Header, d Direction) error {
	if err := h.Validate(d); err != nil {
		return err
	}
	if h.Type == SessionConfig && s.configured {
		return ErrDuplicateConfig
	}
	// A broadcaster sends CONFIG first, except END when aborting preparation.
	if d == FromBroadcaster && !s.configured && h.Type != SessionConfig && h.Type != End {
		return ErrConfigRequired
	}
	if !h.Type.video() {
		return nil
	}
	if s.lastSeen == 0 {
		if h.Type != Keyframe || h.FrameID != 1 {
			return ErrSequence
		}
		return nil
	}
	if _, err := NextFrameID(s.lastSeen); err != nil {
		return err
	}
	if h.FrameID <= s.lastSeen {
		return ErrNeedKeyframe
	}
	if h.Type == Delta && (!s.reference || h.FrameID != s.lastVideo+1) {
		return ErrNeedKeyframe
	}
	return nil
}

func (s *streamState) resolve(h Header, valid bool) {
	// Wire replay protection advances even when codec validation fails; the
	// canonical reference ID advances only on successful validation.
	s.lastSeen = h.FrameID
	s.reference = valid
	if valid {
		s.lastVideo = h.FrameID
	}
}

type Reader struct {
	input     io.Reader
	direction Direction
	state     streamState
	pending   *Header
	terminal  error
}

func NewReader(input io.Reader, d Direction) (*Reader, error) {
	if !d.valid() {
		return nil, ErrDirection
	}
	return &Reader{input: input, direction: d}, nil
}

// Config returns a copy of the validated, immutable session configuration.
func (r *Reader) Config() (Config, bool) { return r.state.config, r.state.configured }

// ReadPacket validates headers and session state before allocating <=MaxPayload.
// A video packet must be resolved before the next call, after full codec checks.
//
// ErrNeedKeyframe is recoverable: one bounded, structurally valid video payload
// was discarded, the last good ID is retained, and the reference is invalid.
// The caller should retain the displayed image, rate-limit PLI and continue
// reading until KEY. All wire/I/O errors are sticky and require disconnection.
// EOF at a packet boundary is io.EOF; any truncated header/payload is unexpected
// EOF. No resynchronization scan is attempted. END is returned once, then EOF.
func (r *Reader) ReadPacket() (Packet, error) {
	if r.terminal != nil {
		return Packet{}, r.terminal
	}
	if r.pending != nil {
		return Packet{}, ErrPendingVideo
	}
	var b [HeaderSize]byte
	if _, err := io.ReadFull(r.input, b[:]); err != nil {
		return r.fail(err)
	}
	h, err := DecodeHeader(b[:])
	if err != nil {
		return r.fail(err)
	}
	if err = r.state.check(h, r.direction); err != nil {
		if !errors.Is(err, ErrNeedKeyframe) {
			return r.fail(err)
		}
		r.state.reference = false
		if err := discardPayload(r.input, h.PayloadLength); err != nil {
			return r.fail(err)
		}
		r.state.lastSeen = max(r.state.lastSeen, h.FrameID)
		return Packet{}, ErrNeedKeyframe
	}
	p := Packet{Header: h}
	if h.PayloadLength != 0 {
		p.Payload = make([]byte, int(h.PayloadLength))
		if _, err := io.ReadFull(r.input, p.Payload); err != nil {
			return r.fail(payloadError(err))
		}
	}
	if h.Type == SessionConfig {
		c, err := DecodeConfig(p.Payload)
		if err != nil {
			return r.fail(err)
		}
		r.state.config, r.state.configured = c, true
	}
	if h.Type.video() {
		r.pending = &h
	}
	if h.Type == End {
		r.terminal = io.EOF
	}
	return p, nil
}

// ResolveVideo must be called with true only after the full codec validation
// succeeds. false keeps the last good ID and invalidates the reference; a later
// forward KEY can recover even if the initial KEY's payload failed validation.
// The reader retains neither the packet nor the canonical image.
func (r *Reader) ResolveVideo(valid bool) error {
	if r.pending == nil {
		return ErrNoPendingVideo
	}
	r.state.resolve(*r.pending, valid)
	r.pending = nil
	return nil
}

func (r *Reader) fail(err error) (Packet, error) {
	r.terminal = err
	return Packet{}, err
}

func payloadError(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

func discardPayload(r io.Reader, length uint32) error {
	var scratch [4096]byte
	for length > 0 {
		n := min(length, uint32(len(scratch)))
		if _, err := io.ReadFull(r, scratch[:n]); err != nil {
			return payloadError(err)
		}
		length -= n
	}
	return nil
}

type Writer struct {
	output    io.Writer
	direction Direction
	state     streamState
	terminal  error
}

func NewWriter(output io.Writer, d Direction) (*Writer, error) {
	if !d.valid() {
		return nil, ErrDirection
	}
	return &Writer{output: output, direction: d}, nil
}

// WritePacket validates the entire packet before writing any bytes. Callers
// provide already encoded video; no compression/decompression happens here.
// State commits only after all bytes are written. Any I/O failure is sticky:
// the caller must terminate the session, never retry a partially written packet.
// Validation errors leave state/output unchanged. Positive short writes are
// completed; zero progress or impossible write counts fail without spinning.
func (w *Writer) WritePacket(p Packet) error {
	if w.terminal != nil {
		return w.terminal
	}
	if err := p.Validate(w.direction); err != nil {
		return err
	}
	if err := w.state.check(p.Header, w.direction); err != nil {
		return err
	}
	b, err := EncodeHeader(p.Header)
	if err != nil {
		return err
	}
	if err = writeFull(w.output, b[:]); err == nil {
		err = writeFull(w.output, p.Payload)
	}
	if err != nil {
		w.terminal = fmt.Errorf("protocol: write failed: %w", err)
		return w.terminal
	}
	switch p.Header.Type {
	case SessionConfig:
		w.state.config, _ = DecodeConfig(p.Payload) // Already validated.
		w.state.configured = true
	case Keyframe, Delta:
		w.state.resolve(p.Header, true)
	case End:
		w.terminal = ErrEnded
	}
	return nil
}

func writeFull(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if n < 0 || n > len(b) {
			return io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
