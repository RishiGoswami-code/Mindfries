"use client";

import { useCallback, useEffect, useRef } from "react";
import { checkpointWorkspace } from "@/app/ide/actions";
import type { FileContents } from "@/lib/ide/types";

/** How often the work is copied to the server while the candidate works. */
const INTERVAL_MS = 60 * 1000;
/** A browser refuses a keepalive body much over 64KB. */
const MAX_KEEPALIVE_BYTES = 60_000;

/**
 * Keeps the server's copy of the work current, so a session whose tab is
 * closed, or whose time runs out, still has the code on the server:
 * - every minute while working, when anything changed;
 * - when the tab is hidden (still alive — the most likely moment to leave);
 * - as the tab closes, with a keepalive request that outlives the page
 *   (small workspaces only — see app/api/workspace/checkpoint);
 * - and on demand (`saveNow`), e.g. the moment time runs out.
 *
 * In a sandbox workspace the backend copies the files from the sandbox's
 * disk; the call is only the prompt, once what's saved here is there.
 */
export function useCheckpoints({
  sessionId,
  working,
  sandboxed,
  flushSandbox,
  snapshot,
}: {
  sessionId?: string;
  /** False once the work is over (time's up, interview, submitted): the backend refuses changes then. */
  working: boolean;
  sandboxed: boolean;
  flushSandbox: () => Promise<void>;
  snapshot: () => FileContents;
}) {
  const lastSentRef = useRef("");
  const flushRef = useRef(flushSandbox);
  useEffect(() => {
    flushRef.current = flushSandbox;
  });

  /** Sends a copy now. Resolves to whether the server took it. */
  const saveNow = useCallback(async (): Promise<boolean> => {
    if (!sessionId) return false;
    if (sandboxed) {
      await flushRef.current();
      return (await checkpointWorkspace(sessionId, {})).ok;
    }
    const files = snapshot();
    const serialized = JSON.stringify(files);
    if (serialized === lastSentRef.current) return true;
    const result = await checkpointWorkspace(sessionId, files);
    if (result.ok) lastSentRef.current = serialized;
    return result.ok;
  }, [sessionId, sandboxed, snapshot]);

  useEffect(() => {
    if (!working || !sessionId) return;
    const interval = setInterval(() => void saveNow(), INTERVAL_MS);
    const onHidden = () => {
      if (document.hidden) void saveNow();
    };
    // The page is going away: only a keepalive request survives that.
    const onPageHide = () => {
      if (sandboxed) return;
      const body = JSON.stringify({ sessionId, files: snapshot() });
      if (body.length >= MAX_KEEPALIVE_BYTES || JSON.stringify(snapshot()) === lastSentRef.current) return;
      void fetch("/api/workspace/checkpoint", { method: "POST", headers: { "Content-Type": "application/json" }, body, keepalive: true }).catch(() => undefined);
    };
    document.addEventListener("visibilitychange", onHidden);
    window.addEventListener("pagehide", onPageHide);
    return () => {
      clearInterval(interval);
      document.removeEventListener("visibilitychange", onHidden);
      window.removeEventListener("pagehide", onPageHide);
    };
  }, [working, sessionId, sandboxed, saveNow, snapshot]);

  return { saveNow };
}
