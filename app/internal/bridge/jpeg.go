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
	out.Grow(64 * 1024)
	if err := jpeg.Encode(&out, frame, &jpeg.Options{Quality: 65}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// EncodePreviewJPEG encodes a lightweight self-preview JPEG for the broadcaster
// so local canvas rendering leaves maximum CPU headroom for network encoding.
// Frames at 1280x720 or larger are downsampled 2x before JPEG encoding.
func EncodePreviewJPEG(pixels []byte, width, height int) ([]byte, error) {
	if width >= 1280 && height >= 720 && len(pixels) >= width*height*4 {
		dw, dh := width/2, height/2
		scaled := make([]byte, dw*dh*4)
		srcStride := width * 4
		dstStride := dw * 4
		for y := 0; y < dh; y++ {
			sRow := pixels[(y*2)*srcStride : (y*2)*srcStride+srcStride]
			dRow := scaled[y*dstStride : y*dstStride+dstStride]
			for x := 0; x < dw; x++ {
				sx, dx := x*8, x*4
				dRow[dx] = sRow[sx]
				dRow[dx+1] = sRow[sx+1]
				dRow[dx+2] = sRow[sx+2]
				dRow[dx+3] = 255
			}
		}
		return EncodeJPEG(scaled, dw, dh)
	}
	return EncodeJPEG(pixels, width, height)
}
