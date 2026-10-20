"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { mergeActivity, PanelApiError, toActivity, type ArtifactKind, type PanelApi } from "@/lib/v2/panel/api";
import { fromApi } from "@/lib/v2/panel/adapt";
import type { Diagram, Hypothesis, PanelConfig, ReviewComment } from "@/lib/v2/panel/types";

export type SaveState = "saved" | "saving" | "error" | "conflict";

export interface PanelData {
  config: PanelConfig;
  saveState: SaveState;
  saveDiagram: (d: Diagram) => void;
  saveNotes: (text: string) => void;
  saveReview: (c: ReviewComment[]) => void;
  saveHypotheses: (h: Hypothesis[]) => void;
  /** The candidate's own words about a flag or notice. */
  respond: (activityId: string, text: string) => Promise<string | null>;
  /** Reload from the server (after a conflict). */
  reload: () => Promise<void>;
}

const SAVE_AFTER_MS = 1500;
const REFRESH_MS = 20_000;

/**
 * The live panel's data: the configuration from the API, the tracker kept
 * current over Server-Sent Events, and what the candidate makes (diagram,
 * notes, review comments, hypotheses) saved a moment after they stop typing,
 * each save carrying the version it started from so a second window can't
 * silently overwrite the first.
 */
export function usePanelData(api: PanelApi, initial: ReturnType<typeof fromApi>): PanelData {
  const [config, setConfig] = useState<PanelConfig>(initial.config);
  const [saveState, setSaveState] = useState<SaveState>("saved");
  const versions = useRef<Record<ArtifactKind, number>>(initial.versions);
  const timers = useRef<Partial<Record<ArtifactKind, ReturnType<typeof setTimeout>>>>({});
  const pending = useRef<Partial<Record<ArtifactKind, unknown>>>({});
  const lastActivity = useRef(Math.max(0, ...initial.config.activity.map((a) => Number(a.id))));

  // The tracker, live. EventSource can't set headers, so the panel token rides in
  // the address; when the connection drops it comes back with a fresh token and
  // the last id it saw, and misses nothing.
  useEffect(() => {
    let es: EventSource | null = null;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let closed = false;
    const connect = () => {
      if (closed) return;
      es = new EventSource(api.feedUrl(lastActivity.current));
      es.addEventListener("item", (e) => {
        try {
          const item = toActivity(JSON.parse((e as MessageEvent).data));
          if (!item) return;
          lastActivity.current = Math.max(lastActivity.current, Number(item.id));
          setConfig((c) => ({ ...c, activity: mergeActivity(c.activity, [item]) }));
        } catch {
          /* a line we can't read is skipped */
        }
      });
      es.onerror = () => {
        es?.close();
        retry = setTimeout(connect, 3000);
      };
    };
    connect();
    return () => {
      closed = true;
      es?.close();
      if (retry) clearTimeout(retry);
    };
  }, [api]);

  // Test results, the clock and the session's state change on the server
  // (a check ran, time was added): look every so often. Only those parts are
  // taken, never what the candidate is editing.
  useEffect(() => {
    const id = setInterval(async () => {
      try {
        const fresh = fromApi(await api.config()).config;
        setConfig((c) => ({ ...c, test: fresh.test, status: fresh.status, endsAt: fresh.endsAt, resources: fresh.resources }));
      } catch {
        /* the next look will do */
      }
    }, REFRESH_MS);
    return () => clearInterval(id);
  }, [api]);

  const flush = useCallback(
    async (kind: ArtifactKind) => {
      const body = pending.current[kind];
      if (body === undefined) return;
      pending.current[kind] = undefined;
      setSaveState("saving");
      try {
        versions.current[kind] = await api.save(kind, body, versions.current[kind]);
        setSaveState((s) => (Object.values(pending.current).some((v) => v !== undefined) ? s : "saved"));
      } catch (e) {
        if (e instanceof PanelApiError && e.code === "conflict") {
          versions.current[kind] = Number(e.extra?.current_version ?? versions.current[kind]);
          setSaveState("conflict");
        } else setSaveState("error");
      }
    },
    [api],
  );

  const schedule = useCallback(
    (kind: ArtifactKind, body: unknown) => {
      pending.current[kind] = body;
      setSaveState("saving");
      const t = timers.current[kind];
      if (t) clearTimeout(t);
      timers.current[kind] = setTimeout(() => void flush(kind), SAVE_AFTER_MS);
    },
    [flush],
  );

  // What hasn't been saved yet goes up when the panel is left.
  useEffect(() => {
    const timersAtMount = timers.current;
    const leave = () => {
      for (const kind of Object.keys(pending.current) as ArtifactKind[]) void flush(kind);
    };
    window.addEventListener("pagehide", leave);
    return () => {
      window.removeEventListener("pagehide", leave);
      for (const t of Object.values(timersAtMount)) if (t) clearTimeout(t);
    };
  }, [flush]);

  const saveDiagram = useCallback(
    (diagram: Diagram) => {
      setConfig((c) => ({ ...c, diagram }));
      schedule("diagram", diagram);
    },
    [schedule],
  );
  const saveNotes = useCallback(
    (notes: string) => {
      setConfig((c) => ({ ...c, notes }));
      schedule("notes", { text: notes });
    },
    [schedule],
  );
  const saveReview = useCallback(
    (comments: ReviewComment[]) => {
      setConfig((c) => ({ ...c, review: { ...c.review, comments } }));
      schedule("review", { comments });
    },
    [schedule],
  );
  const saveHypotheses = useCallback(
    (hypotheses: Hypothesis[]) => {
      setConfig((c) => ({ ...c, debug: { ...c.debug, hypotheses } }));
      schedule("hypotheses", { items: hypotheses });
    },
    [schedule],
  );

  const respond = useCallback(
    async (activityId: string, text: string) => {
      try {
        await api.respond(activityId, text);
        setConfig((c) => ({ ...c, activity: c.activity.map((a) => (a.id === activityId ? { ...a, response: text } : a)) }));
        return null;
      } catch (e) {
        return e instanceof Error ? e.message : "Couldn't save that.";
      }
    },
    [api],
  );

  const reload = useCallback(async () => {
    const fresh = fromApi(await api.config());
    versions.current = fresh.versions;
    setConfig((c) => ({ ...fresh.config, activity: mergeActivity(c.activity, fresh.config.activity) }));
    setSaveState("saved");
  }, [api]);

  return useMemo(
    () => ({ config, saveState, saveDiagram, saveNotes, saveReview, saveHypotheses, respond, reload }),
    [config, saveState, saveDiagram, saveNotes, saveReview, saveHypotheses, respond, reload],
  );
}
