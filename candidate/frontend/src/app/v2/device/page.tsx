import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { currentCandidate } from "@/lib/auth/users";
import { db } from "@/lib/supabase";
import { deviceInfo, getConsent, githubStatus, portalReady, V2Error, type DeviceInfo, type GitHubStatus } from "@/lib/v2/api";
import { v2Enabled } from "@/lib/v2/flag";
import { DeviceConfirm, type Choice } from "@/components/v2/device/DeviceConfirm";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Connect your terminal · Mindfries" };

type Search = { code?: string; github?: string; reason?: string };

/**
 * Where `mindfries start` sends the candidate to sign in (RFC 8628): they check
 * the code matches their terminal, see which computer asked, pick the
 * assessment they were invited to, connect GitHub in one click, and confirm.
 * Or refuse, if it wasn't them.
 */
export default async function DevicePage({ searchParams }: { searchParams: Promise<Search> }) {
  if (!v2Enabled()) notFound();
  const sp = await searchParams;
  const code = (sp.code ?? "").trim().toUpperCase();
  const me = await currentCandidate();
  if (!me) redirect(`/login?next=${encodeURIComponent(`/v2/device${code ? `?code=${code}` : ""}`)}`);

  if (!portalReady()) return <Shell title="Not available yet">The workspace service isn&apos;t set up here yet.</Shell>;
  if (!code) return <Shell title="Enter from your terminal">Run <code>mindfries start</code> and open the link it shows.</Shell>;

  let info: DeviceInfo;
  try {
    info = await deviceInfo(code);
  } catch (e) {
    if (e instanceof V2Error && e.status === 404) return <Shell title="That code isn't valid">Run <code>mindfries start</code> again for a new one.</Shell>;
    return <Shell title="Couldn't check that code">{e instanceof V2Error ? e.message : "Try again in a moment."}</Shell>;
  }
  if (info.status !== "pending") {
    const words: Record<string, string> = {
      expired: "That sign-in has expired. Run mindfries start again.",
      approved: "That terminal is already connected. Go back to it.",
      used: "That terminal is already connected. Go back to it.",
      denied: "You refused that sign-in. Nothing was connected.",
    };
    return <Shell title="Nothing to do here">{words[info.status] ?? "That sign-in is over."}</Shell>;
  }

  // The assessments this candidate can start: theirs, V2, not finished.
  const c = db();
  const { data } = c
    ? await c
        .from("assessments")
        .select("id, role, status, companies(name)")
        .eq("candidate_email", me.email)
        .eq("engine", "v2")
        .in("status", ["invited", "in_progress"])
        .order("created_at", { ascending: false })
    : { data: [] };
  const rows = (data ?? []) as unknown as { id: string; role: string; companies: { name: string } | { name: string }[] | null }[];
  const choices: Choice[] = await Promise.all(
    rows.map(async (r) => {
      const company = Array.isArray(r.companies) ? r.companies[0]?.name : r.companies?.name;
      let accepted = false;
      try {
        accepted = (await getConsent(me.id, r.id)).accepted;
      } catch {
        /* shown as not accepted */
      }
      return { id: r.id, role: r.role, company: company ?? "", accepted };
    }),
  );

  let gh: GitHubStatus | null = null;
  try {
    gh = await githubStatus(me.id);
  } catch {
    gh = null;
  }

  return (
    <DeviceConfirm
      userCode={info.user_code}
      device={info.device}
      choices={choices}
      github={gh}
      githubResult={sp.github === "connected" ? "connected" : sp.github === "failed" ? "failed" : undefined}
      githubReason={sp.reason}
      name={me.name}
    />
  );
}

function Shell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <main className="mx-auto max-w-md px-6 py-24 text-center">
      <h1 className="text-2xl font-extrabold text-[#0A1931]">{title}</h1>
      <p className="mt-3 text-[#4A7FA7]">{children}</p>
    </main>
  );
}
