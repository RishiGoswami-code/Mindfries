# Fix: maximum drawdown is reported incorrectly

Risk is overstating (and sometimes understating) drawdown. On a strategy whose
equity only ever rises, `max_drawdown` should be `0.0`, but it reports a loss.
On real paths it reports the wrong figure whenever the lowest point comes
*before* the highest point.

## Your task

Fix `max_drawdown` in `quant/metrics.py` so it measures the worst
**peak-to-trough** decline — the largest drop from a running maximum — returned
as a negative fraction (0.0 if the equity curve never falls).

## How to check

```sh
python3 -m unittest discover -s tests -t .
```

The metrics tests currently fail. Make them pass without weakening them.

## Where to look

`quant/metrics.py`. The equity curve helper is in `quant/series.py`.
