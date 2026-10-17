/**
 * Taking a still from a live video stream — the camera or the shared screen
 * — and keeping it: one JPEG uploaded straight to private storage through a
 * one-off signed URL. Shared by the camera's half-minute stills
 * (camera-snapshots.ts), the screen's keyframes (screen-capture.ts) and the
 * evidence taken at a flagged moment (activity-watch.ts).
 */

export interface StillOptions {
  /** Width of the kept image; height follows the stream's shape. */
  width: number;
  quality: number;
  /** The most bytes one still may be; quality is lowered until it fits. */
  maxBytes?: number;
}

export interface StillTaker {
  /** A frame as a JPEG, or null when there isn't one to be had right now. */
  take(): Promise<Blob | null>;
  /** The video element frames are read from, for a caller that wants to look at pixels itself. */
  video: HTMLVideoElement;
  stop(): void;
}

/** Reads frames off `stream` through a detached, never-shown <video>. */
export function stillTaker(stream: MediaStream, options: StillOptions): StillTaker {
  const video = document.createElement("video");
  video.muted = true;
  video.playsInline = true;
  video.srcObject = stream;
  void video.play().catch(() => undefined);

  const take = async (): Promise<Blob | null> => {
    if (video.readyState < 2 || video.videoWidth === 0) return null;
    const canvas = document.createElement("canvas");
    canvas.width = Math.min(options.width, video.videoWidth);
    canvas.height = Math.round((video.videoHeight / video.videoWidth) * canvas.width) || 240;
    const context = canvas.getContext("2d");
    if (!context) return null;
    context.drawImage(video, 0, 0, canvas.width, canvas.height);
    let quality = options.quality;
    for (;;) {
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", quality));
      if (!blob || !options.maxBytes || blob.size <= options.maxBytes || quality <= 0.25) return blob;
      quality -= 0.1;
    }
  };

  return {
    take,
    video,
    stop: () => {
      video.srcObject = null;
    },
  };
}

export interface StillSink {
  /** Somewhere to PUT a still, and the path it will have. Null when storage isn't available. */
  uploadTarget(kind: "camera" | "screen" | "audio"): Promise<{ uploadUrl: string; path: string } | null>;
}

/** Uploads one still (or audio clip). Returns its path, or null if it couldn't be kept. */
export async function keepStill(blob: Blob, kind: "camera" | "screen" | "audio", sink: StillSink): Promise<string | null> {
  try {
    const target = await sink.uploadTarget(kind);
    if (!target) return null;
    const put = await fetch(target.uploadUrl, { method: "PUT", headers: { "content-type": blob.type || "image/jpeg" }, body: blob });
    return put.ok ? target.path : null;
  } catch {
    return null;
  }
}
