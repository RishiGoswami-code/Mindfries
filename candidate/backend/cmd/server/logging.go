package main

import (
	"io"
	"log/slog"
)

// newLogger is the process's logger: one JSON object per line, on stdout.
//
// slog's default handler writes through the log package to stderr, and
// Railway labels everything on stderr an error — so every "INFO request …
// status=200" showed up in red, and a real failure was lost among them.
// Railway reads a JSON line's "level" and "message" instead, so a line is
// marked by what it says rather than by where it went.
func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	}))
}
