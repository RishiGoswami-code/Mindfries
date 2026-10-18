"use client";

import { useEffect, useRef, useState } from "react";
import { Loader2, Pencil, X } from "lucide-react";
import clsx from "clsx";
import { buttonClass } from "@/components/settings/Card";
import { display, ui } from "@/lib/dashboard/fonts";
import { saveIdentity } from "@/lib/profile/actions";
import { usePreviewMode } from "./PreviewMode";
import type { SavedIdentity } from "@/lib/profile/types";

/** Dispatch on `window` to open the Edit profile form from anywhere on the profile page. */
export const EDIT_PROFILE_EVENT = "mindfries:edit-profile";

type FormState = { name: string; role: string; location: string; openTo: string[]; noticePeriod: string; bio: string };

/**
 * "Edit profile" — the one button that opens the one place everything on
 * this page (besides the resume and the linked accounts, which manage
 * themselves) gets changed.
 *
 * Saves directly to the database via a Server Action (saveIdentity).
 */
export function EditProfileModal({ savedIdentity }: { savedIdentity: SavedIdentity }) {
  const preview = usePreviewMode();

  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<FormState>(() => savedIdentity);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const firstFieldRef = useRef<HTMLInputElement>(null);

  function openModal() {
    setForm(savedIdentity);
    setSaveError(null);
    setFieldError(null);
    setOpen(true);
  }

  // The profile completion checklist's "Profile details" row opens this
  // form from elsewhere on the page (CompletionSection).
  const openRef = useRef(openModal);
  useEffect(() => {
    openRef.current = openModal;
  });
  useEffect(() => {
    const onRequest = () => openRef.current();
    window.addEventListener(EDIT_PROFILE_EVENT, onRequest);
    return () => window.removeEventListener(EDIT_PROFILE_EVENT, onRequest);
  }, []);

  useEffect(() => {
    if (open) firstFieldRef.current?.focus();
  }, [open]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    if (open) window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function save() {
    if (!form.name.trim() || !form.role.trim()) {
      setFieldError("Name and role can't be empty.");
      return;
    }
    setFieldError(null);
    setSaveError(null);
    setSaving(true);

    // A request that never reaches the server rejects rather than returning
    // an error — the network dropped, the tab was offline. Without the catch
    // the button stayed on "Saving…" for good, with nothing said.
    try {
      const res = await saveIdentity({
        name: form.name.trim(),
        role: form.role.trim(),
        location: form.location.trim(),
        openTo: form.openTo,
        noticePeriod: form.noticePeriod,
        bio: form.bio.trim(),
      });
      if (res.ok) {
        setOpen(false);
      } else {
        setSaveError(res.error || "Couldn't save changes.");
      }
    } catch {
      setSaveError("Couldn't reach the server — check your connection and try again. Nothing you typed is lost.");
    } finally {
      setSaving(false);
    }
  }

  if (preview) return null;

  return (
    <>
      <button
        type="button"
        onClick={openModal}
        className="inline-flex h-[48px] items-center gap-3 rounded-2xl border border-white/90 bg-white/80 px-5 text-[15px] font-semibold text-[#1F1B4D] shadow-[0_8px_22px_-14px_rgba(76,58,160,0.5)] transition-colors hover:bg-white"
      >
        <Pencil size={18} strokeWidth={1.9} className="text-[#5B3BE0]" />
        Edit profile
      </button>

      {open && (
        <div
          className={clsx(ui.className, "fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-[#1B1840]/40 p-4 py-10 backdrop-blur-[3px]")}
          onClick={() => setOpen(false)}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Edit profile"
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-xl overflow-hidden rounded-[24px] bg-[#FBFAFE] text-[#1B1840] shadow-[0_40px_90px_-30px_rgba(27,24,64,0.55)]"
          >
            <div className="flex items-start gap-4 px-6 pt-6 pb-4 sm:px-7">
              <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
                <Pencil size={21} strokeWidth={1.9} />
              </span>
              <div className="min-w-0 flex-1">
                <h2 className={clsx(display.className, "text-[24px] leading-tight font-semibold text-[#100E2E]")}>Edit profile</h2>
                <p className="mt-1 text-[14px] text-[#5E5A82]">What hiring teams see at the top of your profile.</p>
              </div>
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close"
                className="-mt-1 -mr-1 flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-[#2A2654] transition-colors hover:bg-[#EEEBFA]"
              >
                <X size={20} />
              </button>
            </div>

            <div className="max-h-[65vh] overflow-y-auto px-6 pt-2 pb-5 sm:px-7">
              {/* ── Fields ──────────────────────────────────────────────── */}
              <div className="grid grid-cols-2 gap-4">
                <Field label="Name">
                  <input
                    ref={firstFieldRef}
                    value={form.name}
                    onChange={(e) => set("name", e.target.value)}
                    className={inputCls}
                  />
                </Field>
                <Field label="Role / headline">
                  <input value={form.role} onChange={(e) => set("role", e.target.value)} className={inputCls} />
                </Field>
                <Field label="Location">
                  <input value={form.location} onChange={(e) => set("location", e.target.value)} className={inputCls} />
                </Field>
                <Field label="Notice period">
                  <select value={form.noticePeriod} onChange={(e) => set("noticePeriod", e.target.value)} className={inputCls}>
                    {["Immediately", "2 weeks", "30 days", "60 days", "90 days"].map((p) => (
                      <option key={p} value={p}>
                        {p}
                      </option>
                    ))}
                  </select>
                </Field>

                <div className="col-span-2">
                  <span className="mb-2 block text-[13px] font-medium text-[#2A2654]">Open to</span>
                  <div className="flex flex-wrap gap-2">
                    {["Remote", "Hybrid", "Onsite"].map((opt) => {
                      const on = form.openTo.includes(opt);
                      return (
                        <button
                          key={opt}
                          type="button"
                          onClick={() => set("openTo", on ? form.openTo.filter((o) => o !== opt) : [...form.openTo, opt])}
                          className={clsx(
                            "rounded-full border px-4 py-2 text-[13.5px] font-medium transition-colors",
                            on
                              ? "border-[#5B3BE0] bg-[#5B3BE0] text-white shadow-[0_6px_16px_-8px_rgba(91,59,224,0.7)]"
                              : "border-[#DCD7EE] bg-white text-[#2A2654] hover:border-[#C9BCF7] hover:bg-[#F6F4FC]"
                          )}
                        >
                          {opt}
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div className="col-span-2">
                  <Field label="About you">
                    <textarea
                      value={form.bio}
                      onChange={(e) => set("bio", e.target.value)}
                      rows={4}
                      maxLength={500}
                      className={clsx(inputCls, "h-auto resize-none py-2.5 leading-relaxed")}
                    />
                  </Field>
                  <p className="mt-1.5 text-right text-[12px] text-[#6E6A8F] tabular-nums">{form.bio.length}/500</p>
                </div>
              </div>

              {fieldError && <p className="mt-3 text-[12.5px] text-[#a6203c]">{fieldError}</p>}
              {saveError && <p className="mt-3 text-[12.5px] text-[#a6203c]">{saveError}</p>}
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-[#ECE9F6] px-6 py-4 sm:px-7">
              <button type="button" onClick={() => setOpen(false)} className={clsx(buttonClass.quiet, "h-10")}>
                Cancel
              </button>
              <button type="button" onClick={save} disabled={saving} className={buttonClass.primary}>
                {saving ? (
                  <>
                    <Loader2 size={15} className="animate-spin" /> Saving…
                  </>
                ) : (
                  "Save changes"
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

const inputCls =
  "h-11 w-full rounded-xl border border-[#DCD7EE] bg-white px-3.5 text-[14.5px] text-[#100E2E] outline-none transition focus:border-[#7C5CFC]";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 flex items-center gap-1.5 text-[13px] font-medium text-[#2A2654]">
        {label}
      </span>
      {children}
    </label>
  );
}
