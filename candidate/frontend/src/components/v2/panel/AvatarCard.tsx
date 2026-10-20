"use client";

import clsx from "clsx";
import { AudioLines, Ellipsis, Mic, MicOff, Sparkles } from "lucide-react";
import { Avatar2D } from "./Avatar2D";
import type { AvatarController } from "./avatar";

const STATUS: Record<string, string> = {
  idle: "Muted",
  listening: "Listening…",
  thinking: "Thinking…",
  speaking: "Speaking…",
};

/**
 * The AI avatar card: the face, what it's doing, and the controls (mute,
 * talk, more). The same card is the whole panel when it's minimised.
 */
export function AvatarCard({ name, controller: av, className, compact }: { name: string; controller: AvatarController; className?: string; compact?: boolean }) {
  return (
    <section className={clsx("flex flex-col rounded-2xl border border-[#E8EDF7] bg-white p-3", className)} aria-label="AI avatar">
      <div className="mb-2 flex items-center gap-2 px-1 text-sm font-medium text-[#2A3355]">
        <Sparkles className="h-4 w-4 text-[#2F5BD3]" aria-hidden />
        AI Avatar
        <span className="ml-auto text-xs font-normal text-[#6B7393]">{name}</span>
      </div>
      <div className="relative min-h-0 flex-1 overflow-hidden rounded-xl bg-gradient-to-b from-[#DCE8FF] to-[#F3F7FF]">
        <Avatar2D state={av.state} level={av.level} className="h-[calc(100%-8px)] w-full" />
        <div className="absolute inset-x-6 bottom-2 flex items-center justify-center gap-2 rounded-full bg-white/95 py-1.5 text-sm font-medium text-[#2F5BD3] shadow-sm" aria-live="polite">
          <AudioLines className={clsx("h-4 w-4", av.state !== "idle" && "animate-pulse")} aria-hidden />
          {STATUS[av.state]}
        </div>
        {av.note && (
          <p className="absolute inset-x-3 top-2 rounded-lg bg-white/90 px-2 py-1 text-center text-xs text-[#6B7393]" role="status">
            {av.note}
          </p>
        )}
      </div>
      <div className={clsx("flex items-center justify-between px-3", compact ? "pt-2" : "pt-3")}>
        <button
          type="button"
          onClick={av.toggleMute}
          aria-label={av.muted ? "Unmute the avatar's microphone" : "Mute the avatar's microphone"}
          title={av.muted ? "Unmute" : "Mute"}
          className="flex h-11 w-11 items-center justify-center rounded-full bg-[#F1F4FA] text-[#3B4568] hover:bg-[#E6EBF5]"
        >
          {av.muted ? <MicOff className="h-5 w-5" /> : <Mic className="h-5 w-5" />}
        </button>
        <button
          type="button"
          onClick={av.talk}
          disabled={av.muted || av.available === false}
          aria-label="Talk to the avatar"
          title="Talk to the avatar"
          className={clsx(
            "flex items-center justify-center rounded-full bg-[#2F6BF0] text-white shadow-[0_8px_20px_-6px_rgba(47,107,240,0.7)] hover:bg-[#2A5FD8] disabled:opacity-50",
            compact ? "h-12 w-12" : "h-16 w-16",
          )}
        >
          <AudioLines className={compact ? "h-6 w-6" : "h-7 w-7"} />
        </button>
        <button
          type="button"
          aria-label="More"
          title="More"
          disabled
          className="flex h-11 w-11 items-center justify-center rounded-full bg-[#F1F4FA] text-[#3B4568] disabled:opacity-60"
        >
          <Ellipsis className="h-5 w-5" />
        </button>
      </div>
    </section>
  );
}
