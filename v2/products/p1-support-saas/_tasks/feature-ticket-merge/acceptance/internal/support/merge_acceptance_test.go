package support

import "testing"

// Hidden acceptance tests for the ticket-merge feature. Applied by the
// verification harness only — never shipped to the candidate. Reuses the clk /
// t0 / newSeeded helpers from support_test.go in the same package.

func TestMergeFoldsDuplicateIntoPrimary(t *testing.T) {
	clock, _ := clk(t0)
	s := newSeeded(clock)

	primary, _ := s.Create("Login broken", "u@t.test", PriorityHigh, "I can't log in")
	dup, _ := s.Create("Still can't login", "u@t.test", PriorityNormal, "same problem here")
	s.Reply(dup.ID, AuthorCustomer, "", "any update?")

	before, _ := s.Store().GetTicket(primary.ID)
	dupBefore, _ := s.Store().GetTicket(dup.ID)

	merged, err := s.Merge(dup.ID, primary.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Primary gains the duplicate's messages plus a system merge note.
	wantAtLeast := len(before.Messages) + len(dupBefore.Messages) + 1
	if len(merged.Messages) < wantAtLeast {
		t.Fatalf("primary should hold its own + the duplicate's messages + a merge note; got %d, want >= %d",
			len(merged.Messages), wantAtLeast)
	}

	// The duplicate's content is now on the primary.
	foundBody, foundSystem := false, false
	for _, m := range merged.Messages {
		if m.Body == "same problem here" {
			foundBody = true
		}
		if m.AuthorType == AuthorSystem {
			foundSystem = true
		}
	}
	if !foundBody {
		t.Fatal("the duplicate's message should be moved onto the primary")
	}
	if !foundSystem {
		t.Fatal("a system note should record the merge on the primary")
	}

	// The duplicate is closed.
	d, _ := s.Store().GetTicket(dup.ID)
	if d.Status != StatusClosed {
		t.Fatalf("the duplicate should be closed after merge, got %s", d.Status)
	}
}

func TestMergeGuards(t *testing.T) {
	clock, _ := clk(t0)
	s := newSeeded(clock)
	tk, _ := s.Create("x", "u@t.test", PriorityNormal, "hi")

	if _, err := s.Merge(tk.ID, tk.ID); err == nil {
		t.Fatal("merging a ticket into itself must error")
	}
	if _, err := s.Merge("missing", tk.ID); err == nil {
		t.Fatal("merging a non-existent duplicate must error")
	}
}
