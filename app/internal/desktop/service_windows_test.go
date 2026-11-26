package desktop

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/preview"
	"crocstream.local/app/internal/session"
	"crocstream.local/app/internal/transport"
)

type fakeCaptureSystem struct{}

func (fakeCaptureSystem) Displays(context.Context) ([]capture.Display, error) {
	return []capture.Display{{ID: "display-1", Width: 1280, Height: 720, Primary: true}}, nil
}

func (fakeCaptureSystem) Run(ctx context.Context, _ capture.Options, _ func(capture.Frame)) error {
	<-ctx.Done()
	return ctx.Err()
}

type waitingListener struct{}

func (waitingListener) Invitation() string { return "cs1-test-invite" }
func (waitingListener) Close() error       { return nil }
func (waitingListener) Accept(ctx context.Context) (io.ReadWriteCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type waitingFactory struct{}

func (waitingFactory) Host(ctx context.Context, _ transport.Options, _ func()) (transport.Listener, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return waitingListener{}, nil
}

func (waitingFactory) Join(ctx context.Context, _ string, _ transport.Options, _ func()) (io.ReadWriteCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	const origin = "http://wails.localhost"
	p, err := preview.New(fakeCaptureSystem{}, origin)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{
		controller: p,
		stream:     session.NewController(fakeCaptureSystem{}, waitingFactory{}, origin),
	}
}

func TestServiceMutualExclusionAndShutdown(t *testing.T) {
	svc := newTestService(t)
	defer svc.ServiceShutdown()

	displays, err := svc.ListDisplays()
	if err != nil || len(displays) != 1 || displays[0].ID != "display-1" {
		t.Fatalf("unexpected displays: %v, err=%v", displays, err)
	}

	opts := capture.Options{DisplayID: "display-1", Width: 1280, Height: 720, FPS: 24, Tau: 4, KeyInterval: 30}
	relay := transport.Options{Address: "127.0.0.1:9009", Password: "relay-password-123"}

	if _, err := svc.StartPreview(opts); err != nil {
		t.Fatalf("StartPreview failed: %v", err)
	}
	if _, err := svc.StartBroadcast(opts, relay); !errors.Is(err, session.ErrActive) {
		t.Fatalf("expected ErrActive when starting broadcast during preview, got %v", err)
	}
	if _, err := svc.JoinSession("cs1-code", relay); !errors.Is(err, session.ErrActive) {
		t.Fatalf("expected ErrActive when joining session during preview, got %v", err)
	}
	if err := svc.StopPreview(); err != nil {
		t.Fatalf("StopPreview failed: %v", err)
	}

	if _, err := svc.StartBroadcast(opts, relay); err != nil {
		t.Fatalf("StartBroadcast failed after preview stopped: %v", err)
	}
	if _, err := svc.StartPreview(opts); !errors.Is(err, session.ErrActive) {
		t.Fatalf("expected ErrActive when starting preview during broadcast, got %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for svc.GetInvitation() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if svc.GetInvitation() == "" {
		t.Fatal("expected non-empty invitation while broadcaster is waiting")
	}

	if err := svc.StopSession(); err != nil {
		t.Fatalf("StopSession failed: %v", err)
	}
	if svc.GetSessionStats().State != session.Idle {
		t.Fatalf("expected Idle session state after StopSession, got %s", svc.GetSessionStats().State)
	}
}

func TestServiceGetDiagnostics(t *testing.T) {
	svc := newTestService(t)
	defer svc.ServiceShutdown()

	d := svc.GetDiagnostics()
	if d.Goroutines < 1 || d.LogicalCPUs < 1 {
		t.Fatalf("invalid runtime diagnostics: %+v", d)
	}
	if !d.ProcessMeasured || !d.CPUMeasured {
		t.Fatalf("expected Windows process and CPU metrics to be measured: %+v", d)
	}
	if d.WorkingSetBytes == 0 || d.Handles == 0 {
		t.Fatalf("expected nonzero working set and handles: %+v", d)
	}
}

func TestServiceGetDefaultRelay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.json")
	svc := newTestService(t)
	defer svc.ServiceShutdown()
	svc.relayConfigPath = func() (string, error) { return path, nil }

	if got := svc.GetDefaultRelay(); got != (transport.Options{}) {
		t.Fatalf("expected empty default relay when file is absent, got %+v", got)
	}

	if err := os.WriteFile(path, []byte(`{"address":"198.51.100.10:9009","password":"example-relay-pass"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := svc.GetDefaultRelay()
	if got.Address != "198.51.100.10:9009" || got.Password != "example-relay-pass" {
		t.Fatalf("unexpected default relay: %+v", got)
	}

	if err := os.WriteFile(path, []byte(`{"address":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetDefaultRelay(); got != (transport.Options{}) {
		t.Fatalf("expected empty default relay on malformed file, got %+v", got)
	}
}
