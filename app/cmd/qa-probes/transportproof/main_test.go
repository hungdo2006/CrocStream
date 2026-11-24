package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"
	"time"
)

func TestHarnessParserRejectsMalformedLengths(t *testing.T) {
	for _, kind := range []byte{0, 1, 2, 3, 4, 5, 6, 255} {
		for _, length := range []uint32{0, 1, blockSize, blockSize + 1, ^uint32(0)} {
			p := make([]byte, 13)
			p[0] = kind
			binary.BigEndian.PutUint32(p[9:], length)
			if length <= blockSize {
				p = append(p, make([]byte, int(length))...)
			}
			_, err := readMessage(bytes.NewReader(p))
			valid := (kind == 1 && length == blockSize) || (kind >= 2 && kind <= 6 && length == 0)
			if (err == nil) != valid {
				t.Fatalf("kind=%d length=%d err=%v", kind, length, err)
			}
		}
	}
	for cut := 0; cut < 13; cut++ {
		if _, err := readMessage(bytes.NewReader(make([]byte, cut))); err == nil {
			t.Fatal("truncation accepted")
		}
	}
}

func TestVerifyResults(t *testing.T) {
	var all []result
	for _, role := range []string{"host", "viewer"} {
		sender := byte(0)
		if role == "viewer" {
			sender = 1
		}
		h := sha256.New()
		b := make([]byte, blockSize)
		for seq := uint64(0); seq < 320; seq++ {
			fill(b, seq, sender)
			_, _ = h.Write(b)
		}
		all = append(all, result{Role: role, BytesSent: 320 * blockSize,
			BytesReceived: 320 * blockSize, SHA256: hex.EncodeToString(h.Sum(nil)),
			PingsSent: 10, PongsReceived: 10, KeyRequestsReceived: 10, DurationSeconds: 600})
	}
	if err := verifyResults(all, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := verifyResults([]result{all[1], all[0]}, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func([]result) []result
	}{
		{"missing", func(r []result) []result { return r[:1] }},
		{"same-role", func(r []result) []result { r[1].Role = r[0].Role; return r }},
		{"unknown-role", func(r []result) []result { r[0].Role = "other"; return r }},
		{"count", func(r []result) []result { r[0].BytesSent += blockSize; return r }},
		{"small", func(r []result) []result { r[0].BytesReceived = 0; return r }},
		{"huge", func(r []result) []result { r[0].BytesReceived = ^uint64(0); return r }},
		{"unaligned", func(r []result) []result { r[0].BytesReceived++; return r }},
		{"missing-pong", func(r []result) []result { r[0].PongsReceived--; return r }},
		{"no-ping", func(r []result) []result { r[0].PingsSent = 0; return r }},
		{"many-pings", func(r []result) []result { r[0].PingsSent = 321; return r }},
		{"key-count", func(r []result) []result { r[0].KeyRequestsReceived--; return r }},
		{"short", func(r []result) []result { r[0].DurationSeconds = 599; return r }},
		{"long", func(r []result) []result { r[0].DurationSeconds = 646; return r }},
		{"nan", func(r []result) []result { r[0].DurationSeconds = math.NaN(); return r }},
		{"infinite", func(r []result) []result { r[0].DurationSeconds = math.Inf(1); return r }},
		{"hash", func(r []result) []result { r[0].SHA256 = "wrong"; return r }},
		{"interval", func(r []result) []result { r[0].DataIntervalMS = 101; return r }},
		{"write-nan", func(r []result) []result { r[0].DataWriteMeanMS = math.NaN(); return r }},
		{"write-negative", func(r []result) []result { r[0].DataWriteMaxMS = -1; return r }},
	} {
		t.Run(change.name, func(t *testing.T) {
			if err := verifyResults(change.edit(append([]result(nil), all...)), 10*time.Minute); err == nil {
				t.Fatal("accepted invalid results")
			}
		})
	}
	for _, d := range []time.Duration{0, 31 * time.Minute} {
		if err := verifyResults(all, d); err == nil {
			t.Fatal("accepted invalid expected duration")
		}
	}
}

func TestNormalizeInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, 20 * time.Millisecond, 100 * time.Millisecond} {
		got, err := normalizeInterval(interval)
		if err != nil || got < 20*time.Millisecond || got > 100*time.Millisecond {
			t.Fatal(got, err)
		}
	}
	for _, interval := range []time.Duration{-1, 19 * time.Millisecond, 101 * time.Millisecond, 20500 * time.Microsecond} {
		if _, err := normalizeInterval(interval); err == nil {
			t.Fatal("accepted unsupported pacing", interval)
		}
	}
}
