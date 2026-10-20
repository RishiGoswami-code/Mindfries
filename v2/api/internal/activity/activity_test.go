package activity

import (
	"testing"
	"time"
)

func TestValid(t *testing.T) {
	ok := Item{Severity: Flag, Source: Panel, Category: Signal, Text: "x"}
	if !ok.Valid() {
		t.Fatal("a flag from the panel")
	}
	for _, bad := range []Item{
		{Severity: "alarm", Source: Panel, Text: "x"},
		{Severity: Info, Source: "someone", Text: "x"},
		{Severity: Info, Source: Panel, Category: "verdict", Text: "x"},
		{Severity: Info, Source: Panel},
	} {
		if bad.Valid() {
			t.Errorf("%+v should be invalid", bad)
		}
	}
}

func TestHubDeliversToTheRightSession(t *testing.T) {
	h := NewHub()
	a, cancelA := h.Subscribe("s1")
	b, cancelB := h.Subscribe("s2")
	defer cancelB()
	h.Publish("s1", Item{ID: 1, Text: "one"})
	select {
	case it := <-a:
		if it.ID != 1 {
			t.Fatal(it)
		}
	case <-time.After(time.Second):
		t.Fatal("s1 should hear its own item")
	}
	select {
	case it := <-b:
		t.Fatalf("s2 must not hear s1: %v", it)
	default:
	}
	cancelA()
	if h.Subscribers("s1") != 0 {
		t.Fatal("cancel removes the subscriber")
	}
	cancelA() // cancelling twice is harmless
	h.Publish("s1", Item{ID: 2})
}

func TestSlowSubscriberIsClosedNotBlocking(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("s")
	defer cancel()
	for i := 0; i < 200; i++ { // nobody reads
		h.Publish("s", Item{ID: int64(i)})
	}
	n := 0
	for range ch {
		n++
	}
	if n == 0 || n > 64 {
		t.Fatalf("a slow reader gets what fit, then the channel closes: %d", n)
	}
	if h.Subscribers("s") != 0 {
		t.Fatal("the slow subscriber was dropped")
	}
}
