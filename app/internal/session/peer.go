// Package session integrates codec and control packets over an authenticated
// duplex stream. It owns one parser and one writer per peer, bounded queues and
// cancellation; it knows nothing about croc crypto, capture APIs or Wails.
package session

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"crocstream.local/app/internal/capture"
	"crocstream.local/app/internal/delta"
	"crocstream.local/app/internal/protocol"
	"crocstream.local/app/internal/telemetry"
)

var ErrControlFlood = errors.New("session: control queue full")

type State string

const (
	Idle           State = "Idle"
	Preparing      State = "Preparing"
	Waiting        State = "WaitingForViewer"
	Connecting     State = "Connecting"
	Authenticating State = "Authenticating"
	Awaiting       State = "AwaitingKeyframe"
	Live           State = "Live"
	Watching       State = "Watching"
	Recovering     State = "Recovering"
	Stopping       State = "Stopping"
	Disconnected   State = "Disconnected"
	Failed         State = "Error"
)

type Counters struct {
	Emitted            uint64  `json:"emitted"`
	Decoded            uint64  `json:"decoded"`
	Received           uint64  `json:"received"`
	Keys               uint64  `json:"keys"`
	Deltas             uint64  `json:"deltas"`
	PLIRequested       uint64  `json:"pliRequested"`
	PLIAccepted        uint64  `json:"pliAccepted"`
	Recoveries         uint64  `json:"recoveries"`
	Pongs              uint64  `json:"pongs"`
	LastID             uint32  `json:"lastID"`
	RTTMs              float64 `json:"rttMs"`
	RTTKnown           bool    `json:"rttKnown"`
	Ended              bool    `json:"ended"`
	SentBytes          uint64  `json:"sentBytes"`
	ReceivedBytes      uint64  `json:"receivedBytes"`
	SentKeyBytes       uint64  `json:"sentKeyBytes"`
	SentDeltaBytes     uint64  `json:"sentDeltaBytes"`
	ReceivedKeyBytes   uint64  `json:"receivedKeyBytes"`
	ReceivedDeltaBytes uint64  `json:"receivedDeltaBytes"`
}
type Metrics struct {
	mu    sync.Mutex
	c     Counters
	meter *telemetry.Meter
}

func NewMetrics() *Metrics { return &Metrics{meter: telemetry.New(nil)} }
func (m *Metrics) Read() (Counters, telemetry.Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, _ := m.meter.Snapshot()
	return m.c, s
}
func (m *Metrics) update(f func(*Counters)) { m.mu.Lock(); f(&m.c); m.mu.Unlock() }
func (m *Metrics) Record(e telemetry.Event) { _ = m.meter.Record(e) }

// Hooks are synchronous and must return promptly. Images are owned snapshots;
// UI handoff should put them in a latest-only queue, not perform JPEG work here.
// Reference is optional test/diagnostic observation; never log/store pixels.
type Hooks struct {
	State     func(State)
	Image     func(capture.Frame)
	Reference func(uint32, []byte)
}

func (h Hooks) state(s State) {
	if h.State != nil {
		h.State(s)
	}
}

type Peer struct {
	conn        io.ReadWriteCloser
	host        bool
	tau         uint8
	metrics     *Metrics
	hooks       Hooks
	pong        chan [8]byte
	pli         atomic.Bool
	wake        chan struct{}
	recover     chan struct{}
	pingMu      sync.Mutex
	ping        [8]byte
	pingAt      time.Time
	pingWaiting bool
	remoteEnd   atomic.Bool
}

func NewPeer(conn io.ReadWriteCloser, host bool, m *Metrics, h Hooks) *Peer {
	return NewPeerWithTau(conn, host, m, h, delta.DefaultTau)
}

func NewPeerWithTau(conn io.ReadWriteCloser, host bool, m *Metrics, h Hooks, tau uint8) *Peer {
	return &Peer{conn: conn, host: host, tau: tau, metrics: m, hooks: h, pong: make(chan [8]byte, 8), wake: make(chan struct{}, 1), recover: make(chan struct{}, 1)}
}

// RequestRecovery models local decoder invalidation (also used by tests).
// The receive loop still owns reference mutation and rate-limited wire PLI.
func (p *Peer) RequestRecovery() {
	select {
	case p.recover <- struct{}{}:
	default:
	}
}
func (p *Peer) control(packet protocol.Packet, lastPLI *time.Time) error {
	switch packet.Header.Type {
	case protocol.Ping:
		var nonce [8]byte
		copy(nonce[:], packet.Payload)
		select {
		case p.pong <- nonce:
		default:
			return ErrControlFlood
		}
	case protocol.Pong:
		p.pingMu.Lock()
		if p.pingWaiting && string(packet.Payload) == string(p.ping[:]) {
			elapsed := time.Since(p.pingAt)
			p.pingWaiting = false
			p.metrics.update(func(c *Counters) {
				c.Pongs++
				c.RTTKnown = true
				c.RTTMs = float64(elapsed) / float64(time.Millisecond)
			})
		}
		p.pingMu.Unlock()
	case protocol.RequestKeyframe:
		if lastPLI.IsZero() || time.Since(*lastPLI) >= time.Second {
			*lastPLI = time.Now()
			p.pli.Store(true)
			p.metrics.update(func(c *Counters) { c.PLIAccepted++ })
		}
	case protocol.End:
		p.remoteEnd.Store(true)
		p.metrics.update(func(c *Counters) { c.Ended = true })
		return io.EOF
	}
	return nil
}
func controlPacket(kind protocol.Type, payload []byte) protocol.Packet {
	return protocol.Packet{Header: protocol.Header{Type: kind, PayloadLength: uint32(len(payload))}, Payload: payload}
}
func (p *Peer) pingPacket() (*protocol.Packet, error) {
	p.pingMu.Lock()
	defer p.pingMu.Unlock()
	if p.pingWaiting {
		if time.Since(p.pingAt) > 30*time.Second {
			return nil, errors.New("session: unanswered ping timeout")
		}
		return nil, nil
	}
	if _, err := rand.Read(p.ping[:]); err != nil {
		return nil, err
	}
	p.pingAt = time.Now()
	p.pingWaiting = true
	packet := controlPacket(protocol.Ping, append([]byte(nil), p.ping[:]...))
	return &packet, nil
}

func (p *Peer) receive(ctx context.Context, r *protocol.Reader) error {
	var decoder *delta.Decoder
	var cfg protocol.Config
	var lastRequest, lastAccepted time.Time
	recovering := false
	request := func() {
		recovering = true
		if lastRequest.IsZero() || time.Since(lastRequest) >= time.Second {
			lastRequest = time.Now()
			p.pli.Store(true)
			select {
			case p.wake <- struct{}{}:
			default:
			}
		}
	}
	for {
		packet, err := r.ReadPacket()
		h := r.LastReceived()
		if h.Type == protocol.Keyframe || h.Type == protocol.Delta {
			p.metrics.Record(telemetry.Event{Kind: telemetry.Received, VideoType: h.Type, Bytes: protocol.HeaderSize + int(h.PayloadLength)})
			p.metrics.update(func(c *Counters) {
				c.Received++
				c.ReceivedBytes += uint64(protocol.HeaderSize) + uint64(h.PayloadLength)
				if h.Type == protocol.Keyframe {
					c.ReceivedKeyBytes += uint64(protocol.HeaderSize) + uint64(h.PayloadLength)
				} else {
					c.ReceivedDeltaBytes += uint64(protocol.HeaderSize) + uint64(h.PayloadLength)
				}
			})
		}
		if errors.Is(err, protocol.ErrNeedKeyframe) && !p.host {
			if decoder != nil {
				decoder.Invalidate()
			}
			p.hooks.state(Recovering)
			request()
			continue
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		switch packet.Header.Type {
		case protocol.SessionConfig:
			cfg, _ = r.Config()
			decoder, err = delta.NewDecoder(cfg)
			if err != nil {
				return err
			}
		case protocol.Keyframe, protocol.Delta:
			if decoder == nil {
				return protocol.ErrConfigRequired
			}
			select {
			case <-p.recover:
				decoder.Invalidate()
				p.hooks.state(Recovering)
				request()
			default:
			}
			m, decodeErr := decoder.Decode(ctx, packet)
			if err := r.ResolveVideo(decodeErr == nil); err != nil {
				return err
			}
			if decodeErr != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				p.hooks.state(Recovering)
				request()
				continue
			}
			p.metrics.Record(telemetry.Event{Kind: telemetry.Decoded, CodecTime: m.DecodeTime})
			p.metrics.update(func(c *Counters) {
				c.Decoded++
				c.LastID = m.FrameID
				if m.Kind == protocol.Keyframe {
					c.Keys++
					if recovering {
						c.Recoveries++
					}
				} else {
					c.Deltas++
				}
			})
			recovering = false
			p.hooks.state(Watching)
			if p.hooks.Image != nil || p.hooks.Reference != nil {
				image, id, _ := decoder.Snapshot()
				if p.hooks.Reference != nil {
					p.hooks.Reference(id, image)
				}
				if p.hooks.Image != nil {
					p.hooks.Image(capture.Frame{RGBA: image, Width: int(cfg.Width), Height: int(cfg.Height), At: time.Now()})
				}
			}
		default:
			if err := p.control(packet, &lastAccepted); err != nil {
				return err
			}
		}
	}
}

// Run is synchronous. frames is host-only, raw-capture latest-only admission.
// The session must stop producing when ctx is canceled. No video packet is
// queued/dropped after encoding; this goroutine is the only stream writer.
func (p *Peer) Run(ctx context.Context, cfg protocol.Config, frames <-chan capture.Frame) (result error) {
	direction, reverse := protocol.FromViewer, protocol.FromBroadcaster
	if p.host {
		direction, reverse = reverse, direction
	}
	w, _ := protocol.NewWriter(p.conn, direction)
	r, _ := protocol.NewReader(p.conn, reverse)
	life, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancellation closes blocked I/O after a short chance for best-effort END.
	stop := context.AfterFunc(ctx, func() { time.AfterFunc(200*time.Millisecond, func() { _ = p.conn.Close() }) })
	defer stop()
	readDone := make(chan error, 1)
	readStopped := make(chan struct{})
	go func() {
		err := p.receive(life, r)
		readDone <- err
		close(readStopped)
		// A failed reader must also unblock a writer stuck in the same duplex.
		if err != nil && !errors.Is(err, io.EOF) {
			_ = p.conn.Close()
		} else {
			// Even clean END/EOF must interrupt an already blocked video write.
			// Give the sole writer a bounded opportunity to reply with END.
			time.AfterFunc(200*time.Millisecond, func() { _ = p.conn.Close() })
		}
	}()
	readFinished := false
	defer func() {
		// Validated END can race with a Ping/Pong or an in-flight video write.
		// Socket closure following that END is normal peer shutdown, not media
		// corruption; context cancellation and unexpected disconnect stay errors.
		if result != nil && p.remoteEnd.Load() && ctx.Err() == nil {
			result = nil
		}
		cancel()
		closer := time.AfterFunc(200*time.Millisecond, func() { _ = p.conn.Close() })
		_ = w.WritePacket(controlPacket(protocol.End, nil))
		_ = p.conn.Close()
		closer.Stop()
		if !readFinished {
			<-readDone
		}
	}()
	var encoder *delta.Encoder
	if p.host {
		var err error
		encoder, err = delta.NewEncoder(cfg, p.tau)
		if err != nil {
			return err
		}
		config, err := protocol.EncodeConfig(cfg)
		if err != nil {
			return err
		}
		if err = w.WritePacket(controlPacket(protocol.SessionConfig, config[:])); err != nil {
			return err
		}
	} else {
		p.hooks.state(Awaiting)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	first := true
	var lastSentPLI time.Time
	receivedResult := func() error {
		readFinished = true
		err := <-readDone
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case <-readStopped:
			return receivedResult()
		default:
		}
		// Replies have bounded priority at packet boundaries. Encoder work already
		// in progress completes; PLI is applied at the next encoder turn.
		for n := 0; n < cap(p.pong); n++ {
			select {
			case nonce := <-p.pong:
				if err := w.WritePacket(controlPacket(protocol.Pong, nonce[:])); err != nil {
					return err
				}
			default:
				n = cap(p.pong)
			}
		}
		if !p.host && (lastSentPLI.IsZero() || time.Since(lastSentPLI) >= time.Second) && p.pli.Swap(false) {
			if err := w.WritePacket(controlPacket(protocol.RequestKeyframe, nil)); err != nil {
				return err
			}
			lastSentPLI = time.Now()
			p.metrics.update(func(c *Counters) { c.PLIRequested++ })
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.wake:
		case <-readStopped:
			return receivedResult()
		case nonce := <-p.pong:
			if err := w.WritePacket(controlPacket(protocol.Pong, nonce[:])); err != nil {
				return err
			}
		case <-ticker.C:
			packet, err := p.pingPacket()
			if err != nil {
				return err
			}
			if packet != nil {
				if err = w.WritePacket(*packet); err != nil {
					return err
				}
			}
		case f, ok := <-frames:
			if !ok {
				return nil
			}
			if !p.host {
				return errors.New("session: viewer cannot capture")
			}
			select {
			case <-readStopped:
				return receivedResult()
			default:
			}
			if p.pli.Swap(false) {
				encoder.RequestKeyframe()
			}
			m, err := encoder.Emit(ctx, w, f.RGBA, f.At.UnixMilli())
			if err != nil {
				return err
			}
			p.metrics.Record(telemetry.Event{Kind: telemetry.Emitted, VideoType: m.Kind, Bytes: m.PacketBytes, BaselineBytes: m.BaselineBytes, CodecTime: m.EncodeTime, BaselineTime: m.BaselineTime})
			p.metrics.update(func(c *Counters) {
				c.Emitted++
				c.SentBytes += uint64(m.PacketBytes)
				if m.Kind == protocol.Keyframe {
					c.SentKeyBytes += uint64(m.PacketBytes)
				} else {
					c.SentDeltaBytes += uint64(m.PacketBytes)
				}
				c.LastID = m.FrameID
				if m.Kind == protocol.Keyframe {
					c.Keys++
				} else {
					c.Deltas++
				}
			})
			if p.hooks.Reference != nil {
				image, id := encoder.Snapshot()
				p.hooks.Reference(id, image)
			}
			if first {
				p.hooks.state(Live)
				first = false
			}
		}
	}
}
