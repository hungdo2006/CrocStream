package session

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/transport"
)

type authFailureFactory struct{ failedFactory }

func (authFailureFactory) Join(_ context.Context, _ string, _ transport.Options, auth func()) (io.ReadWriteCloser, error) {
	auth()
	return nil, errors.New("private-code-token-password")
}

func TestFailureReportsObservedStageWithoutSecret(t *testing.T) {
	for _, tc := range []struct {
		name    string
		factory transport.Factory
		message string
	}{
		{"before-auth", failedFactory{}, "Không thiết lập được phiên."},
		{"during-auth", authFailureFactory{}, "Xác thực peer thất bại"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewController(nil, tc.factory, "http://wails.localhost")
			defer c.Shutdown()
			if _, err := c.Join("private-code", transport.Options{Address: "127.0.0.1:1", Password: "private-password"}); err != nil {
				t.Fatal(err)
			}
			waitFor(t, func() bool { return !c.Active() })
			s := c.Stats()
			if s.State != Failed || !strings.HasPrefix(s.Error, tc.message) || strings.Contains(s.Error, "private") ||
				s.RetryAfterMs <= 0 || s.Captured != 0 || s.Counters.Decoded != 0 {
				t.Fatal("failure stage/privacy/retry measurement")
			}
		})
	}
}

func TestRetryBudgetSnapshot(t *testing.T) {
	c := NewController(nil, failedFactory{}, "http://wails.localhost")
	now := time.Now()
	c.lastJoin = now
	c.joinAttempts = []time.Time{now.Add(-10 * time.Second), now.Add(-8 * time.Second), now.Add(-6 * time.Second), now.Add(-4 * time.Second), now}
	if got := c.retryAfterLocked(now); got < 50000 || got > 50001 {
		t.Fatalf("minute budget: %d", got)
	}
	if got := c.retryAfterLocked(now.Add(51 * time.Second)); got != 0 {
		t.Fatalf("expired budget: %d", got)
	}
	c.joinAttempts = nil
	if got := c.retryAfterLocked(now); got != 2001 {
		t.Fatalf("cooldown: %d", got)
	}
}

func TestWaitingPreviewMeasurementsAndRotation(t *testing.T) {
	source := &syntheticSource{}
	c := NewController(source, transport.Croc{}, "http://wails.localhost")
	defer c.Shutdown()
	opts := relay(t)
	var previous string
	for i := 0; i < 2; i++ {
		s, err := c.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, opts)
		if err != nil {
			t.Fatal(err)
		}
		if s.Endpoint.Token == previous {
			t.Fatal("bridge token reused")
		}
		previous = s.Endpoint.Token
		waitFor(t, func() bool { return c.Stats().Captured > 3 && c.Stats().MeanJPEGMs > 0 })
		stats := c.Stats()
		if stats.State != Waiting || stats.Counters.Emitted != 0 || stats.CaptureFPS == 0 ||
			stats.CaptureQueue > 1 || stats.ImageQueue > 1 || stats.Uptime <= 0 {
			t.Fatal("waiting metrics or bounded queues invalid")
		}
		stopController(t, c)
	}
}

func TestLastFrameKindIsAcceptedMedia(t *testing.T) {
	opts := relay(t)
	host := NewController(&syntheticSource{}, transport.Croc{}, "http://wails.localhost")
	viewer := NewController(nil, transport.Croc{}, "http://wails.localhost")
	defer host.Shutdown()
	defer viewer.Shutdown()
	if _, err := host.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, opts); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return host.Stats().State == Waiting })
	if _, err := viewer.Join(host.Invitation(), opts); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return viewer.Stats().Counters.Decoded > 1 })
	for _, c := range []*Controller{host, viewer} {
		s := c.Stats()
		if s.Counters.LastKind != "KEY" && s.Counters.LastKind != "DELTA" {
			t.Fatal("missing accepted kind")
		}
	}
}

type hungSource struct {
	release chan struct{}
	entered chan struct{}
}

func (s hungSource) Displays(context.Context) ([]capture.Display, error) { return nil, nil }
func (s hungSource) Run(context.Context, capture.Options, func(capture.Frame)) error {
	close(s.entered)
	<-s.release
	return nil
}

func TestStopTimeoutDoesNotDeclareIdle(t *testing.T) {
	s := hungSource{make(chan struct{}), make(chan struct{})}
	c := NewController(s, transport.Croc{}, "http://wails.localhost")
	defer c.Shutdown()
	if _, err := c.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, relay(t)); err != nil {
		t.Fatal(err)
	}
	<-s.entered
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := c.Stop(ctx)
	if err == nil || c.Stats().State != Stopping || !c.Active() || c.Invitation() != "" {
		close(s.release)
		t.Fatal("shutdown timeout hidden")
	}
	close(s.release)
	stopController(t, c)
	if c.Stats().State != Idle {
		t.Fatal("resources not released after source returns")
	}
}
