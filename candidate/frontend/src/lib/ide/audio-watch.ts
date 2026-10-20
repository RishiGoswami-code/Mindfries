/**
 * Listening for voices and noise during a proctored session (issue #120).
 *
 * The microphone is opened with the camera. Nothing is recorded
 * continuously: the sound level is measured four times a second in the
 * browser, and only when something lasts — voices, or a loud noise — is
 * that moment flagged, with a short clip kept as its evidence. The clip
 * starts when the sound starts (not when the flag is raised), so a reviewer
 * hears the beginning of it.
 *
 * How it decides, simply (the rules and their numbers are in
 * audio-decide.ts):
 *   - The room's normal level keeps being learned — a fan, traffic or a
 *     steady background is not an event.
 *   - "Voices" is sound clearly above that level with most of its energy in
 *     the band human speech occupies (300–3400 Hz), filling more than a fifth
 *     of the last 20 seconds. Less than that is background, and allowed.
 *   - "Noise" is a loud sound outside that pattern, lasting a couple of
 *     seconds.
 *
 * Like the camera checks, these are for a person to judge — a candidate
 * thinking aloud trips "voices" too — and the report says so. The research
 * this follows (Atoum et al., 2017) used voice detection as one signal among
 * several, never alone.
 */

import { AudioDecider, TICK_MS, type AudioKind } from "./audio-decide";

/** Length of the evidence clip. */
const CLIP_MS = 10_000;

export type AudioEvent = { kind: AudioKind; clip: Promise<Blob | null> };

/**
 * Starts listening to the microphone track(s) of `stream`. `onEvent` is
 * called for each moment worth flagging. Returns the function that stops it.
 */
export function watchAudio(stream: MediaStream, onEvent: (event: AudioEvent) => void): () => void {
  const tracks = stream.getAudioTracks();
  if (tracks.length === 0 || typeof AudioContext === "undefined") return () => undefined;
  const audioOnly = new MediaStream(tracks);
  const context = new AudioContext();
  const source = context.createMediaStreamSource(audioOnly);
  const analyser = context.createAnalyser();
  analyser.fftSize = 2048;
  source.connect(analyser);
  const bins = new Float32Array(analyser.frequencyBinCount);
  const wave = new Float32Array(analyser.fftSize);
  const hzPerBin = context.sampleRate / analyser.fftSize;
  const speechLo = Math.floor(300 / hzPerBin);
  const speechHi = Math.ceil(3400 / hzPerBin);

  const decider = new AudioDecider();
  let clip: { recorder: MediaRecorder; done: Promise<Blob | null>; used: boolean } | null = null;
  let stopped = false;

  // The clip begins at the first loud tick, and is kept only if a flag uses it.
  const startClip = () => {
    if (clip || typeof MediaRecorder === "undefined") return;
    const type = MediaRecorder.isTypeSupported("audio/webm;codecs=opus") ? "audio/webm;codecs=opus" : "";
    let recorder: MediaRecorder;
    try {
      recorder = new MediaRecorder(audioOnly, { ...(type ? { mimeType: type } : {}), audioBitsPerSecond: 16_000 });
    } catch {
      return;
    }
    const chunks: Blob[] = [];
    const done = new Promise<Blob | null>((resolve) => {
      recorder.ondataavailable = (e) => e.data.size && chunks.push(e.data);
      recorder.onstop = () => resolve(chunks.length ? new Blob(chunks, { type: recorder.mimeType || "audio/webm" }) : null);
      recorder.onerror = () => resolve(null);
    });
    recorder.start();
    const current = { recorder, done, used: false };
    clip = current;
    setTimeout(() => {
      if (recorder.state !== "inactive") recorder.stop();
      // A clip no flag claimed is let go; the next sound starts a new one.
      setTimeout(() => {
        if (clip === current) clip = null;
      }, 0);
    }, CLIP_MS);
  };

  const tick = () => {
    if (stopped) return;
    analyser.getFloatTimeDomainData(wave);
    let sum = 0;
    for (const v of wave) sum += v * v;
    const db = 20 * Math.log10(Math.sqrt(sum / wave.length) + 1e-9);

    analyser.getFloatFrequencyData(bins);
    let total = 0;
    let speech = 0;
    for (let i = 1; i < bins.length; i++) {
      const power = Math.pow(10, bins[i] / 10);
      total += power;
      if (i >= speechLo && i <= speechHi) speech += power;
    }
    const share = total > 0 ? speech / total : 0;
    const { kind, flag: flagged } = decider.tick(db, share);
    if (kind && kind !== "quiet") startClip();
    if (flagged) {
      const evidence = clip && !clip.used ? clip : null;
      if (evidence) evidence.used = true;
      onEvent({ kind: flagged, clip: evidence ? evidence.done : Promise.resolve(null) });
    }
  };

  const timer = setInterval(tick, TICK_MS);
  return () => {
    stopped = true;
    clearInterval(timer);
    if (clip && clip.recorder.state !== "inactive") clip.recorder.stop();
    source.disconnect();
    void context.close().catch(() => undefined);
  };
}
