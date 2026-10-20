/**
 * The pure parts of the avatar's Gemini Live conversation: the messages the
 * browser sends and understands, the audio conversions, and how what is said
 * becomes the transcript the hiring team reads. No browser APIs in here, so it
 * can be checked in Node (live.check.ts).
 *
 * The browser talks to Gemini directly over a WebSocket with a short-lived
 * token the V2 API minted. The model, the voice and the avatar's instructions
 * are locked inside that token, so the page sends only what it must.
 */

export const SEND_RATE = 16000; // what we send: 16 kHz mono PCM
export const RECEIVE_RATE = 24000; // what Gemini sends back

// ---- what we send ----------------------------------------------------------

export const setupMessage = (model: string) => ({ setup: { model: model.startsWith("models/") ? model : `models/${model}` } });

export const audioMessage = (pcm: Int16Array) => ({
  realtimeInput: { audio: { data: base64Encode(new Uint8Array(pcm.buffer, pcm.byteOffset, pcm.byteLength)), mimeType: `audio/pcm;rate=${SEND_RATE}` } },
});

export const videoMessage = (jpegBase64: string) => ({ realtimeInput: { video: { data: jpegBase64, mimeType: "image/jpeg" } } });

export const audioStreamEnd = () => ({ realtimeInput: { audioStreamEnd: true } });

// ---- what we receive -------------------------------------------------------

export interface ServerEvent {
  setupComplete?: boolean;
  /** Audio the avatar is speaking: 16-bit PCM at RECEIVE_RATE. */
  audio?: Int16Array;
  /** What the candidate said, as Gemini heard it (a fragment). */
  heard?: string;
  /** What the avatar said (a fragment). */
  said?: string;
  turnComplete?: boolean;
  /** The candidate spoke over the avatar: stop playing what is queued. */
  interrupted?: boolean;
  error?: string;
}

export function parseServerMessage(m: unknown): ServerEvent {
  const o = (m ?? {}) as Record<string, unknown>;
  const ev: ServerEvent = {};
  if (o.setupComplete !== undefined) ev.setupComplete = true;
  if (typeof o.error === "object" && o.error) ev.error = String((o.error as { message?: string }).message ?? "The avatar stopped.");
  const sc = o.serverContent as Record<string, unknown> | undefined;
  if (sc) {
    const parts = ((sc.modelTurn as { parts?: unknown[] } | undefined)?.parts ?? []) as { inlineData?: { data?: string; mimeType?: string } }[];
    const chunks: Int16Array[] = [];
    for (const p of parts) {
      const d = p.inlineData;
      if (d?.data && (d.mimeType ?? "").startsWith("audio/")) chunks.push(bytesToInt16(base64Decode(d.data)));
    }
    if (chunks.length) ev.audio = concat(chunks);
    const heard = (sc.inputTranscription as { text?: string } | undefined)?.text;
    const said = (sc.outputTranscription as { text?: string } | undefined)?.text;
    if (heard) ev.heard = heard;
    if (said) ev.said = said;
    if (sc.turnComplete) ev.turnComplete = true;
    if (sc.interrupted) ev.interrupted = true;
  }
  return ev;
}

// ---- audio -------------------------------------------------------------------

export function floatTo16(f: Float32Array): Int16Array {
  const out = new Int16Array(f.length);
  for (let i = 0; i < f.length; i++) {
    const s = Math.max(-1, Math.min(1, f[i]));
    out[i] = s < 0 ? Math.round(s * 0x8000) : Math.round(s * 0x7fff);
  }
  return out;
}

export function int16ToFloat(i: Int16Array): Float32Array {
  const out = new Float32Array(i.length);
  for (let k = 0; k < i.length; k++) out[k] = i[k] / (i[k] < 0 ? 0x8000 : 0x7fff);
  return out;
}

/** Lowers the sample rate by averaging, which is all speech needs (e.g. 48 kHz down to 16 kHz). */
export function downsample(input: Float32Array, from: number, to: number): Float32Array {
  if (to >= from) return input;
  const ratio = from / to;
  const out = new Float32Array(Math.floor(input.length / ratio));
  for (let i = 0; i < out.length; i++) {
    const start = Math.floor(i * ratio);
    const end = Math.min(input.length, Math.floor((i + 1) * ratio));
    let sum = 0;
    for (let j = start; j < end; j++) sum += input[j];
    out[i] = sum / Math.max(1, end - start);
  }
  return out;
}

/** Loudness 0..1, for the avatar's mouth. */
export function rms(f: Float32Array): number {
  if (!f.length) return 0;
  let sum = 0;
  for (let i = 0; i < f.length; i++) sum += f[i] * f[i];
  return Math.min(1, Math.sqrt(sum / f.length) * 3);
}

function concat(parts: Int16Array[]): Int16Array {
  const out = new Int16Array(parts.reduce((n, p) => n + p.length, 0));
  let at = 0;
  for (const p of parts) {
    out.set(p, at);
    at += p.length;
  }
  return out;
}

function bytesToInt16(b: Uint8Array): Int16Array {
  const even = b.byteLength - (b.byteLength % 2);
  const copy = new Uint8Array(even);
  copy.set(b.subarray(0, even));
  return new Int16Array(copy.buffer);
}

export function base64Encode(bytes: Uint8Array): string {
  let s = "";
  for (let i = 0; i < bytes.length; i += 0x8000) s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(s);
}

export function base64Decode(s: string): Uint8Array {
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

// ---- the transcript ----------------------------------------------------------

export interface Utterance {
  seq: number;
  role: "candidate" | "avatar";
  text: string;
  at: string;
}

/**
 * Gathers the fragments Gemini reports into whole utterances: one per speaker
 * turn, numbered from 1, so the server can keep them in order and ignore a
 * repeat if an upload is retried.
 */
export class TranscriptBuffer {
  private seq = 0;
  private cur: { role: "candidate" | "avatar"; text: string; at: string } | null = null;
  private done: Utterance[] = [];

  add(role: "candidate" | "avatar", fragment: string, now: Date = new Date()) {
    if (!fragment) return;
    if (this.cur && this.cur.role !== role) this.close();
    if (!this.cur) this.cur = { role, text: "", at: now.toISOString() };
    this.cur.text += fragment;
  }

  /** The speaker's turn ended. */
  close() {
    if (this.cur && this.cur.text.trim()) this.done.push({ seq: ++this.seq, role: this.cur.role, text: this.cur.text.trim(), at: this.cur.at });
    this.cur = null;
  }

  /** What is ready to send; it stays until `sent` says it went. */
  ready(): Utterance[] {
    return this.done.slice();
  }

  sent(upToSeq: number) {
    this.done = this.done.filter((u) => u.seq > upToSeq);
  }

  /** Closes the open turn too: for the end of the conversation. */
  flush(): Utterance[] {
    this.close();
    return this.ready();
  }
}
