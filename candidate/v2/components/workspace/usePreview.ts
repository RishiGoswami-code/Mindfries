"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { buildPreview, releaseBuild, type PreviewBuild } from "@/lib/ide/preview/build-preview";
import { CHANNELS, output } from "@/lib/ide/output";
import type { VfsBridge } from "@/lib/ide/vfs-bridge";
import type { FileContents, TreeNode } from "@/lib/ide/types";

export interface PreviewState {
  html: string;
  title: string;
  root: string;
  watching: boolean;
}

/**
 * The live preview of a web project: `dev` / `npm run dev` opens it, every
 * edit rebuilds it, and stopping the dev server freezes the last build.
 *
 * `onOpened` and `onClosed` let the workspace open and close the preview's
 * tab; they're read through refs, so the controller handed to the terminal
 * is one stable object for the life of the workspace.
 */
export function usePreview({
  vfs,
  tree,
  files,
  onOpened,
  onClosed,
}: {
  vfs: VfsBridge;
  tree: TreeNode[];
  files: FileContents;
  onOpened: () => void;
  onClosed: () => void;
}) {
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const buildRef = useRef<PreviewBuild | null>(null);
  const openedRef = useRef(onOpened);
  const closedRef = useRef(onClosed);
  useEffect(() => {
    openedRef.current = onOpened;
    closedRef.current = onClosed;
  });

  // The running dev server logs each rebuild into the terminal, as a real one does.
  const listenersRef = useRef(new Set<(line: string) => void>());
  const emitRebuild = (line: string) => {
    output.append(CHANNELS.preview, line);
    for (const listener of listenersRef.current) listener(line);
  };

  const controller = useMemo(
    () => ({
      open: (build: { html: string; title: string; root: string; objectUrls: string[] }) => {
        releaseBuild(buildRef.current);
        buildRef.current = { objectUrls: build.objectUrls } as PreviewBuild;
        setPreview({ html: build.html, title: build.title, root: build.root, watching: true });
        openedRef.current();
      },
      stop: () => setPreview((current) => (current ? { ...current, watching: false } : current)),
      onRebuild: (listener: (line: string) => void) => {
        listenersRef.current.add(listener);
        return () => {
          listenersRef.current.delete(listener);
        };
      },
    }),
    []
  );

  /** Ends the preview: its tab closes and its build is released. */
  const close = () => {
    releaseBuild(buildRef.current);
    buildRef.current = null;
    setPreview(null);
    closedRef.current();
  };

  // Rebuild on edits, debounced. Depends on the root, not the html, so writing the result can't retrigger it.
  const root = preview?.watching ? preview.root : null;
  useEffect(() => {
    if (root === null) return;
    const timeout = setTimeout(() => {
      const started = performance.now();
      buildPreview(vfs, root)
        .then((build) => {
          releaseBuild(buildRef.current);
          buildRef.current = build;
          setPreview((current) => (current && current.root === root ? { ...current, html: build.html } : current));
          emitRebuild(`rebuilt in ${Math.round(performance.now() - started)}ms`);
        })
        // A broken intermediate edit keeps the last build that worked.
        .catch((err) => emitRebuild(`build failed: ${err instanceof Error ? err.message : String(err)}`));
    }, 400);
    return () => clearTimeout(timeout);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- vfs reads live state; rebuilding is driven by tree/files changing
  }, [root, tree, files]);

  return { preview, controller, close };
}
