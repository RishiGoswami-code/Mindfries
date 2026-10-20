package support

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event is something that happened to a ticket, delivered to subscribers.
type Event struct {
	Type     string         `json:"type"` // e.g. "ticket.created", "ticket.resolved"
	TicketID string         `json:"ticket_id"`
	At       time.Time      `json:"at"`
	Data     map[string]any `json:"data,omitempty"`
}

// Sender delivers a webhook payload to a URL. HTTP in production; a fake in
// tests. Returning an error triggers a retry.
type Sender interface {
	Send(url string, body []byte) error
}

// Subscription is an endpoint that wants certain event types.
type Subscription struct {
	ID     string
	URL    string
	Events []string // event types; empty means all
}

// Delivery records the outcome of delivering one event to one subscription.
type Delivery struct {
	SubID     string
	EventType string
	Attempts  int
	OK        bool
	LastErr   string
}

// Dispatcher fans ticket events out to subscriptions with bounded retries and
// exponential backoff. It captures V1's telemetry lesson: a failed delivery is
// retried a bounded number of times, never forever.
type Dispatcher struct {
	subs        []Subscription
	sender      Sender
	maxAttempts int
	base        time.Duration
	sleep       func(time.Duration)
}

// NewDispatcher builds a dispatcher. maxAttempts < 1 is treated as 1. base is
// the first backoff interval (doubled each retry). sleep defaults to
// time.Sleep; pass a no-op in tests.
func NewDispatcher(sender Sender, maxAttempts int, base time.Duration, sleep func(time.Duration)) *Dispatcher {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	return &Dispatcher{sender: sender, maxAttempts: maxAttempts, base: base, sleep: sleep}
}

// Subscribe registers an endpoint for the given event types (none = all).
func (d *Dispatcher) Subscribe(id, url string, events ...string) {
	d.subs = append(d.subs, Subscription{ID: id, URL: url, Events: events})
}

// Dispatch delivers ev to every matching subscription and returns one Delivery
// per subscription tried.
func (d *Dispatcher) Dispatch(ev Event) ([]Delivery, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	var out []Delivery
	for _, sub := range d.subs {
		if !wants(sub, ev.Type) {
			continue
		}
		out = append(out, d.deliver(sub, ev.Type, body))
	}
	return out, nil
}

func (d *Dispatcher) deliver(sub Subscription, evType string, body []byte) Delivery {
	dl := Delivery{SubID: sub.ID, EventType: evType}
	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		dl.Attempts = attempt
		err := d.sender.Send(sub.URL, body)
		if err == nil {
			dl.OK = true
			dl.LastErr = ""
			return dl
		}
		dl.LastErr = err.Error()
		if attempt < d.maxAttempts {
			d.sleep(d.base << (attempt - 1)) // base, 2·base, 4·base, …
		}
	}
	return dl
}

func wants(sub Subscription, evType string) bool {
	if len(sub.Events) == 0 {
		return true
	}
	for _, e := range sub.Events {
		if e == evType {
			return true
		}
	}
	return false
}
