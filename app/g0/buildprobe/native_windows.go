package main

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A G0 dependency feasibility check, not the production capture adapter.
// Calls only memory-DC operations. Screen APIs are resolved but never invoked.
var (
	user32       = windows.NewLazySystemDLL("user32.dll")
	gdi32        = windows.NewLazySystemDLL("gdi32.dll")
	createDC     = gdi32.NewProc("CreateCompatibleDC")
	deleteDC     = gdi32.NewProc("DeleteDC")
	createDIB    = gdi32.NewProc("CreateDIBSection")
	selectObject = gdi32.NewProc("SelectObject")
	deleteObject = gdi32.NewProc("DeleteObject")
	bitBlt       = gdi32.NewProc("BitBlt")
	gdiFlush     = gdi32.NewProc("GdiFlush")
)

type bitmapHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}

type memoryBitmap struct {
	dc, bitmap, old uintptr
	pixels          []byte
}

func newMemoryBitmap(width, height int32) (*memoryBitmap, error) {
	// This probe deliberately cannot allocate a full-screen surface.
	if width < 1 || height < 1 || width > 16 || height > 16 {
		return nil, errors.New("probe bitmap dimensions out of bounds")
	}
	b := &memoryBitmap{}
	b.dc, _, _ = createDC.Call(0)
	if b.dc == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	info := struct {
		Header bitmapHeader
		Color  uint32
	}{Header: bitmapHeader{Size: 40, Width: width, Height: -height,
		Planes: 1, BitCount: 32, SizeImage: uint32(width * height * 4)}}
	var bits unsafe.Pointer
	b.bitmap, _, _ = createDIB.Call(b.dc, uintptr(unsafe.Pointer(&info)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if b.bitmap == 0 || bits == nil {
		_ = b.close()
		return nil, errors.New("CreateDIBSection failed")
	}
	b.old, _, _ = selectObject.Call(b.dc, b.bitmap)
	if b.old == 0 || b.old == ^uintptr(0) {
		b.old = 0
		_ = b.close()
		return nil, errors.New("SelectObject failed")
	}
	b.pixels = unsafe.Slice((*byte)(bits), int(width*height*4))
	return b, nil
}

func (b *memoryBitmap) close() error {
	var errs []error
	// No Go slice may be used after DeleteObject frees its Win32 backing memory.
	b.pixels = nil
	if b.old != 0 {
		if r, _, _ := selectObject.Call(b.dc, b.old); r == 0 || r == ^uintptr(0) {
			errs = append(errs, errors.New("restore bitmap failed"))
		}
		b.old = 0
	}
	if b.bitmap != 0 {
		if r, _, _ := deleteObject.Call(b.bitmap); r == 0 {
			errs = append(errs, errors.New("DeleteObject failed"))
		}
		b.bitmap = 0
	}
	if b.dc != 0 {
		if r, _, _ := deleteDC.Call(b.dc); r == 0 {
			errs = append(errs, errors.New("DeleteDC failed"))
		}
		b.dc = 0
	}
	return errors.Join(errs...)
}

func syntheticCopy() (err error) {
	source, err := newMemoryBitmap(2, 2)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.close()) }()
	target, err := newMemoryBitmap(2, 2)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.close()) }()
	// BGRA DIB pixels are generated locally, not read from any display.
	want := []byte{0, 0, 255, 255, 0, 255, 0, 255, 255, 0, 0, 255, 255, 255, 255, 255}
	copy(source.pixels, want)
	clear(target.pixels)
	const srcCopy = 0x00CC0020
	if r, _, _ := bitBlt.Call(target.dc, 0, 0, 2, 2, source.dc, 0, 0, srcCopy); r == 0 {
		return errors.New("synthetic BitBlt failed")
	}
	if r, _, _ := gdiFlush.Call(); r == 0 {
		return errors.New("GdiFlush failed")
	}
	if !bytes.Equal(target.pixels, want) {
		return errors.New("synthetic DIB pixels differ")
	}
	return nil
}

func verifyNativeCapture() error {
	for _, proc := range []*windows.LazyProc{
		createDC, deleteDC, createDIB, selectObject, deleteObject, bitBlt, gdiFlush,
		user32.NewProc("GetDC"), user32.NewProc("ReleaseDC"),
		user32.NewProc("EnumDisplayMonitors"), user32.NewProc("GetMonitorInfoW"),
	} {
		if err := proc.Find(); err != nil {
			return fmt.Errorf("required Windows API %s unavailable", proc.Name)
		}
	}
	return syntheticCopy()
}
