/**
 * Telemetry buffer — batches events client-side and flushes them to
 * candidate/backend's real ingestion endpoint (POST /api/v1/sessions/{id}/events,
 * via the same-origin /api/telemetry route so the browser's session cookie
 * comes along) rather than sending one request per event.
 *
 * What it guarantees, and doesn't:
 * - A batch is never more than MAX_BATCH events, so a backlog can always be
 *   delivered — the server refuses more than 100 in one request, and a queue
 *   that only ever retried in one piece would be refused forever once it
 *   grew past that.
 * - A failed send (network, 5xx) is retried on the next interval. A batch
 *   the server refuses as malformed (4xx) is dropped: sending it again can
 *   only be refused again, and it would block everything behind it.
 * - The queue is capped (MAX_QUEUE). During a long outage the newest events
 *   past the cap are dropped, and a warning says so — the alternative is a
 *   tab that grows without limit.
 * - On the way out (destroy), what's left goes by sendBeacon in pieces small
 *   enough for the browser to accept.
 *
 * No React, no DOM beyond `fetch`/`navigator.sendBeacon` — same
 * "plain module, React binds to it" shape as output.ts, so it's usable
 * from anywhere without dragging a component along.
 */

export interface TelemetryEvent {
  type: string;
  payload: unknown;
}

const FLUSH_INTERVAL_MS = 5000;
/** Events per request. Well under the server's limit of 100. */
const MAX_BATCH = 25;
/** The most kept while the server can't be reached. */
const MAX_QUEUE = 2000;
/** Browsers refuse a keepalive or beacon body much over 64KB. */
const MAX_KEEPALIVE_BYTES = 60_000;

export class TelemetryBuffer {
  private queue: TelemetryEvent[] = [];
  private timer: ReturnType<typeof setInterval> | null = null;
  private destroyed = false;
  private flushing: Promise<void> | null = null;
  private droppedWhileFull = 0;
  /** Events taken off the queue for a send that hasn't answered yet — still counted against the cap. */
  private inFlight = 0;

  constructor(private readonly sessionId: string) {
    this.timer = setInterval(() => void this.flush(), FLUSH_INTERVAL_MS);
  }

  record(type: string, payload: unknown): void {
    if (this.destroyed) return;
    if (this.queue.length + this.inFlight >= MAX_QUEUE) {
      this.droppedWhileFull += 1;
      if (this.droppedWhileFull === 1) console.warn("telemetry: queue full — dropping new events until the server answers");
      return;
    }
    this.queue.push({ type, payload });
    if (this.queue.length >= MAX_BATCH) void this.flush();
  }

  /** Sends everything queued, one batch at a time. Resolves when it's sent, or when a send fails and is left for the next interval. */
  flush(): Promise<void> {
    // One flush at a time: two in flight could send the same events twice.
    if (this.flushing) return this.flushing;
    this.flushing = this.drain().finally(() => {
      this.flushing = null;
    });
    return this.flushing;
  }

  private async drain(): Promise<void> {
    while (this.queue.length > 0) {
      const batch = this.queue.splice(0, MAX_BATCH);
      const body = JSON.stringify({ sessionId: this.sessionId, events: batch });
      this.inFlight = batch.length;
      try {
        const res = await fetch("/api/telemetry", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body,
          // Survives a navigation that starts right after this call — only
          // when the body is small enough for the browser to allow it.
          keepalive: body.length < MAX_KEEPALIVE_BYTES,
        });
        this.inFlight = 0;
        if (res.ok) {
          this.droppedWhileFull = 0;
          continue;
        }
        if (res.status >= 400 && res.status < 500) {
          // Refused as it stands: retrying sends the same refusal, and it
          // would hold up every event behind it.
          console.warn("telemetry: batch refused", res.status, "— dropped", batch.length, "events");
          continue;
        }
        this.queue.unshift(...batch);
        console.warn("telemetry: flush failed", res.status, "— will retry");
        return;
      } catch (err) {
        this.inFlight = 0;
        this.queue.unshift(...batch);
        console.warn("telemetry: flush failed", err, "— will retry");
        return;
      }
    }
  }

  /** Stops the interval and sends whatever's left with sendBeacon, which
   * (unlike fetch) is designed to survive the page actually going away. */
  destroy(): void {
    if (this.destroyed) return;
    this.destroyed = true;
    if (this.timer) clearInterval(this.timer);
    if (this.queue.length === 0 || typeof navigator === "undefined" || !navigator.sendBeacon) return;
    while (this.queue.length > 0) {
      const batch = this.queue.splice(0, MAX_BATCH);
      const body = JSON.stringify({ sessionId: this.sessionId, events: batch });
      // A batch too big for a beacon (one huge payload) can't go this way at all.
      if (body.length >= MAX_KEEPALIVE_BYTES) continue;
      navigator.sendBeacon("/api/telemetry", new Blob([body], { type: "application/json" }));
    }
  }
}
