package delta

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"

	"crocstream.local/app/internal/protocol"
)

var (
	ErrPixels = errors.New("delta: invalid opaque packed RGBA")
	ErrRecord = errors.New("delta: malformed tile record")
)

type geometry struct{ x, y, w, h int }

func tile(c protocol.Config, index int) geometry {
	cols := (int(c.Width) + protocol.TileSize - 1) / protocol.TileSize
	x, y := index%cols*protocol.TileSize, index/cols*protocol.TileSize
	return geometry{x, y, min(protocol.TileSize, int(c.Width)-x), min(protocol.TileSize, int(c.Height)-y)}
}

func opaque(b []byte) bool {
	n := len(b)
	i := 0
	for ; i+15 < n; i += 16 {
		if b[i+3]&b[i+7]&b[i+11]&b[i+15] != 255 {
			return false
		}
	}
	for ; i+3 < n; i += 4 {
		if b[i+3] != 255 {
			return false
		}
	}
	return true
}

// Tile borrows Pixels from the decompressed record. It is only returned after
// the entire record validates; callers must not mutate it while applying it.
type Tile struct {
	Index  uint16
	Pixels []byte
}

// DecodeDelta validates configuration, count, strictly ascending indices, exact
// lengths and alpha before returning any tile. No canonical image is mutated.
func DecodeDelta(ctx context.Context, c protocol.Config, raw []byte) ([]Tile, error) {
	limits, err := c.Limits()
	if err != nil {
		return nil, err
	}
	if len(raw) < 2 || len(raw) > limits.DeltaBytes {
		return nil, ErrRecord
	}
	count := int(binary.BigEndian.Uint16(raw))
	if count > limits.TileCount {
		return nil, ErrRecord
	}
	tiles := make([]Tile, 0, count)
	pos, last := 2, -1
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(raw)-pos < 2 {
			return nil, ErrRecord
		}
		index := int(binary.BigEndian.Uint16(raw[pos : pos+2]))
		pos += 2
		if index <= last || index >= limits.TileCount {
			return nil, ErrRecord
		}
		g := tile(c, index)
		n := g.w * g.h * 4
		if n > len(raw)-pos || !opaque(raw[pos:pos+n]) {
			return nil, ErrRecord
		}
		tiles = append(tiles, Tile{uint16(index), raw[pos : pos+n]})
		pos += n
		last = index
	}
	if pos != len(raw) {
		return nil, ErrRecord
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return tiles, nil
}

func copyTile(dst, src []byte, c protocol.Config, index int, pack bool) {
	g := tile(c, index)
	for y := 0; y < g.h; y++ {
		full := ((g.y+y)*int(c.Width) + g.x) * 4
		packed, n := y*g.w*4, g.w*4
		if pack {
			copy(dst[packed:packed+n], src[full:full+n])
		} else {
			copy(dst[full:full+n], src[packed:packed+n])
		}
	}
}

func dirtyTiles(ctx context.Context, c protocol.Config, current, reference []byte, count int, tau uint8) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dirty := make([]int, 0, count)
	width := int(c.Width)
	height := int(c.Height)
	stride := width * 4
	cols := (width + protocol.TileSize - 1) / protocol.TileSize
	tauInt := int(tau)

	for index := 0; index < count; index++ {
		if index&31 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		gx := (index % cols) * protocol.TileSize
		gy := (index / cols) * protocol.TileSize
		gw := min(protocol.TileSize, width-gx)
		gh := min(protocol.TileSize, height-gy)
		rowBytes := gw * 4
		threshold := tauInt * 3 * gw * gh
		sum := 0
		isDirty := false

		for y := gy; y < gy+gh; y++ {
			off := y*stride + gx*4
			curRow := current[off : off+rowBytes]
			refRow := reference[off : off+rowBytes]
			if bytes.Equal(curRow, refRow) {
				continue
			}
			_ = refRow[rowBytes-1]
			for x := 0; x < rowBytes; x += 4 {
				d0 := int(curRow[x]) - int(refRow[x])
				if d0 < 0 {
					d0 = -d0
				}
				d1 := int(curRow[x+1]) - int(refRow[x+1])
				if d1 < 0 {
					d1 = -d1
				}
				d2 := int(curRow[x+2]) - int(refRow[x+2])
				if d2 < 0 {
					d2 = -d2
				}
				sum += d0 + d1 + d2
			}
			// Exact integer comparison: MAE > tau, monotonic per row so early exit is bit-exact.
			if sum > threshold {
				isDirty = true
				break
			}
		}
		if isDirty {
			dirty = append(dirty, index)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return dirty, nil
}

func encodeRecord(c protocol.Config, current []byte, indices []int) []byte {
	return encodeRecordInto(nil, c, current, indices)
}

func encodeRecordInto(buf []byte, c protocol.Config, current []byte, indices []int) []byte {
	size := 2 + 2*len(indices)
	for _, index := range indices {
		g := tile(c, index)
		size += g.w * g.h * 4
	}
	if cap(buf) < size {
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}
	binary.BigEndian.PutUint16(buf, uint16(len(indices)))
	pos := 2
	for _, index := range indices {
		binary.BigEndian.PutUint16(buf[pos:pos+2], uint16(index))
		pos += 2
		g := tile(c, index)
		n := g.w * g.h * 4
		copyTile(buf[pos:pos+n], current, c, index, true)
		pos += n
	}
	return buf
}
