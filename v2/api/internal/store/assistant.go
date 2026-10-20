package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/assistant"
)

// The assistant's turns and steps (v2.assistant_turns, v2.assistant_steps)
// and what they cost (v2.model_usage).

// Turn is one prompt and the model's whole reply.
type Turn struct {
	ID             string
	SessionID      string
	ConversationID string
	Channel        string // agent | avatar
	Prompt         string
	Status         string // running | waiting_tools | completed | failed | cancelled
	StartedAt      time.Time
	InputTokens    int
	OutputTokens   int
}

var ErrTurnNotFound = errors.New("turn not found")

func (d *DB) CreateTurn(ctx context.Context, t Turn) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.assistant_turns (id, session_id, conversation_id, channel, prompt, status, started_at)
		 values ($1, $2, $3, $4, $5, 'running', $6)`,
		t.ID, t.SessionID, t.ConversationID, t.Channel, t.Prompt, t.StartedAt)
	return err
}

func (d *DB) TurnByID(ctx context.Context, id string) (Turn, error) {
	var t Turn
	err := d.pool.QueryRow(ctx,
		`select id, session_id, conversation_id, channel, prompt, status, started_at, input_tokens, output_tokens
		   from v2.assistant_turns where id = $1`, id,
	).Scan(&t.ID, &t.SessionID, &t.ConversationID, &t.Channel, &t.Prompt, &t.Status, &t.StartedAt, &t.InputTokens, &t.OutputTokens)
	if errors.Is(err, pgx.ErrNoRows) {
		return Turn{}, ErrTurnNotFound
	}
	return t, err
}

// SetTurnStatus moves a turn on; completed, failed and cancelled end it.
func (d *DB) SetTurnStatus(ctx context.Context, id, status, errCode string, at time.Time) error {
	_, err := d.pool.Exec(ctx,
		`update v2.assistant_turns set status = $2, error_code = nullif($3, ''),
		   ended_at = case when $2 in ('completed', 'failed', 'cancelled') then $4::timestamptz else ended_at end
		 where id = $1`, id, status, errCode, at)
	return err
}

// ClaimTurn moves a waiting turn back to running, once, so two
// tool-result posts for the same turn can't both continue it.
func (d *DB) ClaimTurn(ctx context.Context, id string) (bool, error) {
	ct, err := d.pool.Exec(ctx, `update v2.assistant_turns set status = 'running' where id = $1 and status = 'waiting_tools'`, id)
	return ct.RowsAffected() == 1, err
}

// AddStep appends a step to a turn.
func (d *DB) AddStep(ctx context.Context, turnID, kind, tool string, body json.RawMessage, at time.Time) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.assistant_steps (turn_id, step, kind, tool, body, at)
		 values ($1, coalesce((select max(step) from v2.assistant_steps where turn_id = $1), 0) + 1, $2, nullif($3, ''), $4::jsonb, $5)`,
		turnID, kind, tool, string(body), at)
	return err
}

// ConversationSteps are a conversation's steps in order, as the model's history.
func (d *DB) ConversationSteps(ctx context.Context, sessionID, conversationID string) ([]assistant.Step, error) {
	rows, err := d.pool.Query(ctx,
		`select s.kind, s.body from v2.assistant_steps s join v2.assistant_turns t on t.id = s.turn_id
		  where t.session_id = $1 and t.conversation_id = $2 and s.kind in ('transcript_in', 'transcript_out', 'tool_call', 'tool_result')
		  order by t.started_at, t.id, s.step`, sessionID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []assistant.Step
	for rows.Next() {
		var st assistant.Step
		var body []byte
		if err := rows.Scan(&st.Kind, &body); err != nil {
			return nil, err
		}
		st.Body = json.RawMessage(body)
		out = append(out, st)
	}
	return out, rows.Err()
}

// PendingCalls are the turn's tool calls that have no result yet.
func (d *DB) PendingCalls(ctx context.Context, turnID string) ([]string, error) {
	rows, err := d.pool.Query(ctx,
		`select c.body->>'call_id' from v2.assistant_steps c
		  where c.turn_id = $1 and c.kind = 'tool_call'
		    and not exists (select 1 from v2.assistant_steps r where r.turn_id = c.turn_id and r.kind = 'tool_result'
		                       and r.body->>'call_id' = c.body->>'call_id')
		  order by c.step`, turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ConversationKnown reports whether a session already has a conversation.
func (d *DB) ConversationKnown(ctx context.Context, sessionID, conversationID string) (bool, error) {
	var ok bool
	err := d.pool.QueryRow(ctx,
		`select exists(select 1 from v2.assistant_turns where session_id = $1 and conversation_id = $2)`, sessionID, conversationID).Scan(&ok)
	return ok, err
}

// RecordUsage adds what a model call cost to its turn and to the ledger.
func (d *DB) RecordUsage(ctx context.Context, turnID, sessionID, purpose, model string, in, out int, at time.Time) error {
	if turnID != "" {
		if _, err := d.pool.Exec(ctx, `update v2.assistant_turns set input_tokens = input_tokens + $2, output_tokens = output_tokens + $3 where id = $1`, turnID, in, out); err != nil {
			return err
		}
	}
	_, err := d.pool.Exec(ctx,
		`insert into v2.model_usage (session_id, purpose, model, input_tokens, output_tokens, at) values (nullif($1, ''), $2, $3, $4, $5, $6)`,
		sessionID, purpose, model, in, out, at)
	return err
}

// SessionTokens is everything the assistant and the avatar have used in a session.
func (d *DB) SessionTokens(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := d.pool.QueryRow(ctx, `select coalesce(sum(input_tokens + output_tokens), 0) from v2.model_usage where session_id = $1`, sessionID).Scan(&n)
	return n, err
}

// TokensSince is all model use (any session) since a time, for the daily cap.
func (d *DB) TokensSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := d.pool.QueryRow(ctx, `select coalesce(sum(input_tokens + output_tokens), 0) from v2.model_usage where at >= $1`, since).Scan(&n)
	return n, err
}

// The avatar's conversation is one long "turn" per session in the same tables
// as the terminal assistant's, tagged channel 'avatar', so a reviewer reads one
// record. Utterances carry the browser's own sequence numbers; ones already
// stored are skipped, so a retried upload doesn't double the transcript.

// AvatarTurnID is the id of a session's avatar conversation.
func AvatarTurnID(sessionID string) string { return "avt_" + sessionID }

// AddUtterances stores new utterances and returns how many were new.
func (d *DB) AddUtterances(ctx context.Context, sessionID string, us []assistant.Utterance, at time.Time) (int, error) {
	id := AvatarTurnID(sessionID)
	if _, err := d.pool.Exec(ctx,
		`insert into v2.assistant_turns (id, session_id, conversation_id, channel, prompt, status, started_at)
		 values ($1, $2, 'avatar', 'avatar', '', 'running', $3) on conflict (id) do nothing`, id, sessionID, at); err != nil {
		return 0, err
	}
	var last int
	if err := d.pool.QueryRow(ctx,
		`select coalesce(max((body->>'seq')::int), 0) from v2.assistant_steps where turn_id = $1`, id).Scan(&last); err != nil {
		return 0, err
	}
	stored := 0
	for _, u := range us {
		if u.Seq <= last {
			continue
		}
		kind := assistant.KindOut
		if u.Role == "candidate" {
			kind = assistant.KindIn
		}
		body, _ := json.Marshal(map[string]any{"text": u.Text, "seq": u.Seq, "role": u.Role, "at": u.At})
		if err := d.AddStep(ctx, id, kind, "", body, at); err != nil {
			return stored, err
		}
		last = u.Seq
		stored++
	}
	return stored, nil
}

// AvatarTranscript is what was said, in order.
func (d *DB) AvatarTranscript(ctx context.Context, sessionID string) ([]assistant.Utterance, error) {
	rows, err := d.pool.Query(ctx, `select body from v2.assistant_steps where turn_id = $1 order by step`, AvatarTurnID(sessionID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []assistant.Utterance{}
	for rows.Next() {
		var body []byte
		var u assistant.Utterance
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if json.Unmarshal(body, &u) == nil {
			out = append(out, u)
		}
	}
	return out, rows.Err()
}
