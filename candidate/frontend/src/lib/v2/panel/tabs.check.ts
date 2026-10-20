/**
 * Checks tabs.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/v2/panel/tabs.check.ts
 *
 * The modes are the real ones from v2/products/<p>/manifest.json.
 */
import { firstTab, tabsFor } from "./tabs.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}
const ids = (modes: string[], kind?: string) => tabsFor(modes, kind).map((t) => t.id).join(",");

// P1 backend, debugging task.
check("P1 backend", ids(["implementation", "debugging", "testing", "delivery"], "debug") === "notes,resources,debug,test,activity", ids(["implementation", "debugging", "testing", "delivery"], "debug"));
check("P1 backend opens on Debugging", firstTab(tabsFor(["implementation", "debugging", "testing"], "debug"), "debug") === "debug");
// P3 ML researcher: research, debugging, testing.
check("P3 ml_researcher", ids(["research", "debugging", "testing"]) === "notes,resources,debug,test,activity");
// A system-design role.
check("architect gets System Design first", ids(["design", "review"]) === "design,notes,resources,review,activity");
check("architect opens on System Design", firstTab(tabsFor(["design", "review"])) === "design");
// Unknown modes add nothing; the three fixed tabs are always there.
check("unknown modes", ids(["communication", "delivery"]) === "notes,resources,activity");
check("case and spaces", ids([" Testing ", "ARCHITECTURE"]) === "design,notes,resources,test,activity");
check("no modes opens on Notes", firstTab(tabsFor([])) === "notes");

if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
console.log("\nall passed");
