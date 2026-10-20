package fakeapi

import "github.com/mindfries/v2/cli/internal/api"

// sessionExt holds what the watcher, assistant and submission parts of the
// contract record per session.
type sessionExt struct {
	lastSeq    int64
	built      *built
	events     map[int64]api.Event
	notes      []string
	snapshots  map[int64]string
	heartbeats []api.Heartbeat
	submission *submission
	bundle     []byte
	assistant  assistantState
	ghRepo     *api.GitHubRepo
	// The candidate's GitHub token, from the OAuth callback, until the CLI
	// collects it; then only ghCollected stays.
	ghToken     string
	ghErr       string
	ghCollected bool
	ghKey       string // the connect URL's key
	ghWaiting   string // the connect URL, while the CLI waits
	// The monitoring panel and the begin request.
	monKey string
	mon    monState
	begin  *api.BeginRequest
}

func newExt() sessionExt {
	return sessionExt{events: map[int64]api.Event{}, snapshots: map[int64]string{}, assistant: assistantState{turns: map[string]*turn{}}}
}
