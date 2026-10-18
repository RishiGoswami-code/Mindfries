"use client";

import { useEffect, useRef, useState } from "react";
import { Camera, CheckCircle2, AlertTriangle, ArrowRight, ArrowLeft, RefreshCcw } from "lucide-react";
import clsx from "clsx";
import { buttonClass } from "@/components/settings/Card";
import { display } from "@/lib/dashboard/fonts";

type CheckStatus = "requesting" | "ok" | "denied" | "unavailable";

interface DeviceCheckStepProps {
  onBack: () => void;
  onContinue: () => void;
}

/**
 * Step 2: Device & Camera Check.
 *
 * Grants camera permission early — before the clock starts — so the candidate
 * isn't surprised by a browser prompt inside the workspace. Shows a live
 * preview so they can confirm the camera is actually pointing at them.
 *
 * If permission is already granted (e.g. returning to this step), the preview
 * auto-starts. Errors are shown with plain human language, not browser error codes.
 */
export function DeviceCheckStep({ onBack, onContinue }: DeviceCheckStepProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  // Starts as "requesting": the check begins on mount, so there is no idle
  // moment to show — and the effect below must not set state synchronously.
  const [status, setStatus] = useState<CheckStatus>("requesting");

  // A promise chain rather than async/await so every setStatus sits in a
  // callback: the mount effect starts this, and must not set state itself.
  // Starting inside .then also turns a missing navigator.mediaDevices (an
  // insecure page) into a rejection, the same as the old try/catch did.
  const requestCamera = () =>
    Promise.resolve()
      .then(() => navigator.mediaDevices.getUserMedia({ video: true, audio: false }))
      .then(
        (stream) => {
          streamRef.current = stream;
          if (videoRef.current) {
            videoRef.current.srcObject = stream;
          }
          setStatus("ok");
        },
        (err: unknown) => {
          if (err instanceof DOMException && (err.name === "NotFoundError" || err.name === "DevicesNotFoundError")) {
            setStatus("unavailable");
          } else {
            setStatus("denied");
          }
        },
      );

  const retryCamera = () => {
    setStatus("requesting");
    void requestCamera();
  };

  // Auto-start check on mount
  useEffect(() => {
    void requestCamera();
    return () => {
      // Clean up stream when leaving this step
      streamRef.current?.getTracks().forEach((t) => t.stop());
    };
  }, []);

  const isOk = status === "ok";

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <p className="flex items-center gap-4 text-[13px] font-medium tracking-[0.28em] text-[#6E6A8F] uppercase">
          Step 2 — Device check <span className="h-px w-9 bg-[#C9BCF7]" />
        </p>
        <h1 className={clsx(display.className, "mt-3 text-[34px] leading-[1.05] font-semibold tracking-tight text-[#100E2E] sm:text-[40px]")}>Camera check</h1>
        <p className="mt-2.5 text-[15.5px] leading-relaxed text-[#5E5A82]">
          The workspace requires your camera throughout the session. Let&apos;s make sure it&apos;s working before the timer starts.
        </p>
      </div>

      {/* Camera preview card */}
      <div className="overflow-hidden rounded-[22px] border border-white/90 bg-[#16143A] shadow-[0_20px_44px_-24px_rgba(27,24,64,0.6)]">
        <div className="relative aspect-video w-full bg-[radial-gradient(120%_140%_at_100%_100%,#3B2C86_0%,#221C58_38%,#16143A_70%)]">
          {/* Live preview */}
          <video
            ref={videoRef}
            autoPlay
            playsInline
            muted
            className={clsx(
              "h-full w-full object-cover transition-opacity duration-500",
              isOk ? "opacity-100" : "opacity-0"
            )}
          />

          {/* Overlay for non-ok states */}
          {!isOk && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-center px-8">
              {status === "requesting" ? (
                <>
                  <Camera size={32} className="text-[#B6A4F8] animate-pulse" />
                  <p className="text-[14px] text-[#D6D0F2]">Waiting for permission…</p>
                </>
              ) : status === "denied" ? (
                <>
                  <AlertTriangle size={32} className="text-[#F49AAB]" />
                  <p className="text-[15px] font-semibold text-white">Camera access was denied</p>
                  <p className="text-[13px] text-[#D6D0F2]">
                    Allow camera access in your browser settings, then try again.
                  </p>
                </>
              ) : (
                <>
                  <AlertTriangle size={32} className="text-[#F49AAB]" />
                  <p className="text-[15px] font-semibold text-white">No camera found</p>
                  <p className="text-[13px] text-[#D6D0F2]">
                    Connect a camera to continue. If you&apos;re on a laptop, check that the camera is not covered.
                  </p>
                </>
              )}
            </div>
          )}

          {/* Status badge */}
          {isOk && (
            <div className="absolute bottom-3 left-3 flex items-center gap-1.5 rounded-full bg-black/50 px-2.5 py-1 text-xs text-white backdrop-blur">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#5BD49B]" />
              Camera live
            </div>
          )}
        </div>

        {/* Footer of the card */}
        <div className="flex items-center justify-between border-t border-white/10 px-5 py-3.5">
          <div className="flex items-center gap-2">
            {isOk ? (
              <>
                <CheckCircle2 size={16} className="text-[#5BD49B]" />
                <span className="text-[13px] font-medium text-[#E4E0F7]">Camera ready</span>
              </>
            ) : (
              <>
                <Camera size={16} className="text-[#D6D0F2]/60" />
                <span className="text-[13px] text-[#D6D0F2]/70">
                  {status === "requesting" ? "Requesting…" : "Camera not detected"}
                </span>
              </>
            )}
          </div>
          {(status === "denied" || status === "unavailable") && (
            <button
              type="button"
              onClick={retryCamera}
              className="flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-[13px] font-medium text-[#E4E0F7] transition-colors hover:bg-white/10"
            >
              <RefreshCcw size={13} />
              Try again
            </button>
          )}
        </div>
      </div>

      {/* Checklist */}
      <div className="rounded-[22px] border border-white/90 bg-white/70 p-5 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)] backdrop-blur">
        <p className="mb-3.5 text-[12.5px] font-medium tracking-[0.2em] text-[#6E6A8F] uppercase">Requirements</p>
        <ul className="space-y-2.5">
          {[
            { label: "Camera visible and working", met: isOk },
            { label: "Browser has camera permission", met: isOk },
            { label: "You are visible in the preview", met: isOk },
          ].map((item) => (
            <li key={item.label} className="flex items-center gap-3 text-[14px]">
              {item.met ? (
                <CheckCircle2 size={18} className="shrink-0 text-[#1E9A57]" />
              ) : (
                <span className="h-[18px] w-[18px] shrink-0 rounded-full border-2 border-[#C9BCF7]" />
              )}
              <span className={item.met ? "font-medium text-[#100E2E]" : "text-[#5E5A82]"}>{item.label}</span>
            </li>
          ))}
        </ul>
      </div>

      {/* Navigation */}
      <div className="flex items-center justify-between">
        <button type="button" onClick={onBack} className={clsx(buttonClass.quiet, "h-11 px-4 text-[14px]")}>
          <ArrowLeft size={16} />
          Back
        </button>
        <button id="device-check-continue" type="button" disabled={!isOk} onClick={onContinue} className={clsx(buttonClass.primary, "h-11 px-6 text-[15px]")}>
          Continue
          <ArrowRight size={16} />
        </button>
      </div>
    </div>
  );
}
