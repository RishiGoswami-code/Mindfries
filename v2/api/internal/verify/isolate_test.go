package verify

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestMain doubles as a tiny program for the isolation tests to run as the
// "candidate's check": it reports what it can see, or sleeps.
func TestMain(m *testing.M) {
	switch os.Getenv("VERIFY_HELPER") {
	case "env":
		fmt.Printf("secret=%q home_in_work=%v ci=%q\n", os.Getenv("V2_DATABASE_URL"), strings.Contains(os.Getenv("HOME"), ".home"), os.Getenv("CI"))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func spec(t *testing.T, helper string) Spec {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/f.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Spec{ProductDir: dir, Check: []string{os.Args[0]}, ExtraEnv: []string{"VERIFY_HELPER=" + helper}}
}

func TestIsolatedCheckCannotSeeTheGradersSecrets(t *testing.T) {
	t.Setenv("V2_DATABASE_URL", "postgres://user:hunter2@db/prod")
	t.Setenv("VERIFY_HELPER", "env")

	// Without isolation the check inherits everything (what authoring uses).
	open, err := Run(Spec{ProductDir: spec(t, "").ProductDir, Check: []string{os.Args[0]}})
	if err != nil || !strings.Contains(open.Output, "hunter2") {
		t.Fatalf("control: the unisolated check sees the environment: %q %v", open.Output, err)
	}

	s := spec(t, "env")
	s.Isolate = true
	res, err := Run(s)
	if err != nil || !res.Passed {
		t.Fatalf("%+v %v", res, err)
	}
	if strings.Contains(res.Output, "hunter2") || !strings.Contains(res.Output, `secret=""`) {
		t.Fatalf("an isolated check must not see the grader's secrets: %q", res.Output)
	}
	if !strings.Contains(res.Output, "home_in_work=true") || !strings.Contains(res.Output, `ci="1"`) {
		t.Fatalf("it gets its own home inside the throwaway workspace: %q", res.Output)
	}
}
