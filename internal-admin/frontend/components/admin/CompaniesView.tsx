"use client";

import { useState, useTransition } from "react";
import { Building2, CircleCheck, Hourglass } from "lucide-react";
import { MetricCard, MetricGrid, Panel, flat } from "@/components/admin/cards";
import { SampleBadge } from "@/components/admin/SampleBadge";
import { countWithin, cumulative, DAY, WEEK } from "@/lib/overview";
import type { Company, GameTemplate, MemberRole, Plan } from "@/lib/types";
import { Button, Field, Input, Modal, PageHeader, Pill, Select } from "@/components/ui";
import { companyTone, fmtDate, planLabel } from "@/lib/format";
import { companyRoles, inviteCandidate, onboardCompanyAccount, setCompanyStatusAction } from "@/app/admin/actions";
import type { CompanyRole } from "@/lib/db";
import { toast } from "@/components/admin/toast";

/**
 * `asOf` is decided on the server and passed in, so the server render and
 * the browser draw the same graphs; `sample` says the rows are fixtures —
 * same pattern as LibraryView. Real writes go through onboardCompanyAccount /
 * setCompanyStatusAction (app/admin/actions.ts) against the real `companies`
 * table; nothing here holds its own source of truth anymore.
 */
export function CompaniesView({
  initial,
  publishedTemplates,
  asOfIso,
  sample,
}: {
  initial: Company[];
  publishedTemplates: GameTemplate[];
  asOfIso: string;
  sample: boolean;
}) {
  const asOf = new Date(asOfIso);
  const [rows, setRows] = useState<Company[]>(initial);
  const [open, setOpen] = useState(false);
  const [pending, start] = useTransition();

  // onboarding form
  const [name, setName] = useState("");
  const [website, setWebsite] = useState("");
  const [plan, setPlan] = useState<Plan>("starter");
  const [seats, setSeats] = useState(5);
  const [emails, setEmails] = useState("");
  const [defaults, setDefaults] = useState<string[]>([]);
  const [activateNow, setActivateNow] = useState(true);

  function reset() {
    setName("");
    setWebsite("");
    setPlan("starter");
    setSeats(5);
    setEmails("");
    setDefaults([]);
    setActivateNow(true);
  }

  function toggleDefault(id: string) {
    setDefaults((d) => (d.includes(id) ? d.filter((x) => x !== id) : [...d, id]));
  }

  function onboard() {
    const parsed = emails
      .split(/[\n,]/)
      .map((e) => e.trim())
      .filter(Boolean);
    // The id here is a synthetic, display-only React key — createCompany's
    // input (below) doesn't take one, since the stored `team` jsonb column
    // only ever needed email/role; toCompany() synthesizes its own on read.
    const team = parsed.map((email, i) => ({ email, role: (i === 0 ? "admin" : "reviewer") as MemberRole }));
    const input = {
      name: name.trim(),
      website: website.trim(),
      plan,
      status: (activateNow ? "active" : "onboarding") as Company["status"],
      seats,
      team,
      defaultTemplateIds: defaults,
    };
    start(async () => {
      const res = await onboardCompanyAccount(input);
      if (!res.ok) {
        toast.error("Couldn't onboard the company", res.error);
        return;
      }
      toast.success("Company onboarded", input.name);
      // Optimistic — the persisted row (with its real id/createdAt) also
      // arrives on the next server render via revalidatePath.
      const optimisticTeam = team.map((m, i) => ({ ...m, id: String(i) }));
      setRows((r) => [{ ...input, team: optimisticTeam, id: crypto.randomUUID(), createdAt: new Date().toISOString() }, ...r]);
      reset();
      setOpen(false);
    });
  }

  function cyclePause(c: Company) {
    const next = c.status === "paused" ? "active" : c.status === "active" ? "paused" : "active";
    setRows((r) => r.map((x) => (x.id === c.id ? { ...x, status: next } : x)));
    start(async () => {
      const res = await setCompanyStatusAction(c.id, next);
      if (!res.ok) toast.error("Couldn't update the company", res.error);
    });
  }

  // Invite a candidate on a company's behalf. The invitation is for one of
  // the company's roles, or a new one described here; the candidate gets
  // their own task generated from the role's brief, and their own link,
  // which the modal shows once the invitation exists. `inviteFor` is which
  // company's modal is open; null means closed.
  const [inviteFor, setInviteFor] = useState<Company | null>(null);
  const [roles, setRoles] = useState<CompanyRole[] | null>(null);
  const [inviteRoleId, setInviteRoleId] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const [newBrief, setNewBrief] = useState("");
  const [newVariant, setNewVariant] = useState("bug_fix");
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteName, setInviteName] = useState("");
  const [inviteDue, setInviteDue] = useState("");
  const [invited, setInvited] = useState<{ email: string; link: string | null; note: string } | null>(null);

  function openInvite(c: Company) {
    setInviteFor(c);
    setRoles(null);
    setInviteRoleId("");
    setNewTitle("");
    setNewBrief("");
    setNewVariant("bug_fix");
    setInviteEmail("");
    setInviteName("");
    setInviteDue("");
    setInvited(null);
    start(async () => {
      const list = await companyRoles(c.id).catch(() => []);
      setRoles(list);
      setInviteRoleId(list[0]?.id ?? "new");
    });
  }

  function invite() {
    if (!inviteFor) return;
    const company = inviteFor;
    start(async () => {
      const res = await inviteCandidate({
        companyId: company.id,
        roleId: inviteRoleId !== "new" ? inviteRoleId : undefined,
        newRole: inviteRoleId === "new" ? { title: newTitle, assessmentBrief: newBrief, taskVariant: newVariant } : undefined,
        candidateEmail: inviteEmail,
        candidateName: inviteName.trim() || undefined,
        dueDate: inviteDue || undefined,
      }).catch((e: unknown) => ({ ok: false as const, error: e instanceof Error ? e.message : "Couldn't reach the server" }));
      if (!res.ok) {
        toast.error("Couldn't send the invitation", res.error);
        return;
      }
      toast.success("Candidate invited", `${inviteEmail} → ${company.name}`);
      setInvited({ email: inviteEmail, link: res.link, note: res.note });
    });
  }

  const roleReady = inviteRoleId === "new" ? newTitle.trim().length > 0 : !!inviteRoleId;

  const active = rows.filter((c) => c.status === "active").length;
  const onboarding = rows.filter((c) => c.status === "onboarding").length;
  const newThisMonth = countWithin(rows.map((c) => c.createdAt), asOf, 30 * DAY);

  return (
    <>
      <PageHeader
        eyebrow="Company Onboarding"
        title="Companies"
        action={
          <div className="flex items-center gap-3">
            {sample && <SampleBadge asOf={asOf} />}
            <Button onClick={() => setOpen(true)}>+ Onboard company</Button>
          </div>
        }
      />

      <MetricGrid columns={3}>
        <MetricCard
          id="total" label="Total companies" value={rows.length} icon={Building2} tone="violet"
          trend={newThisMonth ? { text: `+${newThisMonth} this month`, direction: "up", good: true } : flat("None new this month")}
          series={cumulative(rows.map((c) => c.createdAt), asOf, 12, WEEK)} seriesLabel="Companies, running total over the last 12 weeks"
        />
        <MetricCard
          id="active" label="Active" value={active} icon={CircleCheck} tone="green"
          trend={flat(`${rows.filter((c) => c.status === "paused").length} paused`)}
          series={cumulative(rows.filter((c) => c.status === "active").map((c) => c.createdAt), asOf, 12, WEEK)} seriesLabel="Active companies, running total over the last 12 weeks"
        />
        <MetricCard id="onboarding" label="Onboarding" value={onboarding} icon={Hourglass} tone="amber" trend={flat(onboarding ? "Setting up now" : "None in progress")} />
      </MetricGrid>

      <Panel title="All companies" count={`${rows.length} total`} subtitle="Every company account, its plan, team and starting games.">
        {rows.length === 0 ? (
          <div className="px-6 py-10 text-center text-sm text-dim">No companies onboarded yet.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-hair bg-[#fafafc] text-left text-[13px] font-semibold text-ink">
                  <th className="px-5 py-3 font-semibold">Company</th>
                  <th className="px-5 py-3 font-semibold">Plan</th>
                  <th className="px-5 py-3 font-semibold">Status</th>
                  <th className="px-5 py-3 font-semibold">Seats</th>
                  <th className="px-5 py-3 font-semibold">Team</th>
                  <th className="px-5 py-3 font-semibold">Defaults</th>
                  <th className="px-5 py-3 font-semibold">Created</th>
                  <th className="px-5 py-3 text-right font-semibold">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-hair">
                {rows.map((c) => (
                  <tr key={c.id} className="hover:bg-black/[0.015]">
                    <td className="px-5 py-4">
                      <div className="font-semibold">{c.name}</div>
                      <div className="text-xs text-dim">{c.website}</div>
                    </td>
                    <td className="px-5 py-4">{planLabel[c.plan]}</td>
                    <td className="px-5 py-4">
                      <Pill tone={companyTone[c.status]}>{c.status}</Pill>
                    </td>
                    <td className="px-5 py-4 mono">{c.seats}</td>
                    <td className="px-5 py-4 mono">{c.team.length}</td>
                    <td className="px-5 py-4 mono">{c.defaultTemplateIds.length}</td>
                    <td className="px-5 py-4 text-dim">{fmtDate(c.createdAt)}</td>
                    <td className="px-5 py-4 text-right">
                      <div className="flex justify-end gap-2">
                        <button
                          onClick={() => openInvite(c)}
                          disabled={pending}
                          className="rounded-lg border border-hair px-3 py-1.5 text-xs font-semibold text-dim hover:border-hair-bright hover:text-ink disabled:opacity-50"
                        >
                          Invite
                        </button>
                        <button
                          onClick={() => cyclePause(c)}
                          disabled={pending}
                          className="rounded-lg border border-hair px-3 py-1.5 text-xs font-semibold text-dim hover:border-hair-bright hover:text-ink disabled:opacity-50"
                        >
                          {c.status === "paused" ? "Activate" : c.status === "active" ? "Pause" : "Activate"}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title="Onboard company"
        footer={
          <>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={onboard} disabled={!name.trim() || pending}>
              {pending ? "Saving…" : activateNow ? "Create & activate" : "Create"}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <Field label="Company name">
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Acme Robotics" />
            </Field>
            <Field label="Website">
              <Input value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="acme.io" />
            </Field>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Field label="Plan / tier">
              <Select value={plan} onChange={(e) => setPlan(e.target.value as Plan)}>
                {(Object.keys(planLabel) as Plan[]).map((p) => (
                  <option key={p} value={p}>
                    {planLabel[p]}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Seats">
              <Input
                type="number"
                min={1}
                value={seats}
                onChange={(e) => setSeats(Math.max(1, Number(e.target.value) || 1))}
              />
            </Field>
          </div>
          <Field label="Team members" hint="One email per line. The first becomes the company admin; the rest are reviewers.">
            <textarea
              value={emails}
              onChange={(e) => setEmails(e.target.value)}
              rows={3}
              placeholder={"admin@acme.io\nreviewer@acme.io"}
              className="w-full rounded-xl border border-hair bg-surface-2 px-3 py-2 text-sm outline-none transition focus:border-accent focus:bg-surface"
            />
          </Field>
          <Field label="Default assessment templates" hint="What this company can pick from on day one.">
            <div className="space-y-2">
              {publishedTemplates.map((t) => (
                <label key={t.id} className="flex cursor-pointer items-center gap-3 rounded-xl border border-hair px-3 py-2 hover:border-hair-bright">
                  <input
                    type="checkbox"
                    checked={defaults.includes(t.id)}
                    onChange={() => toggleDefault(t.id)}
                    className="h-4 w-4 accent-[#7c3aed]"
                  />
                  <span className="text-sm font-medium">{t.name}</span>
                </label>
              ))}
            </div>
          </Field>
          <label className="flex cursor-pointer items-center gap-3">
            <input
              type="checkbox"
              checked={activateNow}
              onChange={(e) => setActivateNow(e.target.checked)}
              className="h-4 w-4 accent-[#7c3aed]"
            />
            <span className="text-sm font-medium">Activate immediately</span>
          </label>
        </div>
      </Modal>

      <Modal
        open={inviteFor !== null}
        onClose={() => setInviteFor(null)}
        title={inviteFor ? `Invite a candidate — ${inviteFor.name}` : "Invite a candidate"}
        footer={
          invited ? (
            <Button onClick={() => setInviteFor(null)}>Done</Button>
          ) : (
            <>
              <Button variant="ghost" onClick={() => setInviteFor(null)}>
                Cancel
              </Button>
              <Button onClick={invite} disabled={!roleReady || !inviteEmail.trim() || pending || roles === null}>
                {pending ? "Sending…" : "Send invitation"}
              </Button>
            </>
          )
        }
      >
        {invited ? (
          <div className="space-y-3">
            <p className="text-sm">
              <span className="font-semibold">{invited.email}</span> is invited. {invited.note}
            </p>
            {invited.link ? (
              <div className="space-y-1.5">
                <div className="text-xs font-semibold text-dim">Their own link — it works only for that email address</div>
                <div className="flex items-center gap-2 rounded-lg border border-hair px-3 py-2">
                  <code className="min-w-0 flex-1 truncate text-xs">{invited.link}</code>
                  <button
                    type="button"
                    className="shrink-0 text-xs font-semibold hover:underline"
                    onClick={() => void navigator.clipboard?.writeText(invited.link as string).then(() => toast.success("Link copied", invited.email))}
                  >
                    Copy
                  </button>
                </div>
              </div>
            ) : (
              <p className="text-xs text-dim">Set CANDIDATE_PORTAL_URL on this portal to get a link to send.</p>
            )}
          </div>
        ) : (
          <div className="space-y-4">
            <Field label="Role">
              <Select value={inviteRoleId} onChange={(e) => setInviteRoleId(e.target.value)} disabled={roles === null}>
                {roles === null && <option value="">Loading the company&apos;s roles…</option>}
                {(roles ?? []).map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.title}
                    {r.assessmentBrief ? "" : " (no brief yet)"}
                  </option>
                ))}
                {roles !== null && <option value="new">+ A new role…</option>}
              </Select>
            </Field>
            {inviteRoleId === "new" && (
              <div className="space-y-3 rounded-lg border border-hair p-3">
                <Field label="Role title">
                  <Input value={newTitle} onChange={(e) => setNewTitle(e.target.value)} placeholder="Backend Engineer" />
                </Field>
                <Field label="What the assessment tests" hint="Each candidate's own task is generated from this">
                  <textarea
                    value={newBrief}
                    onChange={(e) => setNewBrief(e.target.value)}
                    rows={4}
                    maxLength={4000}
                    className="w-full rounded-lg border border-hair px-3 py-2 text-sm"
                    placeholder="Mid-level backend engineer. Python services and REST APIs. Look for debugging discipline and clear tests."
                  />
                </Field>
                <Field label="Kind of task">
                  <Select value={newVariant} onChange={(e) => setNewVariant(e.target.value)}>
                    <option value="bug_fix">Find and fix a bug</option>
                    <option value="debug">Debug a failure</option>
                    <option value="feature">Build a feature</option>
                    <option value="refactor">Refactor</option>
                  </Select>
                </Field>
                <p className="text-xs text-dim">The role is added to the company&apos;s portal, where its team can see it.</p>
              </div>
            )}
            <Field label="Candidate email">
              <Input type="email" value={inviteEmail} onChange={(e) => setInviteEmail(e.target.value)} placeholder="jane@example.com" />
            </Field>
            <div className="grid grid-cols-2 gap-4">
              <Field label="Candidate name" hint="Optional">
                <Input value={inviteName} onChange={(e) => setInviteName(e.target.value)} placeholder="Jane Doe" />
              </Field>
              <Field label="Due date" hint="Optional">
                <Input type="date" value={inviteDue} onChange={(e) => setInviteDue(e.target.value)} />
              </Field>
            </div>
            <p className="text-xs text-dim">
              The candidate gets their own task, written for them from the role&apos;s brief, and their own link. It also shows on their dashboard
              when they sign in with this address.
            </p>
          </div>
        )}
      </Modal>
    </>
  );
}
