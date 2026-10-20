import math
import unittest

from quant.backtest import backtest
from quant.data import forward_fill, missing_count, pct_missing
from quant.features import moving_average, rolling_volatility, zscore
from quant.metrics import annualized_return, cumulative_return, max_drawdown, sharpe
from quant.portfolio import portfolio_returns
from quant.series import equity_curve, simple_returns


class TestSeries(unittest.TestCase):
    def test_simple_returns(self):
        self.assertEqual(simple_returns([100, 110, 99]), [0.1, -0.1])
        self.assertEqual(simple_returns([100]), [])

    def test_non_positive_price_raises(self):
        with self.assertRaises(ValueError):
            simple_returns([100, 0, 50])

    def test_equity_curve(self):
        eq = equity_curve([0.1, -0.1])
        self.assertAlmostEqual(eq[-1], 1.1 * 0.9)


class TestMetrics(unittest.TestCase):
    def test_cumulative_return(self):
        self.assertAlmostEqual(cumulative_return([0.1, -0.1]), 1.1 * 0.9 - 1)

    def test_sharpe_zero_variance(self):
        self.assertEqual(sharpe([0.01, 0.01, 0.01]), 0.0)

    def test_sharpe_positive(self):
        # Steady positive returns with some noise -> positive Sharpe.
        self.assertGreater(sharpe([0.01, 0.02, 0.015, 0.005, 0.012]), 0)

    def test_annualized_return(self):
        # 1% per period, 252 periods/yr, over exactly a year of data.
        self.assertAlmostEqual(annualized_return([0.01] * 252), 1.01 ** 252 - 1, places=6)

    def test_max_drawdown(self):
        # Up 10%, then down ~18.2% (equity 1.1 -> 0.9): drawdown = 0.9/1.1 - 1.
        dd = max_drawdown([0.1, -0.1 / 1.1 * 1.1, (0.9 / 1.1 - 1)])
        self.assertLess(dd, 0)
        # Monotonic-up curve never draws down.
        self.assertEqual(max_drawdown([0.01, 0.02, 0.03]), 0.0)


class TestFeatures(unittest.TestCase):
    def test_moving_average(self):
        self.assertEqual(moving_average([1, 2, 3, 4], 2), [1.5, 2.5, 3.5])
        self.assertEqual(moving_average([1, 2], 3), [])

    def test_rolling_volatility(self):
        vol = rolling_volatility([1, 2, 3, 4, 5], 3)
        self.assertEqual(len(vol), 3)
        self.assertTrue(all(v > 0 for v in vol))

    def test_zscore(self):
        z = zscore([1, 2, 3])
        self.assertAlmostEqual(sum(z), 0.0, places=9)
        self.assertEqual(zscore([5, 5, 5]), [0.0, 0.0, 0.0])


class TestPortfolio(unittest.TestCase):
    def test_weighted_combo(self):
        pr = portfolio_returns([0.5, 0.5], [[0.1, 0.0], [0.0, 0.2]])
        self.assertEqual(pr, [0.05, 0.1])

    def test_weights_must_sum_to_one(self):
        with self.assertRaises(ValueError):
            portfolio_returns([0.5, 0.2], [[0.1], [0.1]])

    def test_length_mismatch_raises(self):
        with self.assertRaises(ValueError):
            portfolio_returns([1.0], [[0.1], [0.2]])


class TestData(unittest.TestCase):
    def test_forward_fill_and_stats(self):
        prices = [None, 100.0, None, 102.0, None]
        filled = forward_fill(prices)
        self.assertEqual(filled, [100.0, 100.0, 102.0, 102.0])  # leading None dropped
        self.assertEqual(missing_count(prices), 3)
        self.assertAlmostEqual(pct_missing(prices), 3 / 5)

    def test_all_missing_raises(self):
        with self.assertRaises(ValueError):
            forward_fill([None, None])


class TestBacktest(unittest.TestCase):
    def test_lookahead_safe_and_cost(self):
        prices = [100.0, 110.0, 99.0]  # returns: +0.10, -0.10
        # Hold long for the first period only.
        res = backtest(prices, [1.0, 0.0, 0.0], cost=0.0)
        self.assertEqual(len(res.strategy_returns), 2)
        self.assertAlmostEqual(res.strategy_returns[0], 0.10)  # captured period-1 up move
        self.assertAlmostEqual(res.strategy_returns[1], 0.0)   # flat for the down move

    def test_turnover_and_cost_charged(self):
        prices = [100.0, 110.0, 121.0]
        res = backtest(prices, [1.0, 1.0, 1.0], cost=0.01)
        # Position opened at period 0 (0 -> 1), no change after: turnover 1.
        self.assertAlmostEqual(res.turnover, 1.0)
        self.assertAlmostEqual(res.strategy_returns[0], 0.10 - 0.01)

    def test_length_mismatch_raises(self):
        with self.assertRaises(ValueError):
            backtest([100.0, 110.0], [1.0])


if __name__ == "__main__":
    unittest.main()
