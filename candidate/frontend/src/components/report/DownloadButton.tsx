"use client";

import { Download } from "lucide-react";

/**
 * "Download report": the browser's own print, where "Save as PDF" keeps the
 * report as a file. The portal's sidebar, top bar and these buttons are
 * left off the page when printing.
 */
export function DownloadButton() {
  return (
    <button
      type="button"
      onClick={() => window.print()}
      className="inline-flex h-12 items-center gap-2.5 rounded-2xl border border-[#E4E0F2] bg-white px-5 text-[15px] font-medium text-[#1B1840] shadow-[0_4px_12px_-8px_rgba(27,24,64,0.35)] hover:bg-[#F6F4FC] print:hidden"
    >
      <Download size={18} /> Download report
    </button>
  );
}
