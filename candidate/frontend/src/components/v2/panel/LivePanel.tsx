"use client";

import { useEffect, useMemo } from "react";
import { createPortal } from "react-dom";
import { createPanelApi, createTokenStore } from "@/lib/v2/panel/api";
import { fromApi, type ApiConfig } from "@/lib/v2/panel/adapt";
import { FloatingPanel, type LiveControls } from "./FloatingPanel";
import { useAvatarLive } from "./useAvatarLive";
import { useMonitoring } from "./useMonitoring";
import { usePanelData } from "./usePanelData";
import { usePictureInPicture } from "./usePictureInPicture";

const REFRESH_TOKEN_EVERY_MS = 10 * 60 * 1000;
const EXPANDED = { width: 1100, height: 700 };
const MINIMISED = { width: 320, height: 450 };

/**
 * A candidate's real panel: the V2 API's data, their camera, screen and
 * microphone shared from this page, the avatar's voice, and a pop-out window
 * that stays above their editor. The token it holds is short-lived and
 * refreshed through the portal's own route; it never sees the portal secret.
 */
export function LivePanel({ sessionId, apiUrl, token, initial }: { sessionId: string; apiUrl: string; token: string; initial: ApiConfig }) {
  // The token changes over time but isn't something to draw: it lives in a tiny store the API client reads from.
  const tokens = useMemo(() => createTokenStore(token), [token]);
  const api = useMemo(() => createPanelApi({ base: apiUrl, token: tokens.get }), [apiUrl, tokens]);

  useEffect(() => {
    const id = setInterval(async () => {
      try {
        const res = await fetch("/api/v2/panel-token", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ session: sessionId }),
        });
        if (res.ok) tokens.set(((await res.json()) as { token: string }).token);
      } catch {
        /* the next try; the token has minutes left */
      }
    }, REFRESH_TOKEN_EVERY_MS);
    return () => clearInterval(id);
  }, [sessionId, tokens]);

  const first = useMemo(() => fromApi(initial), [initial]);
  const data = usePanelData(api, first);
  const monitoring = useMonitoring(api, data.config.monitoring, data.config.status ?? "ready");
  const running = monitoring.sessionStatus === "active";
  const avatar = useAvatarLive(api, { enabled: data.config.avatar.enabled, running, frame: monitoring.frame });
  const pip = usePictureInPicture();

  const live: LiveControls = {
    data,
    monitoring,
    avatar,
    popOut: pip.supported
      ? {
          isOpen: !!pip.window,
          open: () => void pip.open(EXPANDED),
          close: pip.close,
          resize: (minimised) => pip.resize(minimised ? MINIMISED : EXPANDED),
        }
      : undefined,
  };

  if (pip.window) {
    return (
      <>
        <div className="max-w-md rounded-2xl border border-[#E8EDF7] bg-white p-6 text-center shadow-sm">
          <h1 className="text-lg font-semibold text-[#10183A]">Your panel is floating above your other windows</h1>
          <p className="mt-2 text-sm text-[#6B7393]">
            Keep this tab open: your camera, screen and microphone are shared from here. If you close it, the sharing stops.
          </p>
          <button type="button" onClick={pip.close} className="mt-4 rounded-lg bg-[#2F5BD3] px-4 py-2 text-sm font-medium text-white">
            Bring the panel back
          </button>
        </div>
        {createPortal(<FloatingPanel config={data.config} live={live} fill />, pip.window.document.body)}
      </>
    );
  }
  return <FloatingPanel config={data.config} live={live} />;
}
