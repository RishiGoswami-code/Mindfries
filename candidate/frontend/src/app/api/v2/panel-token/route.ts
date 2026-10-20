import { NextResponse } from "next/server";
import { currentCandidate } from "@/lib/auth/users";
import { panelToken, V2Error } from "@/lib/v2/api";
import { v2Enabled } from "@/lib/v2/flag";

export const dynamic = "force-dynamic";

/**
 * A fresh panel token for the signed-in candidate's own session. The panel
 * page asks every ten minutes, before the old one (15 minutes) runs out. The
 * V2 API only issues one when the session belongs to this candidate, so the
 * session id in the request can't be someone else's.
 */
export async function POST(req: Request) {
  if (!v2Enabled()) return NextResponse.json({ error: "Not found" }, { status: 404 });
  const me = await currentCandidate();
  if (!me) return NextResponse.json({ error: "Please sign in again." }, { status: 401 });
  const body = (await req.json().catch(() => ({}))) as { session?: string };
  if (!body.session || !/^ses_[0-9a-f]{8,64}$/.test(body.session)) return NextResponse.json({ error: "Bad session." }, { status: 400 });
  try {
    const t = await panelToken(body.session, me.id);
    return NextResponse.json({ token: t.token, expires_at: t.expires_at }, { headers: { "Cache-Control": "no-store" } });
  } catch (e) {
    const status = e instanceof V2Error && e.status >= 400 && e.status < 600 ? e.status : 502;
    return NextResponse.json({ error: e instanceof V2Error ? e.message : "Couldn't refresh." }, { status });
  }
}
