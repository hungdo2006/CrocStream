package telemetry

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"crocstream.local/app/internal/protocol"
)

func record(t testing.TB, m *Meter, e Event) {
	t.Helper()
	if err := m.Record(e); err != nil {
		t.Fatal(err)
	}
}
func snapshot(t testing.TB, m *Meter) Snapshot {
	t.Helper()
	s, err := m.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestExactRollingWindowAndFairSavings(t *testing.T) {
	now := time.Unix(100, 0)
	m := New(func() time.Time { return now })
	s := snapshot(t, m)
	if s.SavingsKnown || s.BaselineKnown || s.Sent.KeyMeanKnown || !s.Complete {
		t.Fatal("empty state")
	}
	record(t, m, Event{Kind: Captured})
	record(t, m, Event{Kind: Queued})
	record(t, m, Event{Kind: Emitted, VideoType: protocol.Keyframe, Bytes: 1024, BaselineBytes: 1024, CodecTime: 4 * time.Millisecond, BaselineTime: 3 * time.Millisecond})
	now = now.Add(500 * time.Millisecond)
	record(t, m, Event{Kind: Captured})
	record(t, m, Event{Kind: Dropped})
	record(t, m, Event{Kind: Emitted, VideoType: protocol.Delta, Bytes: 512, BaselineBytes: 2048, CodecTime: 8 * time.Millisecond, BaselineTime: 4 * time.Millisecond})
	record(t, m, Event{Kind: Received, VideoType: protocol.Keyframe, Bytes: 1024})
	record(t, m, Event{Kind: Received, VideoType: protocol.Delta, Bytes: 512}) // Corrupt body: received, never decoded.
	record(t, m, Event{Kind: Decoded, CodecTime: 2 * time.Millisecond})
	record(t, m, Event{Kind: Rendered})
	s = snapshot(t, m)
	if s.CapturedFPS != 2 || s.QueuedCaptures != 1 || s.EmittedFPS != 2 || s.DecodedFPS != 1 || s.RenderedFPS != 1 || s.DroppedCaptures != 1 ||
		s.Sent.KiBPerSecond != 1.5 || s.BaselineKiBPerSecond != 3 || !s.SavingsKnown || s.SavingsPercent != 50 ||
		s.Sent.MeanKeyKiB != 1 || s.Sent.MeanDeltaKiB != 0.5 || s.MeanEncodeMs != 6 || s.MeanBaselineMs != 3.5 ||
		s.Received.Bytes != 1536 || s.Received.Frames != 2 || s.MeanDecodeMs != 2 {
		t.Fatal(s)
	}
	now = now.Add(500 * time.Millisecond) // Exact lower boundary is excluded.
	s = snapshot(t, m)
	if s.EmittedFPS != 1 || s.Sent.Keys != 0 || s.Sent.KeyMeanKnown || s.Sent.Bytes != 512 || s.BaselineBytes != 2048 || s.SavingsPercent != 75 {
		t.Fatal(s)
	}
	now = now.Add(time.Second)
	s = snapshot(t, m)
	if s.EmittedFPS != 0 || s.SavingsKnown || s.Sent.Bytes != 0 || s.Uptime != 2*time.Second {
		t.Fatal(s)
	}
}

func TestNegativeZeroAndUnknownSavings(t *testing.T) {
	for _, tt := range []struct {
		bytes, base int
		known       bool
		percent     float64
	}{
		{1024, 1024, true, 0}, {2048, 1024, true, -100}, {1024, 0, false, 0},
	} {
		m := New(nil)
		record(t, m, Event{Kind: Emitted, VideoType: protocol.Delta, Bytes: tt.bytes, BaselineBytes: tt.base})
		s := snapshot(t, m)
		if s.SavingsKnown != tt.known || s.SavingsPercent != tt.percent || math.IsNaN(s.SavingsPercent) {
			t.Fatal(s)
		}
	}
	m := New(nil)
	record(t, m, Event{Kind: Emitted, VideoType: protocol.Keyframe, Bytes: 100, BaselineBytes: 100})
	record(t, m, Event{Kind: Emitted, VideoType: protocol.Delta, Bytes: 50})
	if s := snapshot(t, m); s.SavingsKnown || s.BaselineKnown {
		t.Fatal("partial baseline labeled exact")
	}
}

func TestCapacityAndClockFailuresAreExplicit(t *testing.T) {
	now := time.Unix(100, 0)
	m := New(func() time.Time { return now })
	for i := 0; i < Capacity; i++ {
		record(t, m, Event{Kind: Captured})
	}
	if err := m.Record(Event{Kind: Captured}); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	if s := snapshot(t, m); s.Complete || s.CapturedFPS != Capacity || s.SavingsKnown {
		t.Fatal(s)
	}
	now = now.Add(time.Second)
	if s := snapshot(t, m); !s.Complete || s.CapturedFPS != 0 {
		t.Fatal("ring did not expire", s)
	}
	record(t, m, Event{Kind: Captured}) // Exercise ring wrap.
	now = now.Add(-time.Nanosecond)
	if err := m.Record(Event{Kind: Captured}); !errors.Is(err, ErrClock) {
		t.Fatal(err)
	}
	if _, err := m.Snapshot(); !errors.Is(err, ErrClock) {
		t.Fatal(err)
	}
}

func TestRejectsControlsAndImpossibleMeasurements(t *testing.T) {
	tests := []Event{
		{}, {Kind: 99}, {Kind: Captured, Bytes: 1},
		{Kind: Emitted, VideoType: protocol.Ping, Bytes: 26},
		{Kind: Emitted, VideoType: protocol.Keyframe, Bytes: 18},
		{Kind: Emitted, VideoType: protocol.Keyframe, Bytes: protocol.MaxPayload + 19},
		{Kind: Emitted, VideoType: protocol.Keyframe, Bytes: 100, BaselineBytes: -1},
		{Kind: Emitted, VideoType: protocol.Keyframe, Bytes: 100, BaselineBytes: 18},
		{Kind: Decoded, VideoType: protocol.Delta, Bytes: 100, BaselineBytes: 100},
		{Kind: Decoded, VideoType: protocol.Delta, Bytes: 100, CodecTime: -1},
		{Kind: Emitted, VideoType: protocol.Delta, Bytes: 100, BaselineBytes: 100, CodecTime: time.Millisecond, BaselineTime: time.Second},
		{Kind: Emitted, VideoType: protocol.Delta, Bytes: 100, CodecTime: time.Millisecond, BaselineTime: time.Millisecond},
	}
	m := New(nil)
	for _, e := range tests {
		if err := m.Record(e); !errors.Is(err, ErrEvent) {
			t.Fatal(e, err)
		}
	}
	if s := snapshot(t, m); s.Sent.Frames != 0 || s.Received.Frames != 0 || !s.Complete {
		t.Fatal(s)
	}
}

func TestConcurrentMeasurementAndPresentation(t *testing.T) {
	// Constant clock makes the expected exact count independent of scheduling.
	m := New(func() time.Time { return time.Unix(100, 0) })
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if err := m.Record(Event{Kind: Captured}); err != nil {
					t.Error(err)
				}
				if _, err := m.Snapshot(); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if s := snapshot(t, m); s.CapturedFPS != 400 {
		t.Fatal(s)
	}
}
