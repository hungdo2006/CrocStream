package capture

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"crocstream.local/app/internal/frame"
	"golang.org/x/sys/windows"
)

var (
	user32       = windows.NewLazySystemDLL("user32.dll")
	gdi32        = windows.NewLazySystemDLL("gdi32.dll")
	enumMonitors = user32.NewProc("EnumDisplayMonitors")
	monitorInfo  = user32.NewProc("GetMonitorInfoW")
	threadDPI    = user32.NewProc("SetThreadDpiAwarenessContext")
	getDC        = user32.NewProc("GetDC")
	releaseDC    = user32.NewProc("ReleaseDC")
	createDC     = gdi32.NewProc("CreateCompatibleDC")
	deleteDC     = gdi32.NewProc("DeleteDC")
	createDIB    = gdi32.NewProc("CreateDIBSection")
	selectObject = gdi32.NewProc("SelectObject")
	deleteObject = gdi32.NewProc("DeleteObject")
	stretchBlt   = gdi32.NewProc("StretchBlt")
	stretchMode  = gdi32.NewProc("SetStretchBltMode")
	flush        = gdi32.NewProc("GdiFlush")
)

type rect struct{ Left, Top, Right, Bottom int32 }
type monitor struct {
	Size         uint32
	Bounds, Work rect
	Flags        uint32
	Device       [32]uint16
}
type bitmapHeader struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	X, Y                   int32
	Used, Important        uint32
}
type screen struct {
	display Display
	bounds  rect
}
type Windows struct{}

// Callback is allocated once (syscall callbacks cannot be freed). LPARAM is an
// opaque ID into a per-call registry; no Go pointer crosses the callback ABI.
type enumeration struct {
	screens []screen
	failed  bool
}

var (
	enumStates sync.Map
	enumID     atomic.Uintptr
)

var enumCallback = windows.NewCallback(func(handle, dc, bounds, param uintptr) uintptr {
	value, ok := enumStates.Load(param)
	if !ok {
		return 0
	}
	state := value.(*enumeration)
	info := monitor{Size: uint32(unsafe.Sizeof(monitor{}))}
	if ok, _, _ := monitorInfo.Call(handle, uintptr(unsafe.Pointer(&info))); ok == 0 {
		state.failed = true
		return 0
	}
	w, h := int(info.Bounds.Right-info.Bounds.Left), int(info.Bounds.Bottom-info.Bounds.Top)
	if w <= 0 || h <= 0 || w > 32768 || h > 32768 || len(state.screens) >= 16 {
		state.failed = true
		return 0
	}
	state.screens = append(state.screens, screen{Display{windows.UTF16ToString(info.Device[:]), w, h, info.Flags&1 != 0}, info.Bounds})
	return 1
})

func enumerate() ([]screen, error) {
	var state enumeration
	id := enumID.Add(1)
	enumStates.Store(id, &state)
	ok, _, _ := enumMonitors.Call(0, 0, enumCallback, id)
	enumStates.Delete(id)
	if ok == 0 || state.failed || len(state.screens) == 0 {
		return nil, ErrSource
	}
	return state.screens, nil
}

func enterDPI() (func(), error) {
	if err := threadDPI.Find(); err != nil {
		return nil, ErrCapture
	}
	prior, _, _ := threadDPI.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4).
	if prior == 0 {
		return nil, ErrCapture
	}
	return func() { threadDPI.Call(prior) }, nil
}

func (Windows) Displays(ctx context.Context) ([]Display, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore, err := enterDPI()
	if err != nil {
		return nil, err
	}
	defer restore()
	screens, err := enumerate()
	if err != nil {
		return nil, err
	}
	result := make([]Display, len(screens))
	for i, s := range screens {
		result[i] = s.display
	}
	return result, nil
}

type surface struct {
	dc, bitmap, old uintptr
	pixels          []byte
}

func newSurface(screenDC uintptr, w, h int) (*surface, error) {
	if w < 1 || h < 1 || w > 1280 || h > 720 {
		return nil, ErrCapture
	}
	s := &surface{}
	s.dc, _, _ = createDC.Call(screenDC)
	if s.dc == 0 {
		return nil, ErrCapture
	}
	info := struct {
		Header bitmapHeader
		Color  uint32
	}{Header: bitmapHeader{
		Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32, SizeImage: uint32(w * h * 4)}}
	var pixels unsafe.Pointer
	s.bitmap, _, _ = createDIB.Call(s.dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if s.bitmap == 0 || pixels == nil {
		s.close()
		return nil, ErrCapture
	}
	s.old, _, _ = selectObject.Call(s.dc, s.bitmap)
	if s.old == 0 || s.old == ^uintptr(0) {
		s.old = 0
		s.close()
		return nil, ErrCapture
	}
	s.pixels = unsafe.Slice((*byte)(pixels), w*h*4)
	if ok, _, _ := stretchMode.Call(s.dc, 3); ok == 0 {
		s.close()
		return nil, ErrCapture
	} // COLORONCOLOR, deterministic nearest-like GDI scaling.
	return s, nil
}
func (s *surface) close() error {
	s.pixels = nil
	var err error
	if s.old != 0 {
		if ok, _, _ := selectObject.Call(s.dc, s.old); ok == 0 || ok == ^uintptr(0) {
			err = ErrCapture
		}
		s.old = 0
	}
	if s.bitmap != 0 {
		if ok, _, _ := deleteObject.Call(s.bitmap); ok == 0 {
			err = ErrCapture
		}
		s.bitmap = 0
	}
	if s.dc != 0 {
		if ok, _, _ := deleteDC.Call(s.dc); ok == 0 {
			err = ErrCapture
		}
		s.dc = 0
	}
	return err
}

func (Windows) Run(ctx context.Context, o Options, deliver func(Frame)) (err error) {
	cfg, err := o.Config()
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore, err := enterDPI()
	if err != nil {
		return err
	}
	defer restore()
	screens, err := enumerate()
	if err != nil {
		return err
	}
	var chosen *screen
	for i := range screens {
		if screens[i].display.ID == o.DisplayID {
			chosen = &screens[i]
			break
		}
	}
	if chosen == nil {
		return ErrSource
	}
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return ErrCapture
	}
	defer func() {
		if ok, _, _ := releaseDC.Call(0, dc); ok == 0 {
			err = errors.Join(err, ErrCapture)
		}
	}()
	s, err := newSurface(dc, int(o.Width), int(o.Height))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.close()) }()
	x, y, w, h := Fit(chosen.display.Width, chosen.display.Height, int(o.Width), int(o.Height))
	ticker := time.NewTicker(time.Second / time.Duration(o.FPS))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		start := time.Now()
		current, listErr := enumerate()
		if listErr != nil {
			return listErr
		}
		found := false
		for _, display := range current {
			if display == *chosen {
				found = true
				break
			}
		}
		if !found {
			return ErrSource
		}
		clear(s.pixels) // Opaque black letterboxing after Normalize.
		if ok, _, _ := stretchBlt.Call(s.dc, uintptr(x), uintptr(y), uintptr(w), uintptr(h), dc,
			uintptr(chosen.bounds.Left), uintptr(chosen.bounds.Top), uintptr(chosen.display.Width), uintptr(chosen.display.Height),
			0x00CC0020|0x40000000); ok == 0 {
			return ErrCapture
		} // SRCCOPY | CAPTUREBLT.
		if ok, _, _ := flush.Call(); ok == 0 {
			return ErrCapture
		}
		pixels, normErr := frame.Normalize(ctx, cfg, s.pixels, int(o.Width)*4, frame.BGRA)
		if normErr != nil {
			return normErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		deliver(Frame{pixels, int(o.Width), int(o.Height), start, time.Since(start)})
	}
}
