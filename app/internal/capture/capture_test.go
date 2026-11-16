package capture

import "testing"

func TestAspectFitAndConfig(t *testing.T) {
	for _, tt := range []struct{ sw, sh, dw, dh, x, y, w, h int }{
		{1920, 1080, 1280, 720, 0, 0, 1280, 720},
		{1920, 1200, 1280, 720, 64, 0, 1152, 720},
		{1080, 1920, 640, 360, 219, 0, 202, 360},
		{0, 0, 640, 360, 0, 0, 0, 0},
	} {
		x, y, w, h := Fit(tt.sw, tt.sh, tt.dw, tt.dh)
		if x != tt.x || y != tt.y || w != tt.w || h != tt.h {
			t.Fatal(tt, x, y, w, h)
		}
	}
	if _, err := (Options{}).Config(); err == nil {
		t.Fatal("missing source")
	}
}
