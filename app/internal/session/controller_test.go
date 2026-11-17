package session

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/transport"
)

type syntheticSource struct {
	opened, closed atomic.Int32
	fail           bool
}

func (s *syntheticSource) Displays(context.Context) ([]capture.Display, error) {
	return []capture.Display{{ID: "synthetic", Width: 640, Height: 360}}, nil
}
func (s *syntheticSource) Run(ctx context.Context, _ capture.Options, deliver func(capture.Frame)) error {
	s.opened.Add(1)
	defer s.closed.Add(1)
	if s.fail {
		return capture.ErrSource
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			deliver(testFrame(i))
		}
	}
}
func waitFor(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("expected state timeout")
		case <-ticker.C:
		}
	}
}
func stopController(t *testing.T, c *Controller) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestController100SecureLifecycles(t *testing.T) {
	opts := relay(t)
	source := &syntheticSource{}
	host := NewController(source, transport.Croc{}, "http://wails.localhost")
	viewer := NewController(nil, transport.Croc{}, "http://wails.localhost")
	// This verifies secure transport lifecycle, not online invitation-guess limits.
	viewer.joinCooldown = 0
	viewer.joinLimit = 1000
	defer host.Shutdown()
	defer viewer.Shutdown()
	if source.opened.Load() != 0 {
		t.Fatal("capture before Start")
	}
	for i := 0; i < 100; i++ {
		h, err := host.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, opts)
		if err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return host.Stats().State == Waiting })
		code := host.Invitation()
		if code == "" {
			t.Fatal("missing invitation")
		}
		if _, err := host.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, opts); err == nil {
			t.Fatal("overlapping host")
		}
		v, err := viewer.Join(code, opts)
		if err != nil {
			t.Fatal(err)
		}
		if h.Endpoint.Token == v.Endpoint.Token || h.Endpoint.URL == v.Endpoint.URL {
			t.Fatal("role credentials shared")
		}
		waitFor(t, func() bool { return viewer.Stats().Counters.Decoded > 0 })
		if i%2 == 0 {
			stopController(t, host)
			waitFor(t, func() bool { return !viewer.Active() })
			if viewer.Stats().State != Disconnected || !viewer.Stats().Counters.Ended {
				t.Fatal("missing END/disconnect")
			}
		} else {
			stopController(t, viewer)
			waitFor(t, func() bool { return !host.Active() })
		}
		if host.Invitation() != "" {
			t.Fatal("invitation retained")
		}
	}
	if source.opened.Load() != 100 || source.closed.Load() != 100 {
		t.Fatal("capture not released")
	}
}

type blockedFactory struct{}

func (blockedFactory) Host(ctx context.Context, _ transport.Options, _ func()) (transport.Listener, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (blockedFactory) Join(ctx context.Context, _ string, _ transport.Options, _ func()) (io.ReadWriteCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestPreparationCancellationAndSourceFailure(t *testing.T) {
	opts := transport.Options{Address: "127.0.0.1:1234", Password: "private"}
	c := NewController(&syntheticSource{}, blockedFactory{}, "http://wails.localhost")
	if _, err := c.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, opts); err != nil {
		t.Fatal(err)
	}
	stopController(t, c)
	if c.Invitation() != "" || c.Stats().State != Idle {
		t.Fatal("canceled host kept state")
	}
	if _, err := c.Join("cs1-test", opts); err != nil {
		t.Fatal(err)
	}
	if err := c.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Join("cs1-test", opts); err == nil {
		t.Fatal("join after shutdown")
	}
	source := &syntheticSource{fail: true}
	c = NewController(source, transport.Croc{}, "http://wails.localhost")
	defer c.Shutdown()
	if _, err := c.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, relay(t)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return !c.Active() })
	if c.Stats().Error != "Nguồn màn hình đã mất hoặc capture thất bại." || source.closed.Load() != 1 {
		t.Fatal("source failure hidden")
	}
}
func TestWrongInvitationFailsWithoutCapture(t *testing.T) {
	c := NewController(nil, transport.Croc{}, "http://wails.localhost")
	defer c.Shutdown()
	if _, err := c.Join("invalid", relay(t)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return !c.Active() })
	if c.Stats().State != Failed || c.Stats().Counters.Decoded != 0 {
		t.Fatal("bad invite accepted")
	}
	if err := (transport.Options{Address: "localhost", Password: ""}).Validate(); err == nil {
		t.Fatal("invalid relay accepted")
	}
}
func TestLatestOnlyCaptureAdmission(t *testing.T) {
	q := make(chan capture.Frame, 1)
	for i := 0; i < 100; i++ {
		dropped := latest(q, capture.Frame{Width: i})
		if dropped != (i > 0) {
			t.Fatal("drop accounting")
		}
	}
	if len(q) != 1 || (<-q).Width != 99 {
		t.Fatal("unbounded or stale queue")
	}
}

type failedFactory struct{}

func (failedFactory) Host(context.Context, transport.Options, func()) (transport.Listener, error) {
	return nil, errors.New("private server error")
}
func (failedFactory) Join(context.Context, string, transport.Options, func()) (io.ReadWriteCloser, error) {
	return nil, errors.New("private secret text")
}
func TestErrorsAreSanitized(t *testing.T) {
	c := NewController(nil, failedFactory{}, "http://wails.localhost")
	if _, err := c.Join("secret", transport.Options{Address: "127.0.0.1:1", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return !c.Active() })
	if c.Stats().Error == "" || c.Invitation() != "" {
		t.Fatal("unsanitized lifecycle")
	}
}

func TestJoinAttemptsAreRateLimitedWithoutCodeParsing(t *testing.T) {
	opts := transport.Options{Address: "127.0.0.1:9009", Password: "private"}
	c := NewController(nil, failedFactory{}, "http://wails.localhost")
	defer c.Shutdown()
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := c.Join("arbitrary-codephrase-from-real-upstream", opts); err != nil {
			t.Fatal("invented regex rejected input", err)
		}
		waitFor(t, func() bool { return !c.Active() })
		if attempt == 0 {
			if _, err := c.Join("another-code", opts); err == nil {
				t.Fatal("uncontrolled retry accepted")
			}
			time.Sleep(2 * time.Second)
		}
	}
}

type progressFactory struct {
	authenticated chan struct{}
	begin         chan struct{}
}
type progressListener struct {
	auth          func()
	authenticated chan struct{}
	begin         <-chan struct{}
	once          atomic.Bool
}

func (f *progressFactory) Host(_ context.Context, _ transport.Options, auth func()) (transport.Listener, error) {
	return &progressListener{auth: auth, authenticated: f.authenticated, begin: f.begin}, nil
}
func (f *progressFactory) Join(ctx context.Context, _ string, _ transport.Options, auth func()) (io.ReadWriteCloser, error) {
	if auth != nil {
		auth()
	}
	select {
	case f.authenticated <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (l *progressListener) Invitation() string { return "cs1-private" }
func (l *progressListener) Accept(ctx context.Context) (io.ReadWriteCloser, error) {
	select {
	case <-l.begin:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if l.once.CompareAndSwap(false, true) {
		if l.auth != nil {
			l.auth()
		}
		select {
		case l.authenticated <- struct{}{}:
		default:
		}
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (l *progressListener) Close() error { return nil }

func TestAuthenticationProgressAndInvitationLifetime(t *testing.T) {
	opts := transport.Options{Address: "127.0.0.1:9009", Password: "private"}
	ph := &progressFactory{authenticated: make(chan struct{}, 1), begin: make(chan struct{})}
	host := NewController(&syntheticSource{}, ph, "http://wails.localhost")
	defer host.Shutdown()
	if _, err := host.Host(capture.Options{DisplayID: "synthetic", Width: 640, Height: 360, FPS: 24}, opts); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return host.Stats().State == Waiting })
	if host.Invitation() == "" {
		t.Fatal("host code unavailable while waiting")
	}
	close(ph.begin)
	select {
	case <-ph.authenticated:
	case <-time.After(time.Second):
		t.Fatal("no Accept progress")
	}
	waitFor(t, func() bool { return host.Stats().State == Authenticating })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := host.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if host.Invitation() != "" || host.Stats().State != Idle {
		t.Fatal("invitation/state survived Stop")
	}
	pv := &progressFactory{authenticated: make(chan struct{}, 1)}
	viewer := NewController(nil, pv, "http://wails.localhost")
	defer viewer.Shutdown()
	if _, err := viewer.Join("cs1-valid", opts); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pv.authenticated:
	case <-time.After(time.Second):
		t.Fatal("no Join progress")
	}
	waitFor(t, func() bool { return viewer.Stats().State == Authenticating })
	if err := viewer.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
