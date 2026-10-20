/** The Mindfries brain mark, drawn inline so it scales and takes colour. */
export function MindfriesMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden fill="none">
      <defs>
        <linearGradient id="mf-mark" x1="4" y1="4" x2="28" y2="28" gradientUnits="userSpaceOnUse">
          <stop stopColor="#3B6EF0" />
          <stop offset="1" stopColor="#1E40AF" />
        </linearGradient>
      </defs>
      <path
        d="M15 5.5c-2.6-1.6-6.2-.4-6.8 2.6-2.5.3-4 2.9-3 5.2-1.7 1.2-1.9 3.7-.5 5.2-1 2.3.6 5 3.1 5.1.7 2.9 4.3 3.9 6.6 2.1M17 5.5c2.6-1.6 6.2-.4 6.8 2.6 2.5.3 4 2.9 3 5.2 1.7 1.2 1.9 3.7.5 5.2 1 2.3-.6 5-3.1 5.1-.7 2.9-4.3 3.9-6.6 2.1M16 5v22M11 11h2.5M10 16h3.5m-2.5 5h2.5M21 11h-2.5M22 16h-3.5m2.5 5h-2.5"
        stroke="url(#mf-mark)"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
