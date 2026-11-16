package desktop

import (
	"context"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/preview"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct{ controller *preview.Controller }

func New(origin string) (*Service, error) {
	c, err := preview.New(capture.Windows{}, origin)
	if err != nil {
		return nil, err
	}
	return &Service{c}, nil
}
func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error { return nil }
func (s *Service) ServiceShutdown() error                                                 { return s.controller.Shutdown() }
func (s *Service) ListDisplays() ([]capture.Display, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.controller.Displays(ctx)
}
func (s *Service) StartPreview(o capture.Options) (preview.Started, error) {
	return s.controller.Start(o)
}
func (s *Service) StopPreview() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.controller.Stop(ctx)
}
func (s *Service) GetStats() preview.Stats { return s.controller.Stats() }
