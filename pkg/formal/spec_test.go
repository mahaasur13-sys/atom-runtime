// Package formal — ATOM-036 Formal Spec Generator tests.
package formal

import (
	"testing"
)

func TestBuildEventStoreSpec(t *testing.T) {
	spec := BuildEventStoreSpec()

	if spec.Name != "EventStore" {
		t.Fatalf("expected name EventStore, got %s", spec.Name)
	}
	if len(spec.Variables) != 4 {
		t.Fatalf("expected 4 variables, got %d", len(spec.Variables))
	}
	if len(spec.Invariants) != 5 {
		t.Fatalf("expected 5 invariants, got %d", len(spec.Invariants))
	}
}

func TestBuildSnapshotSpec(t *testing.T) {
	spec := BuildSnapshotSpec()

	if spec.Name != "Snapshot" {
		t.Fatalf("expected name Snapshot, got %s", spec.Name)
	}
	if len(spec.Invariants) != 1 {
		t.Fatalf("expected 1 invariant, got %d", len(spec.Invariants))
	}
}

func TestBuildByzantineSpec(t *testing.T) {
	spec := BuildByzantineSpec()

	if spec.Name != "Byzantine" {
		t.Fatalf("expected name Byzantine, got %s", spec.Name)
	}
	if len(spec.Invariants) != 2 {
		t.Fatalf("expected 2 invariants, got %d", len(spec.Invariants))
	}
}

func TestBuildConsensusSpec(t *testing.T) {
	spec := BuildConsensusSpec()

	if spec.Name != "Consensus" {
		t.Fatalf("expected name Consensus, got %s", spec.Name)
	}
	if len(spec.Invariants) != 3 {
		t.Fatalf("expected 3 invariants, got %d", len(spec.Invariants))
	}
}

func TestGenerator_Generate(t *testing.T) {
	g := New()
	spec := BuildEventStoreSpec()
	out := g.Generate(spec)

	if out == "" {
		t.Fatal("Generate returned empty string")
	}
	// Verify TLA+ module structure
	if !contains(out, "------------------------------ MODULE EventStore ------------------------------") {
		t.Fatal("missing MODULE header")
	}
	if !contains(out, "VARIABLES log, seq, hash, clock") {
		t.Fatal("missing VARIABLES declaration")
	}
	if !contains(out, "Init ==") {
		t.Fatal("missing Init")
	}
	if !contains(out, "INV1") {
		t.Fatal("missing INV1 invariant")
	}
	if !contains(out, "=============================================================================") {
		t.Fatal("missing footer")
	}
}

func TestGenerator_AllSpecsGenerateable(t *testing.T) {
	g := New()
	specs := []RuntimeSpec{
		BuildEventStoreSpec(),
		BuildSnapshotSpec(),
		BuildByzantineSpec(),
		BuildConsensusSpec(),
	}
	for _, spec := range specs {
		out := g.Generate(spec)
		if out == "" {
			t.Fatalf("Generate returned empty for %s", spec.Name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
