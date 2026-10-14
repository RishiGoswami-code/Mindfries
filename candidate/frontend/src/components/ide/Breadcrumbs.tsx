import clsx from "clsx";
import { ChevronRight } from "lucide-react";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

export function Breadcrumbs({ path, theme }: { path: string | null; theme: IdeTheme }) {
  const palette = idePalette(theme);
  if (!path) return null;

  const segments = path.split("/");

  return (
    <div
      className={clsx(
        "flex h-8 shrink-0 items-center gap-1 overflow-x-auto border-b px-4 text-[12.5px]",
        palette.border,
        palette.breadcrumbBg
      )}
    >
      {segments.map((seg, i) => {
        const isLast = i === segments.length - 1;
        return (
          <span key={i} className="flex items-center gap-1">
            {i > 0 && <ChevronRight size={12} className={palette.breadcrumbText} />}
            <span className={isLast ? palette.breadcrumbTextActive : palette.breadcrumbText}>{seg}</span>
          </span>
        );
      })}
    </div>
  );
}
