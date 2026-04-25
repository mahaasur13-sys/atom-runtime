// Package invariant — Runtime invariant enforcement (ATOM-027).
// ATOM constraints: INV1–INV5.
//
// RULE: Any violation → panic immediately (fail-fast).
// No logging, no graceful degradation — the system halts.
package invariant

import (
	"fmt"
)

// InvariantGuard checks all ATOM invariants on every event.
// Embedded in EventStore.Append, WAL recovery, and Replay.
type InvariantGuard struct{}

// New creates an InvariantGuard.
func New() *InvariantGuard { return &InvariantGuard{} }

// CheckEvent validates event against all invariants.
// Panics with exact invariant name on violation.
func (g *InvariantGuard) CheckEvent(e Event, prev *Event) {
	// INV4: seq must be >= 1
	if e.Seq < 1 {
		panic(fmt.Sprintf("INV4 violated: seq=%d (< 1) traceID=%s tick=%d",
			e.Seq, e.TraceID, e.Tick))
	}

	if prev != nil {
		// INV2: seq must be prev+1 (no gaps)
		if e.Seq != prev.Seq+1 {
			panic(fmt.Sprintf("INV2 violated: seq gap — prev=%d curr=%d traceID=%s",
				prev.Seq, e.Seq, e.TraceID))
		}

		// INV1: hash chain must be unbroken
		if e.PrevHash != prev.EventHash {
			panic(fmt.Sprintf("INV1 violated: hash chain broken — prevHash=%s expected=%s traceID=%s tick=%d",
				e.PrevHash, prev.EventHash, e.TraceID, e.Tick))
		}

		// INV3: tick must be >= prev.Tick
		if e.Tick < prev.Tick {
			panic(fmt.Sprintf("INV3 violated: tick went backwards — prevTick=%d currTick=%d traceID=%s",
				prev.Tick, e.Tick, e.TraceID))
		}
	} else {
		// First event in trace: prevHash must be empty
		if e.PrevHash != "" {
			panic(fmt.Sprintf("INV1 violated: first event has non-empty prevHash=%s traceID=%s",
				e.PrevHash, e.TraceID))
		}
	}

	// INV5: EventHash must be non-empty
	if e.EventHash == "" {
		panic(fmt.Sprintf("INV5 violated: EventHash is empty traceID=%s tick=%d", e.TraceID, e.Tick))
	}
}

// CheckSeqMonotonic enforces that seq numbers are strictly increasing per trace.
// Call after loading WAL on recovery.
func (g *InvariantGuard) CheckSeqMonotonic(events []Event) {
	if len(events) < 2 {
		return
	}

	// Group by traceID
	byTrace := map[string]uint64{}
	for _, e := range events {
		last, ok := byTrace[e.TraceID]
		if ok && e.Seq <= last {
			panic(fmt.Sprintf("INV2 violated: seq non-increasing in trace %s — last=%d curr=%d",
				e.TraceID, last, e.Seq))
		}
		byTrace[e.TraceID] = e.Seq
	}
}

// CheckHashChain verifies the entire event chain is intact.
// Call after WAL recovery before replay.
func (g *InvariantGuard) CheckHashChain(events []Event) {
	if len(events) < 2 {
		return
	}

	for i := 1; i < len(events); i++ {
		if events[i].PrevHash != events[i-1].EventHash {
			panic(fmt.Sprintf("INV1 violated at index %d: chain broken traceID=%s tick=%d",
				i, events[i].TraceID, events[i].Tick))
		}
	}
}

// Event is the ATOM event record.
type Event struct {
	TraceID    string
	Seq        uint64
	Tick       uint64
	Type       string
	PrevHash   string
	EventHash  string
	SchemaID   string
	SchemaVer  string
	EpochMs    int64
}

// Guard is the global invariant checker.
var Guard = New()