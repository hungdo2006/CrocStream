package capture

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeSurfaceABIAndCleanup(t *testing.T) {
	if unsafe.Sizeof(bitmapHeader{}) != 40 || unsafe.Sizeof(monitor{}) != 104 {
		t.Fatal("Win32 ABI mismatch")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	proc := user32.NewProc("GetGuiResources")
	before, _, _ := proc.Call(uintptr(windows.CurrentProcess()), 0)
	for i := 0; i < 100; i++ {
		s, err := newSurface(0, 640, 360)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.pixels) != 640*360*4 {
			t.Fatal("surface bounds")
		}
		if err = s.close(); err != nil {
			t.Fatal(err)
		}
		if err = s.close(); err != nil {
			t.Fatal("idempotent close", err)
		}
	}
	after, _, _ := proc.Call(uintptr(windows.CurrentProcess()), 0)
	if after != before {
		t.Fatal("GDI leak", before, after)
	}
	if _, err := newSurface(0, 65535, 65535); err == nil {
		t.Fatal("unbounded native allocation")
	}
}

func TestWindowIDCropTitleAndInvalidWindowReject(t *testing.T) {
	if id := formatWindowID(0x1a2b); id != "window:1a2b" {
		t.Fatalf("unexpected formatted window ID: %q", id)
	}
	if h, ok := parseWindowID("window:1a2b"); !ok || h != 0x1a2b {
		t.Fatalf("failed to parse valid window ID: %x %v", h, ok)
	}
	for _, bad := range []string{"", `\\.\DISPLAY1`, "window:", "window:0", "window:zzzz", "window:1234567890abcdef1"} {
		if _, ok := parseWindowID(bad); ok {
			t.Fatalf("accepted invalid window ID: %q", bad)
		}
	}
	if got := truncateTitle("  Google Chrome  ", 20); got != "Google Chrome" {
		t.Fatalf("unexpected title: %q", got)
	}
	if got := truncateTitle("1234567890", 6); got != "12345…" {
		t.Fatalf("unexpected truncated title: %q", got)
	}
	// Trim 7px shadow margins on left/right/bottom.
	x, y, w, h := computeWindowCrop(rect{0, 0, 1014, 707}, rect{7, 0, 1007, 700})
	if x != 7 || y != 0 || w != 1000 || h != 700 {
		t.Fatalf("unexpected crop: %d,%d %dx%d", x, y, w, h)
	}
	// Out-of-bounds extended rect falls back to full window rect.
	x, y, w, h = computeWindowCrop(rect{100, 100, 500, 400}, rect{0, 0, 600, 500})
	if x != 0 || y != 0 || w != 400 || h != 300 {
		t.Fatalf("unexpected fallback crop: %d,%d %dx%d", x, y, w, h)
	}
	// Invalid window ID must return ErrSource without starting capture.
	err := (Windows{}).Run(context.Background(), Options{DisplayID: "window:0", Width: 640, Height: 360, FPS: 15}, func(Frame) {})
	if !errors.Is(err, ErrSource) {
		t.Fatalf("expected ErrSource for invalid window ID, got %v", err)
	}
}

func TestActualCapture100Lifecycles(t *testing.T) {
	if os.Getenv("CROCSTREAM_CAPTURE_CONSENT") != "1" {
		t.Skip("real capture requires explicit operator consent")
	}
	source := Windows{}
	displays, err := source.Displays(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	selected := displays[0]
	for _, d := range displays {
		if d.Primary {
			selected = d
			break
		}
	}
	proc := user32.NewProc("GetGuiResources")
	before, _, _ := proc.Call(uintptr(windows.CurrentProcess()), 0)
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		frames := 0
		err := source.Run(ctx, Options{DisplayID: selected.ID, Width: 1280, Height: 720, FPS: 24}, func(f Frame) {
			frames++
			if f.Width != 1280 || f.Height != 720 || len(f.RGBA) != 1280*720*4 {
				t.Error("invalid captured dimensions")
			}
			for p := 3; p < len(f.RGBA); p += 4 {
				if f.RGBA[p] != 255 {
					t.Error("non-opaque capture")
					break
				}
			}
			cancel()
		})
		cancel()
		if !errors.Is(err, context.Canceled) || frames != 1 {
			t.Fatal("capture lifecycle", i, err, frames)
		}
	}
	after, _, _ := proc.Call(uintptr(windows.CurrentProcess()), 0)
	if before != after {
		t.Fatal("actual capture leaked GDI", before, after)
	}
	t.Logf("100 actual Start/one-frame/Stop cycles; source=%dx%d; GDI=%d->%d", selected.Width, selected.Height, before, after)
}
