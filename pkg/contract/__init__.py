# Package contract — Cross-language determinism contract (ATOM-CL).
# Python mirror of atom-runtime/pkg/contract/contract.go.
# CL1: Go output == Python output (bit-level).
# CL2: No map iteration without sorting.
# CL3: RFC 8785 canonical JSON only.
import hashlib
import json
import struct
from typing import Any, Dict, List, Union

__all__ = ["DeterministicContext", "encode", "hash_payload", "hash_event", "encode_dict"]


class DeterministicContext:
    """Binds an event to a specific execution point (CL1)."""

    __slots__ = ("trace_id", "tick", "event_id")

    def __init__(self, trace_id: str, tick: int, event_id: str = ""):
        self.trace_id = trace_id
        self.tick = tick
        self.event_id = event_id

    def __repr__(self):
        return f"Ctx(trace={self.trace_id!r} tick={self.tick} id={self.event_id!r})"


def _encode_value(v: Any) -> bytes:
    """Encode a single JSON value as RFC 8785 canonical bytes."""
    if isinstance(v, dict):
        return encode(v)
    elif isinstance(v, list):
        return _encode_list(v)
    elif isinstance(v, str):
        return json.dumps(v, separators=(",", ":")).encode("utf-8")
    else:
        return json.dumps(v, separators=(",", ":")).encode("utf-8")


def _encode_list(items: List[Any]) -> bytes:
    """Encode a JSON array as RFC 8785 canonical bytes."""
    parts = [_encode_value(x) for x in items]
    return b"[" + b",".join(parts) + b"]"


def encode(value: Any) -> bytes:
    """
    RFC 8785 canonical JSON encoder.
    CL2: Map keys are sorted alphabetically (deterministic ordering).
    CL3: No whitespace, ASCII-only.
    """
    if isinstance(value, dict):
        keys = sorted(value.keys())
        parts = []
        for k in keys:
            v = value[k]
            key_b = json.dumps(k, separators=(",", ":")).encode("utf-8")
            val_b = _encode_value(v)
            parts.append(key_b + b":" + val_b)
        return b"{" + b",".join(parts) + b"}"
    elif isinstance(value, list):
        return _encode_list(value)
    else:
        return json.dumps(value, separators=(",", ":")).encode("utf-8")


def encode_dict(d: Dict[str, Any]) -> bytes:
    """Convenience wrapper for dict encoding."""
    return encode(d)


def hash_payload(payload: bytes) -> str:
    """SHA-256 of RFC 8785 canonical payload."""
    return hashlib.sha256(payload).hexdigest()


def hash_event(ctx: DeterministicContext, seq: int, event_type: str,
              payload: bytes, prev_hash: str) -> str:
    """
    Deterministic event hash.
    Formula: SHA256(traceID ‖ seq ‖ eventType ‖ HashPayload(payload) ‖ prevHash)
    CL1: Must be bit-identical to Go contract.HashEvent().
    """
    payload_hash = hash_payload(payload)
    raw = f"{ctx.trace_id}|{seq}|{event_type}|{payload_hash}|{prev_hash}"
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()
