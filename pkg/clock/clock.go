// Package clock — GEB-authoritative logical clock (ATOM-LC).
// ATOM constraints: C1 (no time.Now), C3 (no random), G1-G5.
// Go 1.19 compatible (no atomic.Uint64.Addr).
package clock

import (
	"sync"
)

// LogicalClock is the SINGLE SOURCE of execution time truth.
// Only GEB may call Advance(). All other components call Now().
type LogicalClock struct {
	mu      sync.Mutex
	tick    uint64
	epochMs int64
}

// New creates a LogicalClock with initial tick=0.
func New() *LogicalClock {
	return &LogicalClock{}
}

// NewWithEpoch creates a clock with explicit epoch (for recovery).
func NewWithEpoch(epochMs int64) *LogicalClock {
	return &LogicalClock{epochMs: epochMs}
}

// Now returns the current tick (read-only, lock-free).
// LC3: Now() is monotonic — tick only increases.
func (lc *LogicalClock) Now() uint64 {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.tick
}

// Advance increments tick and returns the new value.
// GEB-only method. LC1: tickₙ₊₁ = tickₙ + 1.
// LC2: Advance() ONLY called by GEB.
func (lc *LogicalClock) Advance() uint64 {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.tick++
	return lc.tick
}

// AdvanceTo sets tick to target+1 (batch advancement).
func (lc *LogicalClock) AdvanceTo(target uint64) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if target >= lc.tick {
		lc.tick = target + 1
	} else {
		lc.tick++
	}
}

// EpochMs returns wall-clock epoch for metrics export layer only.
// NOT used in execution path.
func (lc *LogicalClock) EpochMs() int64 {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.epochMs
}

// SetEpochMs updates the epoch (for checkpoint recovery).
func (lc *LogicalClock) SetEpochMs(epochMs int64) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.epochMs = epochMs
}
