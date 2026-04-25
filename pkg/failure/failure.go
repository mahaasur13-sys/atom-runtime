// Package failure — Controlled failure injection framework.
// ATOM-026: Deterministic fault injection for ATOMFederationOS.
//
// RULE: All injection operations use a fixed seed so that
// every test run produces IDENTICAL failures. The system
// under test must survive all of them deterministically.
package failure

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
)

// Seed is the global injection seed — fixed for reproducibility.
const Seed int64 = 0xdeadbeef

var (
	rng   *rand.Rand
	rngMu sync.Mutex
)

func init() {
	src := NewSeedSource(Seed)
	rng = rand.New(src)
}

// NewSeedSource creates a deterministic rand.Source from a seed.
func NewSeedSource(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

// ── Process Kill ───────────────────────────────────────────────────────────────

// ProcessKill simulates kill -9 with no graceful shutdown.
type ProcessKill struct {
	mu         sync.Mutex
	killed     map[string]bool
	injectAfter int64 // inject after this many calls to ShouldKill
	count      int64
}

func NewProcessKill() *ProcessKill {
	return &ProcessKill{killed: make(map[string]bool)}
}

func (pk *ProcessKill) ShouldKill(nodeID string) bool {
	n := atomic.AddInt64(&pk.count, 1)
	if n > pk.injectAfter {
		pk.mu.Lock()
		pk.killed[nodeID] = true
		pk.mu.Unlock()
		return true
	}
	return false
}

func (pk *ProcessKill) IsDead(nodeID string) bool {
	pk.mu.Lock()
	defer pk.mu.Unlock()
	return pk.killed[nodeID]
}

func (pk *ProcessKill) InjectAfter(calls int64) { pk.injectAfter = calls }
func (pk *ProcessKill) Reset()                 { pk.count = 0 }

// ── WAL Corruption ───────────────────────────────────────────────────────────

type WALCorruptMode int

const (
	CorruptTruncate WALCorruptMode = iota
	CorruptPartialWrite
	CorruptBitFlip
)

type WALCorruptor struct {
	mu       sync.Mutex
	corruptAt int64 // corrupt after this many writes
	count    int64
	mode     WALCorruptMode
	dir      string
}

func NewWALCorruptor(mode WALCorruptMode) *WALCorruptor {
	return &WALCorruptor{mode: mode}
}

func (wc *WALCorruptor) CorruptAt(callCount int64) { wc.corruptAt = callCount }

// CorruptFile corrupts the WAL file at path based on the configured mode.
// Returns the path to the corrupted file.
// Corruption triggers when call count EXCEEDS corruptAt (for fail-at-N semantics).
func (wc *WALCorruptor) CorruptFile(path string) (string, error) {
	n := atomic.AddInt64(&wc.count, 1)
	if n <= wc.corruptAt {
		return path, nil
	}

	wc.mu.Lock()
	defer wc.mu.Unlock()

	switch wc.mode {
	case CorruptTruncate:
		f, err := os.OpenFile(path, os.O_RDWR, 0644)
		if err != nil {
			return "", err
		}
		info, _ := f.Stat()
		size := info.Size()
		f.Truncate(size / 2)
		f.Close()
		return path, nil

	case CorruptPartialWrite:
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			return "", err
		}
		partial := make([]byte, 16)
		binary.LittleEndian.PutUint64(partial, 0xAAAAAAAAAAAAAAAA)
		f.Write(partial[:8])
		f.Close()
		return path, nil

	case CorruptBitFlip:
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if len(data) < 64 {
			return "", fmt.Errorf("file too short for bit flip")
		}
		data[32] ^= (1 << 5)
		out := path + ".corrupt"
		if err := os.WriteFile(out, data, 0644); err != nil {
			return "", err
		}
		return out, nil
	}
	return path, nil
}

func (wc *WALCorruptor) Reset() { atomic.StoreInt64(&wc.count, 0) }

// ── Network Partition ─────────────────────────────────────────────────────────

// Partition represents a network partition between two groups of nodes.
type Partition struct {
	mu          sync.Mutex
	groupA      map[string]bool
	groupB      map[string]bool
	isPartitioned bool
}

func NewPartition() *Partition {
	return &Partition{
		groupA: make(map[string]bool),
		groupB: make(map[string]bool),
	}
}

func (p *Partition) SetGroups(nodesA, nodesB []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, n := range nodesA {
		p.groupA[n] = true
	}
	for _, n := range nodesB {
		p.groupB[n] = true
	}
}

func (p *Partition) IsPartitioned(a, b string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isPartitioned {
		return false
	}
	// Same group = can communicate
	if (p.groupA[a] && p.groupA[b]) || (p.groupB[a] && p.groupB[b]) {
		return false
	}
	// Cross-group = blocked
	return true
}

func (p *Partition) Break() {
	p.mu.Lock()
	p.isPartitioned = true
	p.mu.Unlock()
}

func (p *Partition) Heal() {
	p.mu.Lock()
	p.isPartitioned = false
	p.mu.Unlock()
}

// ── Scheduler Randomization ───────────────────────────────────────────────────

// SchedulerShuffle controls goroutine execution order deterministically.
type SchedulerShuffle struct {
	mu      sync.Mutex
	enabled atomic.Bool
	seed    int64
}

func NewSchedulerShuffle(seed int64) *SchedulerShuffle {
	return &SchedulerShuffle{seed: seed}
}

// Permute returns a deterministic permutation of [0..n-1] based on seed.
func (ss *SchedulerShuffle) Permute(n int) []int {
	if n <= 1 {
		return []int{0}
	}
	ss.mu.Lock()
	r := rand.New(rand.NewSource(ss.seed))
	ss.mu.Unlock()

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	// Fisher-Yates with deterministic seed
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	return order
}

// ── Metrics ───────────────────────────────────────────────────────────────────────

type TestResult struct {
	Test   string `json:"test"`
	Seed   int64  `json:"seed"`
	Ticks  int64  `json:"ticks"`
	Result string `json:"result"`
}

type Metrics struct {
	mu      sync.Mutex
	results []TestResult
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) Record(test string, ticks int64, result string) {
	m.mu.Lock()
	m.results = append(m.results, TestResult{
		Test:   test,
		Seed:   Seed,
		Ticks:  ticks,
		Result: result,
	})
	m.mu.Unlock()
}

func (m *Metrics) Results() []TestResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]TestResult(nil), m.results...)
}

func (m *Metrics) Summary() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	summary := make(map[string]int)
	for _, r := range m.results {
		summary[r.Result]++
	}
	return summary
}

// ── Runner ─────────────────────────────────────────────────────────────────────

type Runner struct {
	metrics *Metrics
	failFast bool
}

func NewRunner() *Runner {
	return &Runner{metrics: NewMetrics()}
}

func (r *Runner) Run(name string, fn func() error) {
	err := fn()
	if err != nil {
		r.metrics.Record(name, 0, fmt.Sprintf("FAIL: %v", err))
	} else {
		r.metrics.Record(name, 0, "PASS")
	}
}
