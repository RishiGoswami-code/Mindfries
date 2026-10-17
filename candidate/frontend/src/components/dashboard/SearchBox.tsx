"use client";

import { useEffect, useRef, useSyncExternalStore } from "react";
import clsx from "clsx";
import { Search } from "lucide-react";

const noSubscription = () => () => {};
const isApple = () => /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);

/**
 * Search over the candidate's own assessments: a plain GET form to
 * /assessments?q=…, so it works without JavaScript. With JavaScript, ⌘K
 * (Ctrl K elsewhere) jumps to it from anywhere on the page, and the hint
 * shows whichever of the two this machine uses.
 *
 * `status` keeps a status filter that's already applied, so searching from
 * the Invited tab searches the invitations.
 */
export function SearchBox({
  defaultValue = "",
  status,
  className,
}: {
  defaultValue?: string;
  status?: string;
  className?: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  // Rendered only in the browser: the server can't know which key to show.
  const hint = useSyncExternalStore(noSubscription, () => (isApple() ? "⌘ K" : "Ctrl K"), () => "");

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        input.current?.focus();
        input.current?.select();
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <form action="/assessments" method="get" role="search" className={clsx("min-w-0", className)}>
      {status && <input type="hidden" name="status" value={status} />}
      <label className="flex h-[46px] items-center gap-4 rounded-2xl border border-white/80 bg-white/55 px-5 shadow-[0_1px_2px_rgba(27,24,64,0.04)] backdrop-blur focus-within:border-[#C9BCF7] focus-within:bg-white/80">
        <Search size={20} strokeWidth={1.9} className="shrink-0 text-[#1B1840]" />
        <span className="sr-only">Search your assessments</span>
        <input
          ref={input}
          type="search"
          name="q"
          defaultValue={defaultValue}
          maxLength={100}
          placeholder="Search assessments…"
          className="min-w-0 flex-1 bg-transparent text-[13px] text-[#1B1840] outline-none placeholder:text-[#8A86A8]"
        />
        {hint && (
          <kbd className="hidden shrink-0 rounded-md border border-[#E4E0F2] bg-white/70 px-1.5 py-0.5 font-sans text-[11px] text-[#8A86A8] sm:inline">
            {hint}
          </kbd>
        )}
      </label>
    </form>
  );
}
