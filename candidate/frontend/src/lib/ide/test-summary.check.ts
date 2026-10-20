/**
 * Checks summaryLine in test-results.ts — the line the terminal prints after
 * a test run — in Node:
 *
 *   node --experimental-strip-types src/lib/ide/test-summary.check.ts
 */
import { parseTestRun, summaryLine } from "./test-results.ts";

let failures = 0;
function check(label: string, pass: boolean, detail = "") {
  if (!pass) failures++;
  console.log(`${pass ? "ok  " : "FAIL"}  ${label}${pass || !detail ? "" : `  — ${detail}`}`);
}

const plain = (s: string) => s.replace(/\x1b\[[0-9;]*m/g, "");
const line = (command: string, output: string, exit: number) => plain(summaryLine(parseTestRun(command, output, exit)));

const rule = "-".repeat(70);
let got = line("python -m unittest", `....\n${rule}\nRan 4 tests in 0.001s\n\nOK\n`, 0);
check("a bare unittest OK says how many passed", got === "✔ All 4 tests passed", got);

got = line(
  "python -m unittest -v",
  `test_a (t.T.test_a) ... ok\ntest_b (t.T.test_b) ... FAIL\n\n${"=".repeat(70)}\nFAIL: test_b (t.T.test_b)\n${rule}\nAssertionError: 1 != 2\n\n${rule}\nRan 2 tests in 0.001s\n\nFAILED (failures=1)\n`,
  1,
);
check("a failing run says how many failed, and which", got.startsWith("✘ 1 of 2 tests failed") && got.includes("test_b"), got);

got = line("python -m unittest", "Traceback (most recent call last):\n  ...\nModuleNotFoundError: No module named 'shop'\n", 1);
check("a run that couldn't start says it failed, with the exit code", got === "✘ Tests failed (exit code 1).", got);

got = line("python -m unittest", `\n${rule}\nRan 0 tests in 0.000s\n\nOK\n`, 0);
check("no tests found is said plainly", got === "No tests ran.", got);

console.log(failures === 0 ? "\nAll checks passed." : `\n${failures} check(s) failed.`);
process.exit(failures === 0 ? 0 : 1);
