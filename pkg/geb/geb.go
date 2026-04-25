// Package geb — GlobalExecutionBarrier (ATOM-020).
// ATOM constraints: C1, C3, C4, G1-G5.
// GEB is the ONLY component that may call LogicalClock.Advance().
package geb

import (
	"sync"

	"github.com/mahaasur13-sys/atom-runtime/pkg/clock"
)

// GEB is the single execution time authority.
// All system progress goes through GEB.Tick().
// GEB1: All system progress goes through Tick().
// GEB2: No component can advance clock directly.
type GEB struct {
	mu    sync.Mutex
	clock *clock.LogicalClock
}

// New creates a GEB backed by a LogicalClock.
func New(lc *clock.LogicalClock) *GEB {
	return &GEB{clock: lc}
}

// NewStandalone creates a GEB with a new LogicalClock.
func NewStandalone() *GEB {
	return &GEB{clock: clock.New()}
}

// Tick advances the logical clock by exactly one tick. Thread-safe.
func (g *GEB) Tick() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.clock.Advance()
}

// Now returns the current tick without advancing.
func (g *GEB) Now() uint64 {
	return g.clock.Now()
}

// Snapshot returns the current clock state for checkpointing.
func (g *GEB) Snapshot() Snapshot {
	return Snapshot{Tick: g.Now(), EpochMs: g.clock.EpochMs()}
}

// Snapshot captures clock state for recovery.
type Snapshot struct {
	Tick    uint64
	EpochMs int64
}

// GEBNode is a node-scoped GEB handle.
type GEBNode struct {
	geb    *GEB
	nodeID string
}

// NewNode creates a GEBNode.
func NewNode(nodeID string, g *GEB) *GEBNode {
	return &GEBNode{geb: g, nodeID: nodeID}
}

// Commit marks the node as committed at the current tick.
func (n *GEBNode) Commit() {
	n.geb.Tick()
}
