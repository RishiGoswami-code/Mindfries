"use client";

import clsx from "clsx";
import { CalendarClock, Clock, Navigation } from "lucide-react";
import { display } from "@/lib/dashboard/fonts";
import { EditProfileModal } from "./EditProfileModal";
import type { CandidateProfile, SavedIdentity } from "@/lib/profile/types";

function formatUpdated(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
}

/**
 * Name, role, location, bio — everything the old separate "About you" card
 * held, folded in here so there's exactly one place on the page and one
 * button ("Edit profile", via EditProfileModal) that changes any of it,
 * rather than two sections each with their own edit affordance.
 *
 * Sample until the edit form saves something real (resolveIdentity), the
 * same fallback pattern the resume and linked accounts already use.
 */
interface IdentityCardProps {
  sessionName?: string;
  profile: CandidateProfile | null;
}

export function IdentityCard({ sessionName, profile }: IdentityCardProps) {
  // Always use DB data if profile has been updated, otherwise show just the
  // session name with empty fields so new users see their real name, not sample data.
  const hasUpdated = !!profile?.profile_updated_at;
  const identity: SavedIdentity = {
    name: hasUpdated ? (profile?.name || sessionName || "—") : (sessionName || "—"),
    role: hasUpdated ? (profile?.role || "") : "",
    location: hasUpdated ? (profile?.location || "") : "",
    openTo: hasUpdated ? (profile?.open_to || []) : [],
    noticePeriod: hasUpdated ? (profile?.notice_period || "") : "",
    bio: hasUpdated ? (profile?.bio || "") : "",
    updatedAt: hasUpdated ? (profile?.profile_updated_at ?? null) : null,
  };

  return (
    <section className="rounded-[20px] border border-white/90 bg-white/70 p-6 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-6">
          <span
            className={clsx(
              display.className,
              "flex h-[94px] w-[94px] shrink-0 items-center justify-center rounded-full bg-[#1F1B4D] text-[32px] text-white shadow-[0_10px_24px_-10px_rgba(31,27,77,0.6)]"
            )}
          >
            {identity.name.trim().charAt(0).toUpperCase() || "?"}
          </span>
          <div className="min-w-0">
            <h2 className="text-[27px] leading-tight font-semibold tracking-tight text-[#100E2E] [overflow-wrap:anywhere]">{identity.name}</h2>
            {(identity.role || identity.location) && (
              <p className="mt-1 text-[16px] text-[#6E6A8F]">
                {[identity.role, identity.location].filter(Boolean).join("  ·  ")}
              </p>
            )}
            <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[14.5px] text-[#6E6A8F]">
              {identity.openTo.length > 0 && (
                <span className="inline-flex items-center gap-2">
                  <Navigation size={17} strokeWidth={1.8} /> Open to {identity.openTo.join(", ")}
                </span>
              )}
              {identity.noticePeriod && (
                <span className="inline-flex items-center gap-2 sm:border-l sm:border-[#E2DEF0] sm:pl-4">
                  <Clock size={17} strokeWidth={1.8} /> {identity.noticePeriod} notice
                </span>
              )}
              {identity.updatedAt && (
                <span className="inline-flex items-center gap-2 sm:border-l sm:border-[#E2DEF0] sm:pl-4">
                  <CalendarClock size={17} strokeWidth={1.8} /> Last updated {formatUpdated(identity.updatedAt)}
                </span>
              )}
            </div>
          </div>
        </div>

        <EditProfileModal savedIdentity={identity} />
      </div>

      {identity.bio && <p className="mt-5 max-w-3xl text-[16px] leading-relaxed text-[#2A2654]">{identity.bio}</p>}
    </section>
  );
}
