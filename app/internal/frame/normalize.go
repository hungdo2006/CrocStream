// Package frame normalizes straight (not premultiplied) native pixel samples
// into tightly packed, opaque RGBA. It performs no capture or color conversion.
package frame

import (
	"context"
	"errors"

	"crocstream.local/app/internal/protocol"
)

type Order uint8

const (
	RGBA Order = iota + 1
	BGRA
)

var ErrLayout = errors.New("frame: invalid pixel layout")

// Normalize copies the configured rectangle, skips row padding, swaps B/R for
// BGRA and forces alpha=255. src is borrowed during the call; output is owned by
// the caller. Bottom-up/negative-stride or premultiplied inputs must first be
// adapted by capture. Bounds are checked before allocation, without multiplying
// an untrusted stride (which could overflow).
func Normalize(ctx context.Context, c protocol.Config, src []byte, stride int, order Order) ([]byte, error) {
	limits, err := c.Limits()
	if err != nil {
		return nil, err
	}
	w, h := int(c.Width), int(c.Height)
	row := w * 4
	if (order != RGBA && order != BGRA) || stride < row ||
		len(src) < row || stride > (len(src)-row)/(h-1) {
		return nil, ErrLayout
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dst := make([]byte, limits.RawBytes)
	if order == BGRA {
		for y := 0; y < h; y++ {
			if y&31 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			sRow := src[y*stride : y*stride+row]
			dRow := dst[y*row : y*row+row]
			_ = sRow[row-1]
			_ = dRow[row-1]
			for x := 0; x < row; x += 4 {
				dRow[x] = sRow[x+2]
				dRow[x+1] = sRow[x+1]
				dRow[x+2] = sRow[x]
				dRow[x+3] = 255
			}
		}
	} else {
		for y := 0; y < h; y++ {
			if y&31 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			sRow := src[y*stride : y*stride+row]
			dRow := dst[y*row : y*row+row]
			copy(dRow, sRow)
			_ = dRow[row-1]
			for x := 3; x < row; x += 4 {
				dRow[x] = 255
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return dst, nil
}
