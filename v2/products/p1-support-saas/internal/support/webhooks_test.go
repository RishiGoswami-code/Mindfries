package support

import (
	"errors"
	"testing"
	"time"
)

// flakySender fails the first failUntil-1 calls to any URL, then succeeds.
type flakySender struct {
	calls     int
	failUntil int
}

func (f *flakySender) Send(url string, body []byte) error {
	f.calls++
	if f.calls < f.failUntil {
		return errors.New("temporary failure")
	}
	return nil
}

func TestDispatchRetriesThenSucceeds(t *testing.T) {
	sender := &flakySender{failUntil: 3} // fail, fail, then succeed
	d := NewDispatcher(sender, 3, time.Millisecond, func(time.Duration) {})
	d.Subscribe("s1", "https://hook.test/x")

	dels, err := d.Dispatch(Event{Type: "ticket.created", TicketID: "TCK-1", At: t0})
	if err != nil {
		t.Fatal(err)
	}
	if len(dels) != 1 || !dels[0].OK || dels[0].Attempts != 3 {
		t.Fatalf("expected success on the 3rd attempt, got %+v", dels)
	}
}

func TestDispatchGivesUpAfterMaxAttempts(t *testing.T) {
	sender := &flakySender{failUntil: 99} // always fails
	d := NewDispatcher(sender, 2, time.Millisecond, func(time.Duration) {})
	d.Subscribe("s1", "https://hook.test/x")

	dels, _ := d.Dispatch(Event{Type: "ticket.created", TicketID: "TCK-1", At: t0})
	if dels[0].OK || dels[0].Attempts != 2 {
		t.Fatalf("expected give-up after 2 attempts, got %+v", dels)
	}
}

func TestDispatchFiltersByEventType(t *testing.T) {
	sender := &flakySender{failUntil: 1} // always succeeds
	d := NewDispatcher(sender, 1, 0, func(time.Duration) {})
	d.Subscribe("only-resolved", "https://hook.test/r", "ticket.resolved")

	// A created event should not reach a resolved-only subscription.
	dels, _ := d.Dispatch(Event{Type: "ticket.created", TicketID: "TCK-1", At: t0})
	if len(dels) != 0 {
		t.Fatalf("created event should not match resolved-only sub, got %+v", dels)
	}
	dels, _ = d.Dispatch(Event{Type: "ticket.resolved", TicketID: "TCK-1", At: t0})
	if len(dels) != 1 || !dels[0].OK {
		t.Fatalf("resolved event should be delivered, got %+v", dels)
	}
}
