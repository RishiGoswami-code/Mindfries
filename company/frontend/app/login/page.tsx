import Link from "next/link";
import { AuthSplit, authLink } from "@/components/login/AuthSplit";
import { LoginForm } from "./LoginForm";

export const dynamic = "force-dynamic";

/**
 * Signs an existing company account in. Accounts come from three places: a
 * company signing itself up (/signup — starts with no credits until
 * Mindfries grants access), Mindfries ops onboarding a company from
 * internal-admin, or a company admin inviting a teammate (both of those
 * finish at /set-password).
 */
export default async function LoginPage({ searchParams }: { searchParams: Promise<{ next?: string }> }) {
  const { next } = await searchParams;
  // Only a path on this site survives; anything else falls back to
  // /dashboard, so a crafted ?next= can't bounce someone off to another
  // origin after login.
  const safeNext = typeof next === "string" && next.startsWith("/") && !next.startsWith("//") ? next : "";

  return (
    <AuthSplit
      eyebrow="Company portal"
      headline="Your shortlist, with the evidence behind it."
      aside="New accounts are reviewed by the Mindfries team before credits are added."
    >
      <div className="mb-6 text-center">
        <h2 className="text-[28px] font-extrabold tracking-tight text-ink">Welcome back</h2>
        <p className="mt-1.5 text-[14.5px] text-dim">Sign in to your company workspace.</p>
      </div>
      <LoginForm next={safeNext} />
      <p className="mt-5 text-center text-[14px] text-dim">
        New to Mindfries?{" "}
        <Link href="/signup" className={authLink}>
          Create a company account
        </Link>
      </p>
      <p className="mt-3 text-center text-[12.5px] leading-relaxed text-faint">
        Invited by a teammate? Use the link in your email to set your password.
      </p>
    </AuthSplit>
  );
}
