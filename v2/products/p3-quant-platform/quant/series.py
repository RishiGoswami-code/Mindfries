"""Price/return series operations."""

from __future__ import annotations


def simple_returns(prices: list[float]) -> list[float]:
    """Period-over-period simple returns: (p[i] - p[i-1]) / p[i-1].

    Returns a list of length len(prices) - 1. Raises on a non-positive price,
    which would make a return meaningless.
    """
    if len(prices) < 2:
        return []
    out = []
    for i in range(1, len(prices)):
        prev = prices[i - 1]
        if prev <= 0:
            raise ValueError(f"non-positive price at index {i - 1}: {prev}")
        out.append((prices[i] - prev) / prev)
    return out


def equity_curve(returns: list[float], start: float = 1.0) -> list[float]:
    """Cumulative equity from a return series, beginning at `start`."""
    equity = []
    level = start
    for r in returns:
        level *= 1.0 + r
        equity.append(level)
    return equity
