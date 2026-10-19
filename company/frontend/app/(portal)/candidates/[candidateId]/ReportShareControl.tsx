"use client";

import { useState, useTransition } from "react";
import { CheckCircle2, Eye, EyeOff, Lock, Send, ShieldAlert, Sparkles } from "lucide-react";
import { Button, Pill } from "@/components/ui";
import { fmtDate } from "@/lib/format";
import { toggleReportSharing } from "./actions";

interface ReportShareControlProps {
  applicationId: string;
  sharedWithCandidate?: boolean;
  sharedAt?: string | null;
  sharedBy?: string | null;
  canManage: boolean;
}

export function ReportShareControl({
  applicationId,
  sharedWithCandidate = false,
  sharedAt,
  sharedBy,
  canManage,
}: ReportShareControlProps) {
  const [isPending, startTransition] = useTransition();
  const [error, setError] = useState<string | null>(null);

  const handleToggle = (share: boolean) => {
    setError(null);
    startTransition(async () => {
      const res = await toggleReportSharing(applicationId, share);
      if (res.error) {
        setError(res.error);
      }
    });
  };

  return (
    <div className="hair-card overflow-hidden p-5 transition-all">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="eyebrow">Candidate Report Visibility</span>
            {sharedWithCandidate ? (
              <Pill tone="green">Visible to Candidate</Pill>
            ) : (
              <Pill tone="amber">Hidden from Candidate</Pill>
            )}
          </div>

          <h3 className="mt-1 text-base font-bold text-[color:var(--color-foreground)]">
            {sharedWithCandidate
              ? "Report has been shared with the candidate"
              : "Report is private to your company"}
          </h3>

          <p className="mt-1 text-xs leading-relaxed text-dim">
            {sharedWithCandidate ? (
              <>
                The candidate can now view their evaluation feedback on their portal.
                {sharedAt && (
                  <span className="ml-1 text-faint">
                    Shared on {fmtDate(sharedAt)}
                    {sharedBy ? ` by ${sharedBy}` : ""}.
                  </span>
                )}
              </>
            ) : (
              "The candidate currently sees an \"Under Review\" screen. Proctoring flags, evaluation scores, and observations are not visible until you share this report."
            )}
          </p>

          <div className="mt-2.5 flex items-center gap-2 text-[11px] text-faint">
            <Lock size={12} className="shrink-0 text-faint" />
            <span>Proctoring flags and internal reviewer notes always remain strictly private to your team.</span>
          </div>
        </div>

        {canManage && (
          <div className="flex shrink-0 items-center gap-2">
            {sharedWithCandidate ? (
              <Button
                variant="ghost"
                size="sm"
                disabled={isPending}
                onClick={() => handleToggle(false)}
                className="gap-1.5 text-xs text-dim hover:text-[color:var(--color-foreground)]"
              >
                <EyeOff size={14} />
                {isPending ? "Updating…" : "Revoke candidate access"}
              </Button>
            ) : (
              <Button
                size="sm"
                disabled={isPending}
                onClick={() => handleToggle(true)}
                className="gap-1.5 text-xs font-semibold"
              >
                <Send size={14} />
                {isPending ? "Sharing…" : "Send report to candidate"}
              </Button>
            )}
          </div>
        )}
      </div>

      {error && (
        <div className="mt-3 rounded-xl bg-[#fdecef] px-3 py-2 text-xs text-[#a6203c]">
          {error}
        </div>
      )}
    </div>
  );
}
