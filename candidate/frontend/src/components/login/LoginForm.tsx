"use client";

import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";
import Link from "next/link";
import { authInput, authLink, authSubmit } from "./AuthShell";
import { GithubMark, GitlabMark, GoogleMark, LinkedinMark } from "@/components/icons/BrandIcon";
import { signIn, type LoginState } from "@/app/login/actions";
import type { OAuthProviderId } from "@/lib/auth/oauth-providers";

const SOCIAL = [
  { id: "google", label: "Google", icon: GoogleMark },
  { id: "github", label: "GitHub", icon: GithubMark },
  { id: "gitlab", label: "GitLab", icon: GitlabMark },
  { id: "linkedin", label: "LinkedIn", icon: LinkedinMark },
] as const;

const SUPPORT_EMAIL = "officemindfries@gmail.com";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" className={authSubmit} disabled={pending}>
      {pending ? "Signing in…" : "Sign in"}
    </button>
  );
}

/**
 * A real account check now (checkCredentials, against candidate_users) —
 * see lib/auth/users.ts. The four social buttons are real OAuth now too
 * (lib/auth/oauth-*.ts, /api/auth/oauth/[provider]/{start,callback}) — but
 * only *per provider*, gated on that provider's own CLIENT_ID/CLIENT_SECRET.
 * `configuredProviders` is decided server-side (login/page.tsx, an env
 * check) and handed down as data: a configured button is a real link into
 * the OAuth flow, an unconfigured one still says so plainly rather than
 * linking somewhere that would 404.
 */
export function LoginForm({
  next,
  configuredProviders,
  socialError,
  notice,
}: {
  next: string;
  configuredProviders: OAuthProviderId[];
  socialError: string | null;
  /** Shown above the form — e.g. after a password reset. */
  notice?: string | null;
}) {
  const [state, action] = useActionState<LoginState, FormData>(signIn, { error: null });
  const [socialNotice, setSocialNotice] = useState<string | null>(socialError);

  return (
    <div className="w-full">
      {notice && (
        <p role="status" className="mb-5 rounded-2xl border border-[#CDEFE0] bg-[#EAFAF6] px-4 py-3 text-center text-[13.5px] text-[#1E7A5A]">
          {notice}
        </p>
      )}
      <form action={action} className="space-y-4">
        {next && <input type="hidden" name="next" value={next} />}

        <input
          name="email"
          type="email"
          required
          autoComplete="username"
          autoFocus
          placeholder="you@company.com"
          className={authInput}
        />
        <input
          name="password"
          type="password"
          required
          autoComplete="current-password"
          placeholder="Password"
          className={authInput}
        />

        {state.error && (
          <p role="alert" className="text-center text-[13.5px] text-[#a6203c]">
            {state.error}
          </p>
        )}

        <p className="text-center text-[13.5px] text-[#6E6A8F]">
          <Link href="/forgot-password" className={authLink}>
            Forgot your password?
          </Link>
          {" · "}
          <a href={`mailto:${SUPPORT_EMAIL}`} className={authLink}>
            Something else
          </a>
        </p>

        <Submit />

        <p className="text-center text-[13.5px] text-[#5E5A82]">
          Need an account?{" "}
          <Link href="/signup" className={authLink}>
            Sign up
          </Link>
        </p>
      </form>

      <div className="my-6 flex items-center gap-3">
        <span className="h-px flex-1 bg-[#E4E0F2]" />
        <span className="text-[12.5px] font-medium text-[#6E6A8F]">Or continue with</span>
        <span className="h-px flex-1 bg-[#E4E0F2]" />
      </div>

      <div className="grid grid-cols-2 gap-2.5">
        {SOCIAL.map(({ id, label, icon: Icon }) => {
          const buttonClass =
            "flex h-11 items-center justify-center gap-2 rounded-xl border border-[#E4E0F2] bg-white text-[14px] font-medium text-[#1B1840] shadow-[0_2px_6px_-4px_rgba(27,24,64,0.3)] transition-colors hover:bg-[#F6F4FC]";
          if (configuredProviders.includes(id)) {
            const href = `/api/auth/oauth/${id}/start${next ? `?next=${encodeURIComponent(next)}` : ""}`;
            return (
              <a key={id} href={href} className={buttonClass}>
                <Icon size={15} />
                {label}
              </a>
            );
          }
          return (
            <button
              key={id}
              type="button"
              onClick={() => setSocialNotice(`${label} sign-in isn't connected yet — use email above for now.`)}
              className={buttonClass}
            >
              <Icon size={15} />
              {label}
            </button>
          );
        })}
      </div>

      {socialNotice && (
        <p role="status" className="mt-3 text-center text-[13px] text-[#5E5A82]">
          {socialNotice}
        </p>
      )}
    </div>
  );
}
