"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";

/**
 * Document Picture-in-Picture: a small window the browser keeps above every
 * other window, the way a video call's floating window is. The panel renders
 * into it (a React portal), so the candidate keeps their tracker, tabs and
 * avatar in view while they work in their editor.
 *
 * The page that opened it stays alive behind: the camera, screen and
 * microphone are captured there, so closing or popping out the panel never
 * stops monitoring. Browsers without the API (Firefox, Safari) keep the panel
 * in the page, and say so.
 */
type DocPiP = { requestWindow: (o?: { width?: number; height?: number }) => Promise<Window> };

const api = (): DocPiP | undefined => (typeof window === "undefined" ? undefined : (window as unknown as { documentPictureInPicture?: DocPiP }).documentPictureInPicture);

const noop = () => () => {};

export interface PictureInPicture {
  supported: boolean;
  /** The pop-out window while it is open. */
  window: Window | null;
  open: (size: { width: number; height: number }) => Promise<void>;
  close: () => void;
  resize: (size: { width: number; height: number }) => void;
}

/** Copies the page's styles into the pop-out window, so the panel looks the same there. */
export function copyStyles(from: Document, to: Document) {
  to.documentElement.className = from.documentElement.className;
  to.body.className = from.body.className;
  to.body.style.margin = "0";
  for (const sheet of Array.from(from.styleSheets)) {
    try {
      const style = to.createElement("style");
      style.textContent = Array.from(sheet.cssRules)
        .map((r) => r.cssText)
        .join("\n");
      to.head.appendChild(style);
    } catch {
      // A cross-origin sheet can't be read; link it instead.
      if (sheet.href) {
        const link = to.createElement("link");
        link.rel = "stylesheet";
        link.href = sheet.href;
        to.head.appendChild(link);
      }
    }
  }
}

export function usePictureInPicture(): PictureInPicture {
  const supported = useSyncExternalStore(noop, () => !!api(), () => false);
  const [win, setWin] = useState<Window | null>(null);
  const ref = useRef<Window | null>(null);

  const open = useCallback(async (size: { width: number; height: number }) => {
    const pip = api();
    if (!pip || ref.current) return;
    const w = await pip.requestWindow(size);
    copyStyles(document, w.document);
    w.addEventListener("pagehide", () => {
      ref.current = null;
      setWin(null);
    });
    ref.current = w;
    setWin(w);
  }, []);

  const close = useCallback(() => {
    ref.current?.close();
    ref.current = null;
    setWin(null);
  }, []);

  const resize = useCallback((size: { width: number; height: number }) => {
    try {
      ref.current?.resizeTo(size.width, size.height);
    } catch {
      /* the browser may refuse without a click; the window keeps its size */
    }
  }, []);

  // Leaving the page closes the pop-out with it.
  useEffect(() => () => ref.current?.close(), []);

  return { supported, window: win, open, close, resize };
}
