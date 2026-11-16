package protocol

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHeaderFixtures(t *testing.T) {
	tests := []struct {
		h   Header
		hex string
	}{
		{Header{Type: Keyframe, FrameID: 1}, "c50100000001000000000000000000000000"},
		{Header{Delta, 0x01020304, 0x0102030405060708, 0x00102030}, "c50201020304010203040506070800102030"},
		{Header{Keyframe, 7, -2, 1}, "c50100000007fffffffffffffffe00000001"},
	}
	for _, tt := range tests {
		want := mustHex(t, tt.hex)
		b, err := EncodeHeader(tt.h)
		if err != nil || !bytes.Equal(b[:], want) {
			t.Fatalf("encode fixture: %v", err)
		}
		h, err := DecodeHeader(want)
		if err != nil || h != tt.h {
			t.Fatalf("decode fixture: %v", err)
		}
	}
	if err := tests[0].h.Validate(FromBroadcaster); !errors.Is(err, ErrLength) {
		t.Fatalf("header-only KEY fixture accepted as packet: %v", err)
	}
}

func TestHeaderValidation(t *testing.T) {
	for _, typ := range []Type{Keyframe, Delta, RequestKeyframe, SessionConfig, Ping, Pong, End} {
		var h Header
		h.Type = typ
		switch typ {
		case Keyframe, Delta:
			h.FrameID, h.PayloadLength = 1, 1
		case SessionConfig:
			h.PayloadLength = ConfigSize
		case Ping, Pong:
			h.PayloadLength = 8
		}
		for _, d := range []Direction{FromBroadcaster, FromViewer} {
			allowed := !((typ.video() || typ == SessionConfig) && d == FromViewer) &&
				!(typ == RequestKeyframe && d == FromBroadcaster)
			if err := h.Validate(d); (err == nil) != allowed {
				t.Fatalf("type %x direction %d: %v", typ, d, err)
			}
		}
		d := FromBroadcaster
		if typ == RequestKeyframe {
			d = FromViewer
		}
		bad := h
		bad.PayloadLength = MaxPayload + 1
		if err := bad.Validate(d); !errors.Is(err, ErrLength) {
			t.Fatal(err)
		}
		if typ.video() {
			bad = h
			bad.FrameID = 0
			if err := bad.Validate(d); !errors.Is(err, ErrHeader) {
				t.Fatal(err)
			}
			bad = h
			bad.PayloadLength = MaxPayload
			if err := bad.Validate(d); err != nil {
				t.Fatal(err)
			}
		} else {
			for _, mutate := range []func(*Header){
				func(h *Header) { h.FrameID = 1 },
				func(h *Header) { h.TimestampMs = 1 },
				func(h *Header) { h.TimestampMs = -1 },
				func(h *Header) { h.PayloadLength++ },
			} {
				bad = h
				mutate(&bad)
				if err := bad.Validate(d); err == nil {
					t.Fatalf("invalid control accepted: type %x", typ)
				}
			}
		}
	}
	for _, typ := range []Type{0, 7, 0xFE} {
		if _, err := EncodeHeader(Header{Type: typ}); !errors.Is(err, ErrType) {
			t.Fatal(err)
		}
	}
	if err := (Header{Type: End}).Validate(0); !errors.Is(err, ErrDirection) {
		t.Fatal(err)
	}
}

func TestDecodeHeaderMalformed(t *testing.T) {
	base := mustHex(t, "c50100000001000000000000000000000001")
	for n := 0; n < HeaderSize; n++ {
		if _, err := DecodeHeader(base[:n]); !errors.Is(err, ErrLength) {
			t.Fatalf("length %d: %v", n, err)
		}
	}
	if _, err := DecodeHeader(append(bytes.Clone(base), 0)); !errors.Is(err, ErrLength) {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		offset int
		value  byte
		want   error
	}{
		{0, 0, ErrHeader},
		{1, 7, ErrType},
		{14, 0xFF, ErrLength},
	} {
		b := bytes.Clone(base)
		b[tt.offset] = tt.value
		if _, err := DecodeHeader(b); !errors.Is(err, tt.want) {
			t.Fatal(err)
		}
	}
}

func TestNextFrameID(t *testing.T) {
	for _, last := range []uint32{0, 1, 30, ^uint32(0) - 1} {
		got, err := NextFrameID(last)
		if err != nil || got != last+1 || got == 0 {
			t.Fatalf("next after %d: %d %v", last, got, err)
		}
	}
	if next, err := NextFrameID(^uint32(0)); next != 0 || !errors.Is(err, ErrFrameIDExhausted) {
		t.Fatal(next, err)
	}
}
