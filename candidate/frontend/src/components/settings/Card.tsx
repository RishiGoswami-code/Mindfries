import clsx from "clsx";
import { display } from "@/lib/dashboard/fonts";

/**
 * The Settings page's building blocks: a frosted card with an icon tile,
 * a serif title and a muted line under it, and rows inside it. Markup only,
 * so server and client parts of the page can both use them.
 */
export function SettingsCard({
  icon,
  title,
  subtitle,
  action,
  tone = "violet",
  className,
  children,
}: {
  icon?: React.ReactNode;
  title: string;
  subtitle?: React.ReactNode;
  action?: React.ReactNode;
  tone?: "violet" | "plain";
  className?: string;
  children?: React.ReactNode;
}) {
  return (
    <section className={clsx("min-w-0 rounded-[22px] border border-white/90 bg-white/70 p-5 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur sm:p-6", className)}>
      <div className="flex items-start gap-4">
        {icon && (
          <span
            className={clsx(
              "flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl",
              tone === "violet" ? "bg-[#EFEAFD] text-[#5B3BE0]" : "bg-[#F4F3F8] text-[#1B1840]"
            )}
          >
            {icon}
          </span>
        )}
        <div className="min-w-0 flex-1">
          <h2 className={clsx(display.className, "text-[22px] leading-tight font-semibold text-[#100E2E]")}>{title}</h2>
          {subtitle && <p className="mt-1 text-[14px] leading-snug text-[#5E5A82]">{subtitle}</p>}
        </div>
        {action}
      </div>
      {children && <div className="mt-4">{children}</div>}
    </section>
  );
}

/** One line inside a card: icon, label over detail, and whatever goes on the right. */
export function SettingsRow({
  icon,
  label,
  detail,
  children,
  className,
}: {
  icon?: React.ReactNode;
  label: React.ReactNode;
  detail?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={clsx("flex flex-wrap items-center gap-x-3.5 gap-y-2 py-3.5", className)}>
      {icon && <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#F6F4FC] text-[#4B2BB8]">{icon}</span>}
      {/* Wide enough to read; on a phone the controls wrap underneath instead of squeezing it. */}
      <div className="min-w-[min(100%,190px)] flex-1">
        <p className="text-[14.5px] font-medium text-[#100E2E]">{label}</p>
        {detail && <p className="mt-0.5 text-[13px] leading-snug text-[#6E6A8F]">{detail}</p>}
      </div>
      {children && <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-2">{children}</div>}
    </div>
  );
}

export function StatusPill({ tone, children }: { tone: "green" | "gray" | "violet" | "amber"; children: React.ReactNode }) {
  return (
    <span
      className={clsx(
        "inline-flex items-center rounded-lg px-2.5 py-1 text-[12.5px] font-medium whitespace-nowrap",
        tone === "green" && "bg-[#E3F7EC] text-[#14693a]",
        tone === "gray" && "bg-[#F1F0F6] text-[#5E5A82]",
        tone === "violet" && "bg-[#EFEAFD] text-[#4B2BB8]",
        tone === "amber" && "bg-[#FDF0E1] text-[#9A4B07]"
      )}
    >
      {children}
    </span>
  );
}

export const buttonClass = {
  quiet:
    "inline-flex h-9 items-center justify-center gap-1.5 rounded-xl border border-[#E4E0F2] bg-white px-3.5 text-[13.5px] font-medium text-[#1B1840] shadow-[0_2px_6px_-4px_rgba(27,24,64,0.3)] transition-colors hover:bg-[#F6F4FC] disabled:opacity-50",
  primary:
    "inline-flex h-10 items-center justify-center gap-1.5 rounded-xl bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-4 text-[14px] font-semibold text-white shadow-[0_10px_24px_-12px_rgba(91,59,224,0.8)] transition-transform hover:-translate-y-px disabled:opacity-60 disabled:hover:translate-y-0",
  danger:
    "inline-flex h-10 items-center justify-center gap-1.5 rounded-xl bg-[#DC2643] px-4 text-[14px] font-semibold text-white shadow-[0_10px_24px_-12px_rgba(220,38,67,0.7)] hover:bg-[#C21F3A] disabled:opacity-60",
};
