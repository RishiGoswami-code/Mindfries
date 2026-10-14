import { currentCompanyUser } from "@/lib/auth/company-users";
import { listApplicationsForCompany, listJobRoles, type CandidateApplicationWithRole } from "@/lib/db";
import { PageHeader } from "@/components/ui";
import { listProfileBadges, type ProfileBadge } from "@/lib/candidate-profile";
import type { JobRole } from "@/lib/types";
import { CandidatesBoard } from "./CandidatesBoard";

export const dynamic = "force-dynamic";

export default async function CandidatesPage() {
  const user = await currentCompanyUser();
  let roles: JobRole[] = [];
  let applications: CandidateApplicationWithRole[] = [];
  let badges: Record<string, ProfileBadge> = {};
  if (user) {
    [roles, applications] = await Promise.all([listJobRoles(user.companyId), listApplicationsForCompany(user.companyId)]);
    // Only this company's invited emails are looked up.
    badges = await listProfileBadges(applications.map((a) => a.candidateEmail));
  }

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Candidates" title="All candidates">
        Every candidate, grouped by role.
      </PageHeader>

      <CandidatesBoard roles={roles} applications={applications} badges={badges} />
    </div>
  );
}
