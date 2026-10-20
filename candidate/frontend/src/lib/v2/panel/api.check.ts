/**
 * Checks api.ts and adapt.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/v2/panel/api.check.ts
 */
import { createPanelApi, mergeActivity, PanelApiError, toActivity } from "./api.ts";
import { fromApi } from "./adapt.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}

type Call = { url: string; method?: string; headers: Record<string, string>; body?: unknown };
function fakeFetch(answer: (c: Call) => { status: number; json?: unknown }) {
  const calls: Call[] = [];
  const impl = (async (url: string, init: RequestInit) => {
    const c: Call = { url, method: init.method, headers: init.headers as Record<string, string>, body: init.body };
    calls.push(c);
    const r = answer(c);
    return { ok: r.status >= 200 && r.status < 300, status: r.status, json: async () => r.json ?? {} } as unknown as Response;
  }) as unknown as typeof fetch;
  return { impl, calls };
}

let token = "tok1";
const sample = {
  sessionId: "ses_1", taskTitle: "Fix routing", role: "backend", product: "P1", status: "active", endsAt: "2026-10-21T12:00:00Z",
  tabs: [{ id: "notes", label: "Notes" }, { id: "poster", label: "Nope" }, { id: "activity", label: "Activity", badge: 2 }],
  monitoring: { camera: true, screen: true, audio: false }, avatar: { name: "Tanu", enabled: true },
  diagram: { title: "D", nodes: [], edges: [] }, notes: "hi", resources: [],
  review: { title: "R", files: null, comments: null }, debug: { title: "L", log: null, hypotheses: null },
  test: { title: "T", failing: null, passing: ["A"] },
  activity: [{ id: 7, at: "2026-10-21T10:00:00Z", severity: "flag", source: "watcher", category: "signal", text: "x" }, { id: 8, at: "t", severity: "weird", source: "?", text: "y" }],
  versions: { diagram: 3 },
};

// adapt: nulls become empty lists, unknown tabs are dropped, versions default to 0.
{
  const { config, versions } = fromApi(sample as never);
  check("unknown tabs are dropped", config.tabs.map((t) => t.id).join() === "notes,activity");
  check("null lists become empty", config.review.files.length === 0 && config.review.comments.length === 0 && config.debug.log.length === 0 && config.debug.hypotheses.length === 0 && config.test.failing.length === 0);
  check("the task's title is the panel's title", config.assessmentTitle === "Fix routing");
  check("versions default to 0", versions.diagram === 3 && versions.notes === 0 && versions.review === 0 && versions.hypotheses === 0);
  check("activity ids are strings, odd values fall back", config.activity[0].id === "7" && config.activity[0].severity === "flag" && config.activity[1].severity === "info" && config.activity[1].source === "server");
}

// the client
{
  const f = fakeFetch((c) => {
    if (c.url.endsWith("/v2/panel/config")) return { status: 200, json: sample };
    if (c.url.includes("/artifacts/notes")) return { status: 409, json: { code: "conflict", detail: "changed elsewhere", current_version: 5 } };
    if (c.url.includes("/artifacts/diagram")) return { status: 200, json: { version: 4 } };
    if (c.url.includes("/response")) return { status: 204 };
    if (c.url.includes("/upload/")) return { status: 201, json: { id: "evi_1" } };
    return { status: 500 };
  });
  const api = createPanelApi({ base: "https://api.test", token: () => token, fetchImpl: f.impl });

  const loaded = fromApi(await api.config());
  check("config is adapted", loaded.config.sessionId === "ses_1" && loaded.versions.diagram === 3);
  check("requests carry the panel token", f.calls[0].headers.Authorization === "Bearer tok1");
  token = "tok2";
  await api.upload("camera", new Blob(["x"], { type: "image/jpeg" }));
  check("a refreshed token is used at once", f.calls[1].headers.Authorization === "Bearer tok2");
  check("an upload keeps its content type", f.calls[1].headers["content-type"] === "image/jpeg" && f.calls[1].url.endsWith("/v2/panel/upload/camera"));
  check("a save returns the new version", (await api.save("diagram", { title: "x" }, 3)) === 4);
  check("a save sends the version it saw", JSON.parse(f.calls[2].body as string).version === 3);
  try {
    await api.save("notes", { text: "x" }, 1);
    check("a conflict throws", false);
  } catch (e) {
    check("a conflict carries the current version", e instanceof PanelApiError && e.code === "conflict" && e.status === 409 && e.extra?.current_version === 5);
  }
  check("an empty 204 answer is fine", (await api.respond("12", "it was a leftover window")) === undefined);
  check("the feed address carries the token and where to resume", api.feedUrl(41) === "https://api.test/v2/panel/feed?token=tok2&after=41" && api.feedUrl(0) === "https://api.test/v2/panel/feed?token=tok2");
}

// a network failure is a plain message, not a crash
{
  const api = createPanelApi({ base: "https://api.test", token: () => "t", fetchImpl: (async () => { throw new Error("offline"); }) as unknown as typeof fetch });
  try {
    await api.status({ camera: true, screen: true, audio: true });
    check("offline throws", false);
  } catch (e) {
    check("offline is said plainly", e instanceof PanelApiError && e.code === "unreachable" && /connection/.test(e.message));
  }
}

// the live feed's items
{
  check("a feed item is understood", toActivity({ id: 3, at: "t", severity: "notice", source: "panel", category: "observed", text: "x" })?.severity === "notice");
  check("a malformed item is dropped", toActivity({ id: "3" }) === null && toActivity(null) === null);
  const a = { id: "1", at: "t", severity: "info" as const, source: "server" as const, text: "a" };
  const b = { id: "2", at: "t", severity: "info" as const, source: "server" as const, text: "b" };
  const held = [a];
  check("the same list comes back when nothing is new", mergeActivity(held, [a]) === held);
  check("new items are added in order, repeats ignored", mergeActivity([b], [a, b]).map((i) => i.id).join() === "1,2");
}

if (failures) {
  console.log(`\n${failures} check(s) failed`);
  process.exit(1);
}
console.log("\nall checks passed");
