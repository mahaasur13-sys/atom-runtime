// Package rng — Deterministic RNG for Thompson Sampling (ATOM-016).
// ATOM constraints: C3 (no random.*), R1-R3.
// Uses SHA-256 in counter mode for deterministic floating-point generation.
package rng

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
	"math"
)

// DeterministicRNG produces bit-identical outputs from (seed, traceID, tick, modelID).
// R1: Same inputs → same output (guaranteed).
// R2: No global state mutation.
// R3: No hidden entropy (SHA-256 counter mode with explicit seed).
type DeterministicRNG struct {
	seed uint64
}

// New creates a DeterministicRNG seeded by seed.
// seed is mixed via FNV-64a into a key for SHA-256.
func New(seed uint64) *DeterministicRNG {
	return &DeterministicRNG{seed: seed}
}

// FromClock combines clock tick into seed for time-dependent determinism.
func FromClock(tick uint64, baseSeed uint64) *DeterministicRNG {
	return New(baseSeed ^ tick)
}

// Sample returns a deterministic float64 in [0, 1) from inputs.
// R1: Sample(traceID, tick, modelID) is bit-identical across calls.
// Uses SHA-256 in counter mode to produce deterministic keystream.
func (r *DeterministicRNG) Sample(traceID string, tick uint64, modelID string) float64 {
	// Build 40-byte input: traceID|tick|modelID|seed.
	h := fnv.New64a()
	binary.Write(h, binary.LittleEndian, tick)
	binary.Write(h, binary.LittleEndian, uint64(len(traceID)))
	h.Write([]byte(traceID))
	binary.Write(h, binary.LittleEndian, uint64(len(modelID)))
	h.Write([]byte(modelID))
	binary.Write(h, binary.LittleEndian, r.seed)
	entropy := h.Sum(nil) // 8 bytes

	// Counter mode: SHA256(counter || entropy) for each block.
	counter := make([]byte, 40)
	copy(counter, entropy)
	binary.LittleEndian.PutUint64(counter[8:], 0)
	h1 := sha256.Sum256(counter)
	binary.LittleEndian.PutUint64(counter[8:], 1)
	_ = sha256.Sum256(counter)

	// Use first 8 bytes of h1 as entropy for float64.
	v := binary.LittleEndian.Uint64(h1[:8])
	// IEEE 754 double: set exponent to 0 (value = 2^0 * mantissa).
	// 0x3FF0000000000000 = bias for exponent 0.
	// Mask mantissa into [0.5, 1.0) range.
	v = (v & 0x000FFFFFFFFFFFFF) | 0x3FF0000000000000
	f := math.Float64frombits(v) - 1.0
	// f is now in [0, 1). Clamp for floating-point safety.
	if f < 0 {
		f = 0
	}
	if f >= 1.0 {
		f = 0.9999999999999999
	}
	return f
}
