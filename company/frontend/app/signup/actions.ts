"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { createCompanyAccount, MIN_PASSWORD } from "@/lib/auth/company-signup";
import { SESSION_COOKIE, SESSION_MAX_AGE, signSession, sessionSecret } from "@/lib/auth/session";
import { mailerReady, NOTIFY_EMAIL, sendMail } from "@/lib/mailer";

// A server action is a public HTTP endpoint: inputs are capped and checked
// here before anything reaches the database.

const text = (v: unknown, max: number) => (typeof v === "string" ? v.slice(0, max).trim() : "");
const EMAIL_RE = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

export type SignUpState = { error: string | null };

export async function signUp(_prev: SignUpState, form: FormData): Promise<SignUpState> {
  const secret = sessionSecret();
  if (!secret) {
    return { error: "SESSION_SECRET is missing or too short (needs 32+ characters). Add it to .env.local." };
  }

  const companyName = text(form.get("companyName"), 200);
  const website = text(form.get("website"), 300);
  const name = text(form.get("name"), 200);
  const email = text(form.get("email"), 254).toLowerCase();
  const password = typeof form.get("password") === "string" ? String(form.get("password")).slice(0, 512) : "";

  if (!companyName) return { error: "Add your company's name." };
  if (!name) return { error: "Add your name." };
  if (!EMAIL_RE.test(email)) return { error: "That doesn't look like an email address." };
  if (password.length < MIN_PASSWORD) return { error: `Use at least ${MIN_PASSWORD} characters for the password.` };

  const result = await createCompanyAccount({ companyName, website, name, email, password });
  if (!result.ok) return { error: result.error };

  // Tell the Mindfries team there's a company waiting for access. Best
  // effort: the account exists either way, and shows in internal-admin.
  if (mailerReady()) {
    try {
      await sendMail({
        to: NOTIFY_EMAIL,
        subject: `New company sign-up: ${companyName}`,
        text: `${companyName}${website ? ` (${website})` : ""} signed up for the company portal.\n\nAdmin: ${name} <${email}>\n\nThey have 0 credits until access is granted in internal-admin → Access requests.`,
        replyTo: email,
      });
    } catch {
      // The request is still visible in internal-admin.
    }
  }

  const jar = await cookies();
  jar.set(SESSION_COOKIE, await signSession(result.session, secret), {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: SESSION_MAX_AGE,
  });
  redirect("/dashboard");
}
