package assistant

import (
	"fmt"
	"io"
	"strings"
)

// lines splits text into lines without their newlines.
func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}

type op struct {
	kind byte // ' ', '-', '+'
	text string
}

// diffLines is a longest-common-subsequence line diff. Very large inputs
// fall back to "everything removed, everything added".
func diffLines(a, b []string) []op {
	if len(a)*len(b) > 4_000_000 {
		var out []op
		for _, l := range a {
			out = append(out, op{'-', l})
		}
		for _, l := range b {
			out = append(out, op{'+', l})
		}
		return out
	}
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, op{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, op{'-', a[i]})
			i++
		default:
			out = append(out, op{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, op{'-', a[i]})
	}
	for ; j < m; j++ {
		out = append(out, op{'+', b[j]})
	}
	return out
}

// addedRanges returns the 1-based line ranges in b that the diff adds.
func addedRanges(a, b string) [][2]int {
	var out [][2]int
	line := 0
	for _, o := range diffLines(lines(a), lines(b)) {
		if o.kind == '-' {
			continue
		}
		line++
		if o.kind != '+' {
			continue
		}
		if k := len(out); k > 0 && out[k-1][1] == line-1 {
			out[k-1][1] = line
		} else {
			out = append(out, [2]int{line, line})
		}
	}
	return out
}

// writeDiff prints a unified-style diff with three lines of context.
func writeDiff(w io.Writer, path, before, after string, color bool) {
	ops := diffLines(lines(before), lines(after))
	red, green, cyan, reset := "", "", "", ""
	if color {
		red, green, cyan, reset = "\033[31m", "\033[32m", "\033[36m", "\033[0m"
	}
	fmt.Fprintf(w, "%s--- %s\n+++ %s%s\n", cyan, path, path, reset)
	const ctx = 3
	show := make([]bool, len(ops))
	for i, o := range ops {
		if o.kind != ' ' {
			for k := max(0, i-ctx); k <= min(len(ops)-1, i+ctx); k++ {
				show[k] = true
			}
		}
	}
	skipped := false
	for i, o := range ops {
		if !show[i] {
			skipped = true
			continue
		}
		if skipped {
			fmt.Fprintf(w, "%s@@ … @@%s\n", cyan, reset)
			skipped = false
		}
		switch o.kind {
		case '-':
			fmt.Fprintf(w, "%s-%s%s\n", red, o.text, reset)
		case '+':
			fmt.Fprintf(w, "%s+%s%s\n", green, o.text, reset)
		default:
			fmt.Fprintf(w, " %s\n", o.text)
		}
	}
}
