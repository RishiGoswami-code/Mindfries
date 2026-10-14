/**
 * Counting faces on the proctoring camera, in the browser (issue #120).
 *
 * Uses Google's MediaPipe face detector (BlazeFace, short range), loaded on
 * demand from a CDN so it costs nothing until a session's camera is live.
 * No frame leaves the browser for this: it reports only a count.
 *
 * It raises two kinds of moment for a person to look at — nobody in front of
 * the camera for a while, or more than one person — and never acts on its
 * own. That is deliberate. Studies of remote proctoring have found face
 * detectors miss some faces more than others (Burgess et al., "Watching the
 * watchers", 2022, on skin tone), and that automated flags carry false
 * positives that human reviewers can then over-trust (Belzak et al., 2025).
 * So the report presents these as low-confidence prompts, each with the
 * camera still from that moment, and a session is never stopped by them.
 *
 * If the detector can't be loaded (offline, blocked CDN, an old browser) the
 * check is simply off; the camera stills are still taken.
 */

const VERSION = "1.1.0";
const BUNDLE = `https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@${VERSION}/vision_bundle.mjs`;
const WASM = `https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@${VERSION}/wasm`;
const MODEL = "https://storage.googleapis.com/mediapipe-models/face_detector/blaze_face_short_range/float16/1/blaze_face_short_range.tflite";

/** How often the camera is looked at. */
const CHECK_MS = 5_000;
/** Consecutive checks with nobody in view before it is a moment worth flagging (~15 s). */
const MISSING_CHECKS = 3;
/** Consecutive checks with more than one face before it is flagged (~10 s). */
const MULTIPLE_CHECKS = 2;

type FaceEvent = { kind: "face_missing" } | { kind: "multiple_faces"; faces: number };

interface Detector {
  detectForVideo(video: HTMLVideoElement, timestamp: number): { detections: unknown[] };
  close(): void;
}

// An import the bundler leaves alone: the module is fetched at run time
// from the CDN, not built into the app.
interface VisionModule {
  FilesetResolver: { forVisionTasks(wasmBase: string): Promise<unknown> };
  FaceDetector: { createFromOptions(files: unknown, options: object): Promise<Detector> };
}
const importAtRuntime = new Function("url", "return import(url)") as (url: string) => Promise<VisionModule>;

async function loadDetector(): Promise<Detector | null> {
  try {
    const vision = await importAtRuntime(BUNDLE);
    const files = await vision.FilesetResolver.forVisionTasks(WASM);
    return await vision.FaceDetector.createFromOptions(files, {
      baseOptions: { modelAssetPath: MODEL, delegate: "CPU" },
      runningMode: "VIDEO",
      minDetectionConfidence: 0.5,
    });
  } catch {
    return null;
  }
}

/**
 * Watches `video` (a playing element showing the camera) and calls `onEvent`
 * when a moment is worth flagging. Returns the function that stops it.
 */
export function watchFaces(video: HTMLVideoElement, onEvent: (event: FaceEvent) => void): () => void {
  let stopped = false;
  let detector: Detector | null = null;
  let missing = 0;
  let multiple = 0;
  let timer: ReturnType<typeof setInterval> | undefined;

  void loadDetector().then((d) => {
    if (stopped) {
      d?.close();
      return;
    }
    detector = d;
    if (!d) return;
    timer = setInterval(() => {
      if (stopped || document.hidden || video.readyState < 2) return;
      let faces = 0;
      try {
        faces = d.detectForVideo(video, performance.now()).detections.length;
      } catch {
        return;
      }
      missing = faces === 0 ? missing + 1 : 0;
      multiple = faces > 1 ? multiple + 1 : 0;
      // Once per run of checks, not once per check.
      if (missing === MISSING_CHECKS) onEvent({ kind: "face_missing" });
      if (multiple === MULTIPLE_CHECKS) onEvent({ kind: "multiple_faces", faces });
    }, CHECK_MS);
  });

  return () => {
    stopped = true;
    if (timer) clearInterval(timer);
    detector?.close();
  };
}
