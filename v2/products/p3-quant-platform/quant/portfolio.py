"""Portfolio construction from per-asset return series."""

from __future__ import annotations


def portfolio_returns(weights: list[float], asset_returns: list[list[float]]) -> list[float]:
    """Combine per-asset return series into one portfolio series.

    weights[i] applies to asset_returns[i]. All assets must share the same
    length. Weights must sum to 1 (within a small tolerance) — a portfolio that
    is not fully allocated is almost always a bug, so it is rejected rather than
    silently rescaled.
    """
    if len(weights) != len(asset_returns):
        raise ValueError("weights and assets differ in length")
    if not asset_returns:
        return []
    n = len(asset_returns[0])
    for a in asset_returns:
        if len(a) != n:
            raise ValueError("asset return series have different lengths")
    if abs(sum(weights) - 1.0) > 1e-6:
        raise ValueError(f"weights must sum to 1, got {sum(weights)}")

    out = []
    for t in range(n):
        out.append(sum(weights[i] * asset_returns[i][t] for i in range(len(weights))))
    return out
