package watcher

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/evhash"
	"github.com/mindfries/v2/cli/internal/state"
)

// Queue is the offline event queue: an append-only file of events, each
// chained to the one before by its hash, plus a small file recording how
// far the server has acknowledged. It survives crashes and restarts; events
// leave it only once the server has stored them.
type Queue struct {
	mu       sync.Mutex
	path     string
	metaPath string
	lastSeq  int64
	lastHash string
	acked    int64
	// Now stamps events. Tests replace it.
	Now func() time.Time
}

type queueMeta struct {
	Acked int64 `json:"acked"`
}

// OpenQueue opens (or creates) the queue in dir.
func OpenQueue(dir string) (*Queue, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	q := &Queue{path: filepath.Join(dir, "queue.jsonl"), metaPath: filepath.Join(dir, "queue.meta.json"), lastHash: evhash.Genesis, Now: time.Now}
	if b, err := os.ReadFile(q.metaPath); err == nil {
		var m queueMeta
		if json.Unmarshal(b, &m) == nil {
			q.acked = m.Acked
		}
	}
	evs, err := q.readAll()
	if err != nil {
		return nil, err
	}
	if n := len(evs); n > 0 {
		q.lastSeq, q.lastHash = evs[n-1].Seq, evs[n-1].Hash
	} else if q.acked > 0 {
		// Everything was sent and compacted away; carry on after it.
		q.lastSeq = q.acked
		if b, err := os.ReadFile(q.metaPath + ".last"); err == nil {
			q.lastHash = string(bytes.TrimSpace(b))
		}
	}
	return q, nil
}

func (q *Queue) readAll() ([]api.Event, error) {
	f, err := os.Open(q.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []api.Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e api.Event
		if err := json.Unmarshal(line, &e); err != nil {
			// A torn last line from a crash mid-write; everything before it
			// is intact.
			break
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// Append adds an event and writes it to disk before returning.
func (q *Queue) Append(typ string, data any) (api.Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return api.Event{}, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	e := api.Event{Seq: q.lastSeq + 1, OccurredAt: q.Now().UTC(), Type: typ, Data: raw, PrevHash: q.lastHash}
	if e.Hash, err = evhash.Of(e); err != nil {
		return api.Event{}, err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return api.Event{}, err
	}
	f, err := os.OpenFile(q.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return api.Event{}, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return api.Event{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return api.Event{}, err
	}
	if err := f.Close(); err != nil {
		return api.Event{}, err
	}
	q.lastSeq, q.lastHash = e.Seq, e.Hash
	return e, nil
}

// Pending returns unacknowledged events in order, at most maxEvents and
// about maxBytes of JSON.
func (q *Queue) Pending(maxEvents, maxBytes int) ([]api.Event, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	all, err := q.readAll()
	if err != nil {
		return nil, err
	}
	var out []api.Event
	size := 16
	for _, e := range all {
		if e.Seq <= q.acked {
			continue
		}
		b, _ := json.Marshal(e)
		if len(out) > 0 && (len(out) >= maxEvents || size+len(b)+1 > maxBytes) {
			break
		}
		out = append(out, e)
		size += len(b) + 1
	}
	return out, nil
}

// Ack records that the server has stored every event up to seq, and
// compacts the file once enough has been acknowledged.
func (q *Queue) Ack(seq int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if seq <= q.acked {
		return nil
	}
	if seq > q.lastSeq {
		seq = q.lastSeq
	}
	q.acked = seq
	b, _ := json.Marshal(queueMeta{Acked: q.acked})
	if err := state.WriteFileAtomic(q.metaPath, b, 0o600); err != nil {
		return err
	}
	return q.compact()
}

// compact drops acknowledged events from the file. Called with mu held.
func (q *Queue) compact() error {
	all, err := q.readAll()
	if err != nil {
		return err
	}
	keep := all[:0]
	for _, e := range all {
		if e.Seq > q.acked {
			keep = append(keep, e)
		}
	}
	if len(keep) == len(all) {
		return nil
	}
	var buf bytes.Buffer
	for _, e := range keep {
		b, _ := json.Marshal(e)
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := state.WriteFileAtomic(q.metaPath+".last", []byte(q.lastHash), 0o600); err != nil {
		return err
	}
	return state.WriteFileAtomic(q.path, buf.Bytes(), 0o600)
}

// Depth is how many events wait to be sent.
func (q *Queue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int(q.lastSeq - q.acked)
}

// LastSeq is the newest event's sequence number.
func (q *Queue) LastSeq() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lastSeq
}

// Acked is the newest event the server has stored.
func (q *Queue) Acked() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.acked
}

// String describes the queue, for debugging.
func (q *Queue) String() string {
	return fmt.Sprintf("queue(last=%d acked=%d)", q.LastSeq(), q.Acked())
}
