"use client";

import { Flag, Maximize2, Minus, Settings } from "lucide-react";
import type { PanelTabId } from "@/lib/v2/panel/types";
import { TAB_ICONS } from "./PanelRail";
import { MindfriesMark } from "./MindfriesMark";
import { Countdown } from "./Countdown";

/**
 * The panel's top bar: the Mindfries mark, the active tab, and on the right
 * the time left, the flags, settings, pop out, and minimise. There's no
 * close: the panel stays for the whole session.
 */
export function PanelHeader({
  activeId,
  activeLabel,
  endsAt,
  unread,
  onFlags,
  onMinimise,
}: {
  activeId: PanelTabId;
  activeLabel: string;
  endsAt: string;
  unread: number;
  onFlags: () => void;
  onMinimise: () => void;
}) {
  const ActiveIcon = TAB_ICONS[activeId];
  return (
    <header className="flex items-center gap-4 px-6 pb-3 pt-4">
      <div className="flex items-center gap-2.5 pr-4">
        <MindfriesMark className="h-8 w-8" />
        <span className="text-[22px] font-semibold tracking-tight text-[#10183A]">Mindfries</span>
      </div>
      <div className="flex items-center gap-2 rounded-xl border-b-2 border-[#2F5BD3] bg-[#EEF3FE] px-4 py-2 text-[15px] font-medium text-[#2F5BD3]">
        <ActiveIcon className="h-[18px] w-[18px]" aria-hidden />
        {activeLabel}
      </div>
      <div className="ml-auto flex items-center gap-2.5">
        <Countdown endsAt={endsAt} />
        <button
          type="button"
          onClick={onFlags}
          className="relative flex h-11 items-center gap-1.5 rounded-xl border border-[#E3E8F2] bg-white px-3 text-sm font-medium text-[#3B4568] hover:bg-[#F6F8FC]"
          aria-label={`${unread} flags and notifications`}
          title="Flags and notifications"
        >
          <Flag className="h-4 w-4" aria-hidden />
          {unread > 0 && <span className="rounded-full bg-[#E5484D] px-1.5 text-xs font-semibold text-white">{unread}</span>}
        </button>
        <IconButton label="Settings" disabled>
          <Settings className="h-5 w-5" aria-hidden />
        </IconButton>
        <IconButton label="Pop out above other windows (coming next)" disabled>
          <Maximize2 className="h-5 w-5" aria-hidden />
        </IconButton>
        <IconButton label="Minimise to the AI avatar" onClick={onMinimise}>
          <Minus className="h-5 w-5" aria-hidden />
        </IconButton>
      </div>
    </header>
  );
}

function IconButton({ label, onClick, disabled, children }: { label: string; onClick?: () => void; disabled?: boolean; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      title={label}
      className="flex h-11 w-11 items-center justify-center rounded-xl border border-[#E3E8F2] bg-white text-[#3B4568] hover:bg-[#F6F8FC] disabled:cursor-not-allowed disabled:opacity-50"
    >
      {children}
    </button>
  );
}
