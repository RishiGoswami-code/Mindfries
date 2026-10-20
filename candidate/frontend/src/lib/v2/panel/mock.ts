import { tabsFor } from "./tabs";
import type { PanelConfig } from "./types";

/**
 * Step 1's stand-in for v2/api (v2/PANEL.md §9): a believable session so
 * the panel can be built and reviewed before the live data exists. Times
 * are relative to now so the timer and the feed look current.
 */
export function mockPanel(sessionId: string, now = new Date()): PanelConfig {
  const ago = (min: number) => new Date(now.getTime() - min * 60_000).toISOString();
  const tabs = tabsFor(["design", "review", "debugging", "testing", "research"], "design");
  const activity: PanelConfig["activity"] = [
    { id: "a1", at: ago(38), severity: "info", source: "server", text: "Assessment started. Your time ends in 90 minutes." },
    { id: "a2", at: ago(37), severity: "info", source: "panel", text: "Camera, microphone and screen sharing are on." },
    { id: "a3", at: ago(31), severity: "info", source: "watcher", text: "Opened the workspace in VS Code." },
    { id: "a4", at: ago(24), severity: "notice", source: "watcher", category: "observed", text: "claude.exe is running. Outside coding agents aren't allowed; noted for the reviewer." },
    { id: "a5", at: ago(19), severity: "info", source: "watcher", text: "Commit 3f2a91c: \"Add cache in front of the user service\"." },
    { id: "a6", at: ago(12), severity: "info", source: "watcher", text: "Tests: 41 passed, 2 failed (internal/support)." },
    { id: "a7", at: ago(6), severity: "flag", source: "panel", category: "signal", text: "Screen sharing stopped for 40 seconds. Recorded as a gap." },
    { id: "a8", at: ago(2), severity: "info", source: "server", text: "Pushed to octocandidate/mindfries-rate-limiter." },
  ];
  const flags = activity.filter((a) => a.severity !== "info").length;
  return {
    sessionId,
    assessmentTitle: "Design a content platform backend",
    role: "Backend engineer",
    product: "P1 Support SaaS",
    endsAt: new Date(now.getTime() + 52 * 60_000).toISOString(),
    tabs: tabs.map((t) => (t.id === "activity" ? { ...t, badge: flags } : t)),
    monitoring: { camera: true, screen: true, audio: true },
    avatar: { name: "Aria", enabled: true },
    diagram: {
      title: "Untitled Diagram",
      nodes: [
        { id: "client", x: 20, y: 250, w: 160, h: 64, title: "Client", subtitle: "Web / Mobile", tone: "blue", icon: "client" },
        { id: "lb", x: 205, y: 250, w: 175, h: 64, title: "Load Balancer", subtitle: "Nginx / ALB", tone: "purple", icon: "balancer" },
        { id: "auth", x: 425, y: 150, w: 185, h: 64, title: "Auth Service", tone: "green", icon: "lock" },
        { id: "user", x: 425, y: 232, w: 185, h: 64, title: "User Service", tone: "sky", icon: "user" },
        { id: "content", x: 425, y: 314, w: 185, h: 64, title: "Content Service", tone: "amber", icon: "file" },
        { id: "db", x: 665, y: 250, w: 160, h: 64, title: "Database", subtitle: "PostgreSQL", tone: "red", icon: "database" },
        { id: "storage", x: 185, y: 470, w: 180, h: 64, title: "Object Storage", subtitle: "S3 / Cloud Storage", tone: "green", icon: "storage" },
        { id: "cache", x: 470, y: 470, w: 145, h: 64, title: "Cache", subtitle: "Redis", tone: "violet", icon: "cache" },
      ],
      edges: [
        { id: "e1", from: "client", to: "lb" },
        { id: "e2", from: "lb", to: "auth" },
        { id: "e3", from: "lb", to: "user" },
        { id: "e4", from: "lb", to: "content" },
        { id: "e5", from: "auth", to: "db" },
        { id: "e6", from: "user", to: "db" },
        { id: "e7", from: "content", to: "db" },
        { id: "e8", from: "content", to: "cache" },
        { id: "e9", from: "cache", to: "storage", dashed: true },
        { id: "e10", from: "storage", to: "content", dashed: true },
      ],
      groups: [{ id: "services", nodes: ["auth", "user", "content"] }],
    },
    notes:
      "## Decisions\n- Cache reads of hot content in Redis (TTL 5 min), invalidate on write.\n- Postgres stays the source of truth; object storage for media only.\n\n## Open questions\n- Rate limit at the load balancer or per service?\n",
    resources: [
      { id: "r1", title: "TASK.md", kind: "file", detail: "The task brief (locked)" },
      { id: "r2", title: "Architecture overview", kind: "doc", detail: "docs/architecture.md in the workspace" },
      { id: "r3", title: "PostgreSQL: indexes", kind: "link", detail: "postgresql.org", href: "https://www.postgresql.org/docs/current/indexes.html" },
      { id: "r4", title: "Redis caching patterns", kind: "link", detail: "redis.io", href: "https://redis.io/docs/latest/develop/use/patterns/" },
    ],
    review: {
      comments: [],
      title: "Review: the cache change (3f2a91c)",
      files: [
        {
          path: "internal/support/users.go",
          lines: [
            { kind: " ", n: 41, text: "func (s *Store) User(id string) (*User, error) {" },
            { kind: "+", n: 42, text: "\tif u, ok := s.cache.Get(id); ok {" },
            { kind: "+", n: 43, text: "\t\treturn u, nil" },
            { kind: "+", n: 44, text: "\t}" },
            { kind: " ", n: 45, text: "\tu, err := s.db.User(id)" },
            { kind: "-", n: 46, text: "\treturn u, err" },
            { kind: "+", n: 46, text: "\tif err == nil {" },
            { kind: "+", n: 47, text: "\t\ts.cache.Set(id, u, 5*time.Minute)" },
            { kind: "+", n: 48, text: "\t}" },
            { kind: "+", n: 49, text: "\treturn u, err" },
            { kind: " ", n: 50, text: "}" },
          ],
        },
      ],
    },
    debug: {
      hypotheses: [],
      title: "Logs: tickets pile onto one agent",
      log: [
        "12:01:03 INFO  routing: assign ticket=T-1042 agent=ana load=3",
        "12:01:04 INFO  routing: assign ticket=T-1043 agent=ana load=4",
        "12:01:04 INFO  routing: assign ticket=T-1044 agent=ana load=5",
        "12:01:05 WARN  routing: agent=ana over soft limit (5 > 4)",
        "12:01:05 INFO  routing: assign ticket=T-1045 agent=ana load=6",
      ],
    },
    test: {
      title: "go test ./internal/support/",
      failing: ["TestAutoAssignBalances", "TestAutoAssignSkipsOffline"],
      passing: ["TestTicketCreate", "TestSLABreach", "TestWebhookRetry", "TestMergeKeepsHistory"],
    },
    activity,
  };
}
