package bridge

import (
	"bytes"
	"image"
	"image/jpeg"
)

// EncodeJPEG prepares a packed RGBA snapshot for local display.
func EncodeJPEG(pixels []byte, width, height int) ([]byte, error) {
	frame := &image.RGBA{
		Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height),
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, frame, &jpeg.Options{Quality: 75}); err != nil {
		return nil, err
	}
	if out.Len() > MaxJPEG {
		return nil, errImageLength
	}
	return out.Bytes(), nil
}
