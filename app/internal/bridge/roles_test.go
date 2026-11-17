package bridge

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestViewerBridgeRejectsPreviewRole(t *testing.T) {
	b, err := NewRole(context.Background(), "http://wails.localhost", [32]byte{1}, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	e := b.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	wrong := e
	wrong.URL = strings.TrimSuffix(e.URL, "/viewer") + "/preview"
	c, r, err := dial(ctx, wrong, "http://wails.localhost", []string{Subprotocol, "auth." + e.Token})
	if c != nil {
		c.CloseNow()
		t.Fatal("cross-role accepted")
	}
	if err == nil || r == nil || r.StatusCode != 403 {
		t.Fatal("expected cross-role denial")
	}
	c, _, err = dial(ctx, e, "http://wails.localhost", []string{Subprotocol, "auth." + e.Token})
	if err != nil {
		t.Fatal("viewer role rejected")
	}
	c.CloseNow()
}
