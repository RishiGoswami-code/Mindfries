// Package semver compares the loose versions that tools print
// ("2.43.0.windows.1", "v20.11.1", "go1.22.3", "Python 3.12.1") and checks
// them against simple constraints (">=3.11", ">=18,<23").
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is major.minor.patch. Pre-release tags are ignored except that a
// version with one sorts before the same version without.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

var numRe = regexp.MustCompile(`(\d+)(?:\.(\d+))?(?:\.(\d+))?(-[0-9A-Za-z.-]+)?`)

// Find extracts the first version-looking substring of s.
func Find(s string) (Version, bool) {
	m := numRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	v := Version{Pre: strings.TrimPrefix(m[4], "-")}
	v.Major, _ = strconv.Atoi(m[1])
	if m[2] != "" {
		v.Minor, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		v.Patch, _ = strconv.Atoi(m[3])
	}
	return v, true
}

// MustParse is Find for known-good literals.
func MustParse(s string) Version {
	v, ok := Find(s)
	if !ok {
		panic("semver: bad version " + s)
	}
	return v
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// Compare returns -1, 0 or 1.
func Compare(a, b Version) int {
	for _, d := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if d[0] < d[1] {
			return -1
		}
		if d[0] > d[1] {
			return 1
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	case a.Pre < b.Pre:
		return -1
	default:
		return 1
	}
}

// Less reports a < b.
func Less(a, b Version) bool { return Compare(a, b) < 0 }

// Satisfies checks v against a constraint: comma- or space-separated
// clauses, each an operator (>=, >, <=, <, =, ==) and a version. A bare
// version means ">=". An empty constraint is always satisfied.
func Satisfies(v Version, constraint string) (bool, error) {
	fields := strings.FieldsFunc(constraint, func(r rune) bool { return r == ',' || r == ' ' })
	for _, f := range fields {
		op := ">="
		for _, o := range []string{">=", "<=", "==", ">", "<", "="} {
			if strings.HasPrefix(f, o) {
				op, f = o, strings.TrimPrefix(f, o)
				break
			}
		}
		want, ok := Find(f)
		if !ok {
			return false, fmt.Errorf("bad version constraint %q", constraint)
		}
		c := Compare(v, want)
		var pass bool
		switch op {
		case ">=":
			pass = c >= 0
		case ">":
			pass = c > 0
		case "<=":
			pass = c <= 0
		case "<":
			pass = c < 0
		default:
			pass = c == 0
		}
		if !pass {
			return false, nil
		}
	}
	return true, nil
}
