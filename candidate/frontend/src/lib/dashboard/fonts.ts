import { Inter, Newsreader } from "next/font/google";

/**
 * The dashboard's display serif — the greeting, the practice-run headline,
 * section titles. Only ever at heading sizes; everything a candidate has to
 * read quickly is in `ui` below.
 */
export const display = Newsreader({
  subsets: ["latin"],
  weight: ["500", "600"],
  display: "swap",
});

/** The dashboard's UI sans: labels, counts, buttons, body copy. */
export const ui = Inter({
  subsets: ["latin"],
  display: "swap",
});
