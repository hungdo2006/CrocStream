package bridge

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testBridge(t *testing.T) *Bridge {
	t.Helper()
	var token [32]byte
	token[0] = 7
	b, err := New(context.Background(), "http://wails.localhost", token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}
func dial(ctx context.Context, e Endpoint, origin string, protocols []string) (*websocket.Conn, *http.Response, error) {
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	return websocket.Dial(ctx, e.URL, &websocket.DialOptions{HTTPHeader: header, Subprotocols: protocols})
}
func TestRejectUnauthorizedUpgrade(t *testing.T) {
	b := testBridge(t)
	e := b.Endpoint()
	for _, tt := range []struct {
		name, origin, path string
		protocols          []string
	}{
		{"missing-origin", "", "", []string{Subprotocol, "auth." + e.Token}},
		{"null", "null", "", []string{Subprotocol, "auth." + e.Token}},
		{"foreign", "http://evil.example", "", []string{Subprotocol, "auth." + e.Token}},
		{"scheme", "https://wails.localhost", "", []string{Subprotocol, "auth." + e.Token}},
		{"port", "http://wails.localhost:9245", "", []string{Subprotocol, "auth." + e.Token}},
		{"token", "http://wails.localhost", "", []string{Subprotocol, "auth.invalid"}},
		{"missing-token", "http://wails.localhost", "", []string{Subprotocol}},
		{"viewer", "http://wails.localhost", "/viewer", []string{Subprotocol, "auth." + e.Token}},
		{"query", "http://wails.localhost", "/preview?token=x", []string{Subprotocol, "auth." + e.Token}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			target := e
			if tt.path != "" {
				target.URL = strings.TrimSuffix(e.URL, "/preview") + tt.path
			}
			conn, response, err := dial(ctx, target, tt.origin, tt.protocols)
			if conn != nil {
				conn.CloseNow()
				t.Fatal("unauthorized accepted")
			}
			if err == nil || response == nil || response.StatusCode != 403 {
				t.Fatal("expected generic denial")
			}
		})
	}
}
func TestBinaryPreviewClientCapAndClose(t *testing.T) {
	b := testBridge(t)
	e := b.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := dial(ctx, e, "http://wails.localhost", []string{Subprotocol, "auth." + e.Token})
	if err != nil {
		t.Fatal("valid upgrade failed")
	}
	defer conn.CloseNow()
	if conn.Subprotocol() != Subprotocol {
		t.Fatal("token echoed as subprotocol")
	}
	other, response, err := dial(ctx, e, "http://wails.localhost", []string{Subprotocol, "auth." + e.Token})
	if other != nil {
		other.CloseNow()
		t.Fatal("second client accepted")
	}
	if err == nil || response == nil || response.StatusCode != 429 {
		t.Fatal("client cap")
	}
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	if err := b.Publish(data); err != nil {
		t.Fatal(err)
	}
	data[0] = 0
	typ, received, err := conn.Read(ctx)
	if err != nil || typ != websocket.MessageBinary || received[0] != 0xFF {
		t.Fatal("binary ownership")
	}
	b.Close()
	b.Close()
	if b.Endpoint() != (Endpoint{}) {
		t.Fatal("retained endpoint token")
	}
	if err := b.Publish(data); err != ErrClosed {
		t.Fatal(err)
	}
	addr := strings.TrimPrefix(strings.TrimSuffix(e.URL, "/preview"), "ws://")
	socket, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		socket.Close()
		t.Fatal("listener remained")
	}
}
func TestLatestQueueAndInputLimits(t *testing.T) {
	b := testBridge(t)
	b.mu.Lock()
	b.queue = make(chan []byte, 1)
	q := b.queue
	b.mu.Unlock()
	for i := 0; i < 100; i++ {
		if err := b.Publish([]byte{1, 2, 3, byte(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(q) != 1 || (<-q)[3] != 99 {
		t.Fatal("not latest-only")
	}
	_, dropped := b.Counts()
	if dropped != 99 {
		t.Fatal(dropped)
	}
	if b.Publish(nil) == nil || b.Publish(make([]byte, MaxJPEG+1)) == nil {
		t.Fatal("unbounded input")
	}
}
func TestInvalidTrustedOrigin(t *testing.T) {
	for _, origin := range []string{"", "null", "*", "http://evil.example", "http://wails.localhost/", "http://wails.localhost?x=y"} {
		b, err := New(context.Background(), origin, [32]byte{})
		if err == nil {
			b.Close()
			t.Fatal("invalid allowlist", origin)
		}
	}
}

func TestParentCancellationAndInboundData(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	b, err := New(parent, "http://wails.localhost", [32]byte{3})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	e := b.Endpoint()
	ctx, done := context.WithTimeout(context.Background(), 3*time.Second)
	defer done()
	conn, _, err := dial(ctx, e, "http://wails.localhost", []string{Subprotocol, "auth." + e.Token})
	if err != nil {
		t.Fatal("upgrade")
	}
	defer conn.CloseNow()
	_ = conn.Write(ctx, websocket.MessageBinary, []byte{1, 2})
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("inbound data accepted")
	}
	cancel()
	select {
	case <-b.stopped:
	case <-ctx.Done():
		t.Fatal("parent cancellation did not close bridge")
	}
}

func TestListenerAdmissionBound(t *testing.T) {
	socket, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := &limitedListener{Listener: socket, slots: make(chan struct{}, 1)}
	defer listener.Close()
	accepted := make(chan net.Conn, 2)
	go func() {
		for i := 0; i < 2; i++ {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()
	first, err := net.DialTimeout("tcp", socket.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	owned := <-accepted
	defer owned.Close()
	second, err := net.DialTimeout("tcp", socket.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, err := second.Read(b[:]); err == nil {
		t.Fatal("over-cap connection not closed")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("over-cap connection stalled instead of closing")
	}
	owned.Close()
	third, err := net.DialTimeout("tcp", socket.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	select {
	case c := <-accepted:
		c.Close()
	case <-time.After(time.Second):
		t.Fatal("admission slot not released")
	}
}

func TestSlowClientWriteDeadline(t *testing.T) {
	b := testBridge(t)
	e := b.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := dial(ctx, e, "http://wails.localhost", []string{Subprotocol, "auth." + e.Token})
	if err != nil {
		t.Fatal("upgrade failed")
	}
	defer conn.CloseNow()
	ready := time.NewTicker(time.Millisecond)
	defer ready.Stop()
	for {
		b.mu.Lock()
		attached := b.queue != nil
		b.mu.Unlock()
		if attached {
			break
		}
		select {
		case <-ready.C:
		case <-ctx.Done():
			t.Fatal("client did not attach")
		}
	}
	// Do not read: a 4-MiB write must eventually stop at the one-second deadline.
	payload := make([]byte, MaxJPEG)
	for i := 0; i < 4; i++ {
		if err := b.Publish(payload); err != nil {
			t.Fatal(err)
		}
	}
	check := time.NewTicker(20 * time.Millisecond)
	defer check.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("slow client retained session past deadline")
		case <-check.C:
			b.mu.Lock()
			busy := b.busy
			b.mu.Unlock()
			if !busy {
				return
			}
		}
	}
}
