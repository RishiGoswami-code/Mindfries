"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { requireCompanyPermission, ForbiddenError } from "@/lib/auth/company-users";
import { createJobRole, TASK_VARIANTS } from "@/lib/db";
import { requireHiringAccess } from "@/lib/access";
import type { TaskVariant } from "@/lib/types";

const text = (v: unknown, max: number) => (typeof v === "string" ? v.slice(0, max).trim() : "");

export type CreateRoleState = { error: string | null };

export async function createRole(_prev: CreateRoleState, form: FormData): Promise<CreateRoleState> {
  let companyId: string;
  try {
    companyId = (await requireCompanyPermission("role:write")).companyId;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in." };
  }
  try {
    await requireHiringAccess(companyId);
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Your account doesn't have access yet." };
  }

  const title = text(form.get("title"), 200);
  if (!title) return { error: "Give the role a title." };

  const techStack = text(form.get("techStack"), 500)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

  const durationRaw = text(form.get("durationMin"), 10);
  const durationMin = durationRaw ? Number(durationRaw) : null;
  if (durationMin !== null && (!Number.isFinite(durationMin) || durationMin <= 0)) {
    return { error: "Duration must be a positive number of minutes." };
  }

  // Assessments are by invitation only; every role is.
  const variant = String(form.get("taskVariant") ?? "");
  const taskVariant: TaskVariant = (TASK_VARIANTS as string[]).includes(variant) ? (variant as TaskVariant) : "bug_fix";
  const assessmentBrief = text(form.get("assessmentBrief"), 4000);

  let roleId: string;
  try {
    roleId = (await createJobRole({ companyId, title, techStack, durationMin, visibility: "invite_only", assessmentBrief, taskVariant })).id;
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't create that role — try again." };
  }

  revalidatePath("/roles");
  redirect(`/roles/${roleId}`);
}
