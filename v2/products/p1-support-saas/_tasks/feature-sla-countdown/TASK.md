# Build the SLA badge

Agents triage the queue by how each ticket stands against its SLA, but the
badge that's meant to show this was never finished — it renders a static
placeholder. Make it real.

## What to build

Two files are stubbed for you:

- `console/src/sla.ts` — `slaFor(dueAtISO, now)` should return the SLA state and
  the whole minutes remaining:
  - `minutesLeft` = whole minutes from `now` until `dueAtISO` (negative once past due).
  - `state` is `"breached"` once past due, `"at-risk"` when due within
    `AT_RISK_MINUTES` (60), otherwise `"ok"`.
- `console/src/components/SlaBadge.tsx` — render a pill that:
  - exposes the state as `data-state` and keeps `role="status"`,
  - shows a human label for the state (On track / Due soon / Breached),
  - shows the remaining time (e.g. `30m left`, or `30m overdue` when breached).

`TicketList` already uses `SlaBadge`, so your component must keep the same props
(`dueAt: string`, optional `now?: Date`).

## Run it

```bash
cd console
npm install
npm test
```

Keep the clock injectable (`now`) — don't read `Date.now()` inside the logic, so
the queue renders deterministically.
