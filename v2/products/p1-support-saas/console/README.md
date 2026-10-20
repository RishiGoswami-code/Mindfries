# Support Console

The agent-facing web client for the Support SaaS helpdesk. It reads tickets
from the Go API (`../`, see the product README) and renders the triage queue.

Standard Vite + React + TypeScript — nothing bespoke, so it runs the same on a
candidate's machine as in CI.

```bash
npm install
npm run dev     # http://localhost:5173, talks to the API at VITE_API_BASE (default :8080)
npm test        # vitest
npm run build   # typecheck + production build
```

Where the interesting logic lives (and where tasks are set):

- `src/priority.ts` — the triage ordering policy (pure)
- `src/sla.ts` — SLA state from a due time (pure, clock injected)
- `src/components/SlaBadge.tsx` — the SLA status pill
- `src/components/TicketList.tsx` — the queue
