package delta

import (
	"bytes"
	"testing"

	"crocstream.local/app/internal/protocol"
)

func FuzzDecodeDelta(f *testing.F) {
	c := configuration(true)
	f.Add([]byte{0, 0}, true)
	f.Add([]byte{0xFF, 0xFF}, false)
	f.Add(encodeRecord(c, solid(c, 17), []int{0, 239}), true)
	f.Fuzz(func(t *testing.T, raw []byte, small bool) {
		c := configuration(small)
		tiles, err := DecodeDelta(bg, c, raw)
		if err != nil {
			return
		}
		limits, _ := c.Limits()
		if len(tiles) > limits.TileCount || len(raw) > limits.DeltaBytes {
			t.Fatal("record bounds")
		}
		last := -1
		for _, tile := range tiles {
			if int(tile.Index) <= last || int(tile.Index) >= limits.TileCount {
				t.Fatal("index bounds/order")
			}
			last = int(tile.Index)
		}
	})
}

func FuzzCompressedFrame(f *testing.F) {
	for _, raw := range [][]byte{{0, 0}, bytes.Repeat([]byte{1}, 4096)} {
		data, err := compress(bg, raw)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data, uint16(len(raw)))
	}
	f.Add([]byte{}, uint16(0))
	f.Add([]byte{4, 34, 77, 24, 0x60, 0x40, 0, 0, 0, 0, 0}, uint16(10))
	f.Fuzz(func(t *testing.T, data []byte, budget uint16) {
		out, err := decompress(bg, data, int(budget))
		if err == nil && len(out) > int(budget) {
			t.Fatal("decompression escaped budget")
		}
	})
}

func FuzzDecoderAtomicity(f *testing.F) {
	c := configuration(true)
	key := packet(f, protocol.Keyframe, 1, solid(c, 9))
	f.Add(packet(f, protocol.Delta, 2, []byte{0, 0}).Payload, byte(protocol.Delta), uint32(2))
	f.Add(key.Payload, byte(protocol.Keyframe), uint32(5))
	f.Add([]byte{0}, byte(protocol.Delta), uint32(2))
	f.Fuzz(func(t *testing.T, payload []byte, typ byte, id uint32) {
		if len(payload) > protocol.MaxPayload {
			return
		}
		d := decoder(t, c)
		if _, err := d.Decode(bg, key); err != nil {
			t.Fatal(err)
		}
		before, prior, _ := d.Snapshot()
		p := protocol.Packet{Header: protocol.Header{Type: protocol.Type(typ), FrameID: id, PayloadLength: uint32(len(payload))}, Payload: payload}
		_, err := d.Decode(bg, p)
		after, next, valid := d.Snapshot()
		if err != nil {
			if valid || next != prior || !bytes.Equal(before, after) {
				t.Fatal("failure mutated reference")
			}
		} else if !valid || len(after) != len(before) || !opaque(after) || next != id {
			t.Fatal("accepted output violates invariants")
		}
	})
}
