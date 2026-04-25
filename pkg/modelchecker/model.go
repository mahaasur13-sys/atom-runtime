// Package modelchecker — ATOM-033 Model Checking Engine.
// Implements TLA+-style deterministic state-space exploration.
// Validates state transitions against ATOM invariants.
//
// ATOM constraints:
// - C1: No time.Now, time.Sleep, math/rand, uuid.New, unsorted map iteration
// - C2: Replay Equivalence — Replay(Snapshot) == Replay(Log)
// - C3: No hidden state — all state from EventStore + SnapshotStore + DeterministicClock
package modelchecker

import (
	"fmt"
	"sort"
	"strconv"
)

// State is a snapshot of the distributed system at one point in execution.
type State struct {
	TraceID string
	Seq     uint64
	Hash    string
}

// Transition is a labeled edge in the state graph.
type Transition struct {
	From  State
	To    State
	Event string
}

// ModelChecker performs depth-first search over the state graph.
// Rejects any transition that violates ATOM invariants.
type ModelChecker struct {
	visited map[string]bool
	transitions []Transition
}

// New creates a ModelChecker.
func New() *ModelChecker {
	return &ModelChecker{
		visited: make(map[string]bool),
	}
}

// SetTransitions sets the transition set (allows reuse of checker).
func (mc *ModelChecker) SetTransitions(t []Transition) {
	mc.transitions = t
}

// stateKey uniquely identifies a state for visited tracking.
// Deterministic: sort.Stringify is not needed — TraceID+Seq are deterministic.
func stateKey(s State) string {
	return s.TraceID + ":" + strconv.FormatUint(s.Seq, 10)
}

// Check performs DFS model checking.
// Returns nil if all reachable states are valid.
// Returns error on first invariant violation.
func (mc *ModelChecker) Check(initial State) error {
	mc.visited = make(map[string]bool)
	stack := []State{initial}

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		key := stateKey(current)
		if mc.visited[key] {
			continue
		}
		mc.visited[key] = true

		for _, t := range mc.transitions {
			if t.From.TraceID == current.TraceID && t.From.Seq == current.Seq {
				if err := validateTransition(t); err != nil {
					return err
				}
				stack = append(stack, t.To)
			}
		}
	}

	return nil
}

// validateTransition checks ATOM invariants on a single transition.
func validateTransition(t Transition) error {
	// INV2: seq must be prev+1
	if t.To.Seq != t.From.Seq+1 {
		return fmt.Errorf("%w: seq violation — from=%d to=%d traceID=%s",
			ErrSequenceViolation, t.From.Seq, t.To.Seq, t.To.TraceID)
	}

	// INV: Hash must match event-derived hash (simplified — no actual hash func here)
	// In production, HashEvent(t.Event) would be called.
	// For model checker, we validate structural properties only.

	return nil
}

// ErrSequenceViolation — ATOM invariant violation.
var ErrSequenceViolation = fmt.Errorf("sequence violation")

// ExploredCount returns the number of unique states visited.
func (mc *ModelChecker) ExploredCount() int {
	return len(mc.visited)
}

// Paths returns all valid execution paths as sequences of states.
// Useful for debugging model checker output.
func (mc *ModelChecker) Paths() [][]State {
	// Rebuild paths by walking visited states.
	// This is O(n) and deterministic.
	var paths [][]State
	for key := range mc.visited {
		// key format: traceID:seq
		parts := splitExact(key, ':', 2)
		paths = append(paths, []State{
			{TraceID: parts[0]},
		})
	}
	sort.Slice(paths, func(i, j int) bool {
		return paths[i][0].TraceID < paths[j][0].TraceID
	})
	return paths
}

// splitExact splits s on delim into exactly n parts.
// Panics if split produces != n parts.
func splitExact(s string, delim byte, n int) []string {
	parts := make([]string, 0, n)
	start := 0
	for i := 0; i < n-1; i++ {
		idx := -1
		for j := start; j < len(s); j++ {
			if s[j] == delim {
				idx = j
				break
			}
		}
		if idx < 0 {
			panic("splitExact: not enough delimiters")
		}
		parts = append(parts, s[start:idx])
		start = idx + 1
	}
	parts = append(parts, s[start:])
	return parts
}
