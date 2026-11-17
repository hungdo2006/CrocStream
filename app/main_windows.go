package main

import (
	"io"
	"log/slog"
	"os"

	"crocstream.local/app/frontend"
	"crocstream.local/app/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	service, err := desktop.New(trustedOrigin())
	if err != nil {
		os.Exit(1)
	}
	app := application.New(application.Options{
		Name:        "CrocStream",
		Description: "CrocStream one-to-one screen sharing",
		Windows:     browserOptions(),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Assets: application.AssetOptions{
			Handler:        application.BundledAssetFileServer(frontend.Assets),
			DisableLogging: true,
		},
		Services: []application.Service{application.NewService(service)},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "CrocStream · Share / Watch",
		Width:            1150,
		Height:           760,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: application.NewRGB(2, 6, 23),
	})
	if err := app.Run(); err != nil {
		os.Exit(1)
	}
}
