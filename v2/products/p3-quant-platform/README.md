# P3 — Quant research platform

One of the owned product codebases candidates are assessed on
(`v2/codebase-engine/ANALYSIS.md`). A quantitative research library: compute
returns and features, build portfolios, run a lookahead-safe backtest, and
measure risk (Sharpe, drawdown). Pure Python standard library — no numpy/pandas
— so the suite is hermetic. Maps onto the SPEC §15 quant scenarios (Sharpe,
leakage, missing data, unit/convention mistakes).

## Run it

```sh
python3 -m quant.report        # demo: a moving-average crossover backtest
```

Prints cumulative return, annualized Sharpe, max drawdown, and turnover for a
sample price series.

## Test it

```sh
python3 -m unittest discover -s tests -t .
```

## Layout

```
quant/
  series.py      simple returns, equity curve
  features.py    moving average, rolling volatility, z-score
  metrics.py     cumulative/annualized return, Sharpe, max drawdown
  portfolio.py   weighted portfolio returns (must be fully allocated)
  backtest.py    lookahead-safe backtester with transaction costs
  data.py        missing-data handling and data-quality stats
  report.py      runnable demo
tests/           the suite that defines "correct" for this product
manifest.json    role-coverage manifest
_tasks/          reference assessment tasks (added as the bank grows)
```

## What it assesses

See `manifest.json`: **Quant** (returns/risk, backtesting with lookahead
safety, portfolio construction), **AI/ML-Researcher** (time-aware validation,
leakage prevention, reproducibility), and **Data-Analyst** (missing-data
handling, data-quality metrics). Backend/API is covered by other products, so
P3 does not claim it.
