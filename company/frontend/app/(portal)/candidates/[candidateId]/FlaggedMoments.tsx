import type { CameraRecord, FlaggedMoment } from "@/lib/types";

const clock = (seconds: number) => `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(Math.floor(seconds % 60)).padStart(2, "0")}`;

/**
 * The moments flagged live during the session (issue #120), each beside the
 * camera, the screen and — for sounds — the audio from that moment, and a
 * sample of the screen's own record. Said at the top, because it decides how
 * everything below should be read: these are prompts to look, not findings.
 */
export function FlaggedMoments({ flags, screen }: { flags: FlaggedMoment[]; screen: CameraRecord }) {
  if (flags.length === 0 && screen.total === 0) return null;
  return (
    <div className="hair-card space-y-4 p-5">
      <div>
        <div className="eyebrow">Flagged moments</div>
        <p className="mt-1 text-xs text-dim">
          Flagged live by the candidate&apos;s browser, with what the camera, screen and microphone had at that moment. These are prompts to
          look, not findings — leaving the workspace can be reading documentation, and voices can be someone thinking aloud. Face checks are the
          least reliable: lighting, and in published studies skin tone, affect them.
        </p>
      </div>

      {flags.length === 0 ? (
        <p className="text-sm text-dim">Nothing was flagged during this session.</p>
      ) : (
        <ol className="space-y-3">
          {flags.map((f, i) => (
            <li key={i} className="rounded-xl border border-hair p-3">
              <div className="flex flex-wrap items-baseline gap-2 text-sm">
                <span className="mono text-dim">{clock(f.offsetSeconds)}</span>
                <span className="font-semibold">{f.label}</span>
                {f.detail && <span className="text-xs text-dim">{f.detail}</span>}
                {f.lowConfidence && <span className="text-xs text-[#a6203c]">low confidence — check the still</span>}
              </div>
              <div className="mt-2 flex flex-wrap items-start gap-3">
                {f.cameraUrl && (
                  // eslint-disable-next-line @next/next/no-img-element -- a signed, short-lived link; nothing for the optimiser to do
                  <img src={f.cameraUrl} alt={`Camera at ${clock(f.offsetSeconds)}`} className="h-24 rounded-lg border border-hair object-cover" />
                )}
                {f.screenUrl && (
                  <a href={f.screenUrl} target="_blank" rel="noreferrer" title="Open the screen at full size">
                    {/* eslint-disable-next-line @next/next/no-img-element -- as above */}
                    <img src={f.screenUrl} alt={`Screen at ${clock(f.offsetSeconds)}`} className="h-24 rounded-lg border border-hair object-cover" />
                  </a>
                )}
                {f.audioUrl && <audio controls preload="none" src={f.audioUrl} className="h-10" />}
                {!f.cameraUrl && !f.screenUrl && !f.audioUrl && <span className="text-xs text-dim">No evidence kept (or deleted after 90 days).</span>}
              </div>
            </li>
          ))}
        </ol>
      )}

      {screen.stills.length > 0 && (
        <div>
          <div className="text-xs font-semibold text-dim">
            The screen through the session — {screen.total} still{screen.total === 1 ? "" : "s"}, kept when it changed
            {screen.expired ? `; ${screen.expired} deleted after 90 days` : ""}
          </div>
          <div className="mt-2 flex gap-2 overflow-x-auto pb-1">
            {screen.stills.map((s, i) => (
              <a key={i} href={s.url} target="_blank" rel="noreferrer" className="shrink-0 text-center">
                {/* eslint-disable-next-line @next/next/no-img-element -- as above */}
                <img src={s.url} alt={`Screen at ${clock(s.offsetSeconds)}`} className="h-20 rounded-md border border-hair object-cover" />
                <span className="mono text-[11px] text-dim">{clock(s.offsetSeconds)}</span>
              </a>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
