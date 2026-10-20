"""Sortino ratio: excess return over downside deviation."""

from __future__ import annotations

import math
from statistics import mean


def sortino_ratio(returns: list[float], risk_free: float = 0.0, periods_per_year: int = 252) -> float:
    if not returns:
        return 0.0
    rf_period = risk_free / periods_per_year
    excess = [r - rf_period for r in returns]
    downside_sq = [min(0.0, e) ** 2 for e in excess]
    dd = math.sqrt(sum(downside_sq) / len(excess))
    if dd == 0:
        return 0.0
    return mean(excess) / dd * math.sqrt(periods_per_year)
