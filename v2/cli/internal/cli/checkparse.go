package cli

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// What a check run reports of its output: the most test names and the most
// output the contract allows (check.run in contracts/cli-api.yaml).
const (
	maxTestNames = 200
	maxNameLen   = 200
	maxTailBytes = 4096
)

// testNames is what the output of a test runner says about its tests.
type testNames struct {
	Passing, Failing []string
	// Passed and Failed count every test seen, including any past the cap
	// on names.
	Passed, Failed int
}

var (
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

	goTestRe = regexp.MustCompile(`^\s*--- (PASS|FAIL): (\S+)`)
	goPkgRe  = regexp.MustCompile(`^(ok|FAIL)\s+(\S+)`)

	pyVerboseRe = regexp.MustCompile(`^(\S+::\S+)\s+(PASSED|FAILED|ERROR)\b`)
	pyShortRe   = regexp.MustCompile(`^(FAILED|ERROR) (\S+::\S+)`)

	jsMarkRe   = regexp.MustCompile(`^\s*([✓✔√×✕✗✘])\s+(.+?)\s*$`)
	jsTimingRe = regexp.MustCompile(`\s*(\(\d+(\.\d+)?\s*m?s\)|\d+(\.\d+)?\s*m?s)$`)
	jsFilesRe  = regexp.MustCompile(`\(\d+ tests?\b`)
	jsFailRe   = regexp.MustCompile(`^\s*FAIL\s+(\S.*?)\s*$`)
)

// parseTests finds test names in a runner's output, best effort: go test,
// pytest, and vitest or jest. Output from any other runner gives nothing.
func parseTests(output string) testNames {
	var t testNames
	seen := map[string]bool{}
	add := func(pass bool, name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		if pass {
			t.Passed++
		} else {
			t.Failed++
		}
		if len(name) > maxNameLen {
			name = name[:maxNameLen]
			for !utf8.ValidString(name) {
				name = name[:len(name)-1]
			}
		}
		list := &t.Passing
		if !pass {
			list = &t.Failing
		}
		if len(*list) < maxTestNames {
			*list = append(*list, name)
		}
	}

	// Package and file lines only name something when the runner didn't name
	// its tests (go test without -v; jest's summary).
	var okPkgs, failPkgs, failFiles []string
	for _, line := range strings.Split(stripANSI(output), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case goTestRe.MatchString(line):
			m := goTestRe.FindStringSubmatch(line)
			add(m[1] == "PASS", m[2])
		case pyVerboseRe.MatchString(line):
			m := pyVerboseRe.FindStringSubmatch(line)
			add(m[2] == "PASSED", m[1])
		case pyShortRe.MatchString(line):
			add(false, pyShortRe.FindStringSubmatch(line)[2])
		case jsMarkRe.MatchString(line):
			m := jsMarkRe.FindStringSubmatch(line)
			name := jsTimingRe.ReplaceAllString(m[2], "")
			if !jsFilesRe.MatchString(name) {
				add(strings.ContainsAny(m[1], "✓✔√"), name)
			}
		case goPkgRe.MatchString(line):
			m := goPkgRe.FindStringSubmatch(line)
			if m[1] == "ok" {
				okPkgs = append(okPkgs, m[2])
			} else {
				failPkgs = append(failPkgs, m[2])
			}
		case jsFailRe.MatchString(line):
			name := jsFailRe.FindStringSubmatch(line)[1]
			if strings.Contains(name, " > ") {
				add(false, name) // vitest names the failing test here too
			} else {
				failFiles = append(failFiles, name)
			}
		}
	}
	if t.Passed+t.Failed == 0 {
		for _, p := range okPkgs {
			add(true, p)
		}
		for _, p := range append(failPkgs, failFiles...) {
			add(false, p)
		}
	}
	return t
}

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// outputTail is the end of the output, at most maxTailBytes, cut at a
// character boundary and without colour codes.
func outputTail(output string) string {
	s := strings.TrimRight(stripANSI(output), "\r\n")
	if len(s) <= maxTailBytes {
		return s
	}
	s = s[len(s)-maxTailBytes:]
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	return s
}
