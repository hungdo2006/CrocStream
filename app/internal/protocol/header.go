package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	Magic      = 0xC5
	HeaderSize = 18
	ConfigSize = 11
	MaxPayload = 8 * 1024 * 1024
	MaxRaw     = 1280 * 720 * 4
)

type Type uint8

const (
	Keyframe        Type = 0x01
	Delta           Type = 0x02
	RequestKeyframe Type = 0x03
	SessionConfig   Type = 0x04
	Ping            Type = 0x05
	Pong            Type = 0x06
	End             Type = 0xFF
)

// Direction identifies the sender of this half of the duplex stream.
type Direction uint8

const (
	FromBroadcaster Direction = iota + 1
	FromViewer
)

var (
	ErrHeader           = errors.New("protocol: invalid header")
	ErrType             = errors.New("protocol: unknown packet type")
	ErrLength           = errors.New("protocol: invalid length")
	ErrDirection        = errors.New("protocol: invalid direction")
	ErrConfig           = errors.New("protocol: invalid configuration")
	ErrConfigRequired   = errors.New("protocol: CONFIG required first")
	ErrDuplicateConfig  = errors.New("protocol: duplicate CONFIG")
	ErrSequence         = errors.New("protocol: first video must be KEY id1")
	ErrNeedKeyframe     = errors.New("protocol: reference invalid; request KEY")
	ErrFrameIDExhausted = errors.New("protocol: frame IDs exhausted; start a new session")
	ErrPendingVideo     = errors.New("protocol: resolve previous video before reading")
	ErrNoPendingVideo   = errors.New("protocol: no video to resolve")
	ErrEnded            = errors.New("protocol: stream ended")
)

type Header struct {
	Type          Type
	FrameID       uint32
	TimestampMs   int64
	PayloadLength uint32
}

func (t Type) video() bool { return t == Keyframe || t == Delta }

func (d Direction) valid() bool { return d == FromBroadcaster || d == FromViewer }

func (h Header) envelope() error {
	switch h.Type {
	case Keyframe, Delta, RequestKeyframe, SessionConfig, Ping, Pong, End:
	default:
		return ErrType
	}
	if h.PayloadLength > MaxPayload {
		return ErrLength
	}
	return nil
}

// EncodeHeader encodes only the envelope, including the header-only zero-payload
// KEY fixture in TEST_PLAN. Use Validate for packet-level rules.
func EncodeHeader(h Header) ([HeaderSize]byte, error) {
	var b [HeaderSize]byte
	if err := h.envelope(); err != nil {
		return b, err
	}
	b[0], b[1] = Magic, byte(h.Type)
	binary.BigEndian.PutUint32(b[2:6], h.FrameID)
	binary.BigEndian.PutUint64(b[6:14], uint64(h.TimestampMs))
	binary.BigEndian.PutUint32(b[14:18], h.PayloadLength)
	return b, nil
}

// DecodeHeader requires exactly HeaderSize bytes; it does not scan for magic.
// Direction, control fields and per-type lengths are checked by Validate.
func DecodeHeader(b []byte) (Header, error) {
	if len(b) != HeaderSize {
		return Header{}, ErrLength
	}
	if b[0] != Magic {
		return Header{}, ErrHeader
	}
	h := Header{
		Type:          Type(b[1]),
		FrameID:       binary.BigEndian.Uint32(b[2:6]),
		TimestampMs:   int64(binary.BigEndian.Uint64(b[6:14])),
		PayloadLength: binary.BigEndian.Uint32(b[14:18]),
	}
	if err := h.envelope(); err != nil {
		return Header{}, err
	}
	return h, nil
}

// Validate checks all packet header rules before a reader allocates a payload.
func (h Header) Validate(d Direction) error {
	if err := h.envelope(); err != nil {
		return err
	}
	if !d.valid() {
		return ErrDirection
	}
	if (h.Type.video() || h.Type == SessionConfig) && d != FromBroadcaster {
		return ErrDirection
	}
	if h.Type == RequestKeyframe && d != FromViewer {
		return ErrDirection
	}
	if h.Type.video() {
		if h.FrameID == 0 {
			return fmt.Errorf("%w: zero video ID", ErrHeader)
		}
		if h.PayloadLength == 0 {
			return ErrLength
		}
		return nil
	}
	if h.FrameID != 0 || h.TimestampMs != 0 {
		return fmt.Errorf("%w: nonzero control fields", ErrHeader)
	}
	var size uint32
	switch h.Type {
	case SessionConfig:
		size = ConfigSize
	case Ping, Pong:
		size = 8
	}
	if h.PayloadLength != size {
		return ErrLength
	}
	return nil
}

// NextFrameID never wraps. On exhaustion the session must reconnect, with fresh
// authentication/CONFIG/reference state, rather than reset IDs on this stream.
func NextFrameID(last uint32) (uint32, error) {
	if last == ^uint32(0) {
		return 0, ErrFrameIDExhausted
	}
	return last + 1, nil
}
