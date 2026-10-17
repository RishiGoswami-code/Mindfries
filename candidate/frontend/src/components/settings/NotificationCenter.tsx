"use client";

import { useState, useSyncExternalStore } from "react";
import Link from "next/link";
import clsx from "clsx";
import { BellOff, CalendarClock, CheckCheck, ChevronDown, ClipboardList, FileText, Mail, X, type LucideIcon } from "lucide-react";
import { TONES } from "@/lib/notifications/data";
import { dismissNotification, markNotificationsRead, type BellItem } from "@/lib/notifications/actions";
import type { NotificationKind } from "@/lib/notifications/derive";
import { display } from "@/lib/dashboard/fonts";

const FILTERS: Array<{ id: NotificationKind | "all"; label: string }> = [
  { id: "all", label: "All" },
  { id: "assessment", label: "Assessments" },
  { id: "application", label: "Applications" },
  { id: "interview", label: "Interviews" },
  { id: "message", label: "Messages" },
];

const KIND_ICON: Record<NotificationKind, LucideIcon> = {
  assessment: FileText,
  application: ClipboardList,
  interview: CalendarClock,
  message: Mail,
};

const PAGE = 8;

/** "2m ago", "3h ago", "Yesterday", "12 Oct" — worked out in the browser, so it's the candidate's own clock. */
function ago(iso: string, now: number): string {
  const ms = now - Date.parse(iso);
  if (!Number.isFinite(ms)) return "";
  if (ms < 0) return "Upcoming";
  const min = Math.floor(ms / 60_000);
  if (min < 1) return "Just now";
  if (min < 60) return `${min}m ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h ago`;
  if (h < 48) return "Yesterday";
  return new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

const noop = () => () => {};
// One reading of the clock per page load: a snapshot that changed on every
// read would make React render forever.
let loadedAt: number | null = null;
const clientNow = () => (loadedAt ??= Date.now());

/**
 * Every notification, not just the bell's latest: filter by what it's
 * about, mark all read, dismiss one. The same items as the bell — derived
 * from the candidate's records (lib/notifications/derive.ts) — with read and
 * dismissed kept on the server.
 */
export function NotificationCenter({ initial }: { initial: BellItem[] }) {
  const [items, setItems] = useState(initial);
  const [filter, setFilter] = useState<NotificationKind | "all">("all");
  const [shown, setShown] = useState(PAGE);
  // Null on the server and the first client render, so both agree; the times fill in after.
  const now = useSyncExternalStore(noop, clientNow, () => null);

  const count = (id: NotificationKind | "all") => (id === "all" ? items.length : items.filter((n) => n.kind === id).length);
  const list = filter === "all" ? items : items.filter((n) => n.kind === filter);
  const unread = items.filter((n) => !n.read);

  function markAll() {
    const keys = unread.map((n) => n.id);
    setItems((all) => all.map((n) => ({ ...n, read: true })));
    void markNotificationsRead(keys);
  }

  function open(id: string) {
    setItems((all) => all.map((n) => (n.id === id ? { ...n, read: true } : n)));
    void markNotificationsRead([id]);
  }

  function dismiss(id: string) {
    setItems((all) => all.filter((n) => n.id !== id));
    void dismissNotification(id);
  }

  return (
    <section className="min-w-0 rounded-[22px] border border-white/90 bg-white/70 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur">
      <div className="flex flex-wrap items-start justify-between gap-3 px-5 pt-5 sm:px-6 sm:pt-6">
        <div>
          <h2 className={clsx(display.className, "text-[24px] leading-tight font-semibold text-[#100E2E]")}>Notification center</h2>
          <p className="mt-1 text-[14px] text-[#5E5A82]">Invitations, applications, interviews and messages, newest first.</p>
        </div>
        <button
          type="button"
          onClick={markAll}
          disabled={unread.length === 0}
          className="inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-[13.5px] font-medium text-[#5B3BE0] hover:bg-[#F3EFFD] disabled:text-[#A9A5C2] disabled:hover:bg-transparent"
        >
          Mark all as read <CheckCheck size={16} />
        </button>
      </div>

      <nav className="mt-3 flex gap-1 overflow-x-auto overflow-y-hidden border-b border-[#ECE9F6] px-3 [scrollbar-width:none] sm:px-4" aria-label="Filter notifications">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            type="button"
            onClick={() => {
              setFilter(f.id);
              setShown(PAGE);
            }}
            aria-pressed={filter === f.id}
            className={clsx(
              "relative shrink-0 px-3 pt-2 pb-3 text-[14px] whitespace-nowrap transition-colors",
              filter === f.id ? "font-semibold text-[#4B2BB8] after:absolute after:inset-x-2 after:bottom-0 after:h-[3px] after:rounded-full after:bg-[#5B3BE0]" : "text-[#4C4874] hover:text-[#1B1840]"
            )}
          >
            {f.label} ({count(f.id)})
          </button>
        ))}
      </nav>

      {list.length === 0 ? (
        <div className="flex flex-col items-center px-6 py-14 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#F3EFFD] text-[#5B3BE0]">
            <BellOff size={22} />
          </span>
          <p className="mt-3 text-[15px] font-medium text-[#100E2E]">Nothing here</p>
          <p className="mt-1 max-w-[340px] text-[13.5px] text-[#6E6A8F]">
            {filter === "all" ? "When a company invites you, moves your application, schedules an interview or writes to you, it shows here." : "Nothing of this kind right now."}
          </p>
        </div>
      ) : (
        <ul className="px-2 py-2 sm:px-3">
          {list.slice(0, shown).map((n) => {
            const Icon = KIND_ICON[n.kind];
            const tone = TONES[n.tone];
            return (
              <li key={n.id} className={clsx("group relative flex items-start gap-3.5 rounded-2xl px-3 py-3.5", !n.read && "bg-[#F6F3FE]")}>
                <span className={clsx("flex h-11 w-11 shrink-0 items-center justify-center rounded-xl", tone.card)} style={{ color: tone.iconColor }}>
                  <Icon size={20} strokeWidth={1.8} />
                </span>
                <Link href={n.href} onClick={() => open(n.id)} className="min-w-0 flex-1 after:absolute after:inset-0 after:rounded-2xl">
                  <p className="text-[15px] font-semibold text-[#100E2E]">{n.title}</p>
                  <p className="mt-0.5 text-[13.5px] leading-snug text-[#5E5A82]">{n.body}</p>
                </Link>
                <div className="relative flex shrink-0 items-center gap-2 pt-0.5">
                  <span className="text-[12.5px] whitespace-nowrap text-[#6E6A8F]">{now ? ago(n.at, now) : ""}</span>
                  {!n.read && <span className="h-2 w-2 rounded-full bg-[#5B3BE0]" aria-label="Unread" />}
                  <button
                    type="button"
                    onClick={() => dismiss(n.id)}
                    aria-label={`Dismiss: ${n.title}`}
                    className="flex h-7 w-7 items-center justify-center rounded-lg text-[#8A86A8] opacity-100 hover:bg-white hover:text-[#1B1840] sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
                  >
                    <X size={15} />
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {list.length > shown && (
        <div className="border-t border-[#ECE9F6] py-3 text-center">
          <button type="button" onClick={() => setShown((s) => s + PAGE)} className="inline-flex items-center gap-1 text-[14px] font-medium text-[#5B3BE0] hover:underline">
            Load more <ChevronDown size={16} />
          </button>
        </div>
      )}
    </section>
  );
}
