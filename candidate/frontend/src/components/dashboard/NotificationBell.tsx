"use client";

import { Bell, BellOff, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import clsx from "clsx";
import { display, ui } from "@/lib/dashboard/fonts";
import { TONES } from "@/lib/notifications/data";
import { dismissNotification, loadNotifications, markNotificationsRead, type BellItem } from "@/lib/notifications/actions";
import { useDismissablePanel } from "@/lib/useDismissablePanel";

/** Ids already reported as read in this page's lifetime, so opening the panel twice doesn't send them twice. */
const seen = new Set<string>();

function formatWhen(iso: string): string {
  const ms = Date.now() - Date.parse(iso);
  const day = 86_400_000;
  if (ms < day) return "Today";
  if (ms < 2 * day) return "Yesterday";
  return new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

/**
 * The bell in the top bar — a real dropdown of real notifications, styled
 * to the reference design: a pastel card per tone, the icon in its own
 * lighter badge, a bold title over a muted subtitle, a dismiss X.
 *
 * What it shows is worked out from the candidate's own records each time
 * (lib/notifications/derive.ts): an invitation waiting for an answer, a due
 * date coming up, a report that has finished. Read and dismissed are kept
 * on the server, so they follow the candidate to another browser. Opening
 * the panel marks everything read; a dismissed item stays gone.
 *
 * It asks again when the tab comes back into view, so a tab left open
 * overnight isn't showing yesterday's list.
 */
/** `large` is the dashboard's bigger, violet-dotted trigger; the menu is the same either way. */
export function NotificationBell({ appearance = "default" }: { appearance?: "default" | "large" }) {
  const large = appearance === "large";
  const { open, setOpen, ref } = useDismissablePanel<HTMLDivElement>();
  const [items, setItems] = useState<BellItem[]>([]);

  const refresh = useCallback(() => {
    loadNotifications()
      .then(setItems)
      .catch(() => {
        // A failed read leaves the bell as it was; it will ask again.
      });
  }, []);

  useEffect(() => {
    refresh();
    const onVisible = () => document.visibilityState === "visible" && refresh();
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [refresh]);

  const unreadCount = items.filter((n) => !n.read).length;

  // Opening the panel is reading it. The dot clears at once; the cards keep
  // their unread look until the panel is closed, so it's clear which were new.
  useEffect(() => {
    if (!open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- settles the list once the panel that was showing "new" markers has closed
      setItems((list) => (list.some((n) => !n.read && seen.has(n.id)) ? list.map((n) => (seen.has(n.id) ? { ...n, read: true } : n)) : list));
      return;
    }
    const unread = items.filter((n) => !n.read && !seen.has(n.id)).map((n) => n.id);
    if (unread.length === 0) return;
    unread.forEach((id) => seen.add(id));
    void markNotificationsRead(unread);
  }, [open, items]);

  function dismiss(id: string) {
    setItems((list) => list.filter((n) => n.id !== id));
    void dismissNotification(id);
  }

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-label="Notifications"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className={
          large
            ? "relative rounded-xl p-2 text-[#1B1840] transition-colors hover:bg-white/70"
            : "relative rounded-lg p-2 text-[#1B1840] transition-colors hover:bg-[#EEEBFA]"
        }
      >
        <Bell size={large ? 23 : 17} strokeWidth={large ? 1.75 : 2} />
        {unreadCount > 0 && !open && (
          <span
            className={
              large
                ? "absolute top-1.5 right-1.5 h-2 w-2 rounded-full bg-[#8B6CF6] ring-2 ring-[#F3F0FB]"
                : "absolute top-1.5 right-1.5 h-1.5 w-1.5 rounded-full bg-[#8B6CF6] ring-2 ring-white"
            }
          />
        )}
      </button>

      {open && (
        <div
          className={clsx(
            ui.className,
            "absolute top-full right-0 z-40 mt-3 w-[360px] max-w-[calc(100vw-2rem)] overflow-hidden rounded-[22px] border border-white/90 bg-[#FBFAFE]/95 text-[#1B1840] shadow-[0_28px_60px_-24px_rgba(27,24,64,0.5)] backdrop-blur"
          )}
        >
          <div className="flex items-baseline justify-between gap-3 border-b border-[#ECE9F6] px-5 py-4">
            <h2 className={clsx(display.className, "text-[20px] leading-tight font-semibold text-[#100E2E]")}>Notifications</h2>
            {unreadCount > 0 && <span className="text-[12.5px] text-[#6E6A8F]">{unreadCount} new</span>}
          </div>

          {items.length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-6 py-10 text-center">
              <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
                <BellOff size={20} strokeWidth={1.8} />
              </span>
              <p className="text-[14px] text-[#5E5A82]">You&apos;re all caught up.</p>
            </div>
          ) : (
            <ul className="max-h-[70vh] space-y-1 overflow-y-auto p-2">
              {items.map((n) => {
                const tone = TONES[n.tone];
                const Icon = tone.icon;
                return (
                  <li key={n.id} className={clsx("flex items-start gap-3 rounded-2xl px-3 py-3 transition-colors hover:bg-[#F6F4FC]", !n.read && "bg-[#F6F3FE]")}>
                    <span className={clsx("flex h-10 w-10 shrink-0 items-center justify-center rounded-xl", tone.card)} style={{ color: tone.iconColor }}>
                      <Icon size={18} strokeWidth={1.8} />
                    </span>
                    <Link href={n.href} onClick={() => setOpen(false)} className="min-w-0 flex-1">
                      <p className="text-[14px] leading-snug font-semibold text-[#100E2E]">
                        {n.title}
                        {!n.read && <span className="ml-1.5 inline-block h-2 w-2 rounded-full bg-[#5B3BE0] align-middle" aria-label="new" />}
                      </p>
                      <p className="mt-0.5 text-[13px] leading-snug text-[#5E5A82]">{n.body}</p>
                      <p className="mt-1 text-[11.5px] text-[#6E6A8F]">{formatWhen(n.at)}</p>
                    </Link>
                    <button
                      type="button"
                      onClick={() => dismiss(n.id)}
                      aria-label="Dismiss"
                      className="shrink-0 rounded-full p-1.5 text-[#8A86A8] transition-colors hover:bg-[#EEEBFA] hover:text-[#1B1840]"
                    >
                      <X size={14} />
                    </button>
                  </li>
                );
              })}
            </ul>
          )}

          <Link
            href="/settings"
            onClick={() => setOpen(false)}
            className="block border-t border-[#ECE9F6] px-5 py-3 text-center text-[13px] font-medium text-[#5B3BE0] transition-colors hover:bg-[#F6F4FC]"
          >
            See all notifications
          </Link>
        </div>
      )}
    </div>
  );
}
