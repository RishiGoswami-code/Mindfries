# `v2/contracts` — the API contracts

The agreements between V2's pieces, written before the code on either side.
Every later task is built against them.

**Status:** [`cli-api.yaml`](cli-api.yaml) is drafted (task 1.1), and the CLI and
its fake API are built against it. CI lints every contract here with Redocly.

## What goes here

- `cli-api.yaml`: OpenAPI for everything the CLI and `v2/api` exchange
  ([PLAN task 1.1](../PLAN.md#stage-1--the-cli-9-17)).
- Later, the contract between `v2/web` and `v2/api`.

## Rules

- A change to a contract is its own commit, made before the code that needs
  it.
- `v2/cli/fakeapi` and `v2/api` both serve the same file, and the CLI's
  end-to-end tests run against both.
