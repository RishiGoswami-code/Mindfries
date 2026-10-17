"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, ExternalLink, Loader2, Maximize2, Minus, Plus } from "lucide-react";
import type { PDFDocumentProxy } from "pdfjs-dist";

const ZOOMS = [50, 75, 100, 125, 150, 200];

/**
 * The stored PDF itself, drawn page by page with pdf.js (the same library
 * that reads the resume's text on upload, lib/profile/resumeParse.ts) —
 * a real preview of the real file, with page, zoom and full-screen controls.
 * If it can't be drawn, it says so and offers the file instead.
 */
export function PdfPreview({ url, fileName }: { url: string; fileName: string }) {
  const frame = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const [doc, setDoc] = useState<PDFDocumentProxy | null>(null);
  const [failed, setFailed] = useState(false);
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(100);
  const [width, setWidth] = useState(0);

  useEffect(() => {
    let cancelled = false;
    // Destroying the loading task also frees the document and its worker.
    let task: { destroy: () => Promise<void> } | null = null;
    (async () => {
      try {
        const pdfjs = await import("pdfjs-dist");
        pdfjs.GlobalWorkerOptions.workerSrc = "/vendor/pdf.worker.min.mjs";
        const loading = pdfjs.getDocument({ url });
        task = loading;
        const loaded = await loading.promise;
        if (!cancelled) setDoc(loaded);
      } catch {
        if (!cancelled) setFailed(true);
      }
    })();
    return () => {
      cancelled = true;
      void task?.destroy();
    };
  }, [url]);

  // The page is drawn to fit the frame's width at 100%, so watch the width.
  useEffect(() => {
    const el = frame.current;
    if (!el) return;
    const observer = new ResizeObserver(([entry]) => setWidth(Math.floor(entry.contentRect.width)));
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!doc || !canvas.current || width === 0) return;
    let task: { cancel: () => void; promise: Promise<void> } | null = null;
    let cancelled = false;
    (async () => {
      const p = await doc.getPage(page);
      if (cancelled) return;
      const base = p.getViewport({ scale: 1 });
      const scale = ((width - 32) / base.width) * (zoom / 100);
      const viewport = p.getViewport({ scale });
      const ratio = window.devicePixelRatio || 1;
      const c = canvas.current!;
      c.width = Math.floor(viewport.width * ratio);
      c.height = Math.floor(viewport.height * ratio);
      c.style.width = `${Math.floor(viewport.width)}px`;
      c.style.height = `${Math.floor(viewport.height)}px`;
      task = p.render({ canvas: c, viewport, transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0] });
      await task.promise.catch(() => {});
    })();
    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [doc, page, zoom, width]);

  const pages = doc?.numPages ?? 0;
  const zoomIndex = ZOOMS.indexOf(zoom);

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* A fixed height, so the page scrolls inside it and the controls below stay in view. */}
      <div ref={frame} className="h-[min(60dvh,620px)] min-h-[320px] overflow-auto rounded-2xl bg-[#F1EFF8] p-4">
        {failed ? (
          <div className="flex h-full min-h-[320px] flex-col items-center justify-center gap-3 text-center">
            <p className="text-[14.5px] text-[#4C4874]">A preview of this file couldn&apos;t be drawn here.</p>
            <a href={url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-[14px] font-semibold text-[#5B3BE0] hover:underline">
              Open {fileName} <ExternalLink size={14} />
            </a>
          </div>
        ) : !doc ? (
          <div className="flex h-full min-h-[320px] items-center justify-center text-[#6E6A8F]">
            <Loader2 size={22} className="animate-spin" aria-label="Loading the preview" />
          </div>
        ) : (
          <canvas ref={canvas} className="mx-auto block rounded-md bg-white shadow-[0_12px_30px_-18px_rgba(27,24,64,0.45)]" aria-label={`Page ${page} of ${fileName}`} />
        )}
      </div>

      {doc && (
        <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <ControlButton label="Previous page" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              <ChevronLeft size={18} />
            </ControlButton>
            <span className="min-w-[52px] text-center text-[14px] tabular-nums text-[#2A2654]">
              {page} / {pages}
            </span>
            <ControlButton label="Next page" disabled={page >= pages} onClick={() => setPage((p) => p + 1)}>
              <ChevronRight size={18} />
            </ControlButton>
          </div>

          <div className="flex items-center gap-1 rounded-xl border border-[#E4E0F2] bg-white px-1">
            <ControlButton label="Zoom out" bare disabled={zoomIndex <= 0} onClick={() => setZoom(ZOOMS[zoomIndex - 1])}>
              <Minus size={16} />
            </ControlButton>
            <span className="min-w-[48px] text-center text-[14px] tabular-nums text-[#2A2654]">{zoom}%</span>
            <ControlButton label="Zoom in" bare disabled={zoomIndex >= ZOOMS.length - 1} onClick={() => setZoom(ZOOMS[zoomIndex + 1])}>
              <Plus size={16} />
            </ControlButton>
          </div>

          <ControlButton label="Full screen" onClick={() => void frame.current?.requestFullscreen?.()}>
            <Maximize2 size={16} />
          </ControlButton>
        </div>
      )}
    </div>
  );
}

function ControlButton({
  label,
  disabled,
  bare = false,
  onClick,
  children,
}: {
  label: string;
  disabled?: boolean;
  bare?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={
        bare
          ? "flex h-9 w-9 items-center justify-center rounded-lg text-[#2A2654] hover:bg-[#F1EFF8] disabled:opacity-35"
          : "flex h-10 w-10 items-center justify-center rounded-xl border border-[#E4E0F2] bg-white text-[#2A2654] hover:bg-[#F6F4FC] disabled:opacity-35"
      }
    >
      {children}
    </button>
  );
}
