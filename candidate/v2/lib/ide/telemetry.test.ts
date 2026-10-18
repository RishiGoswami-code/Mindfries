import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest';
import { TelemetryBuffer } from './telemetry';

// Mock the global fetch
const originalFetch = global.fetch;

describe('TelemetryBuffer', () => {
  let buffer: TelemetryBuffer;
  let fetchMock: Mock;

  beforeEach(() => {
    vi.useFakeTimers();
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof fetch;
    buffer = new TelemetryBuffer('sess-123');
  });

  afterEach(() => {
    buffer.destroy();
    vi.useRealTimers();
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('records events and flushes them on interval', async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, status: 200 });

    buffer.record('file_edit', { file: 'main.ts' });
    buffer.record('terminal', { cmd: 'ls' });

    // Ensure they are queued
    // @ts-expect-error accessing private property for testing
    expect(buffer.queue.length).toBe(2);

    // Advance time by 5 seconds to trigger interval flush
    await vi.advanceTimersByTimeAsync(5000);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const fetchArgs = fetchMock.mock.calls[0];
    expect(fetchArgs[0]).toBe('/api/telemetry');
    
    const body = JSON.parse(fetchArgs[1].body);
    expect(body.sessionId).toBe('sess-123');
    expect(body.events.length).toBe(2);
    expect(body.events[0].type).toBe('file_edit');

    // Queue should be empty after successful flush
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(0);
  });

  it('restores batch to queue on non-2xx response (flush race fix)', async () => {
    // Simulate a 500 backend hiccup
    fetchMock.mockResolvedValueOnce({ ok: false, status: 500 });

    buffer.record('file_edit', { file: 'main.ts' });

    // Force an immediate flush without waiting for timer
    await buffer.flush();

    // Since the flush failed, the event should be prepended back to the queue
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(1);
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue[0].type).toBe('file_edit');
  });

  it('restores batch to queue on network error (flush race fix)', async () => {
    // Simulate a network failure where fetch throws
    fetchMock.mockRejectedValueOnce(new Error('Network error'));

    buffer.record('terminal', { cmd: 'npm test' });

    // Force an immediate flush
    await buffer.flush();

    // Event should be restored
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(1);
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue[0].type).toBe('terminal');
  });

  it('triggers flush immediately when MAX_BATCH is reached', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200 });
    
    // MAX_BATCH is 25 in telemetry.ts
    for (let i = 0; i < 24; i++) {
      buffer.record('test_run', { count: i });
    }
    
    // At 24 events, flush should not be called yet
    expect(fetchMock).not.toHaveBeenCalled();
    
    // 25th event triggers flush immediately (without waiting for timer)
    buffer.record('test_run', { count: 24 });
    
    // Wait for the async flush promise to resolve
    await Promise.resolve(); 
    
    expect(fetchMock).toHaveBeenCalledTimes(1);
    
    // Queue should be empty after successful flush
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(0);
  });

  it('sends beacon on destroy', () => {
    const sendBeaconMock = vi.fn().mockReturnValue(true);
    Object.defineProperty(global.navigator, 'sendBeacon', {
      value: sendBeaconMock,
      writable: true,
      configurable: true
    });

    buffer.record('file_edit', { file: 'exit.ts' });
    buffer.destroy();

    expect(sendBeaconMock).toHaveBeenCalledTimes(1);
    expect(sendBeaconMock.mock.calls[0][0]).toBe('/api/telemetry');
    
    // We can't easily parse a Blob in Node.js test environment, 
    // but we can verify it was called.
    
    // Queue should be cleared
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(0);
  });

  it('delivers a backlog bigger than the server accepts, in batches', async () => {
    // An outage leaves 120 events queued; the server takes at most 100 at once.
    fetchMock.mockRejectedValue(new Error('offline'));
    for (let i = 0; i < 120; i++) buffer.record('file_edit', { i });
    await buffer.flush();
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({ ok: true, status: 202 });

    await buffer.flush();

    const sizes = fetchMock.mock.calls.map((call) => JSON.parse(call[1].body).events.length);
    expect(Math.max(...sizes)).toBeLessThanOrEqual(25);
    expect(sizes.reduce((a, b) => a + b, 0)).toBe(120);
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(0);
  });

  it('drops a batch the server refuses as malformed, and carries on', async () => {
    fetchMock.mockResolvedValueOnce({ ok: false, status: 400 });
    buffer.record('bad', {});
    await buffer.flush();
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(0);

    fetchMock.mockResolvedValueOnce({ ok: true, status: 202 });
    buffer.record('good', {});
    await buffer.flush();
    expect(JSON.parse(fetchMock.mock.calls[1][1].body).events[0].type).toBe('good');
  });

  it('stops growing past its cap while the server is unreachable', async () => {
    fetchMock.mockRejectedValue(new Error('offline'));
    vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    for (let i = 0; i < 2500; i++) buffer.record('file_edit', { i });
    await buffer.flush();
    // @ts-expect-error queue is private; read here to see what was buffered
    expect(buffer.queue.length).toBe(2000);
  });

  it('never runs two flushes at once', async () => {
    let release: (v: unknown) => void = () => undefined;
    fetchMock.mockImplementation(() => new Promise((resolve) => (release = resolve)));
    buffer.record('a', {});
    const first = buffer.flush();
    const second = buffer.flush();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    release({ ok: true, status: 202 });
    await Promise.all([first, second]);
  });
});
