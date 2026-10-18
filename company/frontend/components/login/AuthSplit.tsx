// The frame for the company portal's sign-in and sign-up: a deep-indigo
// brand panel beside the form. Below md the panel folds above the form.

export function AuthSplit({
  eyebrow,
  headline,
  children,
  aside,
}: {
  eyebrow: string;
  headline: string;
  /** The form side. */
  children: React.ReactNode;
  /** Under the headline on the brand panel. */
  aside?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen flex-col bg-[#F3F1FA] md:flex-row">
      <section className="relative flex flex-col justify-between gap-10 overflow-hidden px-8 py-10 text-white md:w-1/2 md:px-14 md:py-14 [background:radial-gradient(120%_140%_at_100%_100%,#3B2C86_0%,#221C58_38%,#16143A_70%)]">
        <div className="flex items-center gap-3">
          {/* eslint-disable-next-line @next/next/no-img-element -- tiny static local SVG, no optimization needed */}
          <img src="/mindfries-logo.svg" alt="" width={32} height={32} className="brightness-0 invert" />
          <span className="text-[22px] font-extrabold tracking-tight">Mindfries</span>
          <span className="rounded-full bg-white/12 px-2.5 py-1 text-[11px] font-semibold tracking-[0.14em] text-[#D6D0F2] uppercase">Company</span>
        </div>
        <div className="max-w-md">
          <p className="text-[12.5px] font-medium tracking-[0.28em] text-[#D6D0F2] uppercase">{eyebrow}</p>
          <h1 className="mt-4 text-[36px] leading-[1.05] font-extrabold tracking-tight md:text-[46px]">{headline}</h1>
        </div>
        <div className="text-[14.5px] text-[#E4E0F7]">{aside}</div>
      </section>

      <section className="flex flex-1 items-center justify-center px-5 py-12">
        <div className="w-full max-w-[440px] rounded-[26px] border border-white/90 bg-white/80 p-7 shadow-[0_40px_90px_-40px_rgba(27,24,64,0.45)] backdrop-blur sm:p-9">
          {children}
        </div>
      </section>
    </div>
  );
}

/** Inputs, primary button and links shared by the sign-in and sign-up forms. */
export const authField =
  "h-12 w-full rounded-xl border border-[#DCD7EE] bg-white px-4 text-[15px] text-ink outline-none transition placeholder:text-faint focus:border-accent";
export const authSubmit =
  "inline-flex h-12 w-full items-center justify-center rounded-full bg-gradient-to-r from-[#8b5cf6] to-accent px-6 text-[15px] font-semibold text-white shadow-[0_14px_30px_-14px_rgba(124,58,237,0.8)] transition-[transform,opacity] hover:-translate-y-px disabled:opacity-60 disabled:hover:translate-y-0";
export const authLink = "font-medium text-accent underline-offset-4 hover:underline";
