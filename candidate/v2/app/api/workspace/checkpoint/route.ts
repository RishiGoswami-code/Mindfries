import { NextResponse } from "next/server";
import { BackendAuthError, backendReady, saveCheckpoint } from "@/lib/backend/client";

/**
 * A copy of the workspace, sent as the tab closes. The workspace sends it
 * with `fetch(..., { keepalive: true })`, which outlives the page — a server
 * action can't. The cookie comes along (same origin), and the backend checks
 * that the session is the candidate's, as it does for every checkpoint.
 *
 * Only small workspaces fit: a browser refuses a keepalive body much over
 * 64KB. Larger ones rely on the checkpoint taken when the tab was hidden,
 * which runs while the page is still alive.
 */
export async function POST(request: Request) {
  if (!backendReady()) return NextResponse.json({ ok: false }, { status: 503 });
  let body: { sessionId?: unknown; files?: unknown };
  try {
    body = await request.json();
  } catch {
    return NextResponse.json({ error: "malformed request body" }, { status: 400 });
  }
  const { sessionId, files } = body;
  if (typeof sessionId !== "string" || !files || typeof files !== "object") {
    return NextResponse.json({ error: "sessionId and files are required" }, { status: 400 });
  }
  try {
    await saveCheckpoint(sessionId, files as Record<string, string>);
    return NextResponse.json({ ok: true });
  } catch (err) {
    return NextResponse.json({ ok: false }, { status: err instanceof BackendAuthError ? 401 : 502 });
  }
}
