// Package diff — ATOM-031: State Diff Engine.
// Automatically detects divergence between two system states.
package diff

import (
	"fmt"
	"sync"
)

// State holds the system snapshot.
type State struct {
	mu      sync.RWMutex
	data    map[string]interface{}
	eventSeq []uint64
	hash    string
}

// NewState creates a State.
func NewState() *State {
	return &State{data: make(map[string]interface{}), eventSeq: make([]uint64, 0)}
}

// Set sets a key-value pair.
func (s *State) Set(key string, val interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

// Get returns a value.
func (s *State) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// AppendSeq adds a sequence number.
func (s *State) AppendSeq(seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventSeq = append(s.eventSeq, seq)
}

// SetHash sets the state hash.
func (s *State) SetHash(h string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hash = h
}

// Hash returns the state hash.
func (s *State) Hash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hash
}

// Compare compares two states and returns divergence report.
// Returns empty string if identical.
func Compare(label string, a, b *State) string {
	// Compare hashes first (fast path).
	if a.Hash() != "" && b.Hash() != "" && a.Hash() != b.Hash() {
		return fmt.Sprintf("DIVERGENCE [%s]: hash mismatch\n  nodeA hash: %s\n  nodeB hash: %s",
			label, a.Hash(), b.Hash())
	}

	// Compare event sequences.
	{
		a.mu.RLock()
		b.mu.RLock()
		defer a.mu.RUnlock()
		defer b.mu.RUnlock()

		if len(a.eventSeq) != len(b.eventSeq) {
			return fmt.Sprintf("DIVERGENCE [%s]: seq length mismatch A=%d B=%d",
				label, len(a.eventSeq), len(b.eventSeq))
		}
		for i := range a.eventSeq {
			if a.eventSeq[i] != b.eventSeq[i] {
				return fmt.Sprintf("DIVERGENCE [%s]: seq[%d] mismatch A=%d B=%d",
					label, i, a.eventSeq[i], b.eventSeq[i])
			}
		}
	}

	// Compare data maps.
	{
		a.mu.RLock()
		b.mu.RLock()
		defer a.mu.RUnlock()
		defer b.mu.RUnlock()

		if len(a.data) != len(b.data) {
			return fmt.Sprintf("DIVERGENCE [%s]: data map size mismatch A=%d B=%d",
				label, len(a.data), len(b.data))
		}
		for k, va := range a.data {
			vb, ok := b.data[k]
			if !ok {
				return fmt.Sprintf("DIVERGENCE [%s]: key %q missing in second state", label, k)
			}
			if fmt.Sprintf("%v", va) != fmt.Sprintf("%v", vb) {
				return fmt.Sprintf("DIVERGENCE [%s]: key %q value mismatch %v vs %v",
					label, k, va, vb)
			}
		}
	}

	return ""
}

// DivergenceReport holds the result of a comparison.
type DivergenceReport struct {
	Label     string
	HasDivergence bool
	Details   string
	Tick      uint64
	TraceID   string
}

// NewReport creates a DivergenceReport.
func NewReport(label string, tick uint64, traceID string) *DivergenceReport {
	return &DivergenceReport{Label: label, Tick: tick, TraceID: traceID}
}

// SetDivergence marks the report as divergent.
func (r *DivergenceReport) SetDivergence(details string) {
	r.HasDivergence = true
	r.Details = details
}

// String formats the report.
func (r *DivergenceReport) String() string {
	if !r.HasDivergence {
		return fmt.Sprintf("[%s] OK — no divergence (tick=%d traceID=%s)", r.Label, r.Tick, r.TraceID)
	}
	return fmt.Sprintf("DIVERGENCE [%s]: %s (tick=%d traceID=%s)", r.Label, r.Details, r.Tick, r.TraceID)
}

// GlobalDiff runs a diff across multiple state pairs and returns first divergence.
func GlobalDiff(pairs map[string][2]*State) *DivergenceReport {
	for label, pair := range pairs {
		a, b := pair[0], pair[1]
		if d := Compare(label, a, b); d != "" {
			r := &DivergenceReport{Label: label, HasDivergence: true, Details: d}
			return r
		}
	}
	return &DivergenceReport{HasDivergence: false}
}