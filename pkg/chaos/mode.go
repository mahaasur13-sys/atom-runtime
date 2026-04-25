// Package chaos — ATOM-032 Controlled Chaos Mode.
// Injects failures deterministically using a fixed seed so every run is reproducible.
package chaos

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/mahaasur13-sys/atom-runtime/pkg/failure"
	"github.com/mahaasur13-sys/atom-runtime/pkg/trace"
)

// Mode controls which failure types are active.
type Mode struct {
	mu       sync.Mutex
	seed     int64
	duration uint64 // in ticks
	elapsed  uint64
	active   bool

	// Failure injectors (deterministic via seed).
	processKill *failure.ProcessKill
	partition   *failure.Partition
	corruptor   *failure.WALCorruptor
	scheduler   *failure.SchedulerShuffle

	// Trace recorder.
	tracer *trace.Trace

	// Config: which failures are enabled.
	EnableProcessKill bool
	EnablePartition   bool
	EnableWALCorrupt  bool
	EnableScheduler   bool
}

// New creates a chaos Mode with given seed.
func New(seed int64) *Mode {
	return &Mode{
		seed:          seed,
		duration:      10000,
		processKill:   failure.NewProcessKill(),
		partition:     failure.NewPartition(),
		corruptor:     failure.NewWALCorruptor(failure.CorruptBitFlip),
		scheduler:     failure.NewSchedulerShuffle(seed),
		tracer:        trace.New(fmt.Sprintf("chaos-%d", seed)),
	}
}

// SetDuration sets the chaos duration in ticks.
func (m *Mode) SetDuration(ticks uint64) { m.duration = ticks }

// Start begins chaos injection.
func (m *Mode) Start() {
	m.mu.Lock()
	m.active = true
	m.elapsed = 0
	m.mu.Unlock()
}

// Stop halts chaos injection.
func (m *Mode) Stop() {
	m.mu.Lock()
	m.active = false
	m.mu.Unlock()
}

// Tick advances elapsed time and returns injected failures (if any).
// Returns nil if no failure this tick, or a FailureEvent describing what was injected.
func (m *Mode) Tick() *FailureEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active || m.elapsed >= m.duration {
		return nil
	}

	m.elapsed++

	// Deterministic failure selection based on seed + elapsed.
	r := rand.New(rand.NewSource(m.seed + int64(m.elapsed)))
	roll := r.Intn(100)

	var evt FailureEvent
	evt.Tick = m.elapsed

	switch {
	case m.EnableProcessKill && roll < 10:
		evt.Type = "process_kill"
		evt.Details = fmt.Sprintf("kill at tick %d", m.elapsed)
		m.processKill.InjectAfter(int64(m.elapsed))
	case m.EnablePartition && roll >= 10 && roll < 25:
		evt.Type = "partition"
		evt.Details = fmt.Sprintf("partition at tick %d", m.elapsed)
		m.partition.Break()
	case m.EnableWALCorrupt && roll >= 25 && roll < 40:
		evt.Type = "wal_corruption"
		evt.Details = fmt.Sprintf("WAL corrupt at tick %d", m.elapsed)
		m.corruptor.CorruptAt(int64(m.elapsed))
	case m.EnableScheduler && roll >= 40 && roll < 55:
		evt.Type = "scheduler_shuffle"
		evt.Details = fmt.Sprintf("scheduler reorder at tick %d", m.elapsed)
	default:
		return nil
	}

	// Record to trace.
	m.tracer.Record(trace.Entry{
		Tick:  evt.Tick,
		Event: evt.Type,
	})
	return &evt
}

// FailureEvent describes an injected failure.
type FailureEvent struct {
	Type    string
	Tick    uint64
	Details string
	NodeID  string
}

// GetPartition returns the partition injector.
func (m *Mode) GetPartition() *failure.Partition { return m.partition }

// GetProcessKill returns the process kill injector.
func (m *Mode) GetProcessKill() *failure.ProcessKill { return m.processKill }

// GetWALCorruptor returns the WAL corruptor.
func (m *Mode) GetWALCorruptor() *failure.WALCorruptor { return m.corruptor }

// GetScheduler returns the scheduler shuffler.
func (m *Mode) GetScheduler() *failure.SchedulerShuffle { return m.scheduler }

// GetTrace returns the trace recorder.
func (m *Mode) GetTrace() *trace.Trace { return m.tracer }

// Summary returns a human-readable chaos run summary.
func (m *Mode) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fmt.Sprintf("Chaos seed=%d duration=%d elapsed=%d active=%v",
		m.seed, m.duration, m.elapsed, m.active)
}