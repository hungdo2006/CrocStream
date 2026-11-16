package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestConfigFixture(t *testing.T) {
	fixture := mustHex(t, "01050002d020010118001e")
	c, err := DecodeConfig(fixture)
	if err != nil || c != DefaultConfig() {
		t.Fatal(c, err)
	}
	b, err := EncodeConfig(c)
	if err != nil || !bytes.Equal(b[:], fixture) {
		t.Fatal(err)
	}
}

func TestConfigSupportedValuesAndLimits(t *testing.T) {
	for _, resolution := range []struct {
		w, h uint16
		want Limits
	}{
		{640, 360, Limits{921600, 240, 922082}},
		{1280, 720, Limits{3686400, 920, 3688242}},
	} {
		for _, fps := range []uint8{15, 24, 30} {
			for _, interval := range []uint16{1, 30, 31, 120} {
				c := DefaultConfig()
				c.Width, c.Height = resolution.w, resolution.h
				c.FPS, c.KeyInterval = fps, interval
				b, err := EncodeConfig(c)
				if err != nil {
					t.Fatal(err)
				}
				got, err := DecodeConfig(b[:])
				if err != nil || got != c {
					t.Fatal(got, err)
				}
				limits, err := c.Limits()
				if err != nil || limits != resolution.want {
					t.Fatal(limits, err)
				}
			}
		}
	}
}

func TestConfigRejectsUnsupportedValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"version0", func(c *Config) { c.ProtocolVersion = 0 }},
		{"version2", func(c *Config) { c.ProtocolVersion = 2 }},
		{"zeroWidth", func(c *Config) { c.Width = 0 }},
		{"zeroHeight", func(c *Config) { c.Height = 0 }},
		{"mixed640x720", func(c *Config) { c.Width = 640 }},
		{"mixed1280x360", func(c *Config) { c.Height = 360 }},
		{"unsupportedWidth", func(c *Config) { c.Width = 1920 }},
		{"unsupportedHeight", func(c *Config) { c.Height = 1080 }},
		{"overflowIfNarrow", func(c *Config) { c.Width, c.Height = 65535, 65535 }},
		{"tile0", func(c *Config) { c.TileSize = 0 }},
		{"tile16", func(c *Config) { c.TileSize = 16 }},
		{"pixel", func(c *Config) { c.PixelFormat = 2 }},
		{"compression", func(c *Config) { c.Compression = 2 }},
		{"fps0", func(c *Config) { c.FPS = 0 }},
		{"fps25", func(c *Config) { c.FPS = 25 }},
		{"interval0", func(c *Config) { c.KeyInterval = 0 }},
		{"interval121", func(c *Config) { c.KeyInterval = 121 }},
		{"interval65535", func(c *Config) { c.KeyInterval = 65535 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultConfig()
			tt.mutate(&c)
			if _, err := EncodeConfig(c); !errors.Is(err, ErrConfig) {
				t.Fatal(err)
			}
			if _, err := c.Limits(); !errors.Is(err, ErrConfig) {
				t.Fatal(err)
			}
		})
	}
	// Mutate wire bytes independently of the encoder validation.
	fixture := mustHex(t, "01050002d020010118001e")
	for i := range fixture {
		b := bytes.Clone(fixture)
		b[i] = 0xFF
		if _, err := DecodeConfig(b); !errors.Is(err, ErrConfig) {
			t.Fatalf("wire offset %d: %v", i, err)
		}
	}
	for n := 0; n < ConfigSize; n++ {
		if _, err := DecodeConfig(fixture[:n]); !errors.Is(err, ErrLength) {
			t.Fatal(err)
		}
	}
	if _, err := DecodeConfig(append(fixture, 0)); !errors.Is(err, ErrLength) {
		t.Fatal(err)
	}
}
