package tui

import (
	"fmt"
	"strings"
)

// Menu asks question with numbered options as the live region and returns
// the chosen index. Arrows and Enter, a number, or y / n (first / last
// option) choose; Esc and Ctrl+C choose the last option, which should be
// the safe one. It returns the last option too if the input ends.
func Menu(scr *Screen, keys *Keys, question string, options []string) int {
	sel := 0
	last := len(options) - 1
	draw := func() {
		w := scr.Width()
		var body []string
		for _, q := range strings.Split(question, "\n") {
			body = append(body, Bold+q+Reset)
		}
		for i, o := range options {
			mark, style := "  ", ""
			if i == sel {
				mark, style = Accent+"❯ ", Accent
			}
			body = append(body, fmt.Sprintf("%s%s%d. %s%s", mark, style, i+1, o, Reset))
		}
		lines := Box(body, min(w, 100), Gray)
		lines = append(lines, Gray+"  ↑/↓ to choose · Enter to confirm · Esc for "+strings.ToLower(Strip(options[last]))+Reset)
		scr.Live(lines, -1, 0)
	}
	for {
		draw()
		k := keys.Next()
		switch k.Kind {
		case KeyUp:
			sel = (sel + len(options) - 1) % len(options)
		case KeyDown, KeyTab:
			sel = (sel + 1) % len(options)
		case KeyEnter:
			scr.ClearLive()
			return sel
		case KeyEsc, KeyCtrlC, KeyEOF, KeyCtrlD:
			scr.ClearLive()
			return last
		case KeyRune:
			switch r := k.Rune; {
			case r >= '1' && r <= '9' && int(r-'1') <= last:
				scr.ClearLive()
				return int(r - '1')
			case r == 'y' || r == 'Y':
				scr.ClearLive()
				return 0
			case r == 'n' || r == 'N':
				scr.ClearLive()
				return last
			}
		}
	}
}
