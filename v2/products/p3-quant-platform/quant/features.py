"""Feature construction over a numeric series."""

from __future__ import annotations

from statistics import mean, pstdev, stdev


def moving_average(xs: list[float], window: int) -> list[float]:
    """Simple moving average. Returns len(xs) - window + 1 values (empty if the
    series is shorter than the window)."""
    if window <= 0:
        raise ValueError("window must be positive")
    if len(xs) < window:
        return []
    return [mean(xs[i : i + window]) for i in range(len(xs) - window + 1)]


def rolling_volatility(xs: list[float], window: int) -> list[float]:
    """Rolling sample standard deviation over `window` points."""
    if window < 2:
        raise ValueError("window must be at least 2 for a sample std")
    if len(xs) < window:
        return []
    return [stdev(xs[i : i + window]) for i in range(len(xs) - window + 1)]


def zscore(xs: list[float]) -> list[float]:
    """Standardize a series to zero mean and unit (population) variance.
    Returns zeros if the series has no spread."""
    if not xs:
        return []
    m = mean(xs)
    sd = pstdev(xs)
    if sd == 0:
        return [0.0 for _ in xs]
    return [(x - m) / sd for x in xs]
