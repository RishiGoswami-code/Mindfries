"""Risk and performance metrics.

Conventions matter here (SPEC §15): Sharpe is annualized from per-period
returns with a sample standard deviation, and max drawdown is the worst
peak-to-trough decline expressed as a negative fraction.
"""

from __future__ import annotations

import math
from statistics import mean, stdev

from .series import equity_curve


def cumulative_return(returns: list[float]) -> float:
    """Total compounded return over the series, e.g. 0.20 for +20%."""
    level = 1.0
    for r in returns:
        level *= 1.0 + r
    return level - 1.0


def annualized_return(returns: list[float], periods_per_year: int = 252) -> float:
    """Geometric annualized return from per-period returns."""
    if not returns:
        return 0.0
    growth = 1.0 + cumulative_return(returns)
    if growth <= 0:
        return -1.0
    return growth ** (periods_per_year / len(returns)) - 1.0


def sharpe(returns: list[float], risk_free: float = 0.0, periods_per_year: int = 252) -> float:
    """Annualized Sharpe ratio.

    risk_free is an annual rate; it is converted to a per-period rate before
    subtracting. Uses the sample standard deviation. Returns 0.0 when there is
    no variability or too little data.
    """
    if len(returns) < 2:
        return 0.0
    rf_period = risk_free / periods_per_year
    excess = [r - rf_period for r in returns]
    sd = stdev(excess)
    if sd == 0:
        return 0.0
    return (mean(excess) / sd) * math.sqrt(periods_per_year)


def max_drawdown(returns: list[float]) -> float:
    """Worst peak-to-trough decline of the equity curve, as a negative
    fraction (e.g. -0.25 for a 25% drawdown). 0.0 if the curve never falls."""
    equity = equity_curve(returns)
    if not equity:
        return 0.0
    return min(equity) / max(equity) - 1.0
