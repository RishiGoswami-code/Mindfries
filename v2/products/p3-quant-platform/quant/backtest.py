"""A small, lookahead-safe backtester.

The position for the return from prices[t] to prices[t+1] is decided by
signals[t] — information available at time t only. Using signals[t+1] (or the
same-day return) would leak the future; keeping the offset honest is central to
the quant role (SPEC §15).
"""

from __future__ import annotations

from dataclasses import dataclass, field

from .series import equity_curve, simple_returns


@dataclass
class BacktestResult:
    strategy_returns: list[float] = field(default_factory=list)
    equity: list[float] = field(default_factory=list)
    turnover: float = 0.0


def backtest(prices: list[float], signals: list[float], cost: float = 0.0) -> BacktestResult:
    """Run `signals` (target position per period) against `prices`.

    signals[t] is the position held over period t (prices[t] -> prices[t+1]).
    A transaction cost is charged on the change in position between periods.
    """
    if len(signals) != len(prices):
        raise ValueError("signals and prices must be the same length")

    asset_rets = simple_returns(prices)  # length len(prices) - 1
    strat: list[float] = []
    total_turnover = 0.0
    prev_pos = 0.0
    for i, r in enumerate(asset_rets):
        pos = signals[i]  # decided at time i, applied to the return i -> i+1
        traded = abs(pos - prev_pos)
        total_turnover += traded
        strat.append(pos * r - cost * traded)
        prev_pos = pos

    return BacktestResult(strategy_returns=strat, equity=equity_curve(strat), turnover=total_turnover)
