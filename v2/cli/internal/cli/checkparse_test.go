package cli

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseGoTestVerbose(t *testing.T) {
	out := `=== RUN   TestAllow
=== RUN   TestAllow/under_limit
--- PASS: TestAllow (0.00s)
    --- PASS: TestAllow/under_limit (0.00s)
=== RUN   TestRefill
    limiter_test.go:31: got 0, want 1
--- FAIL: TestRefill (0.01s)
=== RUN   TestSkipped
--- SKIP: TestSkipped (0.00s)
FAIL
FAIL	example.com/limiter	0.012s
FAIL
`
	got := parseTests(out)
	if want := []string{"TestAllow", "TestAllow/under_limit"}; !reflect.DeepEqual(got.Passing, want) {
		t.Errorf("passing %v, want %v", got.Passing, want)
	}
	if want := []string{"TestRefill"}; !reflect.DeepEqual(got.Failing, want) {
		t.Errorf("failing %v, want %v", got.Failing, want)
	}
	if got.Passed != 2 || got.Failed != 1 {
		t.Errorf("counts %d passed %d failed", got.Passed, got.Failed)
	}
}

func TestParseGoTestPackages(t *testing.T) {
	out := "ok  \texample.com/a\t0.004s\nok  \texample.com/b\t(cached)\nFAIL\texample.com/c [build failed]\nFAIL\n"
	got := parseTests(out)
	if want := []string{"example.com/a", "example.com/b"}; !reflect.DeepEqual(got.Passing, want) {
		t.Errorf("passing %v", got.Passing)
	}
	if want := []string{"example.com/c"}; !reflect.DeepEqual(got.Failing, want) {
		t.Errorf("failing %v", got.Failing)
	}
}

func TestParsePytest(t *testing.T) {
	verbose := `============================= test session starts =============================
collecting ... collected 3 items

tests/test_limiter.py::test_allow PASSED                                 [ 33%]
tests/test_limiter.py::test_refill FAILED                                [ 66%]
tests/test_limiter.py::test_burst PASSED                                 [100%]

=================================== FAILURES ===================================
=========================== short test summary info ============================
FAILED tests/test_limiter.py::test_refill - assert 0 == 1
========================= 1 failed, 2 passed in 0.05s ==========================
`
	got := parseTests(verbose)
	if want := []string{"tests/test_limiter.py::test_allow", "tests/test_limiter.py::test_burst"}; !reflect.DeepEqual(got.Passing, want) {
		t.Errorf("passing %v", got.Passing)
	}
	// The FAILED line is in the progress and again in the summary: once.
	if want := []string{"tests/test_limiter.py::test_refill"}; !reflect.DeepEqual(got.Failing, want) {
		t.Errorf("failing %v", got.Failing)
	}

	quiet := "..F\n=========================== short test summary info ============================\nFAILED tests/test_limiter.py::test_refill - assert 0 == 1\nERROR tests/test_other.py::test_setup - fixture 'x' not found\n"
	got = parseTests(quiet)
	if len(got.Passing) != 0 || len(got.Failing) != 2 || got.Failing[1] != "tests/test_other.py::test_setup" {
		t.Errorf("short summary: %+v", got)
	}
}

func TestParseVitestAndJest(t *testing.T) {
	vitest := "\x1b[32m ✓\x1b[39m src/limiter.test.ts \x1b[2m(2 tests)\x1b[22m \x1b[2m5ms\x1b[22m\n" +
		" ✓ src/limiter.test.ts > allows under the limit 2ms\n" +
		" × src/limiter.test.ts > refills over time 3ms\n" +
		" FAIL  src/limiter.test.ts > refills over time\n" +
		"AssertionError: expected 0 to be 1\n"
	got := parseTests(vitest)
	if want := []string{"src/limiter.test.ts > allows under the limit"}; !reflect.DeepEqual(got.Passing, want) {
		t.Errorf("vitest passing %v", got.Passing)
	}
	if want := []string{"src/limiter.test.ts > refills over time"}; !reflect.DeepEqual(got.Failing, want) {
		t.Errorf("vitest failing %v", got.Failing)
	}

	jest := `FAIL src/limiter.test.js
  limiter
    ✓ allows under the limit (2 ms)
    ✕ refills over time (3 ms)

Tests:       1 failed, 1 passed, 2 total
`
	got = parseTests(jest)
	if !reflect.DeepEqual(got.Passing, []string{"allows under the limit"}) || !reflect.DeepEqual(got.Failing, []string{"refills over time"}) {
		t.Errorf("jest: %+v", got)
	}

	// A failed file with no named tests (a syntax error) is named instead.
	got = parseTests("FAIL src/broken.test.js\n  Test suite failed to run\n")
	if !reflect.DeepEqual(got.Failing, []string{"src/broken.test.js"}) {
		t.Errorf("failed suite: %+v", got)
	}
}

func TestParseUnknownRunner(t *testing.T) {
	for _, out := range []string{"", "Build succeeded.\n0 warnings\n", "git version 2.43.0\n", "random PASSED text\nFAILED"} {
		if got := parseTests(out); got.Passed+got.Failed != 0 {
			t.Errorf("%q parsed as %+v", out, got)
		}
	}
}

func TestParseCapsNames(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 450; i++ {
		fmt.Fprintf(&b, "--- PASS: Test%d (0.00s)\n", i)
	}
	fmt.Fprintf(&b, "--- FAIL: Test%s (0.00s)\n", strings.Repeat("x", 500))
	got := parseTests(b.String())
	if len(got.Passing) != maxTestNames || got.Passed != 450 {
		t.Errorf("%d names kept, %d counted", len(got.Passing), got.Passed)
	}
	if len(got.Failing) != 1 || len(got.Failing[0]) != maxNameLen {
		t.Errorf("long name kept as %d bytes", len(got.Failing[0]))
	}
}

func TestOutputTail(t *testing.T) {
	if got := outputTail("short\n"); got != "short" {
		t.Errorf("short output: %q", got)
	}
	long := strings.Repeat("é", 5000) + "the end"
	got := outputTail(long)
	if len(got) > maxTailBytes || !utf8.ValidString(got) || !strings.HasSuffix(got, "the end") {
		t.Errorf("tail is %d bytes, valid=%v", len(got), utf8.ValidString(got))
	}
	if got := outputTail("\x1b[31mred\x1b[0m"); got != "red" {
		t.Errorf("colour codes kept: %q", got)
	}
}
