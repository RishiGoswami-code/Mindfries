// Package evhash computes the hash that chains watcher events together
// (contract: Event). The CLI and the fake API both use it.
package evhash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/canonical"
)

// Genesis is the prev_hash of the first event.
var Genesis = strings.Repeat("0", 64)

// Of returns SHA-256 over the canonical JSON of
// {seq, occurred_at, type, data, prev_hash}.
func Of(e api.Event) (string, error) {
	body := struct {
		Seq        int64           `json:"seq"`
		OccurredAt time.Time       `json:"occurred_at"`
		Type       string          `json:"type"`
		Data       json.RawMessage `json:"data"`
		PrevHash   string          `json:"prev_hash"`
	}{e.Seq, e.OccurredAt.UTC(), e.Type, e.Data, e.PrevHash}
	b, err := canonical.Marshal(body)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
