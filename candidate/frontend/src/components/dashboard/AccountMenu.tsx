"use client";

import { useState, useTransition } from "react";
import Link from "next/link";
import { ChevronDown, LogOut, Settings, ShieldCheck, User } from "lucide-react";
import { useDismissablePanel } from "@/lib/useDismissablePanel";
import { signOut as endSession, signOutEverywhere } from "@/app/login/actions";
import { PoliciesModal } from "./PoliciesModal";
import clsx from "clsx";
import { display, ui } from "@/lib/dashboard/fonts";
import { buttonClass } from "@/components/settings/Card";
import type { CandidateProfile } from "@/lib/profile/types";

/**
 * The avatar, turned into a menu instead of a direct link to /profile — the
 * standard shape (View profile, policies, sign out) once there's more than
 * one place worth going from here.
 *
 * "Sign out" ends the session on this browser: the cookie is cleared. The
 * profile, resume and linked accounts live on the account and stay there,
 * which the confirmation says. "Sign out of all devices" is the other,
 * stronger thing — it withdraws every session the account has, wherever it
 * is signed in (lib/auth/revocation.ts) — for a candidate who left a
 * shared machine signed in or thinks someone else has their session.
 */
interface AccountMenuProps {
  sessionName?: string;
  profile?: CandidateProfile | null;
  /** `large` is the dashboard's trigger: a bigger avatar with a chevron. The menu itself doesn't change. */
  appearance?: "default" | "large";
}

export function AccountMenu({ sessionName, profile, appearance = "default" }: AccountMenuProps) {
  const { open, setOpen, ref } = useDismissablePanel<HTMLDivElement>();
  
  const identity = profile?.profile_updated_at ? {
    name: profile.name,
    role: profile.role,
  } : {
    name: sessionName?.trim() || "Candidate",
    role: "Candidate",
  };
  
  const [confirmingSignOut, setConfirmingSignOut] = useState(false);
  const [showPolicies, setShowPolicies] = useState(false);
  const [pending, startTransition] = useTransition();

  function close() {
    setOpen(false);
    setConfirmingSignOut(false);
  }

  function signOut(everywhere = false) {
    // Clear localStorage profile data just in case it exists from old version
    if (typeof window !== "undefined") {
      window.localStorage.removeItem("mf_profile_identity");
      window.localStorage.removeItem("mf_profile_resume");
      window.localStorage.removeItem("mf_profile_links");
    }
    // endSession() redirects to /login itself once the cookie is cleared —
    // Next resolves that redirect through this transition.
    startTransition(() => {
      void (everywhere ? signOutEverywhere() : endSession());
    });
  }

  return (
    <>
      <div ref={ref} className="relative">
        <button
          type="button"
          aria-label="Your account"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
          className={
            appearance === "large"
              ? "flex items-center gap-3 rounded-full transition-opacity hover:opacity-90"
              : `${display.className} flex h-9 w-9 items-center justify-center rounded-full bg-[#1F1B4D] text-[16px] text-white shadow-[0_4px_12px_-4px_rgba(31,27,77,0.55)] transition-opacity hover:opacity-90`
          }
        >
          {appearance === "large" ? (
            <>
              <span className={`${display.className} flex h-[46px] w-[46px] items-center justify-center rounded-full bg-[#1F1B4D] text-[21px] text-white shadow-[0_6px_16px_-6px_rgba(31,27,77,0.6)]`}>
                {identity.name.trim().charAt(0).toUpperCase() || "?"}
              </span>
              <ChevronDown size={16} strokeWidth={2} className={`text-[#1B1840] transition-transform ${open ? "rotate-180" : ""}`} />
            </>
          ) : (
            identity.name.trim().charAt(0).toUpperCase() || "?"
          )}
        </button>

        {open && (
          <div
            className={clsx(
              ui.className,
              "absolute top-full right-0 z-40 mt-3 w-72 overflow-hidden rounded-[22px] border border-white/90 bg-[#FBFAFE]/95 text-[#1B1840] shadow-[0_28px_60px_-24px_rgba(27,24,64,0.5)] backdrop-blur"
            )}
          >
            <div className="flex items-center gap-3.5 bg-gradient-to-br from-white via-[#F7F5FE] to-[#EFEAFD] px-5 py-4">
              <span className={`${display.className} flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-[#1F1B4D] text-[19px] text-white shadow-[0_6px_16px_-6px_rgba(31,27,77,0.6)]`}>
                {identity.name.trim().charAt(0).toUpperCase() || "?"}
              </span>
              <div className="min-w-0">
                <p className={`${display.className} truncate text-[18px] leading-tight font-semibold text-[#100E2E]`}>{identity.name}</p>
                <p className="truncate text-[12.5px] text-[#6E6A8F]">{identity.role}</p>
              </div>
            </div>

            {confirmingSignOut ? (
              <div className="border-t border-[#ECE9F6] p-4">
                <p className="text-[13px] leading-relaxed text-[#2A2654]">
                  Signs you out on this browser. Your profile, resume and linked accounts stay saved to your account.
                </p>
                <div className="mt-3 flex gap-2">
                  <button
                    type="button"
                    onClick={() => setConfirmingSignOut(false)}
                    disabled={pending}
                    className={clsx(buttonClass.quiet, "flex-1 disabled:opacity-50")}
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={() => signOut()}
                    disabled={pending}
                    className={clsx(buttonClass.danger, "h-9 flex-1 text-[13.5px]")}
                  >
                    {pending ? "Signing out…" : "Sign out"}
                  </button>
                </div>
                <button
                  type="button"
                  onClick={() => signOut(true)}
                  disabled={pending}
                  className="mt-3 w-full text-center text-[12.5px] font-medium text-[#5B3BE0] hover:underline disabled:opacity-50"
                >
                  Sign out of all devices instead
                </button>
              </div>
            ) : (
              <div className="border-t border-[#ECE9F6] p-2">
                <Link
                  href="/profile"
                  onClick={close}
                  className="flex items-center gap-2.5 rounded-xl px-3 py-2.5 text-[14px] font-medium text-[#1B1840] transition-colors hover:bg-[#EEEBFA]"
                >
                  <User size={15} className="text-[#5B3BE0]" />
                  View profile
                </Link>
                <Link
                  href="/settings"
                  onClick={close}
                  className="flex items-center gap-2.5 rounded-xl px-3 py-2.5 text-[14px] font-medium text-[#1B1840] transition-colors hover:bg-[#EEEBFA]"
                >
                  <Settings size={15} className="text-[#5B3BE0]" />
                  Settings
                </Link>
                <button
                  type="button"
                  onClick={() => {
                    setShowPolicies(true);
                    setOpen(false);
                  }}
                  className="flex w-full items-center gap-2.5 rounded-xl px-3 py-2.5 text-left text-[14px] font-medium text-[#1B1840] transition-colors hover:bg-[#EEEBFA]"
                >
                  <ShieldCheck size={15} className="text-[#5B3BE0]" />
                  Privacy & recording policy
                </button>

                <div className="mx-2 my-1.5 border-t border-[#ECE9F6]" />

                <button
                  type="button"
                  onClick={() => setConfirmingSignOut(true)}
                  className="flex w-full items-center gap-2.5 rounded-xl px-3 py-2.5 text-left text-[14px] font-medium text-[#a6203c] transition-colors hover:bg-[#FDEEF1]"
                >
                  <LogOut size={15} />
                  Sign out
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {showPolicies && <PoliciesModal onClose={() => setShowPolicies(false)} />}
    </>
  );
}
