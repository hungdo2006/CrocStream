package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func FuzzDecodeHeader(f *testing.F) {
	f.Add([]byte{})
	f.Add(mustHex(f, "c50100000001000000000000000000000000"))
	f.Add(mustHex(f, "c50500000000000000000000000000000008"))
	f.Add(mustHex(f, "c501ffffffffffffffffffffffffffffff"))
	f.Fuzz(func(t *testing.T, b []byte) {
		h, err := DecodeHeader(b)
		if err != nil {
			return
		}
		if len(b) != HeaderSize || h.PayloadLength > MaxPayload {
			t.Fatal("decoder escaped bounds")
		}
		encoded, err := EncodeHeader(h)
		if err != nil || !bytes.Equal(encoded[:], b) {
			t.Fatal("header roundtrip changed bytes")
		}
	})
}

func FuzzDecodeConfig(f *testing.F) {
	f.Add([]byte{})
	f.Add(mustHex(f, "01050002d020010118001e"))
	f.Add(mustHex(f, "01028001682001010f0078"))
	f.Add(mustHex(f, "01ffffffff20010118001e"))
	f.Fuzz(func(t *testing.T, b []byte) {
		c, err := DecodeConfig(b)
		if err != nil {
			return
		}
		encoded, err := EncodeConfig(c)
		if err != nil || !bytes.Equal(encoded[:], b) {
			t.Fatal("CONFIG roundtrip changed bytes")
		}
		limits, err := c.Limits()
		if err != nil || limits.RawBytes <= 0 || limits.RawBytes > MaxRaw ||
			limits.DeltaBytes > MaxPayload || limits.TileCount > 920 {
			t.Fatal("CONFIG escaped resource bounds")
		}
	})
}

func FuzzDecodePacket(f *testing.F) {
	for _, p := range []Packet{configPacket(f), video(Keyframe, 1), video(Delta, 2), control(RequestKeyframe), control(Ping), control(Pong), control(End)} {
		f.Add(wire(f, p), byte(FromBroadcaster))
		f.Add(wire(f, p), byte(FromViewer))
	}
	f.Add([]byte{}, byte(0))
	f.Fuzz(func(t *testing.T, b []byte, direction byte) {
		p, err := DecodePacket(b, Direction(direction))
		if err != nil {
			return
		}
		if len(p.Payload) > MaxPayload || len(b) != HeaderSize+len(p.Payload) {
			t.Fatal("packet escaped bounds")
		}
		if !bytes.Equal(wire(t, p), b) {
			t.Fatal("packet roundtrip changed bytes")
		}
	})
}

func FuzzReadStream(f *testing.F) {
	f.Add(wire(f, configPacket(f), video(Keyframe, 1), video(Delta, 2), control(End)), byte(FromBroadcaster), true)
	f.Add(wire(f, configPacket(f), video(Keyframe, 1), video(Delta, 3), video(Keyframe, 5)), byte(FromBroadcaster), false)
	f.Add(wire(f, control(Ping), control(RequestKeyframe), control(End)), byte(FromViewer), true)
	f.Add([]byte{}, byte(0), false)
	f.Fuzz(func(t *testing.T, b []byte, d byte, accept bool) {
		// Bound aggregate work per fuzz case; individual advertised payloads are
		// still allowed up to MaxPayload to exercise malicious length headers.
		if len(b) > 64*1024 {
			return
		}
		r, err := NewReader(bytes.NewReader(b), Direction(d))
		if err != nil {
			return
		}
		for i := 0; i <= len(b)/HeaderSize+1; i++ {
			p, err := r.ReadPacket()
			if errors.Is(err, ErrNeedKeyframe) {
				continue
			}
			if err != nil {
				if _, again := r.ReadPacket(); again != err {
					t.Fatal("terminal reader resumed")
				}
				return
			}
			if len(p.Payload) > MaxPayload {
				t.Fatal("reader escaped payload bound")
			}
			if p.Header.Type.video() {
				// Only models the codec's accept/reject contract.
				resolve(t, r, accept)
			}
		}
		t.Fatal("reader made unbounded progress without consuming headers")
	})
}
