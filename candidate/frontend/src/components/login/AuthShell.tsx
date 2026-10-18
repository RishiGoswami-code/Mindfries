import clsx from "clsx";
import { BrandMark } from "@/components/dashboard/CandidateSidebar";
import { display, ui } from "@/lib/dashboard/fonts";
import { LoginBlobs } from "./LoginBlobs";

/**
 * The frame for every page a candidate sees before (or outside) the
 * dashboard — sign in, sign up, password reset, an invitation, not found.
 * Same lavender canvas and glows as CandidateShell, the serif wordmark, and
 * one soft card holding the page's content, so arriving at Mindfries and
 * being inside it look like the same product.
 */
export function AuthShell({
  title,
  subtitle,
  width = "sm",
  children,
}: {
  title?: React.ReactNode;
  subtitle?: React.ReactNode;
  /** `sm` for a form, `md` for a page with more to say (an invitation). */
  width?: "sm" | "md";
  children: React.ReactNode;
}) {
  return (
    <div className={clsx(ui.className, "relative isolate flex min-h-screen items-center justify-center overflow-hidden bg-[#F3F1FA] px-4 py-16 text-[#1B1840] sm:px-6")}>
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 -z-10"
        style={{
          background:
            "radial-gradient(40% 35% at 8% 70%, rgba(214,204,250,0.55), transparent 70%), radial-gradient(35% 30% at 92% 18%, rgba(226,214,252,0.7), transparent 70%), radial-gradient(45% 40% at 55% 0%, rgba(255,255,255,0.9), transparent 70%)",
        }}
      />
      <LoginBlobs />

      <div className={clsx("relative z-10 flex w-full flex-col items-center", width === "md" ? "max-w-[480px]" : "max-w-[420px]")}>
        <div className="mb-8 flex items-center gap-3">
          <BrandMark size={36} />
          <span className={clsx(display.className, "text-[30px] leading-none font-medium tracking-tight text-[#1B1840]")}>Mindfries</span>
        </div>

        <main className="w-full rounded-[26px] border border-white/90 bg-white/75 p-7 shadow-[0_40px_90px_-40px_rgba(27,24,64,0.45)] backdrop-blur-md sm:p-8">
          {(title || subtitle) && (
            <div className="mb-6 text-center">
              {title && <h1 className={clsx(display.className, "text-[30px] leading-tight font-semibold tracking-tight text-[#100E2E]")}>{title}</h1>}
              {subtitle && <p className="mt-2 text-[14.5px] leading-relaxed text-[#5E5A82]">{subtitle}</p>}
            </div>
          )}
          {children}
        </main>
      </div>
    </div>
  );
}

/** Inputs on the sign-in pages — the same field as the dashboard's forms (Job preferences, Edit profile). */
export const authInput =
  "h-12 w-full rounded-xl border border-[#DCD7EE] bg-white px-4 text-[15px] text-[#100E2E] outline-none transition placeholder:text-[#8A86A8] focus:border-[#7C5CFC]";

/** The main action on a sign-in page: the dashboard's violet gradient, full width. */
export const authSubmit =
  "inline-flex h-12 w-full items-center justify-center gap-2 rounded-full bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-6 text-[15px] font-semibold text-white shadow-[0_14px_30px_-14px_rgba(91,59,224,0.8)] transition-[transform,opacity] hover:-translate-y-px disabled:opacity-60 disabled:hover:translate-y-0";

/** A secondary action beside it: white with a lavender edge. */
export const authSecondary =
  "inline-flex h-12 w-full items-center justify-center gap-2 rounded-full border border-[#E4E0F2] bg-white px-6 text-[15px] font-semibold text-[#1B1840] transition-colors hover:bg-[#F6F4FC]";

/** Text links on these pages. */
export const authLink = "font-medium text-[#5B3BE0] underline-offset-4 hover:underline";
