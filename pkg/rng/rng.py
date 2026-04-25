# Package rng — Deterministic RNG for Thompson Sampling (ATOM-016).
# Python mirror of atom-runtime/pkg/rng/rng.go.
# ATOM constraints: C3 (no random.*), R1-R3.
# Uses SHA-256 in counter mode for bit-level parity with Go implementation.
import hashlib
import struct
import math

__all__ = ["DeterministicRNG"]


class DeterministicRNG:
    """
    Produces bit-identical outputs from (seed, traceID, tick, modelID).
    R1: Same inputs → same output (guaranteed).
    R2: No global state mutation.
    R3: No hidden entropy (SHA-256 counter mode with explicit seed).
    """

    def __init__(self, seed: int):
        self.seed = seed

    def sample(self, trace_id: str, tick: int, model_id: str) -> float:
        """
        Deterministic float64 in [0, 1) from inputs.
        R1: sample(traceID, tick, modelID) is bit-identical across calls.
        Must be bit-level identical to Go DeterministicRNG.Sample().
        """
        # Build 8-byte entropy via FNV-64a.
        h = hashlib.fnv64a()
        h.update(struct.pack("<Q", tick))
        h.update(struct.pack("<Q", len(trace_id)))
        h.update(trace_id.encode("utf-8"))
        h.update(struct.pack("<Q", len(model_id)))
        h.update(model_id.encode("utf-8"))
        h.update(struct.pack("<Q", self.seed))
        entropy = h.digest()  # 8 bytes

        # Counter mode: SHA256(counter || entropy) for each block.
        counter = bytearray(40)
        counter[:8] = entropy
        struct.pack_into("<Q", counter, 8, 0)
        h1 = hashlib.sha256(counter).digest()  # first 32 bytes
        struct.pack_into("<Q", counter, 8, 1)
        _ = hashlib.sha256(counter).digest()  # consume block 2

        # Use first 8 bytes of h1 as entropy for float64.
        v = struct.unpack_from("<Q", h1, 0)[0]
        # IEEE 754 double: set exponent to 0 (value = 2^0 * mantissa).
        # 0x3FF0000000000000 = bias for exponent 0.
        v = (v & 0x000FFFFFFFFFFFFF) | 0x3FF0000000000000
        f = math.frexp(struct.unpack("<d", struct.pack("<Q", v))[0])[1] - 1.0
        # Clamp for floating-point safety.
        if f < 0:
            f = 0.0
        if f >= 1.0:
            f = 0.9999999999999999
        return f
