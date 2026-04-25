// Package canon — ATOM canonical event definitions.
// Deterministic event types shared across all ATOM packages.
//
// ATOM constraints:
// - C1: No time.Now, time.Sleep, math/rand, uuid.New, unsorted map iteration
// - C2: Replay Equivalence — Replay(Snapshot) == Replay(Log)
// - C3: No hidden state
package canon

import "fmt"

// Event is the canonical ATOM event type.
// All fields are deterministic — no timestamps, no randomness.
type Event struct {
	TraceID   string
	Seq       uint64
	PrevHash  string
	EventHash string
	Payload   []byte
	Tick      uint64 // logical clock tick
}

// MakeEvent creates a deterministic event for testing.
// No randomness — all fields derived from index.
func MakeEvent(idx int) Event {
	return Event{
		TraceID:   "trace-main",
		Seq:       uint64(idx),
		PrevHash:  prevHash(idx),
		EventHash: eventHash(idx),
		Payload:   []byte("payload"),
		Tick:      uint64(idx),
	}
}

// eventHash returns a deterministic 64-char hex string for index i.
func eventHash(i int) string {
	h := uint64(i*31337 + 0x9e3779b9)
	return fmt.Sprintf("%064d", h)
}

// prevHash returns the prevHash for index i (empty for i==0).
func prevHash(i int) string {
	if i == 0 {
		return ""
	}
	return eventHash(i - 1)
}