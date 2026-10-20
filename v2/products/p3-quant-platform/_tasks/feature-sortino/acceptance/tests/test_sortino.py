import math
import unittest

from quant.sortino import sortino_ratio


class TestSortino(unittest.TestCase):
    def test_no_downside_returns_zero(self):
        # All excess returns non-negative -> downside deviation 0 -> 0.0.
        self.assertEqual(sortino_ratio([0.01, 0.02], 0.0, 1), 0.0)

    def test_known_value(self):
        # returns [0.2, -0.1], rf 0, ppy 1:
        #   excess = [0.2, -0.1]; mean = 0.05
        #   downside_sq = [0, 0.01]; dd = sqrt(0.01/2)
        #   sortino = 0.05 / dd * sqrt(1)
        expected = 0.05 / math.sqrt(0.01 / 2)
        self.assertAlmostEqual(sortino_ratio([0.2, -0.1], 0.0, 1), expected, places=9)

    def test_annualization_scales(self):
        # Same shape, ppy 4 -> scaled by sqrt(4) = 2.
        one = sortino_ratio([0.2, -0.1], 0.0, 1)
        four = sortino_ratio([0.2, -0.1], 0.0, 4)
        self.assertAlmostEqual(four, one * 2, places=9)


if __name__ == "__main__":
    unittest.main()
