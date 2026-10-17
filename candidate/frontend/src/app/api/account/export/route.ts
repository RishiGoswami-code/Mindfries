import { NextResponse } from "next/server";
import { db } from "@/lib/supabase";
import { currentCandidate } from "@/lib/auth/users";
import { listMyApplications } from "@/lib/jobs/data";

/**
 * "Download my data": everything Mindfries holds about the signed-in
 * candidate, as one JSON file. Their account and profile, what was read from
 * the accounts they linked, their documents (listed — the files themselves
 * are on their profile), their applications with interviews and messages,
 * their assessments and sessions, and their settings.
 *
 * Left out on purpose: the password hash and connection ids, which are
 * secrets or internal handles rather than information about the person.
 * Each part that couldn't be read is reported as an error in place, not
 * silently dropped.
 */

/* eslint-disable @typescript-eslint/no-explicit-any */

export async function GET() {
  const session = await currentCandidate();
  if (!session) return NextResponse.json({ error: "Not signed in." }, { status: 401 });
  const c = db();
  if (!c) return NextResponse.json({ error: "The database isn't connected." }, { status: 503 });

  const part = (r: { data: any; error: { message: string } | null }) => (r.error ? { error: r.error.message } : r.data);

  const [account, identities, knowledge, documents, assessments, sessions, notificationState, practice, applications] = await Promise.all([
    c.from("candidate_users").select("*").eq("id", session.id).maybeSingle(),
    c.from("candidate_oauth_identities").select("provider, email_at_link, created_at").eq("candidate_id", session.id),
    c.from("candidate_knowledge").select("source, handle, verified, payload, fetched_at").eq("candidate_id", session.id),
    c.from("candidate_documents").select("kind, title, mime_type, size_bytes, uploaded_at").eq("candidate_id", session.id),
    c.from("assessments").select("id, role, status, due_date, created_at, accepted_at, companies(name)").or(`candidate_id.eq.${session.id},candidate_email.eq."${session.email.toLowerCase().replace(/["\\]/g, "")}"`),
    c.from("sessions").select("id, assessment_id, status, started_at, duration_min, elapsed_min").eq("candidate_id", session.id),
    c.from("candidate_notification_state").select("key, read_at, dismissed_at").eq("candidate_id", session.id),
    c.from("practice_runs").select("*").eq("candidate_id", session.id),
    listMyApplications({ id: session.id, email: session.email }),
  ]);

  let profile: any = part(account);
  if (profile && !profile.error) {
    const { password_hash, failed_attempts, locked_until, links, ...rest } = profile;
    void failed_attempts;
    void locked_until;
    const cleanLinks = Object.fromEntries(
      Object.entries((links ?? {}) as Record<string, any>).map(([platform, link]) => {
        const { connectionId, ...kept } = link ?? {};
        void connectionId;
        return [platform, kept];
      }),
    );
    profile = { ...rest, links: cleanLinks, hasPassword: !!password_hash };
  }

  const body = {
    exportedAt: new Date().toISOString(),
    note: "Everything Mindfries holds about your account. Files (your resume and documents) can be downloaded from your profile.",
    account: profile,
    signInMethods: part(identities),
    linkedAccountReads: part(knowledge),
    documents: part(documents),
    applications: applications.available ? applications.value : { error: "Applications aren't set up on this database." },
    assessments: part(assessments),
    sessions: part(sessions),
    practiceRuns: part(practice),
    notificationState: part(notificationState),
  };

  const day = new Date().toISOString().slice(0, 10);
  return new NextResponse(JSON.stringify(body, null, 2), {
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      "Content-Disposition": `attachment; filename="mindfries-data-${day}.json"`,
      "Cache-Control": "no-store",
    },
  });
}
