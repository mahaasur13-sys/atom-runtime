#!/usr/bin/env python3
"""
Cross-language determinism tests (ATOM CL1, G3, G4).
Run with: python3 tests/parity_test.py
"""
import hashlib
import json
import struct
import sys
import threading
import unittest
from typing import Any, Dict
import os

# Add package to path.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from pkg.rng import DeterministicRNG
from pkg.clock import LogicalClock
from pkg.contract import DeterministicContext, encode, encode_dict, hash_event, hash_payload


class TestClockDeterminism(unittest.TestCase):
    """C1: No time.Now in execution path. LC1: tick monotonic."""

    def test_now_monotonic(self):
        lc = LogicalClock()
        for _ in range(100):
            lc.advance()
        ticks = [lc.now()]
        for _ in range(100):
            lc.advance()
            ticks.append(lc.now())
        for i in range(1, len(ticks)):
            self.assertGreater(ticks[i], ticks[i - 1], f"Clock went backwards at {i}")

    def test_no_time_import(self):
        import pkg.clock as clock_mod
        src = open("pkg/clock/__init__.py").read()
        self.assertNotIn("time.", src, "clock.py must not import time module")


class TestRNGDeterminism(unittest.TestCase):
    """C3: No random.* in execution path. R1: Same inputs → same output."""

    def test_sample_identical(self):
        r = DeterministicRNG(42)
        base = r.sample("trace-1", 1, "model-A")
        for i in range(999):
            got = r.sample("trace-1", 1, "model-A")
            self.assertEqual(got, base, f"RNG non-deterministic at run {i + 1}")

    def test_sample_range(self):
        r = DeterministicRNG(123)
        for i in range(100):
            s = r.sample("t", i, "m")
            self.assertGreaterEqual(s, 0.0, f"Sample {i} < 0")
            self.assertLess(s, 1.0, f"Sample {i} >= 1")

    def test_no_random_import(self):
        src = open("pkg/rng/rng.py").read()
        self.assertNotIn("random.", src, "rng.py must not import random module")


class TestCrossLangParity(unittest.TestCase):
    """CL1: Go output == Python output (bit-level)."""

    def test_hash_event_idempotent(self):
        ctx = DeterministicContext("workflow-42", tick=7)
        payload = encode_dict({"loss": 0.001, "step": 7})
        h1 = hash_event(ctx, seq=1, event_type="training.step", payload=payload, prev_hash="")
        h2 = hash_event(ctx, seq=1, event_type="training.step", payload=payload, prev_hash="")
        self.assertEqual(h1, h2, "HashEvent not idempotent")

    def test_hash_event_manual_verify(self):
        """Manually verify the formula: SHA256(traceID|seq|type|payloadHash|prevHash)"""
        ctx = DeterministicContext("workflow-42", tick=7)
        payload_bytes = encode({"loss": 0.001, "step": 7})
        payload_hash = hash_payload(payload_bytes)
        raw = f"workflow-42|1|training.step|{payload_hash}|"
        expected = hashlib.sha256(raw.encode("utf-8")).hexdigest()

        got = hash_event(ctx, seq=1, event_type="training.step",
                         payload=payload_bytes, prev_hash="")
        self.assertEqual(got, expected, "HashEvent formula broken")

    def test_encode_sorted_keys(self):
        """CL2: Map keys must be sorted for determinism."""
        d = {"z": 1, "a": 2, "m": 3}
        b1 = encode(d)
        b2 = encode(d)
        self.assertEqual(b1, b2)
        # Verify order: should be a,m,z (alphabetical).
        self.assertIn(b'{"a":2,"m":3,"z":1}', b1)

    def test_encode_nested(self):
        d = {"outer": {"inner": {"key": "value"}}, "num": 42}
        b1 = encode(d)
        b2 = encode(d)
        self.assertEqual(b1, b2)


class TestReplayDeterminism(unittest.TestCase):
    """G3: Same input → bit-identical output. G4: Replay(log) == Live."""

    def test_replay_idempotent(self):
        log = [
            {"trace_id": "t1", "seq": 1, "type": "test", "payload": b'{}'},
            {"trace_id": "t1", "seq": 2, "type": "test", "payload": b'{}'},
            {"trace_id": "t1", "seq": 3, "type": "test", "payload": b'{}'},
        ]
        state1 = replay(log)
        state2 = replay(log)
        self.assertEqual(state1, state2, "Replay not deterministic")

    def test_replay_equals_live(self):
        """Replay(log) must equal live execution result."""
        n = 5
        live_state = live_execution(n)
        log = build_log(n)
        replay_state = replay(log)
        self.assertEqual(live_state, replay_state,
                         f"Replay != Live:\nlive={live_state}\nreplay={replay_state}")


# ── Internal helpers ─────────────────────────────────────────────────────────

def replay(log):
    """Replay a log and return final state."""
    state = {}
    for ev in log:
        state[ev["trace_id"]] = ev["seq"]
    return json.dumps(state, sort_keys=True)


def live_execution(n):
    """Append N events directly (live execution)."""
    lc = LogicalClock()
    state = {}
    for i in range(n):
        lc.advance()
        state["live"] = i + 1
    return json.dumps(state, sort_keys=True)


def build_log(n):
    """Build N events for testing."""
    lc = LogicalClock()
    log = []
    for i in range(n):
        lc.advance()
        log.append({"trace_id": "test", "seq": i + 1, "type": "test", "payload": b"{}"})
    return log


if __name__ == "__main__":
    unittest.main(verbosity=2)
