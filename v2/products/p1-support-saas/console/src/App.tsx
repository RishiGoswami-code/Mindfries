// The console shell: load tickets from the API and render the queue. Kept
// deliberately small — the interesting logic is in priority.ts, sla.ts and the
// components, which is where tasks live.

import { useEffect, useState } from "react";
import { fetchTickets } from "./api";
import { TicketList } from "./components/TicketList";
import type { Ticket } from "./types";

export function App() {
  const [tickets, setTickets] = useState<Ticket[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    fetchTickets(ctrl.signal)
      .then(setTickets)
      .catch((e) => {
        if (e.name !== "AbortError") setError(String(e));
      });
    return () => ctrl.abort();
  }, []);

  return (
    <main className="console">
      <h1>Support queue</h1>
      {error && <p className="error">{error}</p>}
      {tickets === null ? <p>Loading…</p> : <TicketList tickets={tickets} />}
    </main>
  );
}
