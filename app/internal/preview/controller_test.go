package preview

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"crocstream.local/app/internal/capture"
)

type fakeSource struct {
	starts, closes atomic.Int32
	fail           bool
	gate           chan struct{}
	cleanupError   error
}

func (f *fakeSource) Displays(context.Context) ([]capture.Display, error) {
	return []capture.Display{{ID: "test", Width: 640, Height: 360}}, nil
}
func (f *fakeSource) Run(ctx context.Context, o capture.Options, deliver func(capture.Frame)) error {
	f.starts.Add(1)
	defer f.closes.Add(1)
	if f.fail {
		return capture.ErrSource
	}
	pixels := make([]byte, int(o.Width)*int(o.Height)*4)
	for i := 3; i < len(pixels); i += 4 {
		pixels[i] = 255
	}
	deliver(capture.Frame{RGBA: pixels, Width: int(o.Width), Height: int(o.Height), At: time.Now(), Duration: time.Millisecond})
	<-ctx.Done()
	if f.gate != nil {
		<-f.gate
	}
	return errors.Join(ctx.Err(), f.cleanupError)
}
func TestNoCaptureBeforeStartAnd100Lifecycles(t *testing.T) {
	source := &fakeSource{}
	c, err := New(source, "http://wails.localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Shutdown()
	if _, err = c.Displays(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.starts.Load() != 0 || c.Stats().State != Idle {
		t.Fatal("captured before consent")
	}
	tokens := map[string]bool{}
	for i := 0; i < 100; i++ {
		start, err := c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24})
		if err != nil {
			t.Fatal(err)
		}
		if tokens[start.Endpoint.Token] || len(start.Endpoint.Token) != 64 {
			t.Fatal("token reused")
		}
		tokens[start.Endpoint.Token] = true
		if _, err := c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24}); err == nil {
			t.Fatal("duplicate Start")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if err := c.Stop(ctx); err != nil {
			t.Fatal(err)
		}
		cancel()
		if c.Stats().State != Idle {
			t.Fatal(c.Stats())
		}
	}
	if source.starts.Load() != 100 || source.closes.Load() != 100 {
		t.Fatal("resource leak")
	}
	if err := c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24}); err == nil {
		t.Fatal("start after shutdown")
	}
}
func TestMissingSourceAndStopTimeout(t *testing.T) {
	source := &fakeSource{fail: true}
	c, _ := New(source, "http://wails.localhost")
	_, err := c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24})
	if err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	s := c.active
	c.mu.Unlock()
	if s != nil {
		select {
		case <-s.done:
		case <-time.After(time.Second):
			t.Fatal("failure did not clean up")
		}
	}
	if c.Active() || c.Stats().State != Failed {
		t.Fatal("failed preview did not release session resources")
	}
	c.Shutdown()
	source = &fakeSource{gate: make(chan struct{})}
	c, _ = New(source, "http://wails.localhost")
	defer c.Shutdown()
	if _, err = c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = c.Stop(ctx); err == nil {
		t.Fatal("hung native falsely stopped")
	}
	if !c.Active() || c.Stats().State != Stopping {
		t.Fatal("timed-out stop must retain active session ownership")
	}
	close(source.gate)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = c.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if c.Active() {
		t.Fatal("stopped preview still owns the session")
	}
}
func TestConfigRejectedWithoutCapture(t *testing.T) {
	s := &fakeSource{}
	c, _ := New(s, "http://wails.localhost")
	defer c.Shutdown()
	for _, o := range []capture.Options{{}, {DisplayID: "test", Width: 640, Height: 720, FPS: 24}, {DisplayID: "test", Width: 640, Height: 360, FPS: 255}} {
		if _, err := c.Start(o); err == nil {
			t.Fatal("invalid configuration")
		}
	}
	if s.starts.Load() != 0 {
		t.Fatal("invalid capture")
	}
}

func TestBridgeClosureEndsLocalPreview(t *testing.T) {
	source := &fakeSource{}
	c, err := New(source, "http://wails.localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Shutdown()
	if _, err := c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24}); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	s := c.active
	c.mu.Unlock()
	s.bridge.Close()
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("capture did not stop when its bridge closed")
	}
	if c.Stats().State != Idle || source.closes.Load() != 1 {
		t.Fatal("normal bridge closure did not release capture cleanly")
	}
}

func TestShutdownActiveSessionAndCleanupError(t *testing.T) {
	for _, cleanupError := range []error{nil, capture.ErrCapture} {
		source := &fakeSource{cleanupError: cleanupError}
		c, err := New(source, "http://wails.localhost")
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Start(capture.Options{DisplayID: "test", Width: 640, Height: 360, FPS: 24})
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Shutdown(); err != nil {
			t.Fatal(err)
		}
		if source.closes.Load() != 1 {
			t.Fatal("shutdown returned before source cleanup")
		}
		want := Idle
		if cleanupError != nil {
			want = Failed
		}
		if c.Stats().State != want {
			t.Fatal("cleanup error hidden or false failure", c.Stats())
		}
	}
}
