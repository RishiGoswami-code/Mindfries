"""Data-quality handling for price series with missing observations.

Missing data is represented by None. Forward-filling is a defensible default,
but silently fabricating prices hides a real problem (SPEC §15), so these
helpers also report how much was missing.
"""

from __future__ import annotations


def missing_count(prices: list[float | None]) -> int:
    """Number of missing (None) observations."""
    return sum(1 for p in prices if p is None)


def pct_missing(prices: list[float | None]) -> float:
    """Fraction of observations that are missing, in [0, 1]."""
    if not prices:
        return 0.0
    return missing_count(prices) / len(prices)


def forward_fill(prices: list[float | None]) -> list[float]:
    """Forward-fill missing values with the last observed price. Leading
    missing values (before any real observation) are dropped, since there is
    nothing to carry forward. Raises if every value is missing."""
    out: list[float] = []
    last: float | None = None
    for p in prices:
        if p is not None:
            last = p
            out.append(p)
        elif last is not None:
            out.append(last)
        # else: leading None, dropped
    if not out:
        raise ValueError("series has no observed prices to fill from")
    return out
