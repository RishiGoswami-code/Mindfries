/**
 * Checks live.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/v2/avatar/live.check.ts
 */
import {
  audioMessage, audioStreamEnd, base64Decode, base64Encode, downsample, floatTo16, int16ToFloat, parseServerMessage, rms, setupMessage,
  TranscriptBuffer, videoMessage,
} from "./live.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  - ${detail}` : ""}`);
}

check("setup names the model", JSON.stringify(setupMessage("gemini-live")) === '{"setup":{"model":"models/gemini-live"}}');
check("setup keeps a models/ prefix", setupMessage("models/x").setup.model === "models/x");
check("video is a jpeg frame", JSON.stringify(videoMessage("QQ==")) === '{"realtimeInput":{"video":{"data":"QQ==","mimeType":"image/jpeg"}}}');
check("the stream can end", JSON.stringify(audioStreamEnd()) === '{"realtimeInput":{"audioStreamEnd":true}}');

// audio round trip
const f = new Float32Array([0, 0.5, -0.5, 1, -1, 2, -2]);
const i16 = floatTo16(f);
check("float to PCM clamps and scales", i16[1] === 16384 && i16[2] === -16384 && i16[3] === 32767 && i16[4] === -32768 && i16[5] === 32767 && i16[6] === -32768);
const back = int16ToFloat(i16);
check("PCM back to float", Math.abs(back[1] - 0.5) < 0.001 && Math.abs(back[4] + 1) < 0.001);
const sent = audioMessage(new Int16Array([1, -2, 300]));
check("audio is sent as 16 kHz PCM", sent.realtimeInput.audio.mimeType === "audio/pcm;rate=16000");
const decoded = new Int16Array(base64Decode(sent.realtimeInput.audio.data).buffer);
check("audio survives base64", decoded.join() === "1,-2,300");
check("base64 handles long input", base64Decode(base64Encode(new Uint8Array(100000).fill(7))).length === 100000);

// what the server sends
{
  const pcm = new Int16Array([10, -10, 20]);
  const b64 = base64Encode(new Uint8Array(pcm.buffer));
  const ev = parseServerMessage({ serverContent: { modelTurn: { parts: [{ inlineData: { mimeType: "audio/pcm;rate=24000", data: b64 } }, { text: "ignored" }] }, outputTranscription: { text: "Hello" }, turnComplete: true } });
  check("audio from the model", ev.audio?.join() === "10,-10,20");
  check("what the avatar said", ev.said === "Hello" && ev.turnComplete === true);
  check("what the candidate said", parseServerMessage({ serverContent: { inputTranscription: { text: "hi there" } } }).heard === "hi there");
  check("an interruption", parseServerMessage({ serverContent: { interrupted: true } }).interrupted === true);
  check("setup complete", parseServerMessage({ setupComplete: {} }).setupComplete === true);
  check("an error is a message", parseServerMessage({ error: { message: "quota" } }).error === "quota");
  check("junk is nothing", Object.keys(parseServerMessage(null)).length === 0 && Object.keys(parseServerMessage({ serverContent: { modelTurn: { parts: [{ inlineData: { mimeType: "image/png", data: "QQ==" } }] } } })).length === 0);
  check("an odd byte count does not crash", parseServerMessage({ serverContent: { modelTurn: { parts: [{ inlineData: { mimeType: "audio/pcm", data: "AAEC" } }] } } }).audio?.length === 1);
}

{
  const x = new Float32Array(48).map((_, i) => (i % 3 === 0 ? 3 : 0));
  const d = downsample(x, 48000, 16000);
  check("48 kHz to 16 kHz keeps a third of the samples", d.length === 16 && Math.abs(d[0] - 1) < 1e-6);
  check("no upsampling", downsample(x, 16000, 48000) === x);
}
check("silence has no level", rms(new Float32Array(100)) === 0 && rms(new Float32Array()) === 0);
check("loud has a level", rms(new Float32Array(100).fill(0.5)) > 0.9 && rms(new Float32Array(100).fill(0.5)) <= 1);

// the transcript
{
  const t = new TranscriptBuffer();
  t.add("avatar", "What made you ", new Date("2026-10-21T10:00:00Z"));
  t.add("avatar", "reach for a cache?");
  t.add("candidate", "Reads dominate.");
  t.add("candidate", " Postgres is the bottleneck.");
  check("a speaker change closes the turn", t.ready().length === 1 && t.ready()[0].text === "What made you reach for a cache?" && t.ready()[0].seq === 1 && t.ready()[0].at === "2026-10-21T10:00:00.000Z");
  const all = t.flush();
  check("flush closes the open turn", all.length === 2 && all[1].role === "candidate" && all[1].seq === 2 && all[1].text === "Reads dominate. Postgres is the bottleneck.");
  t.sent(1);
  check("what was sent is dropped, what was not stays", t.ready().length === 1 && t.ready()[0].seq === 2);
  t.add("avatar", "   ");
  t.close();
  check("blank turns are not kept", t.ready().length === 1);
  t.add("avatar", "And invalidation?");
  check("numbering carries on", t.flush().pop()?.seq === 3);
}

if (failures) {
  console.log(`\n${failures} check(s) failed`);
  process.exit(1);
}
console.log("\nall checks passed");
