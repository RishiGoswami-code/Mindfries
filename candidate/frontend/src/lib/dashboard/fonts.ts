import { Inter, Newsreader } from "next/font/google";

/**
 * The candidate portal's display serif — the greeting, page and card
 * titles, the practice-run headline. Only ever at heading sizes; everything
 * a candidate has to read quickly is in `ui` below.
 *
 * Loaded as the variable font (no `weight` list): one file covers every
 * weight used. Listing weights made Turbopack's production build fail to
 * resolve the font's file ("next/font/google queries have exactly one
 * entry"), though webpack and `next dev` accepted it.
 */
export const display = Newsreader({
  subsets: ["latin"],
  display: "swap",
});

/** The candidate portal's UI sans: labels, counts, buttons, body copy. */
export const ui = Inter({
  subsets: ["latin"],
  display: "swap",
});
