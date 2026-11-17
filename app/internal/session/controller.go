package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"sync"
	"time"

	"crocstream.local/app/internal/bridge"
	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/protocol"
	"crocstream.local/app/internal/telemetry"
	"crocstream.local/app/internal/transport"
)

var ErrActive = errors.New("Một phiên khác đang hoạt động; hãy dừng phiên đó trước.")

type Started struct {
	Endpoint   bridge.Endpoint `json:"endpoint"`
	Generation uint64          `json:"generation"`
}
type Stats struct {
	State                State    `json:"state"`
	Role                 string   `json:"role"`
	Error                string   `json:"error"`
	PreviewError         string   `json:"previewError"`
	Generation           uint64   `json:"generation"`
	Counters             Counters `json:"counters"`
	CaptureFPS           int      `json:"captureFPS"`
	EmittedFPS           int      `json:"emittedFPS"`
	DecodedFPS           int      `json:"decodedFPS"`
	ReceivedFPS          int      `json:"receivedFPS"`
	SentKiB              float64  `json:"sentKiB"`
	ReceivedKiB          float64  `json:"receivedKiB"`
	BaselineKiB          float64  `json:"baselineKiB"`
	Savings              float64  `json:"savings"`
	SavingsKnown         bool     `json:"savingsKnown"`
	BaselineKnown        bool     `json:"baselineKnown"`
	MeasurementsComplete bool     `json:"measurementsComplete"`
	MeasurementOverflow  bool     `json:"measurementOverflow"`
	MeanKeyKiB           float64  `json:"meanKeyKiB"`
	MeanDeltaKiB         float64  `json:"meanDeltaKiB"`
	KeyMeanKnown         bool     `json:"keyMeanKnown"`
	DeltaMeanKnown       bool     `json:"deltaMeanKnown"`
	Dropped              uint64   `json:"dropped"`
	BridgeSent           uint64   `json:"bridgeSent"`
	Uptime               float64  `json:"uptime"`
	RetryAfterMs         int64    `json:"retryAfterMs"`
	Captured             uint64   `json:"captured"`
	CaptureQueue         int      `json:"captureQueue"`
	ImageQueue           int      `json:"imageQueue"`
	BridgeDropped        uint64   `json:"bridgeDropped"`
	MeanEncodeMs         float64  `json:"meanEncodeMs"`
	MeanDecodeMs         float64  `json:"meanDecodeMs"`
	MeanBaselineMs       float64  `json:"meanBaselineMs"`
	MeanJPEGMs           float64  `json:"meanJPEGMs"`
}
type activeSession struct {
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	bridge     *bridge.Bridge
	metrics    *Metrics
	stats      Stats
	invitation string // Returned separately by trusted binding, never in Stats.
	peer       *Peer
	stopping   bool
	dropped    uint64
	captured   uint64
	frames     chan capture.Frame
	images     chan capture.Frame
	jpegTotal  time.Duration
	jpegCount  uint64
}
type Controller struct {
	mu           sync.Mutex
	source       capture.System
	factory      transport.Factory
	origin       string
	active       *activeSession
	last         Stats
	generation   uint64
	shutdown     bool
	joinAttempts []time.Time
	lastJoin     time.Time
	joinCooldown time.Duration
	joinWindow   time.Duration
	joinLimit    int
}

func NewController(source capture.System, factory transport.Factory, origin string) *Controller {
	return &Controller{source: source, factory: factory, origin: origin, last: Stats{State: Idle}, joinCooldown: 2 * time.Second, joinWindow: time.Minute, joinLimit: 5}
}
func (c *Controller) Active() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.active != nil }
func (c *Controller) start(host bool, o capture.Options, relay transport.Options, code string) (Started, error) {
	if err := relay.Validate(); err != nil {
		return Started{}, err
	}
	if host {
		if _, err := o.Config(); err != nil {
			return Started{}, err
		}
	}
	if !host && (len(code) == 0 || len(code) > 256) {
		return Started{}, errors.New("Mã phiên không hợp lệ.")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shutdown || c.active != nil {
		return Started{}, errors.New("Phiên đang chạy hoặc đang dừng.")
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return Started{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	role, path, state := "viewer", "viewer", Connecting
	if host {
		role, path, state = "broadcaster", "preview", Preparing
	}
	b, err := bridge.NewRole(ctx, c.origin, token, path)
	if err != nil {
		cancel()
		return Started{}, errors.New("Không mở được bridge cục bộ.")
	}
	c.generation++
	s := &activeSession{ctx: ctx, cancel: cancel, done: make(chan struct{}), bridge: b, metrics: NewMetrics(),
		stats: Stats{State: state, Role: role, Generation: c.generation}}
	c.active = s
	go c.run(s, host, o, relay, code)
	return Started{Endpoint: b.Endpoint(), Generation: c.generation}, nil
}
func (c *Controller) Host(o capture.Options, relay transport.Options) (Started, error) {
	return c.start(true, o, relay, "")
}
func (c *Controller) Join(code string, relay transport.Options) (Started, error) {
	now := time.Now()
	c.mu.Lock()
	if c.shutdown || c.active != nil {
		c.mu.Unlock()
		return Started{}, ErrActive
	}
	if c.joinCooldown > 0 && !c.lastJoin.IsZero() && now.Sub(c.lastJoin) < c.joinCooldown {
		c.mu.Unlock()
		return Started{}, errors.New("Chờ trước khi thử kết nối lại.")
	}
	cut := now.Add(-c.joinWindow)
	kept := c.joinAttempts[:0]
	for _, attempt := range c.joinAttempts {
		if attempt.After(cut) {
			kept = append(kept, attempt)
		}
	}
	c.joinAttempts = kept
	if c.joinLimit > 0 && len(c.joinAttempts) >= c.joinLimit {
		c.mu.Unlock()
		return Started{}, errors.New("Quá nhiều lần thử; chờ một phút rồi thử lại.")
	}
	c.lastJoin = now
	c.joinAttempts = append(c.joinAttempts, now)
	c.mu.Unlock()
	return c.start(false, capture.Options{}, relay, code)
}
func (c *Controller) Invitation() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil || c.active.stopping {
		return ""
	}
	return c.active.invitation
}
func latest(q chan capture.Frame, f capture.Frame) bool {
	select {
	case q <- f:
		return false
	default:
	}
	dropped := false
	select {
	case <-q:
		dropped = true
	default:
	}
	select {
	case q <- f:
	default:
		dropped = true
	}
	return dropped
}
func (c *Controller) setState(s *activeSession, state State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == s && !s.stopping {
		s.stats.State = state
	}
}

func (c *Controller) run(s *activeSession, host bool, o capture.Options, relay transport.Options, code string) {
	ctx := s.ctx
	// Work/bridge cancellation is immediate. Transport gets at most 200ms for
	// the sole writer's best-effort END, then its context forcibly closes I/O.
	network, closeNetwork := context.WithCancel(context.Background())
	stopNetwork := context.AfterFunc(ctx, func() { time.AfterFunc(200*time.Millisecond, closeNetwork) })
	defer stopNetwork()
	defer closeNetwork()
	var failure error
	var conn io.ReadWriteCloser
	var listener transport.Listener
	var workers sync.WaitGroup
	images := make(chan capture.Frame, 1)
	frames := make(chan capture.Frame, 1)
	c.mu.Lock()
	s.images, s.frames = images, frames
	c.mu.Unlock()
	defer func() {
		s.cancel()
		if conn != nil {
			_ = conn.Close()
		}
		if listener != nil {
			_ = listener.Close()
		}
		s.bridge.Close()
		workers.Wait()
		c.mu.Lock()
		s.invitation = ""
		if s.stopping {
			s.stats.State = Idle
		} else if failure != nil && conn == nil {
			if s.stats.Error == "" {
				if s.stats.State == Authenticating {
					s.stats.Error = "Xác thực peer thất bại hoặc hết thời gian. Kiểm tra mã với người phát rồi tạo phiên mới."
				} else {
					s.stats.Error = "Không thiết lập được phiên. Kiểm tra relay, mật khẩu và mã mời; thử lại sau."
				}
			}
			s.stats.State = Failed
		} else {
			s.stats.State = Disconnected
			if failure != nil && s.stats.Error == "" {
				s.stats.Error = "Kết nối đã ngắt hoặc dữ liệu không hợp lệ. Hãy tạo phiên mới."
			}
		}
		c.last = c.snapshotLocked(s)
		c.active = nil
		close(s.done)
		c.mu.Unlock()
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case f := <-images:
				begin := time.Now()
				var out bytes.Buffer
				err := jpeg.Encode(&out, &image.RGBA{Pix: f.RGBA, Stride: f.Width * 4, Rect: image.Rect(0, 0, f.Width, f.Height)}, &jpeg.Options{Quality: 75})
				elapsed := time.Since(begin)
				if err == nil && ctx.Err() == nil {
					err = s.bridge.Publish(out.Bytes())
				}
				c.mu.Lock()
				s.jpegTotal += elapsed
				s.jpegCount++
				if err != nil && ctx.Err() == nil {
					s.stats.PreviewError = "Preview unavailable"
				} else if err == nil {
					s.stats.PreviewError = ""
				}
				c.mu.Unlock()
			}
		}
	}()
	if host {
		listener, failure = c.factory.Host(network, relay, func() { c.setState(s, Authenticating) })
		relay = transport.Options{} // Clear admission credentials on success or failure.
		if failure != nil {
			return
		}
		relay = transport.Options{} // Do not retain admission password in coordinator.
		c.mu.Lock()
		if !s.stopping {
			s.invitation = listener.Invitation()
			s.stats.State = Waiting
		}
		c.mu.Unlock()
		workers.Add(1)
		go func() {
			defer workers.Done()
			err := c.source.Run(ctx, o, func(f capture.Frame) {
				c.mu.Lock()
				s.captured++
				c.mu.Unlock()
				s.metrics.Record(telemetry.Event{Kind: telemetry.Captured})
				latest(images, f) // Capture snapshots are immutable, owned slices.
				if latest(frames, f) {
					c.mu.Lock()
					s.dropped++
					c.mu.Unlock()
					s.metrics.Record(telemetry.Event{Kind: telemetry.Dropped})
				}
			})
			if ctx.Err() == nil {
				c.mu.Lock()
				s.stats.Error = "Nguồn màn hình đã mất hoặc capture thất bại."
				c.mu.Unlock()
				s.cancel()
			}
			_ = err // UI uses a fixed actionable error, never raw native/log data.
		}()
		conn, failure = listener.Accept(network)
		c.mu.Lock()
		s.invitation = ""
		c.mu.Unlock()
	} else {
		conn, failure = c.factory.Join(network, code, relay, func() { c.setState(s, Authenticating) })
		code = ""
		relay = transport.Options{}
	}
	if failure != nil {
		return
	}
	hooks := Hooks{State: func(state State) { c.setState(s, state) }}
	if !host {
		hooks.Image = func(f capture.Frame) { latest(images, f) }
	}
	peer := NewPeerWithTau(conn, host, s.metrics, hooks, o.Tau)
	c.mu.Lock()
	s.peer = peer
	c.mu.Unlock()
	if host {
		cfg, _ := o.Config()
		failure = peer.Run(ctx, cfg, frames)
	} else {
		failure = peer.Run(ctx, protocol.Config{}, nil)
	}
}

func (c *Controller) snapshotLocked(s *activeSession) Stats {
	out := s.stats
	counts, m := s.metrics.Read()
	out.Counters = counts
	out.CaptureFPS, out.EmittedFPS, out.DecodedFPS, out.ReceivedFPS = m.CapturedFPS, m.EmittedFPS, m.DecodedFPS, m.Received.Frames
	out.SentKiB, out.ReceivedKiB, out.BaselineKiB = m.Sent.KiBPerSecond, m.Received.KiBPerSecond, m.BaselineKiBPerSecond
	out.Savings, out.SavingsKnown, out.MeasurementsComplete = m.SavingsPercent, m.SavingsKnown, m.Complete
	out.BaselineKnown = m.BaselineKnown
	out.MeasurementOverflow = !m.Complete
	video := m.Sent
	if out.Role == "viewer" {
		video = m.Received
	}
	out.MeanKeyKiB, out.MeanDeltaKiB = video.MeanKeyKiB, video.MeanDeltaKiB
	out.KeyMeanKnown, out.DeltaMeanKnown = video.KeyMeanKnown, video.DeltaMeanKnown
	out.Dropped = s.dropped
	out.Captured = s.captured
	out.CaptureQueue, out.ImageQueue = len(s.frames), len(s.images)
	out.BridgeSent, out.BridgeDropped = s.bridge.Counts()
	out.MeanEncodeMs, out.MeanDecodeMs, out.MeanBaselineMs = m.MeanEncodeMs, m.MeanDecodeMs, m.MeanBaselineMs
	if s.jpegCount > 0 {
		out.MeanJPEGMs = s.jpegTotal.Seconds() * 1000 / float64(s.jpegCount)
	}
	out.Uptime = m.Uptime.Seconds()
	return out
}
func (c *Controller) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil {
		out := c.last
		out.CaptureFPS = 0
		out.EmittedFPS = 0
		out.DecodedFPS = 0
		out.ReceivedFPS = 0
		out.RetryAfterMs = c.retryAfterLocked(time.Now())
		return out
	}
	out := c.snapshotLocked(c.active)
	out.RetryAfterMs = c.retryAfterLocked(time.Now())
	return out
}

func (c *Controller) retryAfterLocked(now time.Time) int64 {
	until := c.lastJoin.Add(c.joinCooldown)
	count := 0
	var first time.Time
	for _, at := range c.joinAttempts {
		if at.After(now.Add(-c.joinWindow)) {
			if first.IsZero() {
				first = at
			}
			count++
		}
	}
	if c.joinLimit > 0 && count >= c.joinLimit && first.Add(c.joinWindow).After(until) {
		until = first.Add(c.joinWindow)
	}
	if until.After(now) {
		return until.Sub(now).Milliseconds() + 1
	}
	return 0
}
func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	s := c.active
	if s == nil {
		c.mu.Unlock()
		return nil
	}
	s.stopping = true
	s.stats.State = Stopping
	s.invitation = ""
	s.cancel()
	c.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return errors.New("Đang chờ giải phóng phiên; Stop quá thời gian.")
	}
}
func (c *Controller) Shutdown() error {
	c.mu.Lock()
	c.shutdown = true
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.Stop(ctx)
}
func (c *Controller) RequestRecovery() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil && c.active.peer != nil && c.active.stats.Role == "viewer" {
		c.active.peer.RequestRecovery()
	}
}
