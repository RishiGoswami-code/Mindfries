import clsx from "clsx";

const TINTS = [
  "bg-[#EFEAFD] text-[#5B3BE0]",
  "bg-[#E3F6EC] text-[#14693a]",
  "bg-[#FDECEF] text-[#a6203c]",
  "bg-[#E6F1FD] text-[#1E5AA8]",
  "bg-[#FFF3D6] text-[#8A5A00]",
  "bg-[#F1EFF8] text-[#2A2654]",
];

/**
 * A company's mark: its initial on a tint picked from its name, so the same
 * company always looks the same. Companies don't upload logos, and this
 * doesn't pretend they did.
 */
export function CompanyTile({ name, size = 48, className }: { name: string; size?: number; className?: string }) {
  let hash = 0;
  for (const ch of name) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0;
  return (
    <span
      aria-hidden
      className={clsx("flex shrink-0 items-center justify-center rounded-2xl font-semibold", TINTS[hash % TINTS.length], className)}
      style={{ width: size, height: size, fontSize: Math.round(size * 0.42) }}
    >
      {name.trim().charAt(0).toUpperCase() || "?"}
    </span>
  );
}
