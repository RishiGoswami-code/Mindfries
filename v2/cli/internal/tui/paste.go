package tui

import "strings"

// pasteText normalises pasted line endings to "\n".
func pasteText(b []byte) string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}
