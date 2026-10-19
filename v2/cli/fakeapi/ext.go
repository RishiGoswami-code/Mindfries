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
	ghLogin    *api.GitHubLogin
	ghRepo     *api.GitHubRepo
}

func newExt() sessionExt {
	return sessionExt{events: map[int64]api.Event{}, snapshots: map[int64]string{}, assistant: assistantState{turns: map[string]*turn{}}}
}
