package wal

import (
	"fmt"
	"testing"

	"github.com/mahaasur13-sys/atom-runtime/pkg/canon"
)

func TestWAL_CrashRecovery(t *testing.T) {
	path := t.TempDir() + "/wal_test_crash.bin"
	w, err := NewWAL(path)
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer w.Close()

	// Append 100 events deterministically
	for i := 0; i < 100; i++ {
		e := canon.Event{
			Seq:       uint64(i),
			TraceID:   "trace-main",
			EventHash: hashHex(i),
			PrevHash:  prevHex(i),
			Payload:   []byte("payload"),
		}
		if err := w.Append(e); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	// Close and reopen (simulates crash)
	w.Close()

	w2, err := NewWAL(path)
	if err != nil {
		t.Fatalf("NewWAL reopen: %v", err)
	}
	defer w2.Close()

	recovered, err := w2.Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}

	// Must be prefix-identical — Recover(WAL) == Prefix(EventStore.log)
	if len(recovered) != 100 {
		t.Errorf("expected 100 events, got %d", len(recovered))
	}

	for i, e := range recovered {
		if e.Seq != uint64(i) {
			t.Errorf("event[%d].Seq = %d, want %d", i, e.Seq, i)
		}
	}
}

func TestWAL_ChecksumValidation(t *testing.T) {
	path := t.TempDir() + "/wal_test_checksum.bin"
	w, err := NewWAL(path)
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer w.Close()

	// Append a few events
	for i := 0; i < 5; i++ {
		e := canon.Event{
			Seq:       uint64(i),
			TraceID:   "trace-1",
			EventHash: hashHex(i),
			PrevHash:  prevHex(i),
			Payload:   []byte("data"),
		}
		if err := w.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	// Reopen and recover — all entries must have valid checksums
	w2, _ := NewWAL(path)
	defer w2.Close()

	events, err := w2.Recover()
	if err != nil {
		t.Fatalf("Recover with checksum validation: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestWAL_NoDivergence(t *testing.T) {
	path := t.TempDir() + "/wal_test_div.bin"
	w, err := NewWAL(path)
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer w.Close()

	// Append events
	for i := 0; i < 50; i++ {
		e := canon.Event{
			Seq:       uint64(i),
			TraceID:   "trace-div",
			EventHash: hashHex(i),
			PrevHash:  prevHex(i),
			Payload:   []byte("payload"),
		}
		w.Append(e)
	}
	w.Close()

	// Recover twice — must get identical results
	w3, _ := NewWAL(path)
	defer w3.Close()
	r1, _ := w3.Recover()

	w4, _ := NewWAL(path)
	defer w4.Close()
	r2, _ := w4.Recover()

	if len(r1) != len(r2) {
		t.Errorf("divergence detected: run1=%d run2=%d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Seq != r2[i].Seq || r1[i].EventHash != r2[i].EventHash {
			t.Errorf("divergence at event %d", i)
		}
	}
}

func TestWAL_ClosedState(t *testing.T) {
	path := t.TempDir() + "/wal_test_closed.bin"
	w, _ := NewWAL(path)
	w.Close()

	// Append to closed WAL must fail
	err := w.Append(canon.Event{})
	if err == nil {
		t.Error("expected error on closed WAL append")
	}
}

// hashHex and prevHex are deterministic helpers matching canon.Event semantics.
func hashHex(i int) string {
	return fmt.Sprintf("%064d", i)
}

func prevHex(i int) string {
	if i == 0 {
		return ""
	}
	return fmt.Sprintf("%064d", i-1)
}