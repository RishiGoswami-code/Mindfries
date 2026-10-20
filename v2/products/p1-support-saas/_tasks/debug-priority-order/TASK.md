# The triage queue is ordered backwards

Agents report that the support queue shows **low-priority tickets at the top** —
urgent ones sink to the bottom, which is exactly backwards from how they need to
work it. Tie-breaking within a priority looks fine; it's the priority ordering
itself that's wrong.

## Your task

Find and fix the bug so the queue reads most-urgent first
(`urgent → high → normal → low`), with the soonest-due ticket first within a
priority. The ordering policy lives in `console/src/priority.ts`.

## Run it

```bash
cd console
npm install
npm test
```

The shipped tests (`src/priority.test.ts`, `src/components/TicketList.test.tsx`)
reproduce the complaint — they should go green once the order is right. Keep the
function pure (don't mutate the input array).
