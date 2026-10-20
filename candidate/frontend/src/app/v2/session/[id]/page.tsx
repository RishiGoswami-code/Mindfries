import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { currentCandidate } from "@/lib/auth/users";
import { engineUrl, panelConfigFor, panelToken, portalReady, V2Error } from "@/lib/v2/api";
import { v2Enabled } from "@/lib/v2/flag";
import { mockPanel } from "@/lib/v2/panel/mock";
import { FloatingPanel } from "@/components/v2/panel/FloatingPanel";
import { LivePanel } from "@/components/v2/panel/LivePanel";

export const dynamic = "force-dynamic";

export const metadata: Metadata = { title: "Assessment · Mindfries" };

const BACKDROP =
  "flex min-h-screen items-center justify-center bg-[radial-gradient(1200px_600px_at_20%_10%,#DCE7FF_0%,transparent_60%),radial-gradient(900px_500px_at_90%_90%,#E4ECFF_0%,transparent_55%),#F4F7FD] p-6";

/**
 * A V2 session's floating panel (v2/PANEL.md): `mindfries start` opens this
 * page. Behind V2_ENABLED. The signed-in candidate must own the session: the
 * V2 API issues the panel's token only then. `/v2/session/demo` is the design
 * review page, on made-up data.
 */
export default async function V2SessionPage({ params }: { params: Promise<{ id: string }> }) {
  if (!v2Enabled()) notFound();
  const { id } = await params;
  const me = await currentCandidate();
  if (!me) redirect(`/login?next=${encodeURIComponent(`/v2/session/${id}`)}`);

  if (id === "demo") {
    return (
      <div className={BACKDROP}>
        <FloatingPanel config={mockPanel(id)} />
      </div>
    );
  }

  const base = engineUrl();
  if (!portalReady() || !base) return <Notice title="Not available yet">The workspace service isn&apos;t set up here yet.</Notice>;

  let opened: { token: string; initial: Awaited<ReturnType<typeof panelConfigFor>> };
  try {
    const t = await panelToken(id, me.id);
    opened = { token: t.token, initial: await panelConfigFor(t.token) };
  } catch (e) {
    if (e instanceof V2Error && e.status === 404) return <Notice title="No such assessment session">Run <code>mindfries start</code> again from your terminal.</Notice>;
    if (e instanceof V2Error && e.status === 403) return <Notice title="That session belongs to someone else">Sign in with the account you started the assessment with.</Notice>;
    return <Notice title="Couldn't open your panel">{e instanceof V2Error ? e.message : "Try reloading in a moment."}</Notice>;
  }
  return (
    <div className={BACKDROP}>
      <LivePanel sessionId={id} apiUrl={base} token={opened.token} initial={opened.initial} />
    </div>
  );
}

function Notice({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className={BACKDROP}>
      <main className="max-w-md rounded-2xl border border-[#E8EDF7] bg-white p-8 text-center shadow-sm">
        <h1 className="text-xl font-semibold text-[#10183A]">{title}</h1>
        <p className="mt-2 text-sm text-[#6B7393]">{children}</p>
      </main>
    </div>
  );
}
