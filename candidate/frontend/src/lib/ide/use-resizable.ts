import { useCallback, useState, type MouseEvent as ReactMouseEvent } from "react";

interface UseResizableOptions {
  initial: number;
  min: number;
  max: number;
  /** "horizontal" tracks mouse X (for sidebar width); "vertical" tracks mouse Y (for panel height). */
  axis: "horizontal" | "vertical";
  /** For a panel measured from the bottom/right edge, invert the delta direction. */
  invert?: boolean;
}

/** Drag-to-resize a single dimension, clamped to [min, max]. */
export function useResizable({ initial, min, max, axis, invert = false }: UseResizableOptions) {
  const [size, setSize] = useState(initial);
  // True while the mouse is down on the handle — so a panel can animate its
  // size when opened or closed, but follow the mouse exactly while dragged.
  const [dragging, setDragging] = useState(false);

  const startDrag = useCallback(
    (e: ReactMouseEvent) => {
      e.preventDefault();
      const start = axis === "horizontal" ? e.clientX : e.clientY;
      const startSize = size;
      setDragging(true);

      function onMouseMove(ev: MouseEvent) {
        const pos = axis === "horizontal" ? ev.clientX : ev.clientY;
        const rawDelta = pos - start;
        const delta = invert ? -rawDelta : rawDelta;
        setSize(Math.min(max, Math.max(min, startSize + delta)));
      }

      function onMouseUp() {
        setDragging(false);
        window.removeEventListener("mousemove", onMouseMove);
        window.removeEventListener("mouseup", onMouseUp);
      }

      window.addEventListener("mousemove", onMouseMove);
      window.addEventListener("mouseup", onMouseUp);
    },
    [axis, invert, min, max, size]
  );

  return { size, startDrag, dragging };
}
