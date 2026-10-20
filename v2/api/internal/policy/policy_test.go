package policy

import (
	"reflect"
	"testing"
)

func TestNormaliseReviewers(t *testing.T) {
	ok, bad := NormaliseReviewers([]string{" @Zed ", "alice", "ALICE", "", "bad name", "-x", "bob-smith", "a--b"})
	if !reflect.DeepEqual(ok, []string{"alice", "bob-smith", "Zed"}) {
		t.Errorf("ok = %v", ok)
	}
	if !reflect.DeepEqual(bad, []string{"bad name", "-x", "a--b"}) {
		t.Errorf("bad = %v", bad)
	}
	if got, _ := NormaliseReviewers(nil); got == nil || len(got) != 0 {
		t.Errorf("no reviewers should be an empty list, not null: %#v", got)
	}
}

func TestProblem(t *testing.T) {
	c := Default()
	if p := c.Problem(); p != "" {
		t.Fatalf("the default must be valid: %s", p)
	}
	c.DurationMinutes = 5
	if c.Problem() == "" {
		t.Error("5 minutes is too short")
	}
	c = Default()
	c.AssistantMode = "everything"
	if c.Problem() == "" {
		t.Error("unknown assistant mode")
	}
	c = Default()
	c.Reviewers = []string{"fine", "not valid!"}
	if c.Problem() == "" {
		t.Error("a bad reviewer name must be refused")
	}
}

func TestDefaultRequiresAllMonitoring(t *testing.T) {
	c := Default()
	if !c.RequireCamera || !c.RequireScreen || !c.RequireAudio || c.OutsideAgentsAllowed {
		t.Fatalf("%+v", c)
	}
}
