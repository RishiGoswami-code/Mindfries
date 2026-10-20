/**
 * Checks monitoring.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/v2/panel/monitoring.check.ts
 */
import { fitWidth, isEntireScreen, liveFlags, missingWords, pickAudioMime, stillsDue, UPLOAD_EVERY_MS } from "./monitoring.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}
const live = { readyState: "live", enabled: true };

check("nothing shared", JSON.stringify(liveFlags({})) === JSON.stringify({ camera: false, audio: false, screen: false }));
check("camera, mic and the entire screen", Object.values(liveFlags({ camera: [live], mic: [live], screen: [{ ...live, displaySurface: "monitor" }] })).every(Boolean));
check("a single window isn't the screen", liveFlags({ screen: [{ ...live, displaySurface: "window" }] }).screen === false);
check("a browser tab isn't the screen", liveFlags({ screen: [{ ...live, displaySurface: "browser" }] }).screen === false);
check("a browser that doesn't say is believed", isEntireScreen(undefined));
check("an ended track is not live", liveFlags({ camera: [{ readyState: "ended", enabled: true }] }).camera === false);
check("a muted (disabled) track is not live", liveFlags({ mic: [{ readyState: "live", enabled: false }] }).audio === false);

check("no stills before the clock runs", stillsDue(1e6, { camera: 0, screen: 0 }, false).length === 0);
check("both are due the first time", stillsDue(UPLOAD_EVERY_MS, { camera: 0, screen: 0 }, true).join() === "camera,screen");
check("only the late one is due", stillsDue(50_000, { camera: 40_000, screen: 10_000 }, true).join() === "screen");
check("not due again at once", stillsDue(50_000, { camera: 49_000, screen: 49_000 }, true).length === 0);

check("fit keeps the shape", JSON.stringify(fitWidth(3200, 1800, 1600)) === JSON.stringify({ w: 1600, h: 900 }));
check("fit leaves small frames alone", JSON.stringify(fitWidth(640, 480, 1600)) === JSON.stringify({ w: 640, h: 480 }));

check("prefers webm/opus", pickAudioMime(() => true) === "audio/webm;codecs=opus");
check("falls back to mp4 (Safari)", pickAudioMime((t) => t === "audio/mp4") === "audio/mp4");
check("nothing supported", pickAudioMime(() => false) === null);

const req = { camera: true, screen: true, audio: true };
check("what's missing, in words", missingWords({ camera: true, audio: false, screen: false }, req).join("|") === "Turn on your microphone|Share your entire screen");
check("an assessment that doesn't need the mic doesn't ask", missingWords({ camera: true, audio: false, screen: true }, { ...req, audio: false }).length === 0);

if (failures) {
  console.log(`\n${failures} check(s) failed`);
  process.exit(1);
}
console.log("\nall checks passed");
