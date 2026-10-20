"""Runnable demo: a moving-average crossover backtest on a sample series.

    python3 -m quant.report
"""

from __future__ import annotations

from .backtest import backtest
from .features import moving_average
from .metrics import cumulative_return, max_drawdown, sharpe


def sample_prices() -> list[float]:
    # A trending series with a dip, enough points for a short/long crossover.
    base = [100, 101, 103, 102, 105, 108, 107, 110, 113, 112,
            115, 118, 116, 114, 117, 121, 124, 123, 126, 130]
    return [float(p) for p in base]


def crossover_signals(prices: list[float], short: int = 3, long: int = 6) -> list[float]:
    """Long (1) when the short MA is above the long MA, else flat (0). Aligned
    back to the price index, lookahead-safe (uses only MAs available at t)."""
    ma_s = moving_average(prices, short)
    ma_l = moving_average(prices, long)
    signals = [0.0] * len(prices)
    offset = long - 1  # first index with both MAs available
    for t in range(offset, len(prices)):
        if ma_s[t - (short - 1)] > ma_l[t - (long - 1)]:
            signals[t] = 1.0
    return signals


def main() -> None:
    prices = sample_prices()
    signals = crossover_signals(prices)
    result = backtest(prices, signals, cost=0.001)
    print("periods:        ", len(result.strategy_returns))
    print("cumulative:     ", round(cumulative_return(result.strategy_returns), 4))
    print("sharpe (annual):", round(sharpe(result.strategy_returns), 3))
    print("max drawdown:   ", round(max_drawdown(result.strategy_returns), 4))
    print("turnover:       ", round(result.turnover, 3))


if __name__ == "__main__":
    main()
