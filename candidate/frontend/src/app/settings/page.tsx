import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import {
  ArrowRight,
  Bell,
  BellRing,
  CircleUserRound,
  Eye,
  EyeOff,
  FolderGit2,
  Globe,
  KeyRound,
  Link2,
  Lock,
  Mail,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { PoliciesLink } from "@/components/dashboard/PoliciesLink";
import { SettingsCard, SettingsRow, StatusPill, buttonClass } from "@/components/settings/Card";
import { NotificationCenter } from "@/components/settings/NotificationCenter";
import { EmailSwitch, VisibilitySwitch } from "@/components/settings/Toggles";
import { PasswordPanel } from "@/components/settings/PasswordPanel";
import { RepositoryPicker, type PickerProject } from "@/components/settings/RepositoryPicker";
import { DeleteAccountRow, DownloadDataRow, SignOutEverywhereRow } from "@/components/settings/AccountActions";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import { listKnowledge } from "@/lib/profile/knowledge";
import { PLATFORM_ORDER, PLATFORMS } from "@/lib/profile/links";
import type { StoredLink } from "@/lib/profile/storage";
import { loadNotifications } from "@/lib/notifications/actions";
import { getAccountSettings, getSignInMethods } from "@/lib/settings/data";
import { ALWAYS_SHARED, EMAIL_KIND_COPY, EMAIL_KINDS, FIELD_COPY, HIDEABLE_FIELDS } from "@/lib/settings/shape";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Settings · Mindfries",
  description: "Your notifications, what hiring teams see, and your account's security.",
};

const TABS: Array<{ id: Tab; label: string; icon: LucideIcon }> = [
  { id: "notifications", label: "Notifications", icon: Bell },
  { id: "privacy", label: "Privacy", icon: ShieldCheck },
  { id: "security", label: "Security", icon: Lock },
];
type Tab = "notifications" | "privacy" | "security";

const PROVIDER_LABEL: Record<string, string> = { google: "Google", github: "GitHub", gitlab: "GitLab", linkedin: "LinkedIn" };

function formatDate(iso: string | null): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d.toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
}

/**
 * Account settings: notifications, privacy and security, one tab each
 * (?tab=, so a tab is a link).
 *
 * Every control here does what it says. The switches are stored on the
 * account (migration 0026) and honoured where they matter — the company
 * portal doesn't send an email kind that's off, and leaves hidden parts out
 * of the profile it shows. There's no two-factor sign-in, no device list
 * and no public profile on Mindfries, so there are no switches for them.
 */
export default async function SettingsPage({ searchParams }: { searchParams: Promise<{ tab?: string }> }) {
  const { tab: wanted } = await searchParams;
  const tab: Tab = TABS.some((t) => t.id === wanted) ? (wanted as Tab) : "notifications";

  const [session, profile] = await Promise.all([currentCandidate(), getProfile()]);
  const [settings, methods, notifications, knowledge] = await Promise.all([
    session ? getAccountSettings(session.id) : null,
    session ? getSignInMethods(session.id) : null,
    tab === "notifications" ? loadNotifications() : Promise.resolve([]),
    session && tab !== "notifications" ? listKnowledge(session.id) : Promise.resolve([]),
  ]);

  const available = !!settings?.available;
  const hidden = new Set(settings?.hiddenFields ?? []);

  return (
    <CandidateShell sessionName={session?.name} profile={profile}>
      <div className="mt-6 grid gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(340px,1fr)]">
        <header className="px-1">
          <h1 className={clsx(display.className, "text-[40px] leading-none font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[48px]")}>Account settings</h1>
          <p className="mt-3 text-[15px] text-[#5E5A82]">Manage your notifications, what hiring teams see, and your account&apos;s security.</p>
        </header>
        <SideBanner tab={tab} />

        <div className="min-w-0 space-y-5">
          <nav className="flex gap-1 overflow-x-auto rounded-[20px] border border-white/90 bg-white/70 px-2 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur [scrollbar-width:none]" aria-label="Settings sections">
            {TABS.map(({ id, label, icon: Icon }) => (
              <Link
                key={id}
                href={id === "notifications" ? "/settings" : `/settings?tab=${id}`}
                aria-current={tab === id ? "page" : undefined}
                className={clsx(
                  "relative flex shrink-0 items-center gap-2.5 px-4 py-4 text-[15px] transition-colors sm:px-5",
                  tab === id ? "font-semibold text-[#4B2BB8] after:absolute after:inset-x-3 after:bottom-0 after:h-[3px] after:rounded-full after:bg-[#5B3BE0]" : "text-[#2A2654] hover:text-[#100E2E]"
                )}
              >
                <Icon size={19} strokeWidth={1.8} />
                {label}
              </Link>
            ))}
          </nav>

          {!available && (
            <p className="rounded-2xl border border-[#F3DDBA] bg-[#FFF8EC] px-4 py-3 text-[13.5px] text-[#7A4B07]">
              Settings aren&apos;t set up on this server yet, so the switches below can&apos;t be saved. Everything else on this page works.
            </p>
          )}

          {tab === "notifications" && <NotificationCenter initial={notifications} />}

          {tab === "privacy" && (
            <>
              <SettingsCard icon={<Globe size={22} />} title="Who can see your profile" subtitle="Only the companies you've applied to, or that invited you to an assessment.">
                <p className="rounded-2xl bg-[#F6F4FC] px-4 py-3 text-[13.5px] leading-relaxed text-[#4C4874]">
                  There&apos;s no public profile and no candidate search on Mindfries: no other company can find you or look you up. What those companies see is below, and
                  you decide most of it.
                </p>
              </SettingsCard>

              <SettingsCard icon={<Eye size={22} />} title="Information visibility" subtitle="Choose what's shown to the hiring teams that can see your profile.">
                <div className="divide-y divide-[#F0EEF7]">
                  {ALWAYS_SHARED.map((s) => (
                    <SettingsRow key={s.label} icon={<Lock size={17} />} label={s.label} detail={s.detail}>
                      <StatusPill tone="gray">Always shared</StatusPill>
                    </SettingsRow>
                  ))}
                  {HIDEABLE_FIELDS.map((f) => (
                    <SettingsRow key={f} icon={hidden.has(f) ? <EyeOff size={17} /> : <Eye size={17} />} label={FIELD_COPY[f].label} detail={FIELD_COPY[f].detail}>
                      <VisibilitySwitch field={f} shown={!hidden.has(f)} label={`Show ${FIELD_COPY[f].label.toLowerCase()} to hiring teams`} disabled={!available} />
                    </SettingsRow>
                  ))}
                </div>
              </SettingsCard>
            </>
          )}

          {tab === "security" && (
            <>
              <SettingsCard icon={<Lock size={22} />} title="Account security" subtitle="Your password, how you sign in, and where you're signed in.">
                <div className="divide-y divide-[#F0EEF7]">
                  {methods ? <PasswordPanel hasPassword={methods.hasPassword} email={methods.email} /> : null}
                  {methods && (
                    <SettingsRow
                      icon={<Mail size={19} />}
                      label="Email"
                      detail={`${methods.email}${methods.createdAt ? ` · account since ${formatDate(methods.createdAt)}` : ""}`}
                    >
                      <StatusPill tone={methods.emailVerified ? "green" : "amber"}>{methods.emailVerified ? "Confirmed" : "Not confirmed"}</StatusPill>
                    </SettingsRow>
                  )}
                  {methods && (
                    <SettingsRow
                      icon={<KeyRound size={19} />}
                      label="Ways to sign in"
                      detail={
                        [methods.hasPassword ? "Email and password" : null, ...methods.providers.map((p) => PROVIDER_LABEL[p.provider] ?? p.provider)].filter(Boolean).join(" · ") ||
                        "None recorded"
                      }
                    />
                  )}
                  <SignOutEverywhereRow />
                </div>
              </SettingsCard>

              <SettingsCard icon={<Link2 size={22} />} title="Connected accounts" subtitle="Accounts that show your work to hiring teams. Connect or remove them on your profile.">
                <div className="divide-y divide-[#F0EEF7]">
                  {PLATFORM_ORDER.map((id) => {
                    const p = PLATFORMS[id];
                    const link = profile?.links?.[id] as StoredLink | null | undefined;
                    const Icon = p.icon;
                    const state = !link ? "none" : link.verified ? "verified" : id === "portfolio" ? "linked" : "unconfirmed";
                    return (
                      <SettingsRow
                        key={id}
                        icon={
                          <span className="flex h-10 w-10 items-center justify-center rounded-xl text-white" style={{ backgroundColor: p.tint }}>
                            <Icon size={19} />
                          </span>
                        }
                        className="[&>span:first-child]:bg-transparent"
                        label={p.label}
                        detail={link ? (link.verified?.as ? `Signed in as ${link.verified.as}` : link.value) : "Not connected"}
                      >
                        {state === "verified" && <StatusPill tone="green">Connected</StatusPill>}
                        {state === "linked" && <StatusPill tone="green">Linked</StatusPill>}
                        {state === "unconfirmed" && <StatusPill tone="amber">Not confirmed</StatusPill>}
                        <Link href="/profile?open=linked" className={buttonClass.quiet}>
                          {link ? "Manage" : "Connect"}
                        </Link>
                      </SettingsRow>
                    );
                  })}
                </div>
              </SettingsCard>

              <RepositoriesCard knowledge={knowledge} hiddenProjects={settings?.hiddenProjects ?? []} codeHidden={hidden.has("code")} available={available} />
            </>
          )}
        </div>

        <aside className="min-w-0 space-y-5">
          {tab === "notifications" && (
            <>
              <SettingsCard icon={<BellRing size={22} />} title="Email notifications" subtitle="What companies may email you. Everything still shows here and in the bell.">
                <div className="divide-y divide-[#F0EEF7]">
                  {EMAIL_KINDS.map((k) => (
                    <SettingsRow key={k} label={EMAIL_KIND_COPY[k].label} detail={EMAIL_KIND_COPY[k].detail}>
                      <EmailSwitch kind={k} initial={settings?.email[k] ?? true} label={`Email me about ${EMAIL_KIND_COPY[k].label.toLowerCase()}`} disabled={!available} />
                    </SettingsRow>
                  ))}
                  <SettingsRow label="Account and security" detail="Confirming your email and resetting your password. These always go.">
                    <StatusPill tone="gray">Always on</StatusPill>
                  </SettingsRow>
                </div>
              </SettingsCard>
              <SummaryCard
                icon={<ShieldCheck size={22} />}
                title="Privacy"
                lines={[`${HIDEABLE_FIELDS.length - hidden.size} of ${HIDEABLE_FIELDS.length} parts of your profile shown to hiring teams`]}
                href="/settings?tab=privacy"
                cta="Privacy settings"
              />
              <SummaryCard
                icon={<Lock size={22} />}
                title="Security"
                lines={[methods ? (methods.hasPassword ? "Password set" : "No password — you sign in with a linked account") : "", methods?.lastLoginAt ? `Last password sign-in ${formatDate(methods.lastLoginAt)}` : ""].filter(Boolean)}
                href="/settings?tab=security"
                cta="Security settings"
              />
            </>
          )}

          {tab === "privacy" && (
            <>
              <SettingsCard icon={<FolderGit2 size={22} />} title="Your projects" subtitle="Choose which projects from GitHub and GitLab hiring teams see.">
                <Link href="/settings?tab=security#repositories" className={buttonClass.quiet}>
                  Choose projects <ArrowRight size={15} />
                </Link>
              </SettingsCard>
              <SettingsCard icon={<CircleUserRound size={22} />} title="How your data is used" subtitle="What happens to what you give us.">
                <ul className="space-y-2.5 text-[13.5px] leading-snug text-[#4C4874]">
                  <li>Your profile is shown only to the companies above, and only the parts you allow.</li>
                  <li>From a linked GitHub or GitLab account we read your public projects, languages and activity counts. Private projects are counted, never named.</li>
                  <li>What&apos;s recorded during an assessment, and who sees it, is set out in the recording policy.</li>
                </ul>
                <PoliciesLink className="mt-3 inline-flex items-center gap-1 text-[13.5px] font-medium text-[#5B3BE0] hover:underline">
                  Privacy &amp; recording policy <ArrowRight size={14} />
                </PoliciesLink>
              </SettingsCard>
              <SettingsCard icon={<Lock size={22} />} title="Download or delete your data" subtitle="A copy of everything, or your account gone for good.">
                <div className="divide-y divide-[#F0EEF7]">
                  <DownloadDataRow />
                </div>
                <DeleteAccountRow />
              </SettingsCard>
            </>
          )}

          {tab === "security" && (
            <>
              <SettingsCard icon={<FolderGit2 size={22} />} title="GitHub and GitLab access" subtitle="What a connected account lets Mindfries do.">
                <div className="rounded-2xl bg-[#F6F4FC] p-4 text-[13.5px] leading-snug text-[#4C4874]">
                  <p className="font-semibold text-[#100E2E]">Read-only, and your code stays yours</p>
                  <p className="mt-1">
                    Connecting signs you in through the platform with read access. We read your profile, your public projects, their languages and your activity counts.
                    Private projects are counted, never named. Nothing in your account is changed. Removing the account on your profile ends the connection.
                  </p>
                </div>
              </SettingsCard>
              <SettingsCard icon={<ShieldCheck size={22} />} title="Data and sessions" subtitle="Your data, and your account itself.">
                <div className="divide-y divide-[#F0EEF7]">
                  <DownloadDataRow />
                </div>
                <DeleteAccountRow />
              </SettingsCard>
            </>
          )}
        </aside>
      </div>
    </CandidateShell>
  );
}

/** The violet note at the top of the right-hand column. */
function SideBanner({ tab }: { tab: Tab }) {
  const copy = {
    notifications: { icon: Bell, title: "Stay on top of your applications", body: "Invitations, interviews, decisions and messages from hiring teams, in one list." },
    privacy: { icon: ShieldCheck, title: "Your profile, your call", body: "Only companies you've applied to or that invited you see your profile — and only the parts you allow." },
    security: { icon: Lock, title: "Keeping your account yours", body: "Passwords are stored only as salted hashes, and you can sign out every device at once." },
  }[tab];
  const Icon = copy.icon;
  return (
    <div className="flex items-start gap-4 rounded-[22px] border border-white/90 bg-gradient-to-br from-[#F1ECFE] to-[#F8F6FE] p-5 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] sm:p-6">
      <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-[#E4DBFD] text-[#5B3BE0]">
        <Icon size={26} strokeWidth={1.8} />
      </span>
      <div>
        <p className={clsx(display.className, "text-[20px] leading-tight font-semibold text-[#100E2E]")}>{copy.title}</p>
        <p className="mt-1 text-[14px] leading-snug text-[#5E5A82]">{copy.body}</p>
      </div>
    </div>
  );
}

function SummaryCard({ icon, title, lines, href, cta }: { icon: React.ReactNode; title: string; lines: string[]; href: string; cta: string }) {
  return (
    <SettingsCard icon={icon} title={title} subtitle={lines.join(" · ")}>
      <Link href={href} className={buttonClass.quiet}>
        {cta} <ArrowRight size={15} />
      </Link>
    </SettingsCard>
  );
}

function RepositoriesCard({
  knowledge,
  hiddenProjects,
  codeHidden,
  available,
}: {
  knowledge: Awaited<ReturnType<typeof listKnowledge>>;
  hiddenProjects: string[];
  codeHidden: boolean;
  available: boolean;
}) {
  const projects: PickerProject[] = knowledge.flatMap((k) =>
    (k.knowledge.projects ?? []).map((p) => ({ url: p.url, name: p.name, description: p.description, language: p.language, stars: p.stars, source: k.source, handle: k.handle }))
  );
  const privateCount = knowledge.reduce((n, k) => n + (k.knowledge.privateProjects ?? 0), 0);

  return (
    <div id="repositories" className="scroll-mt-6">
      <SettingsCard icon={<FolderGit2 size={22} />} title="Your projects" subtitle="Choose which projects from your linked accounts hiring teams see.">
        {knowledge.length === 0 ? (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-[#F6F4FC] px-4 py-3.5">
            <p className="text-[13.5px] text-[#4C4874]">Connect GitHub or GitLab on your profile and your projects show here.</p>
            <Link href="/profile?open=linked" className={buttonClass.quiet}>
              Connect an account
            </Link>
          </div>
        ) : (
          <>
            {codeHidden && (
              <p className="mb-3 rounded-2xl bg-[#FFF8EC] px-4 py-3 text-[13px] text-[#7A4B07]">
                You&apos;ve hidden GitHub and GitLab from hiring teams altogether (Privacy), so none of these show right now.
              </p>
            )}
            <RepositoryPicker projects={projects} hidden={hiddenProjects} privateCount={privateCount} disabled={!available} />
          </>
        )}
      </SettingsCard>
    </div>
  );
}
