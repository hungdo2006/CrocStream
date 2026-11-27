package capture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"crocstream.local/app/internal/frame"
	"crocstream.local/app/internal/protocol"
	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	gdi32            = windows.NewLazySystemDLL("gdi32.dll")
	dwmapi           = windows.NewLazySystemDLL("dwmapi.dll")
	enumMonitors     = user32.NewProc("EnumDisplayMonitors")
	monitorInfo      = user32.NewProc("GetMonitorInfoW")
	threadDPI        = user32.NewProc("SetThreadDpiAwarenessContext")
	getDC            = user32.NewProc("GetDC")
	releaseDC        = user32.NewProc("ReleaseDC")
	enumWindows      = user32.NewProc("EnumWindows")
	isWindow         = user32.NewProc("IsWindow")
	isWindowVisible  = user32.NewProc("IsWindowVisible")
	isIconic         = user32.NewProc("IsIconic")
	getWindowTextLen = user32.NewProc("GetWindowTextLengthW")
	getWindowText    = user32.NewProc("GetWindowTextW")
	getWindowLongPtr   = user32.NewProc("GetWindowLongPtrW")
	getWindowRect      = user32.NewProc("GetWindowRect")
	getWindowPlacement = user32.NewProc("GetWindowPlacement")
	showWindowAsync    = user32.NewProc("ShowWindowAsync")
	getWindowPID       = user32.NewProc("GetWindowThreadProcessId")
	getShellWindow   = user32.NewProc("GetShellWindow")
	printWindow      = user32.NewProc("PrintWindow")
	dwmGetWindowAttr = dwmapi.NewProc("DwmGetWindowAttribute")
	createDC         = gdi32.NewProc("CreateCompatibleDC")
	deleteDC         = gdi32.NewProc("DeleteDC")
	createDIB        = gdi32.NewProc("CreateDIBSection")
	selectObject     = gdi32.NewProc("SelectObject")
	deleteObject     = gdi32.NewProc("DeleteObject")
	stretchBlt       = gdi32.NewProc("StretchBlt")
	stretchMode      = gdi32.NewProc("SetStretchBltMode")
	setBrushOrgEx    = gdi32.NewProc("SetBrushOrgEx")
	flush            = gdi32.NewProc("GdiFlush")
)

const (
	windowIDPrefix           = "window:"
	gwlExStyle               = ^uintptr(19) // -20
	wsExToolWindow           = 0x00000080
	wsExAppWindow            = 0x00040000
	wsExNoActivate           = 0x08000000
	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14
	pwRenderFullContent      = 0x00000002
	swShowNoActivate         = 4
	halftoneStretchMode      = 4
	maxWindowCaptureDim      = 4096
	maxEnumeratedWindows     = 64
	minWindowWidth           = 120
	minWindowHeight          = 80
	windowRestoreTimeout     = 600 * time.Millisecond
	windowRestorePoll        = 25 * time.Millisecond
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type windowPlacement struct {
	Length         uint32
	Flags          uint32
	ShowCmd        uint32
	MinPosition    point
	MaxPosition    point
	NormalPosition rect
}
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

type windowEnumeration struct {
	windows []Display
	selfPID uint32
	shell   uintptr
}

var (
	enumStates    sync.Map
	enumID        atomic.Uintptr
	winEnumStates sync.Map
	winEnumID     atomic.Uintptr
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
	index := len(state.screens) + 1
	state.screens = append(state.screens, screen{
		display: Display{
			ID:      windows.UTF16ToString(info.Device[:]),
			Kind:    KindDisplay,
			Name:    fmt.Sprintf("Màn hình %d", index),
			Width:   w,
			Height:  h,
			Primary: info.Flags&1 != 0,
		},
		bounds: info.Bounds,
	})
	return 1
})

var enumWindowsCallback = windows.NewCallback(func(hwnd, param uintptr) uintptr {
	value, ok := winEnumStates.Load(param)
	if !ok {
		return 0
	}
	state := value.(*windowEnumeration)
	if len(state.windows) >= maxEnumeratedWindows {
		return 0
	}
	if hwnd == 0 || hwnd == state.shell {
		return 1
	}
	if vis, _, _ := isWindowVisible.Call(hwnd); vis == 0 {
		return 1
	}
	iconic, _, _ := isIconic.Call(hwnd)
	minimized := iconic != 0
	var pid uint32
	getWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 || pid == state.selfPID {
		return 1
	}
	exStyle, _, _ := getWindowLongPtr.Call(hwnd, gwlExStyle)
	if (exStyle&wsExToolWindow != 0 && exStyle&wsExAppWindow == 0) || exStyle&wsExNoActivate != 0 {
		return 1
	}
	if isWindowCloaked(hwnd) {
		return 1
	}
	title := readWindowTitle(hwnd)
	if title == "" {
		return 1
	}
	var bounds rect
	var boundsOK bool
	if minimized {
		bounds, boundsOK = minimizedWindowBounds(hwnd)
	} else {
		_, bounds, boundsOK = windowBounds(hwnd)
	}
	if !boundsOK {
		return 1
	}
	w := int(bounds.Right - bounds.Left)
	h := int(bounds.Bottom - bounds.Top)
	if w < minWindowWidth || h < minWindowHeight || w > 32768 || h > 32768 {
		return 1
	}
	state.windows = append(state.windows, Display{
		ID:        formatWindowID(hwnd),
		Kind:      KindWindow,
		Name:      truncateTitle(title, 90),
		Width:     w,
		Height:    h,
		Minimized: minimized,
	})
	return 1
})

func isWindowCloaked(hwnd uintptr) bool {
	if dwmGetWindowAttr.Find() != nil {
		return false
	}
	var cloaked uint32
	hr, _, _ := dwmGetWindowAttr.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
	return hr == 0 && cloaked != 0
}

func readWindowTitle(hwnd uintptr) string {
	length, _, _ := getWindowTextLen.Call(hwnd)
	if length == 0 {
		return ""
	}
	if length > 255 {
		length = 255
	}
	buf := make([]uint16, length+1)
	n, _, _ := getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return strings.TrimSpace(windows.UTF16ToString(buf[:n]))
}

func truncateTitle(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}

func formatWindowID(hwnd uintptr) string {
	return fmt.Sprintf("%s%x", windowIDPrefix, uint64(hwnd))
}

func parseWindowID(id string) (uintptr, bool) {
	if !strings.HasPrefix(id, windowIDPrefix) {
		return 0, false
	}
	hexPart := strings.TrimPrefix(id, windowIDPrefix)
	if hexPart == "" || len(hexPart) > 16 {
		return 0, false
	}
	v, err := strconv.ParseUint(hexPart, 16, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return uintptr(v), true
}

func windowBounds(hwnd uintptr) (winRect, extRect rect, ok bool) {
	if r, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&winRect))); r == 0 {
		return rect{}, rect{}, false
	}
	ww := int(winRect.Right - winRect.Left)
	wh := int(winRect.Bottom - winRect.Top)
	if ww <= 0 || wh <= 0 || ww > 32768 || wh > 32768 {
		return rect{}, rect{}, false
	}
	extRect = winRect
	if dwmGetWindowAttr.Find() == nil {
		var candidate rect
		if hr, _, _ := dwmGetWindowAttr.Call(hwnd, dwmwaExtendedFrameBounds, uintptr(unsafe.Pointer(&candidate)), unsafe.Sizeof(candidate)); hr == 0 {
			ew := int(candidate.Right - candidate.Left)
			eh := int(candidate.Bottom - candidate.Top)
			if ew > 0 && eh > 0 && ew <= 32768 && eh <= 32768 {
				extRect = candidate
			}
		}
	}
	return winRect, extRect, true
}

func minimizedWindowBounds(hwnd uintptr) (rect, bool) {
	if hwnd == 0 {
		return rect{}, false
	}
	wp := windowPlacement{Length: uint32(unsafe.Sizeof(windowPlacement{}))}
	if ok, _, _ := getWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp))); ok == 0 {
		return rect{}, false
	}
	w := int(wp.NormalPosition.Right - wp.NormalPosition.Left)
	h := int(wp.NormalPosition.Bottom - wp.NormalPosition.Top)
	if w <= 0 || h <= 0 || w > 32768 || h > 32768 {
		return rect{}, false
	}
	return wp.NormalPosition, true
}

func orderWindows(list []Display) []Display {
	if len(list) <= 1 {
		return list
	}
	ordered := make([]Display, 0, len(list))
	for _, item := range list {
		if !item.Minimized {
			ordered = append(ordered, item)
		}
	}
	for _, item := range list {
		if item.Minimized {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

// restoreMinimizedWindow restores a taskbar-minimized window once without
// stealing foreground focus so PrintWindow can capture its rendered surface.
func restoreMinimizedWindow(ctx context.Context, hwnd uintptr) {
	if hwnd == 0 || ctx.Err() != nil {
		return
	}
	if iconic, _, _ := isIconic.Call(hwnd); iconic == 0 {
		return
	}
	_, _, _ = showWindowAsync.Call(hwnd, swShowNoActivate)
	timer := time.NewTimer(windowRestoreTimeout)
	defer timer.Stop()
	poll := time.NewTicker(windowRestorePoll)
	defer poll.Stop()
	for {
		if iconic, _, _ := isIconic.Call(hwnd); iconic == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			return
		case <-poll.C:
		}
	}
}

func computeWindowCrop(winRect, extRect rect) (x, y, w, h int) {
	ww := int(winRect.Right - winRect.Left)
	wh := int(winRect.Bottom - winRect.Top)
	if ww <= 0 || wh <= 0 {
		return 0, 0, 0, 0
	}
	cx := int(extRect.Left - winRect.Left)
	cy := int(extRect.Top - winRect.Top)
	cw := int(extRect.Right - extRect.Left)
	ch := int(extRect.Bottom - extRect.Top)
	if cx < 0 || cy < 0 || cw <= 0 || ch <= 0 || cx+cw > ww || cy+ch > wh {
		return 0, 0, ww, wh
	}
	return cx, cy, cw, ch
}

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

func enumerateWindows() []Display {
	shell, _, _ := getShellWindow.Call()
	state := windowEnumeration{
		selfPID: uint32(os.Getpid()),
		shell:   shell,
	}
	id := winEnumID.Add(1)
	winEnumStates.Store(id, &state)
	_, _, _ = enumWindows.Call(enumWindowsCallback, id)
	winEnumStates.Delete(id)
	return orderWindows(state.windows)
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
	windowsList := enumerateWindows()
	result := make([]Display, 0, len(screens)+len(windowsList))
	for _, s := range screens {
		result = append(result, s.display)
	}
	result = append(result, windowsList...)
	return result, nil
}

type surface struct {
	dc, bitmap, old uintptr
	w, h            int
	pixels          []byte
}

func newSurface(screenDC uintptr, w, h int) (*surface, error) {
	return newSurfaceWithBounds(screenDC, w, h, 1280, 720)
}

func newSurfaceWithBounds(screenDC uintptr, w, h, maxW, maxH int) (*surface, error) {
	if w < 1 || h < 1 || w > maxW || h > maxH {
		return nil, ErrCapture
	}
	s := &surface{w: w, h: h}
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
	if ok, _, _ := stretchMode.Call(s.dc, halftoneStretchMode); ok == 0 {
		s.close()
		return nil, ErrCapture
	}
	_, _, _ = setBrushOrgEx.Call(s.dc, 0, 0, 0)
	return s, nil
}

func (s *surface) close() error {
	if s == nil {
		return nil
	}
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
	if strings.HasPrefix(o.DisplayID, windowIDPrefix) {
		hwnd, ok := parseWindowID(o.DisplayID)
		if !ok {
			return ErrSource
		}
		return runWindowCapture(ctx, o, cfg, hwnd, deliver)
	}
	return runMonitorCapture(ctx, o, cfg, deliver)
}

func runMonitorCapture(ctx context.Context, o Options, cfg protocol.Config, deliver func(Frame)) (err error) {
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
			if display.display.ID == chosen.display.ID && display.bounds == chosen.bounds {
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

func runWindowCapture(ctx context.Context, o Options, cfg protocol.Config, hwnd uintptr, deliver func(Frame)) (err error) {
	if valid, _, _ := isWindow.Call(hwnd); valid == 0 {
		return ErrSource
	}
	if vis, _, _ := isWindowVisible.Call(hwnd); vis == 0 {
		return ErrSource
	}
	restoreMinimizedWindow(ctx, hwnd)
	if err = ctx.Err(); err != nil {
		return err
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
	dst, err := newSurface(dc, int(o.Width), int(o.Height))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, dst.close()) }()

	var src *surface
	defer func() { err = errors.Join(err, src.close()) }()

	ticker := time.NewTicker(time.Second / time.Duration(o.FPS))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		start := time.Now()
		if valid, _, _ := isWindow.Call(hwnd); valid == 0 {
			return ErrSource
		}
		if vis, _, _ := isWindowVisible.Call(hwnd); vis == 0 {
			return ErrSource
		}
		// Temporarily minimized windows cannot be rendered by PrintWindow; skip cleanly.
		if iconic, _, _ := isIconic.Call(hwnd); iconic != 0 {
			continue
		}
		winRect, extRect, ok := windowBounds(hwnd)
		if !ok {
			continue
		}
		winW := min(int(winRect.Right-winRect.Left), maxWindowCaptureDim)
		winH := min(int(winRect.Bottom-winRect.Top), maxWindowCaptureDim)
		if winW < 1 || winH < 1 {
			continue
		}
		if src == nil || src.w != winW || src.h != winH {
			if closeErr := src.close(); closeErr != nil {
				return closeErr
			}
			src = nil
			src, err = newSurfaceWithBounds(dc, winW, winH, maxWindowCaptureDim, maxWindowCaptureDim)
			if err != nil {
				return err
			}
		}
		clear(src.pixels)
		if ok, _, _ := printWindow.Call(hwnd, src.dc, pwRenderFullContent); ok == 0 {
			if ok2, _, _ := printWindow.Call(hwnd, src.dc, 0); ok2 == 0 {
				if valid, _, _ := isWindow.Call(hwnd); valid == 0 {
					return ErrSource
				}
				continue
			}
		}
		cropX, cropY, cropW, cropH := computeWindowCrop(winRect, extRect)
		if cropX+cropW > winW {
			cropW = max(1, winW-cropX)
		}
		if cropY+cropH > winH {
			cropH = max(1, winH-cropY)
		}
		dx, dy, dw, dh := Fit(cropW, cropH, int(o.Width), int(o.Height))
		clear(dst.pixels)
		if ok, _, _ := stretchBlt.Call(dst.dc, uintptr(dx), uintptr(dy), uintptr(dw), uintptr(dh), src.dc,
			uintptr(cropX), uintptr(cropY), uintptr(cropW), uintptr(cropH), 0x00CC0020); ok == 0 {
			return ErrCapture
		} // SRCCOPY.
		if ok, _, _ := flush.Call(); ok == 0 {
			return ErrCapture
		}
		pixels, normErr := frame.Normalize(ctx, cfg, dst.pixels, int(o.Width)*4, frame.BGRA)
		if normErr != nil {
			return normErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		deliver(Frame{pixels, int(o.Width), int(o.Height), start, time.Since(start)})
	}
}
