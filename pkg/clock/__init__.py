# Package clock — Python mirror of ATOM-geb/clock.go
# ATOM constraints: C1 (no time.*), G1-G5, LC1-LC4.
# GEB-authoritative logical clock (no OS sleep, no wall time).
import threading
from typing import Optional

__all__ = ["LogicalClock"]


class LogicalClock:
    """
    SINGLE SOURCE of execution time truth.
    Only GEB may call advance(). All other components call now().
    LC1: tickₙ₊₁ = tickₙ + 1
    LC2: advance() ONLY called by GEB
    LC3: now() is monotonic
    LC4: Sleep does NOT use OS blocking
    """

    __slots__ = ("_tick", "_epoch_ms")

    def __init__(self, epoch_ms: int = 0):
        self._tick: int = 0
        self._epoch_ms: int = epoch_ms

    def now(self) -> int:
        """LC3: monotonic read-only tick."""
        return self._tick

    def advance(self) -> int:
        """
        LC1+LC2: increment tick by exactly 1.
        GEB-only method.
        """
        self._tick += 1
        return self._tick

    def advance_to(self, target: int) -> None:
        """Batch advance: set tick to max(current, target) + 1."""
        if self._tick < target:
            self._tick = target + 1
        else:
            self._tick += 1

    def epoch_ms(self) -> int:
        """Wall-clock epoch for metrics export only (NOT execution path)."""
        return self._epoch_ms

    def set_epoch_ms(self, epoch_ms: int) -> None:
        """Set epoch for checkpoint recovery."""
        self._epoch_ms = epoch_ms
