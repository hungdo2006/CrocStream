package protocol

import (
	"encoding/binary"
	"fmt"
)

const (
	Version             = 1
	TileSize            = 32
	PixelRGBA           = 1
	CompressionLZ4Frame = 1
	DefaultKeyInterval  = 30
	MaxKeyInterval      = 120
)

type Config struct {
	ProtocolVersion uint8
	Width           uint16
	Height          uint16
	TileSize        uint8
	PixelFormat     uint8
	Compression     uint8
	FPS             uint8
	KeyInterval     uint16
}

func DefaultConfig() Config {
	return Config{Version, 1280, 720, TileSize, PixelRGBA, CompressionLZ4Frame, 24, DefaultKeyInterval}
}

func (c Config) Validate() error {
	if c.ProtocolVersion != Version {
		return fmt.Errorf("%w: unsupported version", ErrConfig)
	}
	if !((c.Width == 640 && c.Height == 360) || (c.Width == 1280 && c.Height == 720)) {
		return fmt.Errorf("%w: unsupported resolution pair", ErrConfig)
	}
	if c.TileSize != TileSize || c.PixelFormat != PixelRGBA || c.Compression != CompressionLZ4Frame {
		return fmt.Errorf("%w: unsupported image format", ErrConfig)
	}
	if c.FPS != 15 && c.FPS != 24 && c.FPS != 30 {
		return fmt.Errorf("%w: unsupported FPS", ErrConfig)
	}
	if c.KeyInterval < 1 || c.KeyInterval > MaxKeyInterval {
		return fmt.Errorf("%w: unsupported key interval", ErrConfig)
	}
	return nil
}

// Limits are decompressed-size budgets for the future codec, not allocations.
type Limits struct {
	RawBytes   int
	TileCount  int
	DeltaBytes int
}

func (c Config) Limits() (Limits, error) {
	if err := c.Validate(); err != nil {
		return Limits{}, err
	}
	// Widen before multiplying even though validated v1 pairs already fit.
	w, h := uint64(c.Width), uint64(c.Height)
	raw := w * h * 4
	tiles := ((w + TileSize - 1) / TileSize) * ((h + TileSize - 1) / TileSize)
	delta := 2 + 2*tiles + raw
	if raw > MaxRaw || delta > MaxPayload {
		return Limits{}, ErrLength
	}
	return Limits{int(raw), int(tiles), int(delta)}, nil
}

func EncodeConfig(c Config) ([ConfigSize]byte, error) {
	var b [ConfigSize]byte
	if err := c.Validate(); err != nil {
		return b, err
	}
	b[0] = c.ProtocolVersion
	binary.BigEndian.PutUint16(b[1:3], c.Width)
	binary.BigEndian.PutUint16(b[3:5], c.Height)
	b[5], b[6], b[7], b[8] = c.TileSize, c.PixelFormat, c.Compression, c.FPS
	binary.BigEndian.PutUint16(b[9:11], c.KeyInterval)
	return b, nil
}

func DecodeConfig(b []byte) (Config, error) {
	if len(b) != ConfigSize {
		return Config{}, ErrLength
	}
	c := Config{
		ProtocolVersion: b[0],
		Width:           binary.BigEndian.Uint16(b[1:3]),
		Height:          binary.BigEndian.Uint16(b[3:5]),
		TileSize:        b[5],
		PixelFormat:     b[6],
		Compression:     b[7],
		FPS:             b[8],
		KeyInterval:     binary.BigEndian.Uint16(b[9:11]),
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}
