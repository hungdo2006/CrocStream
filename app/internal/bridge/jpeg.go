package bridge

import (
	"bytes"
	"image"
	"image/jpeg"
)

// EncodeJPEG encodes a packed RGBA snapshot from capture or the media decoder
// for local display.
func EncodeJPEG(pixels []byte, width, height int) ([]byte, error) {
	frame := &image.RGBA{
		Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height),
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, frame, &jpeg.Options{Quality: 75}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
