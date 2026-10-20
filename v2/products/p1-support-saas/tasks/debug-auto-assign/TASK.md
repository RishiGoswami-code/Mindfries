# Fix: tickets pile onto one agent

Support leads are reporting that new tickets keep landing on the same agent
while others sit idle. Auto-assignment is supposed to spread load evenly across
the active agents on a ticket's team — the least-loaded agent should get the
next ticket.

## Your task

Find and fix the bug so that auto-assignment balances load. Don't change the
public behaviour otherwise: an inactive agent must still never be assigned, and
ties should resolve deterministically.

## How to check

```sh
go test ./internal/support/
```

The suite includes a test that assigns several tickets and expects them spread
across agents. It currently fails. Make it pass without weakening it.

## Where to look

Assignment lives in `internal/support/routing.go`. The surrounding domain
(tickets, agents, load) is in the same package.
