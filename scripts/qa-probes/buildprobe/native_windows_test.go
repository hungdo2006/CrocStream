package main

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeBitmapBoundsAndABI(t *testing.T) {
	if unsafe.Sizeof(bitmapHeader{}) != 40 {
		t.Fatal("BITMAPINFOHEADER ABI mismatch")
	}
	for _, size := range [][2]int32{{0, 2}, {2, 0}, {-1, 2}, {17, 2}, {2, 17}} {
		if b, err := newMemoryBitmap(size[0], size[1]); err == nil || b != nil {
			t.Fatal("invalid geometry allocated a native surface")
		}
	}
}

func TestSyntheticNativeCopyAndCleanup(t *testing.T) {
	getResources := user32.NewProc("GetGuiResources")
	count := func() uintptr {
		n, _, _ := getResources.Call(uintptr(windows.CurrentProcess()), 0)
		return n
	}
	before := count()
	for i := 0; i < 100; i++ {
		if err := verifyNativeCapture(); err != nil {
			t.Fatal(err)
		}
	}
	after := count()
	if after != before {
		t.Fatalf("GDI handle count changed: %d -> %d", before, after)
	}
	t.Logf("100 synthetic copies, GDI handles %d -> %d; no screen captured", before, after)
}
