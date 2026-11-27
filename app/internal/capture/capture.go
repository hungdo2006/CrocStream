// Package capture owns display capture independently of codec, transport and UI.
package capture

import (
	"context"
	"errors"
	"time"

	"crocstream.local/app/internal/protocol"
)

var (
	ErrSource  = errors.New("Màn hình đã mất hoặc thay đổi. Hãy chọn lại nguồn.")
	ErrCapture = errors.New("Không thể chụp màn hình Windows. Kiểm tra phiên desktop và quyền truy cập.")
)

const (
	KindDisplay = "display"
	KindWindow  = "window"
)

type Display struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Primary   bool   `json:"primary"`
	Minimized bool   `json:"minimized"`
}

type Options struct {
	DisplayID   string `json:"displayID"`
	Width       uint16 `json:"width"`
	Height      uint16 `json:"height"`
	FPS         uint8  `json:"fps"`
	Tau         uint8  `json:"tau"`
	KeyInterval uint16 `json:"keyInterval"`
}

func (o Options) Config() (protocol.Config, error) {
	c := protocol.DefaultConfig()
	c.Width, c.Height, c.FPS, c.KeyInterval = o.Width, o.Height, o.FPS, o.KeyInterval
	if c.KeyInterval == 0 {
		c.KeyInterval = protocol.DefaultKeyInterval
	}
	if o.DisplayID == "" {
		return c, ErrSource
	}
	return c, c.Validate()
}

type Frame struct {
	RGBA          []byte // Owned by recipient; capture never reuses it.
	Width, Height int
	At            time.Time
	Duration      time.Duration
}

// System.Run must be called only after explicit Start consent. It returns only
// after native resources have been released. deliver must not block.
type System interface {
	Displays(context.Context) ([]Display, error)
	Run(context.Context, Options, func(Frame)) error
}

// Fit preserves source aspect ratio, centered within the configured frame.
func Fit(sw, sh, dw, dh int) (x, y, w, h int) {
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return
	}
	if int64(sw)*int64(dh) > int64(sh)*int64(dw) {
		w = dw
		h = max(1, int(int64(sh)*int64(dw)/int64(sw)))
	} else {
		h = dh
		w = max(1, int(int64(sw)*int64(dh)/int64(sh)))
	}
	return (dw - w) / 2, (dh - h) / 2, w, h
}
