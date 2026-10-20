-- Register product P3 (Quant research platform, Python). Run after 0001–0003.
-- Idempotent: upserts on slug. Bump golden_commit/version when source changes.

insert into v2.products (slug, name, domain, repo_ref, golden_commit, stack, role_coverage, status, version)
values (
  'p3-quant-platform',
  'Quant research platform',
  'quantitative research backtesting trading alpha',
  'v2/products/p3-quant-platform',
  '86a1dea69010a753b23a4111f3aa69520f2a680f',
  '["python"]'::jsonb,
  $manifest$
  {
    "tags": ["quant", "quantitative", "trading", "backtest", "backtesting", "portfolio", "alpha", "strategy", "sharpe", "risk", "timeseries", "returns", "signals", "volatility", "drawdown"],
    "roles": {
      "quant": {
        "subsystems": ["quant/series.py", "quant/metrics.py", "quant/features.py", "quant/backtest.py", "quant/portfolio.py"],
        "competencies": ["return and risk computation", "backtesting with lookahead safety", "portfolio construction", "transaction-cost awareness", "statistical conventions (Sharpe, drawdown)"],
        "modes": ["implementation", "debugging", "research", "testing"],
        "cannot_assess": ["live market data or execution", "derivatives pricing (not in this build)"]
      },
      "ml_researcher": {
        "subsystems": ["quant/backtest.py", "quant/features.py"],
        "competencies": ["time-aware validation", "leakage prevention", "feature construction", "reproducible experiments"],
        "modes": ["research", "debugging", "testing"],
        "cannot_assess": ["deep-learning training workloads"]
      },
      "data_analyst": {
        "subsystems": ["quant/data.py", "quant/metrics.py"],
        "competencies": ["missing-data handling", "data-quality metrics", "return and period analytics"],
        "modes": ["implementation", "research", "testing"],
        "cannot_assess": ["warehouse-scale SQL (in-memory lists here)"]
      }
    }
  }
  $manifest$::jsonb,
  'verified',
  1
)
on conflict (slug) do update set
  name = excluded.name,
  domain = excluded.domain,
  repo_ref = excluded.repo_ref,
  golden_commit = excluded.golden_commit,
  stack = excluded.stack,
  role_coverage = excluded.role_coverage,
  status = excluded.status,
  version = v2.products.version + 1,
  updated_at = now();
