package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// sendEvents posts a batch of watcher events as the CLI does.
func (e *env) sendEvents(cli string, evs ...map[string]any) reply {
	e.t.Helper()
	return e.cli("POST", "/session/events", cli, map[string]any{"events": evs})
}

func event(seq int, typ string, data map[string]any) map[string]any {
	return map[string]any{"seq": seq, "occurred_at": time.Now().UTC().Format(time.RFC3339), "type": typ, "data": data, "prev_hash": "", "hash": ""}
}

func startActive(t *testing.T, e *env) (g map[string]any, cli, sid, panel string) {
	t.Helper()
	g = newGrant()
	cli, sid = e.startSession(g)
	panel = e.panelToken(g, sid)
	e.shareAll(panel)
	if r := e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"}); r.Status != 200 {
		t.Fatalf("begin: %d %s", r.Status, r.Raw)
	}
	return
}

func TestWatcherEventsBecomeTrackerLines(t *testing.T) {
	e := newEnv(t, nil)
	_, cli, _, panel := startActive(t, e)

	r := e.sendEvents(cli,
		event(1, "workspace.ready", map[string]any{"task_version": "v1", "base_commit": "abc", "resumed": false}),
		event(2, "file.changed", map[string]any{"path": "a.go", "change": "modified", "lines_added": 3, "lines_removed": 0, "bytes_after": 10}),
		event(3, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "claude-code", "process_name": "claude.exe"}}}),
		event(4, "git.commit", map[string]any{"commit": "3f2a91c0", "files_changed": 2}),
	)
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// The same batch again (a restarted watcher): stored as duplicates, no repeats in the tracker.
	e.sendEvents(cli, event(3, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "claude-code", "process_name": "claude.exe"}}}))

	got := e.panel("GET", "/v2/panel/activity", panel, nil).JSON()
	items := got["items"].([]any)
	var texts []string
	for _, it := range items {
		texts = append(texts, it.(map[string]any)["text"].(string))
	}
	all := strings.Join(texts, "\n")
	for _, want := range []string{"Assessment started", "Working in VS Code", "set up on your computer", "claude.exe is running", "Commit 3f2a91c: 2 files"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Count(all, "claude.exe is running") != 1 {
		t.Errorf("a re-sent event must show once:\n%s", all)
	}
	if strings.Contains(all, "a.go") {
		t.Errorf("routine file changes are not tracker lines:\n%s", all)
	}
	counts := got["counts"].(map[string]any)
	if counts["flag"].(float64) != 1 {
		t.Errorf("counts: %v", counts)
	}
	// Items carry what the panel needs.
	for _, it := range items {
		m := it.(map[string]any)
		if m["id"] == nil || m["at"] == nil || m["severity"] == nil || m["source"] == nil {
			t.Errorf("incomplete item: %v", m)
		}
	}

	// ?after= resumes.
	first := int64(items[0].(map[string]any)["id"].(float64))
	rest := e.panel("GET", "/v2/panel/activity?after="+itoa(first), panel, nil).JSON()["items"].([]any)
	if len(rest) != len(items)-1 {
		t.Errorf("after the first: %d of %d", len(rest), len(items))
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// sseClient reads a Server-Sent Events stream.
type sseClient struct {
	t      *testing.T
	body   *bufio.Reader
	cancel context.CancelFunc
}

type sseMsg struct{ ID, Event, Data string }

func (e *env) feed(path, token, lastEventID string) *sseClient {
	e.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", e.ts.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		e.t.Fatal(err)
	}
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		cancel()
		e.t.Fatalf("feed: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	e.t.Cleanup(func() { cancel(); res.Body.Close() })
	return &sseClient{t: e.t, body: bufio.NewReader(res.Body), cancel: cancel}
}

// next reads the next message with an id or an event name, within a timeout.
func (c *sseClient) next() sseMsg {
	c.t.Helper()
	type result struct {
		m   sseMsg
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var m sseMsg
		for {
			line, err := c.body.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == "":
				if m.Event != "" || m.Data != "" {
					ch <- result{m: m}
					return
				}
			case strings.HasPrefix(line, "id: "):
				m.ID = line[4:]
			case strings.HasPrefix(line, "event: "):
				m.Event = line[7:]
			case strings.HasPrefix(line, "data: "):
				m.Data = line[6:]
			}
		}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			c.t.Fatalf("stream: %v", r.err)
		}
		return r.m
	case <-time.After(5 * time.Second):
		c.t.Fatal("no message within 5s")
		return sseMsg{}
	}
}

func TestLiveFeedBacklogThenNewItemsThenResume(t *testing.T) {
	e := newEnv(t, nil)
	_, cli, _, panel := startActive(t, e)

	c := e.feed("/v2/panel/feed", panel, "")
	var lastID string
	for { // the backlog (what happened before the page opened), then "ready"
		m := c.next()
		if m.Event == "ready" {
			break
		}
		if m.Event != "item" || m.ID == "" {
			t.Fatalf("%+v", m)
		}
		lastID = m.ID
	}
	if lastID == "" {
		t.Fatal("the backlog should hold at least the start of the assessment")
	}

	// A new event reaches the open feed live.
	e.sendEvents(cli, event(1, "task_file.modified", map[string]any{"path": "TASK.md", "restored": true}))
	m := c.next()
	var it map[string]any
	_ = json.Unmarshal([]byte(m.Data), &it)
	if m.Event != "item" || it["severity"] != "flag" || !strings.Contains(it["text"].(string), "TASK.md was changed") {
		t.Fatalf("live item: %+v", m)
	}
	c.cancel()

	// While the page was away, more happened; reconnecting with the last id gets exactly that.
	e.sendEvents(cli, event(2, "git.commit", map[string]any{"commit": "deadbeef12", "files_changed": 1}))
	e.sendEvents(cli, event(3, "watcher.stopped", map[string]any{"reason": "stop"}))
	c2 := e.feed("/v2/panel/feed", panel, m.ID)
	a, b := c2.next(), c2.next()
	if !strings.Contains(a.Data, "Commit deadbee") || !strings.Contains(b.Data, "stopped (stop)") {
		t.Fatalf("resume should replay only what was missed:\n%s\n%s", a.Data, b.Data)
	}
	if r := c2.next(); r.Event != "ready" {
		t.Fatalf("%+v", r)
	}
}

func TestFeedNeedsAPanelTokenForThatSession(t *testing.T) {
	e := newEnv(t, nil)
	_, _, _, panel := startActive(t, e)
	if r := e.do("GET", "/v2/panel/feed", nil, nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", r.Status)
	}
	// EventSource can't set headers, so the token may come in the query.
	res, err := http.Get(e.ts.URL + "/v2/panel/feed?token=" + panel)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("query token: %v %v", res, err)
	}
	res.Body.Close()
	// Another session's feed is another token's: it never shows this one's items.
	_, _, _, other := startActive(t, e)
	o := e.panel("GET", "/v2/panel/activity", other, nil).JSON()["items"].([]any)
	mine := e.panel("GET", "/v2/panel/activity", panel, nil).JSON()["items"].([]any)
	if len(o) == 0 || len(mine) == 0 {
		t.Fatal("both sessions have their own start line")
	}
	for _, a := range o {
		for _, b := range mine {
			if a.(map[string]any)["id"] == b.(map[string]any)["id"] {
				t.Fatal("sessions must not share tracker items")
			}
		}
	}
}
