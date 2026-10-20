/** The roles V2 assesses, as a candidate reads them. */
export const ROLE_LABELS: Record<string, string> = {
  backend: "Backend Engineer",
  frontend: "Frontend Engineer",
  fullstack: "Full-Stack SDE",
  fde: "Forward Deployed Engineer",
  quant: "Quantitative Engineer",
  data_analyst: "Data Analyst",
  ml_researcher: "AI/ML Researcher",
  ai_engineer: "AI Engineer",
};

export const roleLabel = (role: string): string => ROLE_LABELS[role] ?? role;
