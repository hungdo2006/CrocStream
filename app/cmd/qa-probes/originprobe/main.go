//go:build windows

// A bounded, capture-free observation of actual WebView2 WebSocket Origin.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	dev := flag.Bool("dev", false, "serve development assets from a loopback server")
	flag.Parse()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	path := "/observe/" + hex.EncodeToString(nonce[:])
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	host := listener.Addr().String()
	html := fmt.Sprintf(`<!doctype html><html><body><h1>G3 Origin probe</h1><p>No capture.</p><script>new WebSocket("ws://%s%s");</script></body></html>`, host, path)
	result := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			select {
			case result <- r.Header.Get("Origin"):
			default:
			}
			http.Error(w, "observation only", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, html)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second}
	go server.Serve(listener)
	defer server.Close()
	if *dev {
		os.Setenv("FRONTEND_DEVSERVER_URL", "http://"+host)
	}
	app := application.New(application.Options{
		Name:   "CrocStream G3 Origin Probe",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Assets: application.AssetOptions{Handler: handler, DisableLogging: true},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "CrocStream Origin Probe", Width: 420, Height: 200})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		select {
		case origin := <-result:
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"dev": *dev, "origin": origin})
		case <-ctx.Done():
			fmt.Println(`{"error":"Origin not observed within 20 seconds"}`)
		}
		app.Quit()
	}()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "probe failed")
		os.Exit(1)
	}
}
