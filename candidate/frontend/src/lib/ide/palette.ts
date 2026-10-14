import type { IdeTheme } from "./theme";

/**
 * Mindfries brand palette (five swatches, darkest to lightest):
 *   NAVY_950 #0A1931 · NAVY_800 #1A3D63 · BLUE_600 #4A7FA7 · BLUE_200 #B3CFE5 · WHITE_50 #F6FAFD
 * The light theme uses these five; the dark theme (below) is a deeper navy
 * with indigo and violet. Every class below uses a hex value literally (never built
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
        // Dark: a deep-navy workspace with an indigo wash behind the cards,
        // thin blue borders and violet for whatever is selected or active.
        appBg: "bg-[#0B1027]",
        panelBg: "bg-[#0E1431]",
        // The canvas the cards float on: a subtle indigo gradient, a step
        // lighter at the edges, so the gaps read as gaps.
        canvas: "bg-[linear-gradient(135deg,#0A0E26_0%,#10173D_55%,#1A1550_100%)]",
        border: "border-[#232C5E]",
        text: "text-[#E7EAF6]",
        textMuted: "text-[#9AA3C7]",
        accent: "text-[#A78BFA]",
        hover: "hover:bg-[#7C5CFF]/10",
        active: "bg-[#7C5CFF]/25",
        tabActiveBg: "bg-[#161D45]",
        tabInactiveBg: "bg-[#0B1027]",
        activityBarBg: "bg-[#0E1431]",
        activityIcon: "text-[#9AA3C7]",
        activityIconActive: "text-[#E7EAF6]",
        breadcrumbBg: "bg-[#0B1027]",
        breadcrumbText: "text-[#9AA3C7]",
        breadcrumbTextActive: "text-[#E7EAF6]",
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

/** The primary action (Submit): a violet gradient, the one loud thing in the chrome. */
export const PRIMARY_BUTTON = "bg-gradient-to-r from-[#7C5CFF] to-[#A06BFF] text-white shadow-[0_6px_20px_-6px_rgba(124,92,255,0.7)] hover:brightness-110";

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
    "editor.background": "#0B1027",
    "editor.foreground": "#E7EAF6",
    "editorLineNumber.foreground": "#4A537D",
    "editorLineNumber.activeForeground": "#C4B5FD",
    "editor.lineHighlightBackground": "#141B40",
    "editor.lineHighlightBorder": "#00000000",
    "editor.selectionBackground": "#7C5CFF55",
    "editorCursor.foreground": "#A78BFA",
    "editorGutter.background": "#0B1027",
    "editorIndentGuide.background1": "#1C2450",
    "minimap.background": "#0B1027",
    "scrollbarSlider.background": "#7C5CFF22",
    "scrollbarSlider.hoverBackground": "#7C5CFF44",
    "editorWidget.background": "#10173D",
    "editorWidget.border": "#232C5E",
  },
};
