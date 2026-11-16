// Package telemetry collects bounded, measured application counters. It never
// reads packet payloads, clocks from peers, room codes, or transport secrets.
package telemetry

import (
	"errors"
	"sync"
	"time"

	"crocstream.local/app/internal/protocol"
)

const (
	Window   = time.Second
	Capacity = 4096 // Events retained in the exact rolling window.
)

var (
	ErrEvent    = errors.New("telemetry: invalid measurement")
	ErrCapacity = errors.New("telemetry: window capacity exceeded")
	ErrClock    = errors.New("telemetry: clock moved backwards")
)

type Kind uint8

const (
	Captured Kind = iota + 1
	Queued
	Dropped
	Emitted
	Received
	Decoded
	Rendered
)

// Event counts one observed action. Video fields are only valid for Emitted or
// Received. Bytes include the 18-byte media header, never transport/control bytes.
// BaselineBytes=0 means not measured; only Emitted can carry a baseline.
type Event struct {
	Kind          Kind
	VideoType     protocol.Type
	Bytes         int
	BaselineBytes int
	CodecTime     time.Duration
	BaselineTime  time.Duration
}

func (e Event) validate() error {
	if e.Kind < Captured || e.Kind > Rendered {
		return ErrEvent
	}
	if e.Kind == Decoded {
		if e.VideoType != 0 || e.Bytes != 0 || e.BaselineBytes != 0 || e.BaselineTime != 0 || e.CodecTime < 0 {
			return ErrEvent
		}
		return nil
	}
	if e.Kind != Emitted && e.Kind != Received {
		if e.VideoType != 0 || e.Bytes != 0 || e.BaselineBytes != 0 || e.CodecTime != 0 || e.BaselineTime != 0 {
			return ErrEvent
		}
		return nil
	}
	if (e.VideoType != protocol.Keyframe && e.VideoType != protocol.Delta) ||
		e.Bytes <= protocol.HeaderSize || e.Bytes > protocol.HeaderSize+protocol.MaxPayload ||
		e.CodecTime < 0 || e.BaselineTime < 0 {
		return ErrEvent
	}
	if e.Kind == Received {
		if e.BaselineBytes != 0 || e.BaselineTime != 0 || e.CodecTime != 0 {
			return ErrEvent
		}
	} else if (e.BaselineBytes != 0 && (e.BaselineBytes <= protocol.HeaderSize ||
		e.BaselineBytes > protocol.HeaderSize+protocol.MaxPayload)) ||
		e.BaselineTime > e.CodecTime || (e.BaselineBytes == 0 && e.BaselineTime != 0) {
		return ErrEvent
	}
	return nil
}

type entry struct {
	at    time.Time
	event Event
}

// Meter is safe for concurrent measurement/presentation. now is called under
// its lock; production uses time.Now (monotonic component), tests inject a clock.
// A full ring rejects new events and marks snapshots incomplete for one second;
// it never silently reports partial savings as exact.
type Meter struct {
	mu          sync.Mutex
	now         func() time.Time
	start, last time.Time
	lostAt      time.Time
	lost        bool
	ring        [Capacity]entry
	head, count int
}

func New(now func() time.Time) *Meter {
	if now == nil {
		now = time.Now
	}
	start := now()
	return &Meter{now: now, start: start, last: start}
}

func (m *Meter) advance(now time.Time) error {
	if now.Before(m.last) {
		return ErrClock
	}
	m.last = now
	cutoff := now.Add(-Window)
	for m.count > 0 && !m.ring[m.head].at.After(cutoff) {
		m.ring[m.head] = entry{}
		m.head = (m.head + 1) % Capacity
		m.count--
	}
	if m.lost && !m.lostAt.After(cutoff) {
		m.lost = false
	}
	return nil
}

func (m *Meter) Record(e Event) error {
	if err := e.validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if err := m.advance(now); err != nil {
		return err
	}
	if m.count == Capacity {
		m.lost, m.lostAt = true, now
		return ErrCapacity
	}
	m.ring[(m.head+m.count)%Capacity] = entry{now, e}
	m.count++
	return nil
}

type VideoWindow struct {
	Frames, Keys, Deltas         int
	Bytes                        uint64
	KiBPerSecond                 float64
	MeanKeyKiB, MeanDeltaKiB     float64
	KeyMeanKnown, DeltaMeanKnown bool
}

type Snapshot struct {
	MeanEncodeMs                                     float64
	MeanDecodeMs                                     float64
	QueuedCaptures                                   int
	Uptime                                           time.Duration
	Complete                                         bool
	CapturedFPS, EmittedFPS, DecodedFPS, RenderedFPS int
	DroppedCaptures                                  int
	Sent, Received                                   VideoWindow
	BaselineBytes                                    uint64
	BaselineKiBPerSecond                             float64
	BaselineKnown                                    bool
	MeanBaselineMs                                   float64
	SavingsPercent                                   float64
	SavingsKnown                                     bool
}

func (m *Meter) Snapshot() (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if err := m.advance(now); err != nil {
		return Snapshot{}, err
	}
	s := Snapshot{Uptime: now.Sub(m.start), Complete: !m.lost}
	var keyBytes, deltaBytes [2]uint64
	var encodeMs, decodeMs float64
	var baselineMs float64
	baselines := 0
	for i := 0; i < m.count; i++ {
		e := m.ring[(m.head+i)%Capacity].event
		switch e.Kind {
		case Captured:
			s.CapturedFPS++
		case Queued:
			s.QueuedCaptures++
		case Dropped:
			s.DroppedCaptures++
		case Rendered:
			s.RenderedFPS++
		case Decoded:
			s.DecodedFPS++
			decodeMs += float64(e.CodecTime) / float64(time.Millisecond)
		case Emitted, Received:
			index, video := 0, &s.Sent
			if e.Kind == Received {
				index, video = 1, &s.Received
			} else if e.BaselineBytes != 0 {
				baselines++
				s.BaselineBytes += uint64(e.BaselineBytes)
				baselineMs += float64(e.BaselineTime) / float64(time.Millisecond)
			}
			video.Frames++
			video.Bytes += uint64(e.Bytes)
			if e.Kind == Emitted {
				encodeMs += float64(e.CodecTime) / float64(time.Millisecond)
			}
			if e.VideoType == protocol.Keyframe {
				video.Keys++
				keyBytes[index] += uint64(e.Bytes)
			} else {
				video.Deltas++
				deltaBytes[index] += uint64(e.Bytes)
			}
		}
	}
	for i, v := range []*VideoWindow{&s.Sent, &s.Received} {
		v.KiBPerSecond = float64(v.Bytes) / 1024
		if v.Keys > 0 {
			v.MeanKeyKiB, v.KeyMeanKnown = float64(keyBytes[i])/float64(v.Keys)/1024, true
		}
		if v.Deltas > 0 {
			v.MeanDeltaKiB, v.DeltaMeanKnown = float64(deltaBytes[i])/float64(v.Deltas)/1024, true
		}
	}
	s.EmittedFPS = s.Sent.Frames
	if s.Sent.Frames > 0 {
		s.MeanEncodeMs = encodeMs / float64(s.Sent.Frames)
	}
	if s.DecodedFPS > 0 {
		s.MeanDecodeMs = decodeMs / float64(s.DecodedFPS)
	}
	s.BaselineKnown = s.Complete && s.Sent.Frames > 0 && baselines == s.Sent.Frames
	s.BaselineKiBPerSecond = float64(s.BaselineBytes) / 1024
	if baselines > 0 {
		s.MeanBaselineMs = baselineMs / float64(baselines)
	}
	s.SavingsKnown = s.BaselineKnown && s.BaselineBytes > 0
	if s.SavingsKnown {
		s.SavingsPercent = 100 * (1 - float64(s.Sent.Bytes)/float64(s.BaselineBytes))
	}
	return s, nil
}
