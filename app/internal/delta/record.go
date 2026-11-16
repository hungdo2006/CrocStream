package delta

import (
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
	for i := 3; i < len(b); i += 4 {
		if b[i] != 255 {
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
	dirty := make([]int, 0, count)
	for index := 0; index < count; index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g := tile(c, index)
		sum := 0
		for y := g.y; y < g.y+g.h; y++ {
			for x := g.x; x < g.x+g.w; x++ {
				p := (y*int(c.Width) + x) * 4
				for ch := 0; ch < 3; ch++ {
					diff := int(current[p+ch]) - int(reference[p+ch])
					if diff < 0 {
						diff = -diff
					}
					sum += diff
				}
			}
		}
		// Exact integer comparison: MAE > tau, not >= or truncated division.
		if sum > int(tau)*3*g.w*g.h {
			dirty = append(dirty, index)
		}
	}
	return dirty, nil
}

func encodeRecord(c protocol.Config, current []byte, indices []int) []byte {
	size := 2 + 2*len(indices)
	for _, index := range indices {
		g := tile(c, index)
		size += g.w * g.h * 4
	}
	raw := make([]byte, size)
	binary.BigEndian.PutUint16(raw, uint16(len(indices)))
	pos := 2
	for _, index := range indices {
		binary.BigEndian.PutUint16(raw[pos:pos+2], uint16(index))
		pos += 2
		g := tile(c, index)
		n := g.w * g.h * 4
		copyTile(raw[pos:pos+n], current, c, index, true)
		pos += n
	}
	return raw
}
