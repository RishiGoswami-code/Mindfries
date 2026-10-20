"use client";

import { Flag, Maximize2 } from "lucide-react";
import type { PanelConfig } from "@/lib/v2/panel/types";
import { AvatarCard } from "./AvatarCard";
import type { AvatarController } from "./avatar";
import { Countdown } from "./Countdown";
import { MindfriesMark } from "./MindfriesMark";

/**
 * The panel minimised: only the AI avatar, with everything else folded into
 * one card: the time left, the flags (tap to open them), whether monitoring
 * is on, and expand.
 */
export function MinimisedCard({
  config,
  avatar,
  unread,
  onExpand,
  onOpenActivity,
}: {
  config: PanelConfig;
  avatar: AvatarController;
  unread: number;
  onExpand: () => void;
  onOpenActivity: () => void;
}) {
  const watched = config.monitoring.camera || config.monitoring.screen || config.monitoring.audio;
  return (
    <div
      className="flex h-[400px] w-[300px] flex-col overflow-hidden rounded-[24px] border border-white/70 bg-white/95 shadow-[0_24px_60px_-18px_rgba(37,73,160,0.4)] ring-1 ring-[#DCE6FA]"
      data-testid="v2-panel"
      data-state="minimised"
    >
      <div className="flex items-center gap-2 px-3 pb-1 pt-3">
        <MindfriesMark className="h-6 w-6" />
        <Countdown endsAt={config.endsAt} compact />
        <button
          type="button"
          onClick={onOpenActivity}
          className="flex h-8 items-center gap-1 rounded-lg border border-[#E3E8F2] px-2 text-xs font-medium text-[#3B4568]"
          aria-label={`${unread} flags and notifications`}
          title="Flags and notifications"
        >
          <Flag className="h-3.5 w-3.5" aria-hidden />
          {unread > 0 && <span className="rounded-full bg-[#E5484D] px-1.5 font-semibold text-white">{unread}</span>}
        </button>
        {watched && (
          <span className="flex items-center gap-1 text-xs text-[#1F7A3F]" title="Monitoring is on">
            <span className="h-2 w-2 animate-pulse rounded-full bg-[#E5484D]" aria-hidden />
            Rec
          </span>
        )}
        <button
          type="button"
          onClick={onExpand}
          className="ml-auto flex h-8 w-8 items-center justify-center rounded-lg border border-[#E3E8F2] text-[#3B4568] hover:bg-[#F6F8FC]"
          aria-label="Expand the panel"
          title="Expand"
        >
          <Maximize2 className="h-4 w-4" />
        </button>
      </div>
      <AvatarCard name={config.avatar.name} controller={avatar} compact className="m-2 mt-1 flex-1 border-0 p-1" />
    </div>
  );
}
