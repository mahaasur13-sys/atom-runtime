package snapshot

import (
	"testing"
)

func TestSnapshot_FastReplayParity(t *testing.T) {
	// Build log of 10 events for 2 traces.
	// t1: seq 1-8, t2: seq 1-2.
	log := []Event{
		{TraceID: "t1", Seq: 1, EventHash: "h1"},
		{TraceID: "t1", Seq: 2, EventHash: "h2"},
		{TraceID: "t1", Seq: 3, EventHash: "h3"},
		{TraceID: "t1", Seq: 4, EventHash: "h4"},
		{TraceID: "t2", Seq: 1, EventHash: "i1"},
		{TraceID: "t2", Seq: 2, EventHash: "i2"},
		{TraceID: "t1", Seq: 5, EventHash: "h5"},
		{TraceID: "t1", Seq: 6, EventHash: "h6"},
		{TraceID: "t1", Seq: 7, EventHash: "h7"},
		{TraceID: "t1", Seq: 8, EventHash: "h8"},
	}

	// Snapshot after 5 events (seqs 1-5 for t1, 1-2 for t2).
	snap := New()
	for i := 0; i < 5; i++ {
		snap.LastSeq[log[i].TraceID] = log[i].Seq
		snap.LastHash[log[i].TraceID] = log[i].EventHash
	}
	snap.Timestamp = 5

	// Fast replay: apply remaining 5 events.
	replayed := FastReplay(snap, log[5:])

	// Final seq must match full log.
	want := map[string]uint64{"t1": 8, "t2": 2}
	for traceID, wantSeq := range want {
		gotSeq := replayed.LastSeq[traceID]
		if gotSeq != wantSeq {
			t.Errorf("trace %s: want seq=%d got seq=%d", traceID, wantSeq, gotSeq)
		}
	}
}

func TestSnapshot_SequenceViolationPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on sequence violation")
		}
	}()

	snap := New()
	snap.LastSeq["t1"] = 5

	badEvents := []Event{
		{TraceID: "t1", Seq: 7, EventHash: "x"},
	}

	FastReplay(snap, badEvents)
}

func TestSnapshot_FirstEventMustBeSeq1(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when first event seq != 1")
		}
	}()

	snap := New()
	badEvents := []Event{
		{TraceID: "t1", Seq: 2, EventHash: "x"},
	}

	FastReplay(snap, badEvents)
}

func TestSnapshot_EquivalenceProof(t *testing.T) {
	log := []Event{
		{TraceID: "t1", Seq: 1, EventHash: "a"},
		{TraceID: "t1", Seq: 2, EventHash: "b"},
		{TraceID: "t1", Seq: 3, EventHash: "c"},
	}

	snap := CreateSnapshot(map[string]uint64{"t1": 1}, map[string]string{"t1": "a"}, 1)

	err := EquivalenceProof(snap, log[1:], 2)
	if err != nil {
		t.Fatalf("equivalence proof failed: %v", err)
	}
}

func TestSnapshot_CloneIsIndependent(t *testing.T) {
	orig := New()
	orig.LastSeq["t1"] = 5

	clone := orig.Clone()
	clone.LastSeq["t1"] = 10

	if orig.LastSeq["t1"] != 5 {
		t.Fatal("clone should not mutate original")
	}
}
