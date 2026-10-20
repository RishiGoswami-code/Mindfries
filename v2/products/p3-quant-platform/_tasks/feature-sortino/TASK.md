# Implement the Sortino ratio

Sharpe penalizes all volatility, including the upside. The **Sortino ratio**
divides excess return by *downside* deviation only, which better reflects how a
strategy actually hurts.

## Your task

Implement `sortino_ratio` in `quant/sortino.py`:

```python
def sortino_ratio(returns, risk_free=0.0, periods_per_year=252): ...
```

Definition to implement:

1. Convert the annual `risk_free` to a per-period rate (`risk_free /
   periods_per_year`) and subtract it from each return to get excess returns.
2. Downside deviation = the root-mean-square of the **negative** excess returns,
   using the full count of periods as the denominator: `sqrt(mean(min(0, e)^2))`.
3. Sortino = `mean(excess) / downside_deviation * sqrt(periods_per_year)`.
4. If the downside deviation is `0` (no negative excess returns), return `0.0`.

A stub is provided so the package imports; replace it.

## How to check

```sh
python3 -m unittest discover -s tests -t .
```
