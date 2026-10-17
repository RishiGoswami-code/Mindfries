import type { StoredLinks } from "./types";

/**
 * How complete a profile is, section by section — the "Profile completion"
 * card and its checklist. Pure, so the server works it out from the same
 * records each section shows, and the two can't disagree.
 */

export type SectionId = "resume" | "details" | "linked" | "projects" | "evidence" | "preferences";
export type SectionState = "done" | "in-progress" | "todo";

export interface Section {
  id: SectionId;
  title: string;
  description: string;
  state: SectionState;
  /** What to do next, as the card's "Next:" line phrases it. Only for sections not done. */
  next: string;
}

export interface CompletionInput {
  hasResume: boolean;
  /** The candidate has saved their name and headline at least once. */
  detailsSaved: boolean;
  links: StoredLinks;
  /** Linked accounts that have been read into projects and activity. */
  knowledgeCount: number;
  completedAssessments: number;
  assessmentsInProgress: number;
  documents: number;
  /** Null when job preferences aren't set up in this database yet (migration 0024) — left out rather than counted as undone. */
  preferencesSet: boolean | null;
}

export interface Completion {
  sections: Section[];
  done: number;
  total: number;
  percent: number;
}

export function profileCompletion(input: CompletionInput): Completion {
  const { links } = input;
  const codeAccount = !!links.github || !!links.gitlab;
  const anyAccount = codeAccount || !!links.linkedin || !!links.portfolio;

  const sections: Section[] = [
    {
      id: "resume",
      title: "Resume",
      description: "Upload and manage your resume.",
      state: input.hasResume ? "done" : "todo",
      next: "Upload your resume",
    },
    {
      id: "details",
      title: "Profile details",
      description: "Your name, headline, location and what you're open to.",
      state: input.detailsSaved ? "done" : "todo",
      next: "Fill in your profile details",
    },
    {
      id: "linked",
      title: "Linked accounts",
      description: "Connect GitHub, GitLab, LinkedIn and a portfolio.",
      // A code account is what makes this section say something a hiring
      // team can check; LinkedIn or a portfolio alone is a start.
      state: codeAccount ? "done" : anyAccount ? "in-progress" : "todo",
      next: "Connect GitHub or GitLab",
    },
    {
      id: "projects",
      title: "Projects & activity",
      description: "Your projects, languages and recent activity, read from your code accounts.",
      state: input.knowledgeCount > 0 ? "done" : "todo",
      next: "Connect a code account to show your projects",
    },
    {
      id: "evidence",
      title: "Evidence",
      description: "Your assessments, and certificates or reports you add.",
      state:
        input.completedAssessments > 0 || input.documents > 0 ? "done" : input.assessmentsInProgress > 0 ? "in-progress" : "todo",
      next: "Add a certificate or project report",
    },
  ];
  if (input.preferencesSet !== null) {
    sections.push({
      id: "preferences",
      title: "Job preferences",
      description: "The roles, work mode, locations and salary you're looking for.",
      state: input.preferencesSet ? "done" : "todo",
      next: "Set your job preferences",
    });
  }

  const done = sections.filter((s) => s.state === "done").length;
  return { sections, done, total: sections.length, percent: Math.round((done / sections.length) * 100) };
}
