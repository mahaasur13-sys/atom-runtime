package modelchecker

import (
	"testing"
)

func TestModelChecker_DeterministicPaths(t *testing.T) {
	mc := New()

	initial := State{TraceID: "t1", Seq: 1, Hash: "h0"}

	transitions := []Transition{
		{From: State{"t1", 1, "h0"}, To: State{"t1", 2, "h1"}, Event: "append"},
		{From: State{"t1", 2, "h1"}, To: State{"t1", 3, "h2"}, Event: "append"},
	}

	mc.SetTransitions(transitions)
	err := mc.Check(initial)
	if err != nil {
		t.Fatalf("expected valid path, got error: %v", err)
	}

	if mc.ExploredCount() != 3 {
		t.Fatalf("expected 3 states explored, got %d", mc.ExploredCount())
	}
}

func TestModelChecker_RejectsInvalidSeq(t *testing.T) {
	mc := New()

	// Invalid: seq jumps from 1 to 3 (gap)
	transitions := []Transition{
		{From: State{"t1", 1, "h0"}, To: State{"t1", 3, "h2"}, Event: "append"},
	}

	mc.SetTransitions(transitions)
	err := mc.Check(State{TraceID: "t1", Seq: 1, Hash: "h0"})

	if err == nil {
		t.Fatal("expected sequence violation error, got nil")
	}
}

func TestModelChecker_VisitedDeduplication(t *testing.T) {
	mc := New()

	// Two paths to the same state (diamond pattern)
	// t1:1 -> t1:2 -> t1:3
	// t1:1 -> t1:2 (via alternate route)
	transitions := []Transition{
		{From: State{"t1", 1, "h0"}, To: State{"t1", 2, "h1"}, Event: "a"},
		{From: State{"t1", 2, "h1"}, To: State{"t1", 3, "h2"}, Event: "b"},
		{From: State{"t1", 1, "h0"}, To: State{"t1", 2, "h1"}, Event: "c"}, // duplicate
	}

	mc.SetTransitions(transitions)
	err := mc.Check(State{TraceID: "t1", Seq: 1, Hash: "h0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// State t1:2 should be visited once, not twice.
	if mc.ExploredCount() != 3 {
		t.Fatalf("expected 3 unique states, got %d", mc.ExploredCount())
	}
}

func TestModelChecker_MultipleTraces(t *testing.T) {
	mc := New()

	transitions := []Transition{
		// trace t1
		{From: State{"t1", 1, "h0"}, To: State{"t1", 2, "h1"}, Event: "append"},
		// trace t2 (independent)
		{From: State{"t2", 1, "h0"}, To: State{"t2", 2, "h1"}, Event: "append"},
	}

	mc.SetTransitions(transitions)

	// Check from t1
	err := mc.Check(State{TraceID: "t1", Seq: 1, Hash: "h0"})
	if err != nil {
		t.Fatalf("t1 check failed: %v", err)
	}

	// Check from t2
	err = mc.Check(State{TraceID: "t2", Seq: 1, Hash: "h0"})
	if err != nil {
		t.Fatalf("t2 check failed: %v", err)
	}
}
