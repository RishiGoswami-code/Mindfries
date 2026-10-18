import type { Metadata } from "next";
import { AuthSplit } from "@/components/login/AuthSplit";
import { MIN_PASSWORD } from "@/lib/auth/company-signup";
import { SignupForm } from "./SignupForm";

export const metadata: Metadata = { title: "Create an account" };

export const dynamic = "force-dynamic";

const STEPS = ["Create your account", "Mindfries reviews it and adds credits", "Create roles and invite candidates"];

/**
 * A company signing itself up. The account works straight away, but starts
 * with access pending and 0 credits — hiring unlocks when a Mindfries admin
 * grants access (lib/access.ts, internal-admin /admin/access).
 */
export default function SignupPage() {
  return (
    <AuthSplit
      eyebrow="For hiring teams"
      headline="Hire engineers on what they've actually built."
      aside={
        <ol className="space-y-3">
          {STEPS.map((s, i) => (
            <li key={s} className="flex items-center gap-3">
              <span
                className={
                  i === 0
                    ? "grid h-7 w-7 place-items-center rounded-full bg-[#B6A4F8] text-[13px] font-semibold text-[#16143A]"
                    : "grid h-7 w-7 place-items-center rounded-full border border-white/35 text-[13px] font-semibold"
                }
              >
                {i + 1}
              </span>
              {s}
            </li>
          ))}
        </ol>
      }
    >
      <div className="mb-6 text-center">
        <h2 className="text-[28px] font-extrabold tracking-tight text-ink">Create your company account</h2>
        <p className="mt-1.5 text-[14.5px] text-dim">You&apos;ll be the account&apos;s admin.</p>
      </div>
      <SignupForm minPassword={MIN_PASSWORD} />
    </AuthSplit>
  );
}
