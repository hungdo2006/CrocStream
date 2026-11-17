package protocol

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReceivedAccountingIncludesDiscardButNotTruncation(t *testing.T) {
	r := reader(t, bytes.NewReader(wire(t, configPacket(t), video(Keyframe, 1), video(Delta, 3), video(Keyframe, 4))), FromBroadcaster)
	readOK(t, r)
	readOK(t, r)
	resolve(t, r, true)
	if _, err := r.ReadPacket(); !errors.Is(err, ErrNeedKeyframe) {
		t.Fatal(err)
	}
	h := r.LastReceived()
	if h.Type != Delta || h.FrameID != 3 || h.PayloadLength != 5 {
		t.Fatal("discard accounting lost")
	}
	readOK(t, r)
	resolve(t, r, true)
	if _, err := r.ReadPacket(); err != io.EOF {
		t.Fatal(err)
	}
	if r.LastReceived() != (Header{}) {
		t.Fatal("stale accounting after EOF")
	}
	b := wire(t, configPacket(t), video(Keyframe, 1))
	r = reader(t, bytes.NewReader(b[:len(b)-1]), FromBroadcaster)
	readOK(t, r)
	if _, err := r.ReadPacket(); err == nil || r.LastReceived() != (Header{}) {
		t.Fatal("truncated body counted as complete")
	}
}
