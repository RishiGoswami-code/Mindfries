"use client";

import { useState } from "react";
import { createPortal } from "react-dom";
import { PoliciesModal } from "./PoliciesModal";

/**
 * Opens the same consent copy the account menu's "Privacy & recording
 * policy" shows. The modal is portalled to <body>: this link sits inline in
 * running text, and the modal's headings and lists can't be inside a <p>.
 */
export function PoliciesLink({ children, className }: { children: React.ReactNode; className?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)} className={className}>
        {children}
      </button>
      {open && createPortal(<PoliciesModal onClose={() => setOpen(false)} />, document.body)}
    </>
  );
}
