package semver

import "testing"

func TestFind(t *testing.T) {
	cases := map[string]string{
		"git version 2.43.0.windows.1":    "2.43.0",
		"v20.11.1":                        "20.11.1",
		"go version go1.22.3 linux/amd64": "1.22.3",
		"Python 3.12":                     "3.12.0",
		"0.1.0-dev":                       "0.1.0-dev",
	}
	for in, want := range cases {
		v, ok := Find(in)
		if !ok || v.String() != want {
			t.Errorf("Find(%q) = %v, %v; want %s", in, v, ok, want)
		}
	}
	if _, ok := Find("no digits"); ok {
		t.Error("Find found a version in text without one")
	}
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		v, c string
		want bool
	}{
		{"3.12.1", ">=3.11", true},
		{"3.10.0", ">=3.11", false},
		{"20.1.0", ">=18,<23", true},
		{"23.0.0", ">=18 <23", false},
		{"1.2.3", "1.2", true},
		{"1.2.3", "=1.2.3", true},
		{"1.2.3", "", true},
		{"0.1.0-dev", ">=0.1.0", false},
	}
	for _, c := range cases {
		got, err := Satisfies(MustParse(c.v), c.c)
		if err != nil || got != c.want {
			t.Errorf("Satisfies(%s, %q) = %v, %v; want %v", c.v, c.c, got, err, c.want)
		}
	}
	if _, err := Satisfies(MustParse("1.0.0"), ">=abc"); err == nil {
		t.Error("bad constraint accepted")
	}
}
