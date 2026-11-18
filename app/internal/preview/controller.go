// Package preview owns local capture/JPEG/bridge lifetime. It has no transport
// or delta-codec dependency and never reports a remotely connected session.
package preview

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"image"
	"image/jpeg"
	"sort"
	"sync"
	"time"

	"crocstream.local/app/internal/bridge"
	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/telemetry"
)

type State string

const (
	Idle       State = "Idle"
	Preparing  State = "Preparing"
	Previewing State = "Previewing"
	Stopping   State = "Stopping"
	Failed     State = "Error"
)

type Started struct {
	Endpoint   bridge.Endpoint `json:"endpoint"`
	Generation uint64          `json:"generation"`
}
type Stats struct {
	State           State   `json:"state"`
	Error           string  `json:"error"`
	Generation      uint64  `json:"generation"`
	Captured        uint64  `json:"captured"`
	Encoded         uint64  `json:"encoded"`
	Dropped         uint64  `json:"dropped"`
	Sent            uint64  `json:"sent"`
	BridgeDropped   uint64  `json:"bridgeDropped"`
	CaptureFPS      int     `json:"captureFPS"`
	EncodeFPS       int     `json:"encodeFPS"`
	CaptureP50Ms    float64 `json:"captureP50Ms"`
	CaptureP95Ms    float64 `json:"captureP95Ms"`
	EncodeP50Ms     float64 `json:"encodeP50Ms"`
	EncodeP95Ms     float64 `json:"encodeP95Ms"`
	SamplesComplete bool    `json:"samplesComplete"`
}
type session struct {
	cancel                    context.CancelFunc
	done                      chan struct{}
	bridge                    *bridge.Bridge
	stats                     Stats
	captureTimes, encodeTimes []time.Duration
	captureMeter, encodeMeter *telemetry.Meter
}
type Controller struct {
	mu         sync.Mutex
	source     capture.System
	origin     string
	token      [32]byte
	active     *session
	last       Stats
	generation uint64
	shutdown   bool
}

func New(source capture.System, origin string) (*Controller, error) {
	c := &Controller{source: source, origin: origin, last: Stats{State: Idle}}
	if _, err := rand.Read(c.token[:]); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Controller) Displays(ctx context.Context) ([]capture.Display, error) {
	return c.source.Displays(ctx)
}

func (c *Controller) Start(o capture.Options) (Started, error) {
	if _, err := o.Config(); err != nil {
		return Started{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shutdown || c.active != nil {
		return Started{}, errors.New("Preview đang chạy hoặc đang dừng.")
	}
	var nextToken [32]byte
	if _, err := rand.Read(nextToken[:]); err != nil {
		return Started{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	b, err := bridge.New(ctx, c.origin, c.token)
	if err != nil {
		cancel()
		return Started{}, errors.New("Không mở được preview loopback.")
	}
	c.token = nextToken
	c.generation++
	s := &session{cancel: cancel, done: make(chan struct{}), bridge: b,
		stats:        Stats{State: Preparing, Generation: c.generation, SamplesComplete: true},
		captureMeter: telemetry.New(nil), encodeMeter: telemetry.New(nil)}
	c.active = s
	go c.run(ctx, s, o)
	return Started{b.Endpoint(), c.generation}, nil
}

func (c *Controller) run(ctx context.Context, s *session, o capture.Options) {
	q := make(chan capture.Frame, 1)
	captureDone := make(chan error, 1)
	go func() {
		err := c.source.Run(ctx, o, func(f capture.Frame) {
			c.mu.Lock()
			s.stats.Captured++
			if s.stats.State == Preparing {
				s.stats.State = Previewing
			}
			appendSample(&s.captureTimes, f.Duration, &s.stats.SamplesComplete)
			_ = s.captureMeter.Record(telemetry.Event{Kind: telemetry.Captured})
			c.mu.Unlock()
			select {
			case q <- f:
			default:
				select {
				case <-q:
					c.mu.Lock()
					s.stats.Dropped++
					c.mu.Unlock()
				default:
				}
				select {
				case q <- f:
				default:
					c.mu.Lock()
					s.stats.Dropped++
					c.mu.Unlock()
				}
			}
		})
		captureDone <- err
	}()
	var failure error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-s.bridge.Done():
			break loop
		case err := <-captureDone:
			failure = err
			captureDone = nil
			break loop
		case f := <-q:
			start := time.Now()
			var buffer bytes.Buffer
			err := jpeg.Encode(&buffer, &image.RGBA{Pix: f.RGBA, Stride: f.Width * 4, Rect: image.Rect(0, 0, f.Width, f.Height)}, &jpeg.Options{Quality: 75})
			if err != nil || buffer.Len() > bridge.MaxJPEG {
				failure = errors.New("Không mã hóa được preview.")
				break loop
			}
			if ctx.Err() != nil {
				break loop
			}
			if err := s.bridge.Publish(buffer.Bytes()); err != nil {
				if !errors.Is(err, bridge.ErrClosed) {
					failure = err
				}
				break loop
			}
			c.mu.Lock()
			s.stats.Encoded++
			appendSample(&s.encodeTimes, time.Since(start), &s.stats.SamplesComplete)
			_ = s.encodeMeter.Record(telemetry.Event{Kind: telemetry.Captured})
			c.mu.Unlock()
		}
	}
	s.cancel()
	s.bridge.Close()
	if captureDone != nil {
		err := <-captureDone // Native capture owner must release handles before Idle.
		failure = errors.Join(failure, err)
	}
	c.mu.Lock()
	s.stats.Sent, s.stats.BridgeDropped = s.bridge.Counts()
	if s.bridge.Err() != nil {
		s.stats.State = Failed
		s.stats.Error = "Kênh xem trước cục bộ không khả dụng."
	} else if failure != nil && (!errors.Is(failure, context.Canceled) || errors.Is(failure, capture.ErrCapture)) {
		s.stats.State = Failed
		s.stats.Error = "Nguồn preview không khả dụng. Dừng và chọn lại màn hình."
	} else {
		s.stats.State = Idle
	}
	c.last = c.statsLocked(s)
	c.active = nil
	close(s.done)
	c.mu.Unlock()
}
func appendSample(samples *[]time.Duration, d time.Duration, complete *bool) {
	if len(*samples) < 32768 {
		*samples = append(*samples, d)
	} else {
		*complete = false
	}
}
func quantiles(v []time.Duration) (float64, float64) {
	if len(v) == 0 {
		return 0, 0
	}
	copyV := append([]time.Duration(nil), v...)
	sort.Slice(copyV, func(i, j int) bool { return copyV[i] < copyV[j] })
	return float64(copyV[(len(copyV)-1)*50/100]) / float64(time.Millisecond),
		float64(copyV[(len(copyV)-1)*95/100]) / float64(time.Millisecond)
}
func (c *Controller) statsLocked(s *session) Stats {
	out := s.stats
	out.Sent, out.BridgeDropped = s.bridge.Counts()
	cap, _ := s.captureMeter.Snapshot()
	enc, _ := s.encodeMeter.Snapshot()
	out.CaptureFPS, out.EncodeFPS = cap.CapturedFPS, enc.CapturedFPS
	out.CaptureP50Ms, out.CaptureP95Ms = quantiles(s.captureTimes)
	out.EncodeP50Ms, out.EncodeP95Ms = quantiles(s.encodeTimes)
	return out
}
func (c *Controller) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil {
		out := c.last
		out.CaptureFPS = 0
		out.EncodeFPS = 0
		return out
	}
	return c.statsLocked(c.active)
}
func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	s := c.active
	if s == nil {
		c.mu.Unlock()
		return nil
	}
	s.stats.State = Stopping
	s.cancel()
	c.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return errors.New("Dừng preview quá thời gian; vẫn đang chờ giải phóng capture.")
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
