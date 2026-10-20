import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { currentCandidate } from "@/lib/auth/users";
import { v2Enabled } from "@/lib/v2/flag";
import { mockPanel } from "@/lib/v2/panel/mock";
import { FloatingPanel } from "@/components/v2/panel/FloatingPanel";

export const dynamic = "force-dynamic";

export const metadata: Metadata = { title: "Assessment · Mindfries" };

/**
 * A V2 session's floating panel (v2/PANEL.md). Behind V2_ENABLED: with the
 * switch off this route doesn't exist. Step 1 renders the panel with mock
 * data; later steps load the session from v2/api and pop the panel out
 * above other windows.
 */
export default async function V2SessionPage({ params }: { params: Promise<{ id: string }> }) {
  if (!v2Enabled()) notFound();
  const { id } = await params;
  const me = await currentCandidate();
  if (!me) redirect(`/login?next=${encodeURIComponent(`/v2/session/${id}`)}`);
  const config = mockPanel(id);
  return (
    <div className="flex min-h-screen items-center justify-center bg-[radial-gradient(1200px_600px_at_20%_10%,#DCE7FF_0%,transparent_60%),radial-gradient(900px_500px_at_90%_90%,#E4ECFF_0%,transparent_55%),#F4F7FD] p-6">
      <FloatingPanel config={config} />
    </div>
  );
}
