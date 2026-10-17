"use client";

import { useState } from "react";
import clsx from "clsx";
import { BadgeCheck, ChevronRight, ExternalLink, Loader2, Sparkles, Trash2, X } from "lucide-react";
import { PLATFORM_ORDER, PLATFORMS, type LinkPlatform } from "@/lib/profile/links";
import type { StoredLink } from "@/lib/profile/storage";
import { saveLink, removeLink } from "@/lib/profile/actions";
import { usePreviewMode } from "./PreviewMode";

/**
 * GitHub, GitLab, LinkedIn and a portfolio link — the platforms that say
 * something about a candidate beyond what's typed into a form. The Linked
 * accounts window: how many are connected, one row per platform, and why it
 * matters.
 *
 * A GitHub or GitLab username is read from that platform's public API by
 * the server when it is saved (lib/profile/actions.ts → knowledge.ts) — a
 * username with no account behind it is refused, and what's shown afterwards
 * came from the platform itself. That shows the account exists, so the row
 * says "Account found"; it does not show the account is the candidate's, so
 * it does not say "Verified". LinkedIn and a portfolio have no such API, so
 * those two are stored exactly as entered and marked "Linked".
 *
 * "Verified — yours" is a different claim, made only after the candidate
 * signed in to the account (app/api/connect, lib/composio.ts). For LinkedIn
 * that sign-in yields a name, not a profile address, so the row says who
 * signed in rather than calling the typed address verified.
 */
const CONNECT_RESULT: Record<string, string> = {
  verified: "is confirmed as yours.",
  cancelled: "sign-in wasn't completed, so nothing changed.",
  expired: "sign-in took too long or was started in another browser. Try again.",
  failed: "couldn't be confirmed just now. Try again in a moment.",
  not_configured: "sign-in isn't set up on this site yet.",
  unavailable: "sign-in isn't available for this platform yet.",
};

/** What a row shows under "Connected": the handle or address, without the scheme. */
export function linkHandle(id: LinkPlatform, link: StoredLink): string {
  if (id === "linkedin" && link.verified && !link.value) return link.verified.as;
  return (link.value || link.profile?.profileUrl || "").replace(/^https?:\/\/(www\.)?/, "").replace(/\/$/, "");
}

export function LinkedAccounts({
  links,
  verifiable = [],
  connectResult,
}: {
  links: Partial<Record<LinkPlatform, StoredLink>>;
  /** The platforms that can be signed in to, to prove an account is yours (lib/composio.ts). */
  verifiable?: string[];
  /** What a sign-in that just returned did: which platform, and how it ended. */
  connectResult?: { platform: string; result: string };
}) {
  const preview = usePreviewMode();
  const [open, setOpen] = useState<LinkPlatform | null>(null);

  const connected = PLATFORM_ORDER.filter((id) => links[id]).length;
  const visible = preview ? PLATFORM_ORDER.filter((id) => links[id]) : PLATFORM_ORDER;
  const percent = Math.round((connected / PLATFORM_ORDER.length) * 100);

  return (
    <div className="space-y-4">
      {connectResult && PLATFORMS[connectResult.platform as LinkPlatform] && CONNECT_RESULT[connectResult.result] && (
        <p
          role="status"
          className={
            connectResult.result === "verified"
              ? "rounded-xl border border-[#c5ecd5] bg-[#effbf4] px-4 py-2.5 text-[13.5px] text-[#14693a]"
              : "rounded-xl border border-[#f3dca6] bg-[#fff8e8] px-4 py-2.5 text-[13.5px] text-[#7a5211]"
          }
        >
          {PLATFORMS[connectResult.platform as LinkPlatform].label} {CONNECT_RESULT[connectResult.result]}
        </p>
      )}

      {!preview && (
        <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
          <div className="flex items-baseline justify-between gap-3">
            <p className="text-[16px] font-semibold text-[#100E2E]">
              {connected} of {PLATFORM_ORDER.length} connected
            </p>
            <p className="text-[14.5px] text-[#5E5A82]">{percent}% complete</p>
          </div>
          <div className="mt-3 h-2 overflow-hidden rounded-full bg-[#E9E6F6]">
            <div className="h-full rounded-full bg-gradient-to-r from-[#5B3BE0] to-[#7C5CFC]" style={{ width: `${percent}%` }} />
          </div>
        </div>
      )}

      {preview && connected === 0 ? (
        <p className="rounded-[18px] border border-[#ECE9F6] bg-white p-5 text-[14px] text-[#5E5A82]">No accounts linked yet.</p>
      ) : (
        <ul className="divide-y divide-[#ECE9F6] rounded-[18px] border border-[#ECE9F6] bg-white px-5">
          {visible.map((id) => (
            <PlatformRow
              key={id}
              id={id}
              link={links[id]}
              open={open === id}
              preview={preview}
              canVerify={verifiable.includes(id)}
              onToggle={() => setOpen((cur) => (cur === id ? null : id))}
              onSaved={() => setOpen(null)}
            />
          ))}
        </ul>
      )}

      {!preview && (
        <div className="flex gap-4 rounded-[18px] bg-[#F1EDFC] p-5">
          <Sparkles size={24} className="shrink-0 text-[#5B3BE0]" />
          <div>
            <p className="text-[15px] font-semibold text-[#100E2E]">Why connect accounts?</p>
            <p className="mt-1 text-[13.5px] leading-relaxed text-[#5E5A82]">
              A GitHub or GitLab username is looked up when you add it, and the account&apos;s public projects are read for
              your profile. LinkedIn and a portfolio are stored as you enter them — there&apos;s no public way to check those
              two.
              {verifiable.length > 0 &&
                ` Signing in to ${verifiable.map((id) => PLATFORMS[id as LinkPlatform]?.label ?? id).join(", ")} shows hiring teams the account is yours and lets us read it: your projects, their languages and your recent activity; from LinkedIn, your name, headline and picture. Private projects are only counted — their names are never stored or shown. Nothing is ever posted or changed, and removing the account here ends the connection.`}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

function PlatformRow({
  id,
  link,
  open,
  preview,
  canVerify,
  onToggle,
  onSaved,
}: {
  id: LinkPlatform;
  link: StoredLink | undefined;
  open: boolean;
  preview: boolean;
  /** This platform can be signed in to, to show the account is the candidate's. */
  canVerify: boolean;
  onToggle: () => void;
  onSaved: () => void;
}) {
  const platform = PLATFORMS[id];
  const Icon = platform.icon;
  const [value, setValue] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    const result = platform.normalize(value);
    if (!result.ok) return setError(result.error);
    setPending(true);
    setError(null);
    try {
      const res = await saveLink(id, result.value);
      if (res.ok) {
        setValue("");
        onSaved();
      } else setError(res.error || "Couldn't save that.");
    } catch {
      setError("Couldn't save that — try again.");
    } finally {
      setPending(false);
    }
  }

  async function remove() {
    setPending(true);
    setError(null);
    try {
      const res = await removeLink(id);
      if (!res.ok) setError(res.error || "Couldn't remove that.");
    } catch {
      setError("Couldn't reach the server — check your connection and try again.");
    } finally {
      setPending(false);
    }
  }

  const avatar = link?.stats?.avatarUrl ?? link?.profile?.pictureUrl;
  const href = link ? link.stats?.profileUrl ?? link.profile?.profileUrl ?? (link.value ? platform.profileUrl(link.value) : null) : null;

  return (
    <li className="py-4">
      <div className="flex items-center gap-4">
        {avatar ? (
          // eslint-disable-next-line @next/next/no-img-element -- a remote avatar from the platform's own API; not worth an optimisation pass at 44px
          <img src={avatar} alt="" width={44} height={44} className="h-11 w-11 shrink-0 rounded-full" />
        ) : (
          <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-white" style={{ backgroundColor: platform.tint }}>
            <Icon size={20} />
          </span>
        )}

        <div className="min-w-0 flex-1">
          <p className="text-[15.5px] font-medium text-[#100E2E]">{id === "portfolio" ? "Portfolio / Website" : platform.label}</p>
          <p className="text-[13.5px] leading-snug text-[#6E6A8F]">{platform.pitch}</p>
        </div>

        <div className="hidden w-[210px] shrink-0 sm:block">
          <p className="flex items-center gap-2 text-[14px] text-[#2A2654]">
            <span className={clsx("h-2.5 w-2.5 rounded-full", link ? "bg-[#1A9E6B]" : "bg-[#A9A6BF]")} />
            {link ? (link.verified ? "Verified — yours" : "Connected") : "Not connected"}
          </p>
          {link ? (
            <p className="mt-0.5 truncate pl-[18px] text-[13px] text-[#6E6A8F]">{linkHandle(id, link)}</p>
          ) : (
            !preview && (
              <button
                type="button"
                onClick={onToggle}
                className="mt-1.5 ml-[18px] rounded-lg bg-[#EEEBFA] px-4 py-1.5 text-[13.5px] font-semibold text-[#4B2BB8] hover:bg-[#E4DEF9]"
              >
                Connect
              </button>
            )
          )}
        </div>

        {(link || !preview) && (
          <button
            type="button"
            onClick={onToggle}
            aria-expanded={open}
            aria-label={`${open ? "Close" : "Open"} ${platform.label}`}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-[#1B1840] hover:bg-[#F1EEFA]"
          >
            <ChevronRight size={19} className={clsx("transition-transform", open && "rotate-90")} />
          </button>
        )}
      </div>

      {open && (
        <div className="mt-3 rounded-xl bg-[#F8F7FD] p-4 sm:ml-[60px]">
          {link ? (
            <div className="space-y-2.5">
              <p className="flex items-center gap-1.5 text-[13.5px] text-[#2A2654]">
                {link.verified ? (
                  <>
                    <BadgeCheck size={15} className="text-[#1A9E6B]" />
                    {id === "linkedin" ? `Signed in as ${link.verified.as}` : `Verified — signed in as ${link.verified.as}`}
                  </>
                ) : platform.live ? (
                  link.stats ? (
                    <>
                      <BadgeCheck size={15} className="text-[#1A9E6B]" /> Account found — not yet confirmed as yours
                    </>
                  ) : (
                    "Not looked up yet"
                  )
                ) : (
                  "Linked as you entered it"
                )}
              </p>
              {(link.stats?.publicRepos !== undefined || link.stats?.followers !== undefined) && (
                <p className="text-[13px] text-[#6E6A8F]">
                  {[
                    link.stats?.publicRepos !== undefined ? `${link.stats.publicRepos} public repos` : null,
                    link.stats?.followers !== undefined ? `${link.stats.followers} followers` : null,
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </p>
              )}
              {link.profile?.headline && <p className="text-[13px] text-[#6E6A8F]">{link.profile.headline}</p>}
              <div className="flex flex-wrap items-center gap-3 pt-1">
                {href && (
                  <a href={href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-[13.5px] font-medium text-[#4B2BB8] hover:underline">
                    View <ExternalLink size={13} />
                  </a>
                )}
                {!preview && canVerify && !link.verified && (
                  // A plain link: it leaves this site for the platform's own sign-in.
                  <a href={`/api/connect/${id}/start`} className="text-[13.5px] font-medium text-[#4B2BB8] hover:underline">
                    Confirm it&apos;s yours
                  </a>
                )}
                {!preview && (
                  <button
                    type="button"
                    onClick={() => void remove()}
                    disabled={pending}
                    title={link.connectionId ? "Removes the account and ends the connection" : undefined}
                    className="ml-auto inline-flex items-center gap-1 text-[13px] text-[#6E6A8F] hover:text-[#a6203c] disabled:opacity-50"
                  >
                    <Trash2 size={14} /> Remove
                  </button>
                )}
              </div>
            </div>
          ) : (
            <div>
              <div className="flex items-center gap-2">
                <input
                  autoFocus
                  type="text"
                  value={value}
                  onChange={(e) => {
                    setValue(e.target.value);
                    setError(null);
                  }}
                  onKeyDown={(e) => e.key === "Enter" && void submit()}
                  placeholder={platform.placeholder}
                  disabled={pending}
                  aria-label={`${platform.label} ${platform.live ? "username" : "address"}`}
                  className="h-10 min-w-0 flex-1 rounded-lg border border-[#DCD7EE] bg-white px-3 text-[14px] outline-none focus:border-[#7C5CFC]"
                />
                <button
                  type="button"
                  onClick={() => void submit()}
                  disabled={pending || !value.trim()}
                  className="h-10 shrink-0 rounded-lg bg-[#5B3BE0] px-4 text-[13.5px] font-semibold text-white hover:bg-[#4B2BB8] disabled:opacity-50"
                >
                  {pending ? <Loader2 size={16} className="animate-spin" /> : platform.live ? "Look up and save" : "Save"}
                </button>
                <button type="button" onClick={onToggle} aria-label="Cancel" className="flex h-10 w-10 items-center justify-center rounded-lg text-[#6E6A8F] hover:bg-white">
                  <X size={16} />
                </button>
              </div>
              {canVerify && (
                <a href={`/api/connect/${id}/start`} className="mt-3 inline-block text-[13.5px] font-semibold text-[#4B2BB8] hover:underline">
                  Or sign in with {platform.label} to show it&apos;s yours →
                </a>
              )}
            </div>
          )}
          {error && <p className="mt-2 text-[13px] text-[#a6203c]">{error}</p>}
        </div>
      )}

      {/* On a phone the status column is hidden; say it under the name instead. */}
      <p className="mt-2 pl-[60px] text-[13px] text-[#6E6A8F] sm:hidden">
        {link ? `${link.verified ? "Verified" : "Connected"} · ${linkHandle(id, link)}` : "Not connected"}
      </p>
    </li>
  );
}
