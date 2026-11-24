package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"crocstream.local/app/internal/session"
)

func validResults() []result {
	hash := sha256.Sum256(frame(29).RGBA)
	return []result{
		{Role: "host", Counters: session.Counters{Emitted: 30, Keys: 2, Deltas: 28, PLIAccepted: 1, Pongs: 2, LastID: 30, SentBytes: 10000, RTTKnown: true}, LastHash: hex.EncodeToString(hash[:]), ActiveSeconds: 3},
		{Role: "viewer", Counters: session.Counters{Decoded: 29, Received: 30, Keys: 2, Deltas: 27, PLIRequested: 1, Recoveries: 1, Pongs: 2, LastID: 30, ReceivedBytes: 10000, Ended: true, RTTKnown: true}, LastHash: hex.EncodeToString(hash[:]), ActiveSeconds: 3},
	}
}
func TestVerifyRejectsIncompleteMediaEvidence(t *testing.T) {
	if err := verify(validResults(), 3*time.Second); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]result){
		func(r []result) { r[1].LastHash = "bad" },
		func(r []result) { r[0].LastHash = r[1].LastHash; r[0].Counters.LastID = 29; r[1].Counters.LastID = 29 },
		func(r []result) { r[1].Counters.ReceivedBytes-- },
		func(r []result) { r[1].Counters.Received-- },
		func(r []result) { r[1].Counters.Recoveries = 0 },
		func(r []result) { r[0].Counters.Pongs = 0 },
		func(r []result) { r[0].ActiveSeconds = math.NaN() },
		func(r []result) { r[1].ActiveSeconds = math.Inf(1) },
		func(r []result) { r[1].ActiveSeconds = 1 },
		func(r []result) { r[1].Counters.Ended = false },
	} {
		r := validResults()
		mutate(r)
		if err := verify(r, 3*time.Second); err == nil {
			t.Fatal("accepted incomplete evidence")
		}
	}
}
