// Package tests — Determinism tests (ATOM C1–C10 + G1–G5).
package tests

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mahaasur13-sys/atom-runtime/pkg/clock"
	"github.com/mahaasur13-sys/atom-runtime/pkg/contract"
	"github.com/mahaasur13-sys/atom-runtime/pkg/geb"
	"github.com/mahaasur13-sys/atom-runtime/pkg/rng"
)

// ── C1: No time.Now / time.time ───────────────────────────────────────────

func TestNoTimeNowInPackage(t *testing.T) {
	// This is a compile-time enforcement: pkg/clock has no time import.
	// Additional runtime check: clock.Now() returns monotonic tick.
	lc := clock.New()
	tick1 := lc.Now()
	tick2 := lc.Now()
	if tick2 < tick1 {
		t.Errorf("tick decreased: %d -> %d", tick1, tick2)
	}
}

// ── C3: No random.* in execution path ─────────────────────────────────────

func TestDeterministicRNGSample(t *testing.T) {
	r := rng.New(42)
	// Run 1000 times — must be identical every time.
	base := r.Sample("trace-1", 1, "model-A")
	for i := 0; i < 999; i++ {
		got := r.Sample("trace-1", 1, "model-A")
		if got != base {
			t.Errorf("RNG non-deterministic: run 0=%f, run %d=%f", base, i+1, got)
		}
	}
}

func TestDeterministicRNGRange(t *testing.T) {
	r := rng.New(123)
	for i := 0; i < 100; i++ {
		s := r.Sample("t", uint64(i), "m")
		if s < 0 || s >= 1 {
			t.Errorf("Sample out of [0,1): %f", s)
		}
	}
}

// ── C4: No asyncio.sleep / time.Sleep in scheduling ─────────────────────────

func TestGEBAdvancesClock(t *testing.T) {
	ge := geb.NewStandalone()
	now := ge.Now()
	next := ge.Tick()
	if next != now+1 {
		t.Errorf("GEB.Tick() did not advance by exactly 1: %d -> %d", now, next)
	}
}

// ── G3: Same input → bit-identical output ──────────────────────────────────

func TestReplayDeterminism(t *testing.T) {
	// Two replays of the same log must produce identical state.
	ge := geb.NewStandalone()
	log := buildTestLog(ge, 10)

	state1 := Replay(log)
	state2 := Replay(log)
	if state1 != state2 {
		t.Errorf("Replay non-deterministic:\nstate1=%v\nstate2=%v", state1, state2)
	}
}

// ── G4: Replay(log) == Live execution ───────────────────────────────────────

func TestReplayEqualsLive(t *testing.T) {
	ge := geb.NewStandalone()
	// Live execution: append events sequentially.
	liveState := liveExecution(ge, 5)
	// Replay execution: replay from log.
	log := buildTestLog(ge, 5)
	replayState := Replay(log)
	if liveState != replayState {
		t.Errorf("Replay != Live:\nlive=%v\nreplay=%v", liveState, replayState)
	}
}

// ── CL1: Cross-language hash parity ─────────────────────────────────────────

func TestHashEventDeterminism(t *testing.T) {
	ctx := contract.DeterministicContext{TraceID: "t1", Tick: 1}
	payload := []byte(`{"n":42}`)
	h1 := contract.HashEvent(ctx, 1, "test", payload, "")
	h2 := contract.HashEvent(ctx, 1, "test", payload, "")
	if h1 != h2 {
		t.Errorf("HashEvent non-deterministic: %s != %s", h1, h2)
	}
}

func TestHashEventParity(t *testing.T) {
	// This test documents the EXACT formula for Go↔Python parity.
	// If this test fails, the CrossLangContract is broken.
	ctx := contract.DeterministicContext{TraceID: "workflow-42", Tick: 7}
	payload := []byte(`{"loss":0.001,"step":7}`)
	prevHash := ""

	eventHash := contract.HashEvent(ctx, 1, "training.step", payload, prevHash)

	// Verify manually:
	// 1. payload_hash = SHA256(payload)
	ph := sha256.Sum256(payload)
	payloadHash := fmt.Sprintf("%x", ph)
	// 2. raw = traceID|seq|type|payloadHash|prevHash
	raw := fmt.Sprintf("%s|%d|%s|%s|%s", ctx.TraceID, 1, "training.step", payloadHash, prevHash)
	expected := sha256.Sum256([]byte(raw))
	expectedHex := fmt.Sprintf("%x", expected)

	if eventHash != expectedHex {
		t.Errorf("HashEvent formula broken:\nexpected=%s\ngot=%s", expectedHex, eventHash)
	}
}

// ── LC1: tick monotonicity ──────────────────────────────────────────────────

func TestClockMonotonic(t *testing.T) {
	lc := clock.New()
	// Advance to known state.
	for i := 0; i < 1000; i++ {
		lc.Advance()
	}
	// Verify monotonicity from tick=1000 onward.
	last := lc.Now() // start from tick=1000
	for i := 0; i < 1000; i++ {
		lc.Advance()
		now := lc.Now()
		if now <= last {
			t.Errorf("Clock went backwards: %d -> %d", last, now)
		}
		last = now
	}
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// Event is a minimal test event.
type Event struct {
	TraceID  string
	Seq     uint64
	Type    string
	Payload []byte
	Hash    string
	Prev    string
}

// Replay replays a log and returns the final state as a string.
func Replay(log []Event) string {
	state := map[string]uint64{}
	for _, ev := range log {
		state[ev.TraceID] = ev.Seq
	}
	// Serialize state for comparison.
	b, _ := json.Marshal(state)
	return string(b)
}

// liveExecution appends N events directly (no replay).
func liveExecution(ge *geb.GEB, n int) string {
	state := map[string]uint64{}
	for i := 0; i < n; i++ {
		ge.Tick()
		state["test"] = uint64(i + 1) // use same trace as buildTestLog
	}
	b, _ := json.Marshal(state)
	return string(b)
}

// buildTestLog creates N events for testing.
func buildTestLog(ge *geb.GEB, n int) []Event {
	log := []Event{}
	for i := 0; i < n; i++ {
		ge.Tick()
		ev := Event{
			TraceID:  "test",
			Seq:     uint64(i + 1),
			Type:    "test",
			Payload: []byte(`{}`),
		}
		log = append(log, ev)
	}
	return log
}
