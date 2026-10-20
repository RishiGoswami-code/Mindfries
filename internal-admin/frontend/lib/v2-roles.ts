// Shared, client-safe V2 types and constants (no "server-only" here, so client
// components can import them). The server-side engine client is in lib/v2.ts.

export interface V2Product {
  Slug: string;
  Name: string;
  Domain: string;
  GoldenCommit: string;
  Roles: string[];
}

// The eight roles the V2 product set covers, with human labels.
export const V2_ROLES: { value: string; label: string }[] = [
  { value: "backend", label: "Backend Engineer" },
  { value: "frontend", label: "Frontend Engineer" },
  { value: "fullstack", label: "Full-Stack SDE" },
  { value: "fde", label: "Forward Deployed Engineer" },
  { value: "quant", label: "Quantitative Engineer" },
  { value: "data_analyst", label: "Data Analyst" },
  { value: "ml_researcher", label: "AI/ML Researcher" },
  { value: "ai_engineer", label: "AI Engineer" },
];
