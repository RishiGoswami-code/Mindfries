import type { Metadata } from "next";
import { AuthShell } from "@/components/login/AuthShell";
import { LoginForm } from "@/components/login/LoginForm";
import { isConfigured, OAUTH_PROVIDER_ORDER } from "@/lib/auth/oauth-providers";

export const metadata: Metadata = {
  title: "Sign in · Mindfries",
  description: "Sign in to your Mindfries candidate workspace.",
};

export const dynamic = "force-dynamic";

/**
 * The candidate portal's entry point, shaped after the supplied reference —
 * centred form and scattered decorative shapes, inside AuthShell's soft card
 * on the dashboard's lavender canvas. Real now: LoginForm submits to lib/auth/users.ts's
 * checkCredentials against candidate_users.
 *
 * The four social buttons are real OAuth now too (lib/auth/oauth-*.ts) —
 * *per provider*, gated on that provider's own CLIENT_ID/CLIENT_SECRET being
 * set. Which providers are actually configured is an env check, decided here
 * on the server and handed down as data, not guessed client-side — a button
 * for an unconfigured provider still renders, but as the same honest "isn't
 * connected yet" notice as before, never a link that would 404 or bounce
 * through a provider with no app registered.
 */
export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string; social_error?: string; social_message?: string; reset?: string; ended?: string; deleted?: string }>;
}) {
  const { next, social_error, social_message, reset, ended, deleted } = await searchParams;
  // Only a path on this site survives; anything else falls back to /dashboard,
  // so a crafted ?next= can't bounce someone off to another origin after login.
  const safeNext = typeof next === "string" && next.startsWith("/") && !next.startsWith("//") ? next : "";

  const configuredProviders = OAUTH_PROVIDER_ORDER.filter((id) => isConfigured(id));
  const socialError =
    typeof social_error === "string" && social_error
      ? (typeof social_message === "string" && social_message) || humanizeSocialError(social_error)
      : null;

  return (
    <AuthShell title="Welcome back" subtitle="Sign in to your Mindfries workspace.">
      <LoginForm
        next={safeNext}
        configuredProviders={configuredProviders}
        socialError={socialError}
        notice={
          reset === "1"
            ? "Your password has been changed, and every device was signed out. Sign in with the new one."
            : ended === "1"
              ? "You've been signed out on every device. Sign in again to continue."
              : deleted === "1"
                ? "Your account and everything in it has been deleted."
                : null
        }
      />
    </AuthShell>
  );
}

/** A plain-English fallback for the error codes the OAuth callback route redirects with, when it didn't already attach its own ?social_message=. */
function humanizeSocialError(code: string): string {
  if (code.endsWith("_denied")) return "Sign-in was cancelled.";
  if (code.endsWith("_not_configured")) return "That sign-in option isn't set up yet — use email above for now.";
  if (code.endsWith("_state_mismatch") || code.endsWith("_no_code")) return "That sign-in link expired — try again.";
  return "That sign-in attempt didn't work — try again, or use email above.";
}
