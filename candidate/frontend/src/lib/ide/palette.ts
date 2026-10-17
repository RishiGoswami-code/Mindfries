import type { IdeTheme } from "./theme";

/**
 * Mindfries brand palette (five swatches, darkest to lightest):
 *   NAVY_950 #0A1931 · NAVY_800 #1A3D63 · BLUE_600 #4A7FA7 · BLUE_200 #B3CFE5 · WHITE_50 #F6FAFD
 * The light theme uses these five; the dark theme (below) is a deeper navy
 * with a clear blue accent. Every class below uses a hex value literally (never built
 * via string interpolation — Tailwind's static scanner can't resolve an
 * interpolated arbitrary-value class, only a complete literal one).
 */
export const NAVY_950 = "#0A1931";
export const NAVY_800 = "#1A3D63";
export const BLUE_600 = "#4A7FA7";
export const BLUE_200 = "#B3CFE5";
export const WHITE_50 = "#F6FAFD";

/** Shared color tokens for the IDE chrome, keyed by theme — modeled on VS Code's default Dark+/Light+ themes. */
export function idePalette(theme: IdeTheme) {
  return theme === "dark"
    ? {
        // Dark: a deep-navy workspace, slate-blue borders and a clear blue
        // for whatever is selected or active.
        appBg: "bg-[#0B1222]",
        panelBg: "bg-[#0F172A]",
        // The canvas the cards float on: a step darker than the cards, so
        // the gaps read as gaps.
        canvas: "bg-[#0A101E]",
        border: "border-[#1E2A44]",
        text: "text-[#E2E8F0]",
        textMuted: "text-[#94A3B8]",
        accent: "text-[#60A5FA]",
        hover: "hover:bg-[#2F6BFF]/10",
        active: "bg-[#2F6BFF]/25",
        tabActiveBg: "bg-[#16213A]",
        tabInactiveBg: "bg-[#0B1222]",
        activityBarBg: "bg-[#0F172A]",
        activityIcon: "text-[#94A3B8]",
        activityIconActive: "text-[#E2E8F0]",
        breadcrumbBg: "bg-[#0B1222]",
        breadcrumbText: "text-[#94A3B8]",
        breadcrumbTextActive: "text-[#E2E8F0]",
      }
    : {
        appBg: "bg-[#F6FAFD]",
        panelBg: "bg-[#B3CFE5]/20",
        canvas: "bg-[#B3CFE5]/20",
        border: "border-[#B3CFE5]",
        text: "text-[#0A1931]",
        textMuted: "text-[#4A7FA7]",
        accent: "text-[#4A7FA7]",
        hover: "hover:bg-[#B3CFE5]/30",
        active: "bg-[#B3CFE5]/50",
        tabActiveBg: "bg-[#F6FAFD]",
        tabInactiveBg: "bg-[#B3CFE5]/15",
        activityBarBg: "bg-[#0A1931]",
        activityIcon: "text-[#B3CFE5]",
        activityIconActive: "text-[#F6FAFD]",
        breadcrumbBg: "bg-[#F6FAFD]",
        breadcrumbText: "text-[#4A7FA7]",
        breadcrumbTextActive: "text-[#0A1931]",
      };
}

/** Mindfries brand blue — used for the status bar, replacing VS Code's default blue. */
export const STATUS_BAR_BG = BLUE_600;

/** The primary action (Submit, Run Tests): a solid blue, the one loud thing in the chrome. */
export const PRIMARY_BUTTON = "bg-[#2F6BFF] text-white shadow-[0_6px_20px_-6px_rgba(47,107,255,0.7)] hover:bg-[#3B78FF]";

/** The editor's own dark theme, matched to the chrome (registered in EditorPanel). */
export const MONACO_DARK = {
  base: "vs-dark" as const,
  inherit: true,
  rules: [
    { token: "comment", foreground: "6B7399", fontStyle: "italic" },
    { token: "keyword", foreground: "C792EA" },
    { token: "string", foreground: "F2A365" },
    { token: "number", foreground: "F78C6C" },
    { token: "type", foreground: "82AAFF" },
    { token: "keyword.md", foreground: "82AAFF" },
  ],
  colors: {
    "editor.background": "#0B1222",
    "editor.foreground": "#E2E8F0",
    "editorLineNumber.foreground": "#4A5677",
    "editorLineNumber.activeForeground": "#93C5FD",
    "editor.lineHighlightBackground": "#142038",
    "editor.lineHighlightBorder": "#00000000",
    "editor.selectionBackground": "#2F6BFF55",
    "editorCursor.foreground": "#60A5FA",
    "editorGutter.background": "#0B1222",
    "editorIndentGuide.background1": "#1C2741",
    "minimap.background": "#0B1222",
    "scrollbarSlider.background": "#2F6BFF22",
    "scrollbarSlider.hoverBackground": "#2F6BFF44",
    "editorWidget.background": "#111A2E",
    "editorWidget.border": "#1E2A44",
  },
};
