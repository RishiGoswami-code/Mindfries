// Thin typed client over the Go support API. The base URL is injected so the
// same client works against a local `go run ./cmd/server` and a deployed API.

import type { Ticket } from "./types";

export const API_BASE = import.meta.env?.VITE_API_BASE ?? "http://localhost:8080";

export async function fetchTickets(signal?: AbortSignal): Promise<Ticket[]> {
  const res = await fetch(`${API_BASE}/tickets`, { signal });
  if (!res.ok) throw new Error(`GET /tickets failed: ${res.status}`);
  return (await res.json()) as Ticket[];
}
