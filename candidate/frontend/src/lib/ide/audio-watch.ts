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
 * How it decides, simply:
 *   - The room's normal level is learned from the first few seconds — a fan
 *     or traffic is not an event.
 *   - "Voices" is sound well above that level with most of its energy in
 *     the band human speech occupies (300–3400 Hz), lasting a few seconds.
 *   - "Noise" is a loud sound outside that pattern, lasting a couple of
 *     seconds.
 *
 * Like the camera checks, these are for a person to judge — a candidate
 * thinking aloud trips "voices" too — and the report says so. The research
 * this follows (Atoum et al., 2017) used voice detection as one signal among
 * several, never alone.
 */

/** How often the level is measured. */
const TICK_MS = 250;
/** Seconds of sound used to learn the room's normal level. */
const CALIBRATE_TICKS = 24; // 6 s
/** How far above the room's level counts as a sound worth noticing, in dB. */
const VOICE_ABOVE_DB = 14;
const NOISE_ABOVE_DB = 22;
/** Share of energy in the speech band for a sound to count as voices. */
const SPEECH_SHARE = 0.55;
/** How long a sound must last before it is flagged. */
const VOICE_TICKS = 12; // 3 s within the window
const NOISE_TICKS = 8; // 2 s
/** The window those are counted over. */
const WINDOW_TICKS = 20; // 5 s
/** Length of the evidence clip. */
const CLIP_MS = 10_000;

export type AudioEvent = { kind: "voices_detected" | "loud_noise"; clip: Promise<Blob | null> };

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

  const calibration: number[] = [];
  let floorDb = -60;
  const recent: ("voice" | "noise" | "quiet")[] = [];
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

    if (calibration.length < CALIBRATE_TICKS) {
      calibration.push(db);
      if (calibration.length === CALIBRATE_TICKS) {
        const sorted = [...calibration].sort((a, b) => a - b);
        floorDb = sorted[Math.floor(sorted.length / 2)];
      }
      return;
    }

    analyser.getFloatFrequencyData(bins);
    let total = 0;
    let speech = 0;
    for (let i = 1; i < bins.length; i++) {
      const power = Math.pow(10, bins[i] / 10);
      total += power;
      if (i >= speechLo && i <= speechHi) speech += power;
    }
    const share = total > 0 ? speech / total : 0;
    const kind: "voice" | "noise" | "quiet" =
      db > floorDb + NOISE_ABOVE_DB && share < SPEECH_SHARE ? "noise" : db > floorDb + VOICE_ABOVE_DB && share >= SPEECH_SHARE ? "voice" : "quiet";
    if (kind !== "quiet") startClip();
    recent.push(kind);
    if (recent.length > WINDOW_TICKS) recent.shift();

    const voices = recent.filter((k) => k === "voice").length;
    const noise = recent.filter((k) => k === "noise").length;
    const flagged = voices >= VOICE_TICKS ? "voices_detected" : noise >= NOISE_TICKS ? "loud_noise" : null;
    if (flagged) {
      recent.length = 0; // one flag per stretch of sound
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
