package frame

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"crocstream.local/app/internal/protocol"
)

func TestNormalizeStrideOrderAndOwnership(t *testing.T) {
	c := protocol.DefaultConfig()
	c.Width, c.Height = 640, 360
	row, stride := int(c.Width)*4, int(c.Width)*4+17
	src := bytes.Repeat([]byte{0xCC}, stride*int(c.Height))
	want := make([]byte, row*int(c.Height))
	for y := 0; y < int(c.Height); y++ {
		for x := 0; x < int(c.Width); x++ {
			s, d := y*stride+x*4, y*row+x*4
			r, g, b := byte(x), byte(y), byte(x+y)
			src[s], src[s+1], src[s+2], src[s+3] = b, g, r, 7
			want[d], want[d+1], want[d+2], want[d+3] = r, g, b, 255
		}
	}
	before := bytes.Clone(src)
	got, err := Normalize(context.Background(), c, src, stride, BGRA)
	if err != nil || !bytes.Equal(got, want) || !bytes.Equal(src, before) {
		t.Fatal("BGRA normalization/ownership", err)
	}
	got[0] ^= 0xFF
	if !bytes.Equal(src, before) {
		t.Fatal("output aliases input")
	}
	got, err = Normalize(context.Background(), c, want, row, RGBA)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("packed RGBA", err)
	}
}

func TestNormalizeRejectsBounds(t *testing.T) {
	c := protocol.DefaultConfig()
	row := int(c.Width) * 4
	pixels := make([]byte, row*int(c.Height))
	for _, stride := range []int{-row, 0, row - 1, int(^uint(0) >> 1)} {
		if _, err := Normalize(context.Background(), c, pixels, stride, RGBA); !errors.Is(err, ErrLayout) {
			t.Fatal(stride, err)
		}
	}
	if _, err := Normalize(context.Background(), c, pixels[:len(pixels)-1], row, RGBA); err == nil {
		t.Fatal("accepted short last row")
	}
	if _, err := Normalize(context.Background(), c, pixels, row, 0); err == nil {
		t.Fatal("accepted unsupported order")
	}
	c.Width = 65535
	if _, err := Normalize(context.Background(), c, nil, row, RGBA); err == nil {
		t.Fatal("accepted unsupported dimensions")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c = protocol.DefaultConfig()
	if _, err := Normalize(ctx, c, pixels, row, RGBA); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
