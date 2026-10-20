"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { PanelApi } from "@/lib/v2/panel/api";
import {
  audioMessage,
  downsample,
  floatTo16,
  int16ToFloat,
  parseServerMessage,
  RECEIVE_RATE,
  rms,
  SEND_RATE,
  setupMessage,
  TranscriptBuffer,
  videoMessage,
} from "@/lib/v2/avatar/live";
import type { AvatarState } from "./Avatar2D";
import type { AvatarController } from "./avatar";

const MIC_WORKLET = `
class MicProcessor extends AudioWorkletProcessor {
  constructor() { super(); this.parts = []; this.n = 0; }
  process(inputs) {
    const ch = inputs[0] && inputs[0][0];
    if (ch) {
      this.parts.push(new Float32Array(ch));
      this.n += ch.length;
      if (this.n >= 2048) {
        const out = new Float32Array(this.n);
        let at = 0;
        for (const p of this.parts) { out.set(p, at); at += p.length; }
        this.parts = []; this.n = 0;
        this.port.postMessage(out, [out.buffer]);
      }
    }
    return true;
  }
}
registerProcessor("mf-mic", MicProcessor);
`;

const FRAME_EVERY_MS = 2500;
const TRANSCRIPT_EVERY_MS = 10_000;

/**
 * The avatar, live: the candidate talks with it through the microphone and the
 * avatar answers by voice, over a Gemini Live session the browser opens
 * directly with a short-lived token the V2 API minted (the avatar's
 * instructions are locked inside it). While the conversation runs, a frame of
 * the shared screen and of the camera goes up every few seconds, so the avatar
 * can see what they are working on. What is said is posted back and kept as
 * evidence, in the same record as the terminal assistant's.
 *
 * It starts when the candidate presses Talk, and only while the clock runs.
 */
export function useAvatarLive(
  api: PanelApi,
  o: {
    enabled: boolean;
    running: boolean;
    /** A JPEG of the screen or the camera, base64, or null when that isn't shared. */
    frame: (what: "camera" | "screen", maxWidth: number) => Promise<string | null>;
  },
): AvatarController {
  const [state, setState] = useState<AvatarState>("idle");
  const [level, setLevel] = useState(0);
  const [muted, setMuted] = useState(false);
  const [note, setNote] = useState<string | undefined>();
  const [live, setLive] = useState(false);

  const ws = useRef<WebSocket | null>(null);
  const mic = useRef<MediaStream | null>(null);
  const ctxIn = useRef<AudioContext | null>(null);
  const ctxOut = useRef<AudioContext | null>(null);
  const analyser = useRef<AnalyserNode | null>(null);
  const nextAt = useRef(0);
  const playing = useRef(new Set<AudioBufferSourceNode>());
  const transcript = useRef(new TranscriptBuffer());
  const timers = useRef<ReturnType<typeof setInterval>[]>([]);
  const mutedRef = useRef(false);
  const frameFn = useRef(o.frame);
  useEffect(() => {
    frameFn.current = o.frame;
  }, [o.frame]);

  const sendTranscript = useCallback(async () => {
    const ready = transcript.current.ready();
    if (!ready.length) return;
    try {
      await api.avatarTranscript(ready);
      transcript.current.sent(ready[ready.length - 1].seq);
    } catch {
      /* kept, and sent with the next batch */
    }
  }, [api]);

  const stopPlayback = useCallback(() => {
    for (const s of playing.current) {
      try {
        s.stop();
      } catch {
        /* already ended */
      }
    }
    playing.current.clear();
    nextAt.current = 0;
  }, []);

  const teardown = useCallback(
    (why?: string) => {
      for (const t of timers.current) clearInterval(t);
      timers.current = [];
      const sock = ws.current;
      ws.current = null;
      if (sock && sock.readyState <= 1) sock.close();
      mic.current?.getTracks().forEach((t) => t.stop());
      mic.current = null;
      void ctxIn.current?.close().catch(() => {});
      ctxIn.current = null;
      stopPlayback();
      void ctxOut.current?.close().catch(() => {});
      ctxOut.current = null;
      analyser.current = null;
      transcript.current.close();
      void sendTranscript();
      setLive(false);
      setLevel(0);
      setState("idle");
      setNote(why);
    },
    [sendTranscript, stopPlayback],
  );

  const play = useCallback((pcm: Int16Array) => {
    const ctx = ctxOut.current;
    if (!ctx || !pcm.length) return;
    const f = int16ToFloat(pcm);
    const buf = ctx.createBuffer(1, f.length, RECEIVE_RATE);
    buf.copyToChannel(f as Float32Array<ArrayBuffer>, 0);
    const src = ctx.createBufferSource();
    src.buffer = buf;
    if (analyser.current) src.connect(analyser.current);
    else src.connect(ctx.destination);
    const at = Math.max(ctx.currentTime + 0.02, nextAt.current);
    src.start(at);
    nextAt.current = at + buf.duration;
    playing.current.add(src);
    setState("speaking");
    src.onended = () => {
      playing.current.delete(src);
      if (playing.current.size === 0) setState((s) => (s === "speaking" ? "listening" : s));
    };
  }, []);

  const startMic = useCallback(async () => {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true } });
    mic.current = stream;
    const ctx = new AudioContext();
    ctxIn.current = ctx;
    const url = URL.createObjectURL(new Blob([MIC_WORKLET], { type: "application/javascript" }));
    await ctx.audioWorklet.addModule(url);
    URL.revokeObjectURL(url);
    const node = new AudioWorkletNode(ctx, "mf-mic");
    node.port.onmessage = (e: MessageEvent<Float32Array>) => {
      const sock = ws.current;
      if (!sock || sock.readyState !== WebSocket.OPEN || mutedRef.current) return;
      sock.send(JSON.stringify(audioMessage(floatTo16(downsample(e.data, ctx.sampleRate, SEND_RATE)))));
    };
    ctx.createMediaStreamSource(stream).connect(node);
  }, []);

  const connect = useCallback(async () => {
    setNote("Connecting...");
    let tok;
    try {
      tok = await api.avatarToken();
    } catch (e) {
      return teardown(e instanceof Error ? e.message : "Couldn't start the avatar.");
    }
    // Output: a 24 kHz context and an analyser, so the mouth follows the voice.
    const out = new AudioContext({ sampleRate: RECEIVE_RATE });
    ctxOut.current = out;
    const an = out.createAnalyser();
    an.fftSize = 512;
    an.connect(out.destination);
    analyser.current = an;
    void out.resume();

    const sock = new WebSocket(`${tok.ws_url}?access_token=${encodeURIComponent(tok.token)}`);
    ws.current = sock;
    sock.onopen = () => sock.send(JSON.stringify(setupMessage(tok.model)));
    sock.onmessage = async (m) => {
      const text = typeof m.data === "string" ? m.data : m.data instanceof Blob ? await m.data.text() : new TextDecoder().decode(m.data as ArrayBuffer);
      let ev;
      try {
        ev = parseServerMessage(JSON.parse(text));
      } catch {
        return;
      }
      if (ev.error) return teardown(ev.error);
      if (ev.setupComplete) {
        try {
          await startMic();
        } catch {
          return teardown("The avatar needs your microphone. Allow it in the address bar and try again.");
        }
        setLive(true);
        setNote(undefined);
        setState("listening");
        timers.current.push(
          setInterval(async () => {
            const which = Math.random() < 0.7 ? "screen" : "camera";
            const jpeg = await frameFn.current(which, which === "screen" ? 1280 : 480);
            if (jpeg && ws.current?.readyState === WebSocket.OPEN && !mutedRef.current) ws.current.send(JSON.stringify(videoMessage(jpeg)));
          }, FRAME_EVERY_MS),
          setInterval(() => void sendTranscript(), TRANSCRIPT_EVERY_MS),
          setInterval(() => {
            const a = analyser.current;
            if (!a) return;
            const data = new Float32Array(a.fftSize);
            a.getFloatTimeDomainData(data);
            setLevel(rms(data));
          }, 60),
        );
        return;
      }
      if (ev.interrupted) stopPlayback();
      if (ev.heard) {
        transcript.current.add("candidate", ev.heard);
        setState((s) => (s === "speaking" ? s : "thinking"));
      }
      if (ev.audio) play(ev.audio);
      if (ev.said) transcript.current.add("avatar", ev.said);
      if (ev.turnComplete) {
        transcript.current.close();
        setState((s) => (s === "thinking" ? "listening" : s));
      }
    };
    sock.onerror = () => teardown("The avatar's connection failed. Try again in a moment.");
    sock.onclose = () => {
      if (ws.current === sock) teardown();
    };
  }, [api, play, sendTranscript, startMic, stopPlayback, teardown]);

  // The conversation ends when the clock does, or when the page goes.
  useEffect(() => {
    if (!o.running && ws.current) teardown();
  }, [o.running, teardown]);
  useEffect(() => () => teardown(), [teardown]);

  const available = o.enabled && o.running;
  const talk = useCallback(() => {
    if (live || ws.current) teardown();
    else void connect();
  }, [live, connect, teardown]);

  const toggleMute = useCallback(() => {
    setMuted((m) => {
      mutedRef.current = !m;
      if (ws.current) setState(m ? "listening" : "idle");
      return !m;
    });
  }, []);

  return {
    state,
    level,
    muted,
    talk,
    toggleMute,
    available,
    note: note ?? (!o.enabled ? undefined : !o.running ? "The avatar joins when your clock starts." : live ? undefined : "Press the button to talk."),
  };
}
