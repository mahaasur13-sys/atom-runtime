// Package trace — ATOM-032 Execution Trace Recorder.
// Records every event for step-by-step replay and diff between runs.
package trace

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Entry is one trace step.
type Entry struct {
	TraceID   string `json:"trace_id"`
	Tick      uint64 `json:"tick"`
	Event     string `json:"event"`
	PrevHash  string `json:"prev_hash,omitempty"`
	EventHash string `json:"event_hash,omitempty"`
	NodeID    string `json:"node,omitempty"`
	Seq       uint64 `json:"seq,omitempty"`
	SchemaID  string `json:"schema_id,omitempty"`
	Payload   []byte `json:"payload,omitempty"`
}

// Trace records all events in execution order.
type Trace struct {
	mu     sync.Mutex
	steps  []Entry
	id     string
	startTick uint64
}

// New creates a Trace with given id.
func New(traceID string) *Trace {
	return &Trace{id: traceID, steps: make([]Entry, 0, 1024)}
}

// Record adds an entry.
func (t *Trace) Record(e Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.steps = append(t.steps, e)
}

// RecordAppend is a convenience for append events.
func (t *Trace) RecordAppend(traceID string, tick, seq uint64, prevHash, eventHash, nodeID string) {
	t.Record(Entry{
		TraceID:   traceID,
		Tick:      tick,
		Event:     "append",
		PrevHash:  prevHash,
		EventHash: eventHash,
		NodeID:    nodeID,
		Seq:       seq,
	})
}

// Steps returns a copy of all steps.
func (t *Trace) Steps() []Entry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Entry(nil), t.steps...)
}

// JSON returns the full trace as JSON.
func (t *Trace) JSON() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, _ := json.MarshalIndent(t.steps, "", "  ")
	return string(b)
}

// Replay returns the sequence of events for step-by-step replay.
func (t *Trace) Replay() []Entry {
	return t.Steps()
}

// Diff compares two traces and returns divergence details.
func Diff(a, b *Trace) string {
	stepsA := a.Steps()
	stepsB := b.Steps()

	if len(stepsA) != len(stepsB) {
		return fmt.Sprintf("DIVERGENCE: length mismatch — traceA=%d traceB=%d",
			len(stepsA), len(stepsB))
	}

	for i := range stepsA {
		if stepsA[i].EventHash != stepsB[i].EventHash {
			return fmt.Sprintf("DIVERGENCE DETECTED:\ntrace_id: %s\ntick: %d\nevent: %s\nnodeA hash: %s\nnodeB hash: %s",
				stepsA[i].TraceID, stepsA[i].Tick, stepsA[i].Event,
				stepsA[i].EventHash, stepsB[i].EventHash)
		}
		if stepsA[i].Seq != stepsB[i].Seq {
			return fmt.Sprintf("DIVERGENCE DETECTED:\ntrace_id: %s\ntick: %d\nseq mismatch: A=%d B=%d",
				stepsA[i].TraceID, stepsA[i].Tick, stepsA[i].Seq, stepsB[i].Seq)
		}
	}
	return ""
}

// StateDiff compares two state maps and returns divergence.
func StateDiff(label string, a, b map[string]uint64) string {
	if len(a) != len(b) {
		return fmt.Sprintf("DIVERGENCE [%s]: state map size mismatch %d vs %d",
			label, len(a), len(b))
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok {
			return fmt.Sprintf("DIVERGENCE [%s]: key %s missing in second state", label, k)
		} else if va != vb {
			return fmt.Sprintf("DIVERGENCE [%s]: key %s value mismatch %d vs %d",
				label, k, va, vb)
		}
	}
	return ""
}