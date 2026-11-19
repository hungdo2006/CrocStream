// Package bridge serves session-owned, authenticated loopback JPEG previews.
package bridge

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"crocstream.local/app/internal/latest"
	"github.com/coder/websocket"
)

const MaxJPEG = 4 * 1024 * 1024
const Subprotocol = "crocstream.preview"

var ErrClosed = errors.New("bridge: session closed")
var ErrUnavailable = errors.New("bridge: local server unavailable")

type Endpoint struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type Bridge struct {
	mu            sync.Mutex
	endpoint      Endpoint
	origin        string
	path          string
	server        *http.Server
	listener      net.Listener
	ctx           context.Context
	cancel        context.CancelFunc
	conn          *websocket.Conn
	queue         chan []byte
	busy, closed  bool
	serveErr      error
	wg            sync.WaitGroup
	stopped       chan struct{}
	once          sync.Once
	sent, dropped atomic.Uint64
}

func New(parent context.Context, origin string, token [32]byte) (*Bridge, error) {
	return NewRole(parent, origin, token, "preview")
}

// Each instance has exactly one immutable role, endpoint and session token.
func NewRole(parent context.Context, origin string, token [32]byte, role string) (*Bridge, error) {
	if role != "preview" && role != "viewer" {
		return nil, errors.New("bridge: invalid role")
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "wails.localhost" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("bridge: invalid trusted origin")
	}
	socket, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ln := &limitedListener{Listener: socket, slots: make(chan struct{}, 8)}
	ctx, cancel := context.WithCancel(parent)
	b := &Bridge{origin: origin, path: "/" + role, listener: ln, ctx: ctx, cancel: cancel, stopped: make(chan struct{}),
		endpoint: Endpoint{URL: "ws://" + ln.Addr().String() + "/" + role, Token: hex.EncodeToString(token[:])}}
	b.server = &http.Server{Handler: http.HandlerFunc(b.handle), ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second,
		WriteTimeout: 2 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 4096, ErrorLog: log.New(io.Discard, "", 0)}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		if err := b.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			b.mu.Lock()
			if !b.closed && ctx.Err() == nil {
				b.serveErr = ErrUnavailable
				b.cancel() // The cancellation watcher owns Close and joins Serve.
			}
			b.mu.Unlock()
		}
	}()
	go func() { <-ctx.Done(); b.Close() }()
	return b, nil
}

func (b *Bridge) Endpoint() Endpoint             { b.mu.Lock(); defer b.mu.Unlock(); return b.endpoint }
func (b *Bridge) Counts() (sent, dropped uint64) { return b.sent.Load(), b.dropped.Load() }

// Done closes after the listener and all bridge workers have stopped.
func (b *Bridge) Done() <-chan struct{} { return b.stopped }

// Err reports a terminal local server failure, if one occurred.
func (b *Bridge) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.serveErr
}

func (b *Bridge) handle(w http.ResponseWriter, r *http.Request) {
	deny := func(code int) { http.Error(w, "preview access denied", code) }
	if r.Method != http.MethodGet || r.URL.Path != b.path || r.URL.RawQuery != "" ||
		r.Host != b.listener.Addr().String() || len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != b.origin {
		deny(http.StatusForbidden)
		return
	}
	var offered []string
	for _, value := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(value, ",") {
			offered = append(offered, strings.TrimSpace(part))
		}
	}
	b.mu.Lock()
	token := b.endpoint.Token
	if b.closed {
		b.mu.Unlock()
		deny(http.StatusGone)
		return
	}
	if len(offered) != 2 || offered[0] != Subprotocol ||
		subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(offered[1], "auth.")), []byte(token)) != 1 ||
		!strings.HasPrefix(offered[1], "auth.") {
		b.mu.Unlock()
		deny(http.StatusForbidden)
		return
	}
	if b.busy {
		b.mu.Unlock()
		deny(http.StatusTooManyRequests)
		return
	}
	b.busy = true
	b.wg.Add(1)
	b.mu.Unlock()
	defer b.wg.Done()
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{Subprotocol}, OriginPatterns: []string{b.origin}})
	if err != nil {
		b.mu.Lock()
		b.busy = false
		b.mu.Unlock()
		return
	}
	conn.SetReadLimit(1)
	b.mu.Lock()
	if b.closed {
		b.busy = false
		b.mu.Unlock()
		_ = conn.CloseNow()
		return
	}
	b.conn = conn
	q := make(chan []byte, 1)
	b.queue = q
	b.mu.Unlock()
	ctx, cancelRead := context.WithCancel(b.ctx)
	readDone := make(chan struct{})
	// Own the read worker explicitly. CloseRead's policy-close path in the
	// pinned library can wait on its own goroutine when peer data arrives.
	// Reader handles ping/close, but the first data message terminates this
	// send-only connection immediately, without reading/allocating its payload.
	go func() {
		defer close(readDone)
		_, _, _ = conn.Reader(ctx)
		cancelRead()
	}()
	defer func() {
		cancelRead()
		_ = conn.CloseNow()
		<-readDone
		b.mu.Lock()
		b.conn = nil
		b.queue = nil
		b.busy = false
		b.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case jpeg := <-q:
			writeCtx, cancel := context.WithTimeout(ctx, time.Second)
			err := conn.Write(writeCtx, websocket.MessageBinary, jpeg)
			cancel()
			if err != nil {
				return
			}
			b.sent.Add(1)
		}
	}
}

// Publish copies one JPEG into a latest-only queue; it never blocks on a client.
func (b *Bridge) Publish(jpeg []byte) error {
	if len(jpeg) < 4 || len(jpeg) > MaxJPEG {
		return errors.New("bridge: invalid image length")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	if b.queue == nil {
		b.dropped.Add(1)
		return nil
	}
	copyJPEG := append([]byte(nil), jpeg...)
	if latest.Offer(b.queue, copyJPEG) {
		b.dropped.Add(1)
	}
	return nil
}

// Close is idempotent and closes hijacked connections as well as the listener.
func (b *Bridge) Close() {
	b.once.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.endpoint = Endpoint{}
		conn := b.conn
		b.cancel()
		b.mu.Unlock()
		_ = b.server.Close()
		if conn != nil {
			_ = conn.CloseNow()
		}
		b.wg.Wait()
		close(b.stopped)
	})
}
