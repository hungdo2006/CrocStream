package desktop

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/preview"
	"crocstream.local/app/internal/session"
	"crocstream.local/app/internal/transport"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct {
	mu              sync.Mutex
	controller      *preview.Controller
	stream          *session.Controller
	relayConfigPath func() (string, error)
}

func defaultRelayConfigPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "relay.json"), nil
}

func New(origin string) (*Service, error) {
	c, err := preview.New(capture.Windows{}, origin)
	if err != nil {
		return nil, err
	}
	return &Service{
		controller:      c,
		stream:          session.NewController(capture.Windows{}, transport.Croc{}, origin),
		relayConfigPath: defaultRelayConfigPath,
	}, nil
}
func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error { return nil }
func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.stream.Shutdown()
	previewErr := s.controller.Shutdown()
	if err != nil {
		return err
	}
	return previewErr
}
func (s *Service) ListDisplays() ([]capture.Display, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.controller.Displays(ctx)
}
func (s *Service) StartPreview(o capture.Options) (preview.Started, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stream.Active() {
		return preview.Started{}, session.ErrActive
	}
	return s.controller.Start(o)
}
func (s *Service) StopPreview() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.controller.Stop(ctx)
}
func (s *Service) GetStats() preview.Stats { return s.controller.Stats() }

func (s *Service) StartBroadcast(o capture.Options, relay transport.Options) (session.Started, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.controller.Active() {
		return session.Started{}, session.ErrActive
	}
	return s.stream.Host(o, relay)
}
func (s *Service) JoinSession(code string, relay transport.Options) (session.Started, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.controller.Active() {
		return session.Started{}, session.ErrActive
	}
	return s.stream.Join(code, relay)
}
func (s *Service) StopSession() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.stream.Stop(ctx)
}
func (s *Service) GetSessionStats() session.Stats { return s.stream.Stats() }
func (s *Service) GetInvitation() string          { return s.stream.Invitation() }

// GetDefaultRelay returns optional relay defaults from relay.json beside the executable.
// Missing or invalid files return empty options so manual entry remains available.
func (s *Service) GetDefaultRelay() transport.Options {
	resolve := s.relayConfigPath
	if resolve == nil {
		resolve = defaultRelayConfigPath
	}
	path, err := resolve()
	if err != nil {
		return transport.Options{}
	}
	opts, err := transport.LoadRelayFile(path)
	if err != nil {
		return transport.Options{}
	}
	return opts
}
