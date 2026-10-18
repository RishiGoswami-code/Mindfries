import type { Metadata } from "next";
import { AuthShell } from "@/components/login/AuthShell";
import { SignupForm } from "@/components/login/SignupForm";

export const metadata: Metadata = {
  title: "Create account · Mindfries",
  description: "Create your Mindfries candidate account.",
};

export default async function SignupPage({ searchParams }: { searchParams: Promise<{ next?: string }> }) {
  const { next } = await searchParams;
  // Only a path on this site — the same rule as the sign-in page.
  const safeNext = typeof next === "string" && next.startsWith("/") && !next.startsWith("//") ? next : "";
  return (
    <AuthShell title="Create your account" subtitle="Sign up to take the assessments you're invited to.">
      <SignupForm next={safeNext} />
    </AuthShell>
  );
}
