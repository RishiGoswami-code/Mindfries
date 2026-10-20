package fakeapi

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"sort"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/evhash"
)

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/heartbeat", s.authed(s.handleHeartbeat))
		route("POST /session/events", s.authed(s.handleEvents))
		route("POST /session/snapshots", s.authed(s.handleSnapshot))
	})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request, ss *session) {
	var hb api.Heartbeat
	if !decode(w, r, &hb) {
		return
	}
	ss.ext.heartbeats = append(ss.ext.heartbeats, hb)
	reply := api.HeartbeatReply{
		ServerTime: s.now(), SessionStatus: ss.Status, EndsAt: ss.EndsAt,
		Stop: ss.Status != api.StatusActive,
	}
	if ss.Monitoring.Any() {
		st := s.monitoringLocked(ss)
		reply.Monitoring = &st
	}
	writeJSON(w, http.StatusOK, reply)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, ss *session) {
	var b api.EventBatch
	if !decode(w, r, &b) {
		return
	}
	if !requireActive(w, ss) {
		return
	}
	if len(b.Events) == 0 || len(b.Events) > ss.Limits.MaxEventsPerBatch {
		problem(w, http.StatusRequestEntityTooLarge, api.CodeTooLarge, "Too many events in one batch", "")
		return
	}
	if s.tamper.refuseBatches > 0 {
		s.tamper.refuseBatches--
		problem(w, http.StatusBadRequest, api.CodeMalformed, "Batch refused (scripted)", "")
		return
	}
	reply := api.EventBatchReply{}
	for _, e := range b.Events {
		if e.Seq < 1 || e.Type == "" {
			problem(w, http.StatusBadRequest, api.CodeMalformed, "Bad event", "")
			return
		}
		if _, dup := ss.ext.events[e.Seq]; dup {
			reply.Duplicates++
			continue
		}
		if want, err := evhash.Of(e); err != nil || want != e.Hash {
			ss.ext.notes = append(ss.ext.notes, "event hash mismatch")
		}
		if prev, ok := ss.ext.events[e.Seq-1]; ok && prev.Hash != e.PrevHash {
			ss.ext.notes = append(ss.ext.notes, "event chain broken")
		}
		if e.Seq > ss.ext.lastSeq+1 && ss.ext.lastSeq > 0 {
			ss.ext.notes = append(ss.ext.notes, "event gap")
		}
		ss.ext.events[e.Seq] = e
		if e.Seq > ss.ext.lastSeq {
			ss.ext.lastSeq = e.Seq
		}
		reply.Stored++
	}
	reply.AckedThroughSeq = ss.ext.lastSeq
	if s.tamper.dropReplies > 0 {
		// Stored, but the reply never arrives.
		s.tamper.dropReplies--
		hangUp(w)
		return
	}
	writeJSON(w, http.StatusOK, reply)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request, ss *session) {
	var snap api.Snapshot
	if !decode(w, r, &snap) {
		return
	}
	if !requireActive(w, ss) {
		return
	}
	if len(snap.Patch) > ss.Limits.MaxSnapshotBytes {
		problem(w, http.StatusRequestEntityTooLarge, api.CodeTooLarge, "Snapshot too large", "")
		return
	}
	zr, err := gzip.NewReader(bytes.NewReader(snap.Patch))
	if err != nil {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "Patch isn't gzip", err.Error())
		return
	}
	plain, err := io.ReadAll(zr)
	if err != nil || sha(plain) != snap.PatchSHA256 {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "Patch hash doesn't match", "")
		return
	}
	if _, dup := ss.ext.snapshots[snap.Seq]; !dup {
		ss.ext.snapshots[snap.Seq] = string(plain)
	}
	w.WriteHeader(http.StatusNoContent)
}

// Events returns a session's stored events in order.
func (s *Server) Events(id string) []api.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.sessions[id]
	if ss == nil {
		return nil
	}
	out := make([]api.Event, 0, len(ss.ext.events))
	for _, e := range ss.ext.events {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// Notes returns integrity notes the fake recorded (chain breaks, gaps).
func (s *Server) Notes(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		return append([]string(nil), ss.ext.notes...)
	}
	return nil
}

// Snapshots returns a session's diff snapshots by seq.
func (s *Server) Snapshots(id string) map[int64]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[int64]string{}
	if ss := s.sessions[id]; ss != nil {
		for k, v := range ss.ext.snapshots {
			out[k] = v
		}
	}
	return out
}

// Heartbeats returns how many heartbeats a session sent.
func (s *Server) Heartbeats(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		return len(ss.ext.heartbeats)
	}
	return 0
}

// RefuseBatches makes the next n event batches fail as malformed.
func (s *Server) RefuseBatches(n int) { s.mu.Lock(); s.tamper.refuseBatches = n; s.mu.Unlock() }

// DropReplies stores the next n event batches but hangs up before replying.
func (s *Server) DropReplies(n int) { s.mu.Lock(); s.tamper.dropReplies = n; s.mu.Unlock() }
