import type { StoredLinks } from "./types";

export interface NextStep {
  title: string;
  body: string;
}

/**
 * What's left for the candidate to do on their profile, in the order it's
 * worth doing it. Each item disappears the moment its condition is true —
 * a checklist that still asks for something already done stops being
 * trustworthy the first time it's wrong. Shared by the Next steps tile's
 * count (server) and its list (client), so the two can't disagree.
 */
export function nextSteps(hasResume: boolean, links: StoredLinks): NextStep[] {
  return [
    !hasResume && { title: "Upload your resume", body: "Help teams understand your background." },
    !links.github && !links.gitlab && { title: "Connect at least one code account", body: "GitHub or GitLab." },
  ].filter((x): x is NextStep => !!x);
}
