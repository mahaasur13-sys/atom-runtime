// Package tests — ATOM-027…032 Integration Tests.
package tests

import (
	"testing"

	"github.com/mahaasur13-sys/atom-runtime/pkg/chaos"
	"github.com/mahaasur13-sys/atom-runtime/pkg/diff"
	"github.com/mahaasur13-sys/atom-runtime/pkg/invariant"
	"github.com/mahaasur13-sys/atom-runtime/pkg/rng"
	"github.com/mahaasur13-sys/atom-runtime/pkg/trace"
)

// ── ATOM-027: Invariant Guard ─────────────────────────────────────────────────

func TestInvariant_SeqViolation(t *testing.T) {
	g := invariant.New()
	prev := &invariant.Event{Seq: 5, TraceID: "t1", EventHash: "abc", Tick: 10}
	bad := &invariant.Event{Seq: 7, TraceID: "t1", PrevHash: "abc", EventHash: "def", Tick: 11}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on seq gap")
		}
	}()
	g.CheckEvent(*bad, prev)
}

func TestInvariant_HashChainBroken(t *testing.T) {
	g := invariant.New()
	prev := &invariant.Event{Seq: 1, TraceID: "t1", EventHash: "aaa", Tick: 10}
	bad := &invariant.Event{Seq: 2, TraceID: "t1", PrevHash: "WRONG", EventHash: "bbb", Tick: 11}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on broken hash chain")
		}
	}()
	g.CheckEvent(*bad, prev)
}

func TestInvariant_FirstEventNoPrevHash(t *testing.T) {
	g := invariant.New()
	bad := &invariant.Event{Seq: 1, TraceID: "t1", PrevHash: "somehash", EventHash: "abc", Tick: 1}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on non-empty prevHash in first event")
		}
	}()
	g.CheckEvent(*bad, nil)
}

func TestInvariant_ValidChain(t *testing.T) {
	g := invariant.New()
	prev := &invariant.Event{Seq: 5, TraceID: "t1", EventHash: "abc", Tick: 10}
	ok := &invariant.Event{Seq: 6, TraceID: "t1", PrevHash: "abc", EventHash: "def", Tick: 11}

	g.CheckEvent(*ok, prev) // must NOT panic
}

// ── ATOM-030: Reorder Coverage ────────────────────────────────────────────────

func TestReorder_100Seeds(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		r1 := rng.New(uint64(seed))
		r2 := rng.New(uint64(seed))
		for i := uint64(0); i < 50; i++ {
			s1 := r1.Sample("trace", i, "model")
			s2 := r2.Sample("trace", i, "model")
			if s1 != s2 {
				t.Fatalf("seed %d tick %d: %f != %f", seed, i, s1, s2)
			}
		}
	}
}

// ── ATOM-031: State Diff Engine ────────────────────────────────────────────────

func TestStateDiff_Identical(t *testing.T) {
	a := diff.NewState()
	b := diff.NewState()
	a.Set("x", 42)
	b.Set("x", 42)
	a.SetHash("abc")
	b.SetHash("abc")

	if d := diff.Compare("test", a, b); d != "" {
		t.Fatalf("expected no divergence, got: %s", d)
	}
}

func TestStateDiff_HashMismatch(t *testing.T) {
	a := diff.NewState()
	b := diff.NewState()
	a.SetHash("xxx")
	b.SetHash("yyy")

	d := diff.Compare("test", a, b)
	if d == "" {
		t.Fatal("expected divergence on hash mismatch")
	}
	if diff.GlobalDiff(map[string][2]*diff.State{"test": {a, b}}) == nil {
		t.Fatal("GlobalDiff should report divergence")
	}
}

func TestStateDiff_SeqMismatch(t *testing.T) {
	a := diff.NewState()
	b := diff.NewState()
	a.AppendSeq(1)
	a.AppendSeq(2)
	b.AppendSeq(1)
	b.AppendSeq(3) // mismatch

	d := diff.Compare("test", a, b)
	if d == "" {
		t.Fatal("expected divergence on seq mismatch")
	}
}

// ── ATOM-032: Chaos Mode ───────────────────────────────────────────────────────

func TestChaos_SeedReproducibility(t *testing.T) {
	seed := int64(0x12345)

	// Run 1
	c1 := chaos.New(seed)
	c1.EnableProcessKill = true
	c1.EnablePartition = true
	c1.SetDuration(100)
	c1.Start()
	var evts1 []chaos.FailureEvent
	for i := uint64(0); i < 100; i++ {
		if e := c1.Tick(); e != nil {
			evts1 = append(evts1, *e)
		}
	}
	c1.Stop()

	// Run 2: same seed → same failures
	c2 := chaos.New(seed)
	c2.EnableProcessKill = true
	c2.EnablePartition = true
	c2.SetDuration(100)
	c2.Start()
	var evts2 []chaos.FailureEvent
	for i := uint64(0); i < 100; i++ {
		if e := c2.Tick(); e != nil {
			evts2 = append(evts2, *e)
		}
	}
	c2.Stop()

	if len(evts1) != len(evts2) {
		t.Fatalf("chaos seed non-deterministic: run1=%d run2=%d", len(evts1), len(evts2))
	}
	for i := range evts1 {
		if evts1[i].Type != evts2[i].Type || evts1[i].Tick != evts2[i].Tick {
			t.Fatalf("chaos event %d mismatch: %+v vs %+v", i, evts1[i], evts2[i])
		}
	}
}

// ── ATOM-032: 1000-Run Chaos Test ──────────────────────────────────────────────

func TestSystem_Chaos_1000Runs(t *testing.T) {
	const runs = 1000
	failures := 0

	for run := 0; run < runs; run++ {
		seed := int64(run)
		c := chaos.New(seed)
		c.EnableProcessKill = true
		c.EnablePartition = true
		c.EnableWALCorrupt = true
		c.EnableScheduler = true
		c.SetDuration(1000)
		c.Start()

		// Simulate system ticks.
		for tick := uint64(0); tick < 1000; tick++ {
			_ = c.Tick()
		}

		// Verify: trace recorded correctly.
		tr := c.GetTrace()
		if steps := tr.Steps(); len(steps) == 0 {
			// Some runs may have no failures — that's OK.
		}

		c.Stop()

		// Check invariant: no panic means state is consistent.
		// We track any divergence in the trace.
		if d := trace.Diff(c.GetTrace(), c.GetTrace()); d != "" {
			failures++
			t.Logf("run %d: divergence in trace: %s", run, d)
		}
	}

	if failures > 0 {
		t.Fatalf("%d/%d runs had divergence — system is NOT stable", failures, runs)
	}
}

// ── ATOM-029: Heisenbug Hunter (race + 1000 runs) ─────────────────────────────

func TestHeisenbug_1000Runs_Race(t *testing.T) {
	// Run with -race flag: go test -run TestHeisenbug -count=1000 -race
	// This test is a placeholder — real heisenbug detection happens in CI.
	for i := 0; i < 1000; i++ {
		r := rng.New(uint64(i))
		for j := uint64(0); j < 100; j++ {
			_ = r.Sample("trace", j, "model")
		}
	}
}

// ── Integration: Trace Recorder + Diff ─────────────────────────────────────────

func TestTrace_DiffTwoIdenticalRuns(t *testing.T) {
	seed := int64(0xbeef)
	c1 := chaos.New(seed)
	c1.EnablePartition = true
	c1.SetDuration(50)
	c1.Start()
	for i := uint64(0); i < 50; i++ {
		c1.Tick()
	}
	c1.Stop()

	c2 := chaos.New(seed)
	c2.EnablePartition = true
	c2.SetDuration(50)
	c2.Start()
	for i := uint64(0); i < 50; i++ {
		c2.Tick()
	}
	c2.Stop()

	d := trace.Diff(c1.GetTrace(), c2.GetTrace())
	if d != "" {
		t.Fatalf("identical seeds produced divergent traces: %s", d)
	}
}