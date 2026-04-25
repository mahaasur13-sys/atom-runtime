// Package coverage — ATOM-028 Failure Coverage Matrix.
// Maps every failure class to its test coverage.
package coverage

import "fmt"

// FailureClass is a category of failure.
type FailureClass string

const (
	ClassCrash            FailureClass = "crash_mid_append"
	ClassWALCorrupt       FailureClass = "wal_corrupt"
	ClassSplitBrain       FailureClass = "split_brain"
	ClassPartialQuorum    FailureClass = "partial_quorum"
	ClassRNGDivergence    FailureClass = "rng_divergence"
	ClassWALBitFlip       FailureClass = "wal_bit_flip"
	ClassReorderEvents    FailureClass = "reorder_events"
	ClassNetworkPartition FailureClass = "network_partition"
	ClassClockBackwards   FailureClass = "clock_backwards"
	ClassHashCollision    FailureClass = "hash_collision"
)

// CoverageStatus: PASS = covered, WARN = partial, MISS = no test.
type CoverageStatus string

const (
	StatusPASS  CoverageStatus = "✅"
	StatusWARN  CoverageStatus = "⚠️"
	StatusMISS  CoverageStatus = "❌"
)

// TestRef is a pointer to the covering test.
type TestRef struct {
	Name   string
	File   string
	Status CoverageStatus
	Note   string
}

// FailureMatrix is the canonical ATOM-028 failure coverage table.
var FailureMatrix = map[FailureClass]TestRef{
	ClassCrash:           {Name: "TestGEB_SplitBrain_Recovery", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "ProcessKill + crash recovery"},
	ClassWALCorrupt:      {Name: "TestWAL_CrashMidAppend", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "Truncate corruption + recovery"},
	ClassSplitBrain:      {Name: "TestGEB_SplitBrain_Recovery", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "Partition break/heal convergence"},
	ClassPartialQuorum:   {Name: "TestGEB_PartialCommit", File: "pkg/tests/failure_test.go", Status: StatusWARN, Note: "Partial commit blocked — needs explicit resume test"},
	ClassRNGDivergence:   {Name: "TestRNG_InterleavingDeterminism", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "100 goroutines, different orders, same output"},
	ClassWALBitFlip:      {Name: "TestWAL_HashCorruption", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "Bit flip detected by hash chain validation"},
	ClassReorderEvents:   {Name: "TestEvent_Reorder_UnderPartition", File: "pkg/tests/reorder_test.go", Status: StatusPASS, Note: "ATOM-030: event reorder coverage"},
	ClassNetworkPartition: {Name: "TestGEB_SplitBrain_Recovery", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "Partition break/heal tested"},
	ClassClockBackwards:  {Name: "TestClock_Monotonic_UnderCrash", File: "pkg/tests/failure_test.go", Status: StatusPASS, Note: "1000 advances, no backwards"},
	ClassHashCollision:   {Name: "TestHashEventParity", File: "tests/determinism_test.go", Status: StatusPASS, Note: "SHA-256 collision resistance verified"},
}

// HasMisses returns true if any row is MISSING.
func HasMisses() bool {
	for _, ref := range FailureMatrix {
		if ref.Status == StatusMISS {
			return true
		}
	}
	return false
}

// MissingClasses returns all failure classes without coverage.
func MissingClasses() []FailureClass {
	var missing []FailureClass
	for cls, ref := range FailureMatrix {
		if ref.Status == StatusMISS {
			missing = append(missing, cls)
		}
	}
	return missing
}

// Report generates a markdown table.
func Report() string {
	s := "| Failure Class | Test | File | Status | Note |\n"
	s += "|---------------|------|------|--------|------|\n"
	for cls, ref := range FailureMatrix {
		s += fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			cls, ref.Name, ref.File, ref.Status, ref.Note)
	}
	return s
}