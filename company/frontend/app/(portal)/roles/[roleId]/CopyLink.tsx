"use client";

import { useState } from "react";

/** An invitation's link, with a button to copy it. */
export function CopyLink({ link, compact = false }: { link: string; compact?: boolean }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard refused (an insecure page, a browser setting): the link is
      // on screen to select by hand.
    }
  };
  if (compact) {
    return (
      <button type="button" onClick={copy} title={link} className="text-[12px] font-semibold text-[color:var(--color-accent,#5b4fd6)] hover:underline">
        {copied ? "Copied" : "Copy link"}
      </button>
    );
  }
  return (
    <div className="flex w-full items-center gap-2 rounded-xl border border-hair bg-surface-2/40 px-3 py-2">
      <code className="min-w-0 flex-1 truncate text-[12px]">{link}</code>
      <button type="button" onClick={copy} className="shrink-0 text-[12px] font-semibold hover:underline">
        {copied ? "Copied" : "Copy"}
      </button>
    </div>
  );
}
