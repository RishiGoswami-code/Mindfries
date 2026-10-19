package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// VersionHeader is sent on every request.
const VersionHeader = "Mindfries-CLI-Version"

// Client talks to the V2 API. Token is empty until the setup code has been
// exchanged.
type Client struct {
	BaseURL    string
	Token      string
	CLIVersion string
	HTTP       *http.Client
}

func New(baseURL, cliVersion string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		CLIVersion: cliVersion,
		HTTP:       &http.Client{Timeout: 60 * time.Second},
	}
}

// AsProblem returns the contract problem inside err, if there is one.
func AsProblem(err error) (*Problem, bool) {
	var p *Problem
	if errors.As(err, &p) {
		return p, true
	}
	return nil, false
}

// HasCode reports whether err is a problem with one of the codes.
func HasCode(err error, codes ...string) bool {
	p, ok := AsProblem(err)
	if !ok {
		return false
	}
	for _, c := range codes {
		if p.Code == c {
			return true
		}
	}
	return false
}

// IsAuthFailure reports whether the token is no longer usable.
func IsAuthFailure(err error) bool {
	return HasCode(err, CodeTokenMissing, CodeTokenInvalid, CodeTokenExpired, CodeTokenRevoked)
}

// IsRetryable reports whether a request may succeed if sent again later:
// network errors, 5xx and rate limits. A problem with a 4xx status other
// than 429 is not.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if p, ok := AsProblem(err); ok {
		return p.Status >= 500 || p.Status == http.StatusTooManyRequests
	}
	return !errors.Is(err, context.Canceled)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(VersionHeader, c.CLIVersion)
	req.Header.Set("User-Agent", "mindfries-cli/"+c.CLIVersion)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func (c *Client) send(req *http.Request) (*http.Response, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, readProblem(resp)
	}
	return resp, nil
}

func readProblem(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	p := &Problem{}
	if err := json.Unmarshal(body, p); err != nil || p.Code == "" {
		p = &Problem{Code: CodeInternal, Title: http.StatusText(resp.StatusCode), Status: resp.StatusCode}
		if s := strings.TrimSpace(string(body)); s != "" && len(s) < 300 {
			p.Detail = s
		}
	}
	if p.Status == 0 {
		p.Status = resp.StatusCode
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if n, err := strconv.Atoi(ra); err == nil {
			p.RetryAfter = time.Duration(n) * time.Second
		}
	}
	return p
}

func (c *Client) doJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	ct := ""
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
		ct = "application/json"
	}
	req, err := c.newRequest(ctx, method, path, body, ct)
	if err != nil {
		return err
	}
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("reading %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	var v VersionInfo
	return &v, c.doJSON(ctx, http.MethodGet, "/version", nil, &v)
}

func (c *Client) Exchange(ctx context.Context, in ExchangeRequest) (*ExchangeResponse, error) {
	var out ExchangeResponse
	return &out, c.doJSON(ctx, http.MethodPost, "/sessions/exchange", in, &out)
}

func (c *Client) Session(ctx context.Context) (*Session, error) {
	var s Session
	return &s, c.doJSON(ctx, http.MethodGet, "/session", nil, &s)
}

func (c *Client) Revoke(ctx context.Context, reason string) error {
	return c.doJSON(ctx, http.MethodPost, "/session/revoke", RevokeRequest{Reason: reason}, nil)
}

// BundleManifest returns the manifest and the exact bytes received, which
// the signature check needs.
func (c *Client) BundleManifest(ctx context.Context) (*BundleManifest, []byte, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/session/bundle", nil, &raw); err != nil {
		return nil, nil, err
	}
	var m BundleManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, fmt.Errorf("reading the task manifest: %w", err)
	}
	return &m, raw, nil
}

// BundleArchive writes the archive to w, starting at offset bytes in (for a
// resumed download). It returns the number of bytes written.
func (c *Client) BundleArchive(ctx context.Context, w io.Writer, offset int64) (int64, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/session/bundle/archive", nil, "")
	if err != nil {
		return 0, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.send(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if offset > 0 && resp.StatusCode != http.StatusPartialContent {
		return 0, ErrRangeIgnored
	}
	return io.Copy(w, resp.Body)
}

// ErrRangeIgnored means the server sent the whole archive instead of the
// requested range, so the caller must start the download again.
var ErrRangeIgnored = errors.New("server ignored the range request")

func (c *Client) Heartbeat(ctx context.Context, hb Heartbeat) (*HeartbeatReply, error) {
	var r HeartbeatReply
	return &r, c.doJSON(ctx, http.MethodPost, "/session/heartbeat", hb, &r)
}

func (c *Client) SendEvents(ctx context.Context, batch EventBatch) (*EventBatchReply, error) {
	var r EventBatchReply
	return &r, c.doJSON(ctx, http.MethodPost, "/session/events", batch, &r)
}

func (c *Client) SendSnapshot(ctx context.Context, s Snapshot) error {
	return c.doJSON(ctx, http.MethodPost, "/session/snapshots", s, nil)
}

func (c *Client) CreateSubmission(ctx context.Context, m SubmissionManifest) (*Submission, error) {
	var s Submission
	return &s, c.doJSON(ctx, http.MethodPost, "/session/submission", m, &s)
}

func (c *Client) GetSubmission(ctx context.Context) (*Submission, error) {
	var s Submission
	return &s, c.doJSON(ctx, http.MethodGet, "/session/submission", nil, &s)
}

func (c *Client) UploadBundle(ctx context.Context, submissionID string, r io.Reader, size int64) (*Submission, error) {
	req, err := c.newRequest(ctx, http.MethodPut, "/session/submission/"+submissionID+"/bundle", r, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var s Submission
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// StartTurn sends the candidate's message and returns the reply stream.
func (c *Client) StartTurn(ctx context.Context, in AssistantTurnRequest) (*Stream, error) {
	return c.stream(ctx, "/session/assistant/turns", in)
}

// SendToolResults continues a turn that ended with tool.calls_pending.
func (c *Client) SendToolResults(ctx context.Context, turnID string, in ToolResults) (*Stream, error) {
	return c.stream(ctx, "/session/assistant/turns/"+turnID+"/tool-results", in)
}

func (c *Client) stream(ctx context.Context, path string, in any) (*Stream, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(b), "application/json")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	// A turn can take longer than the client's normal timeout.
	hc := *c.HTTP
	hc.Timeout = 0
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, readProblem(resp)
	}
	return &Stream{body: resp.Body, r: bufio.NewReader(resp.Body)}, nil
}

// Stream reads server-sent assistant events.
type Stream struct {
	body io.ReadCloser
	r    *bufio.Reader
}

// Next returns the next event, or io.EOF when the stream has ended.
func (s *Stream) Next() (StreamEvent, error) {
	var data strings.Builder
	event := ""
	parse := func() (StreamEvent, error) {
		var ev StreamEvent
		if err := json.Unmarshal([]byte(data.String()), &ev); err != nil {
			return StreamEvent{}, fmt.Errorf("bad stream event: %w", err)
		}
		if ev.Type == "" {
			ev.Type = event
		}
		return ev, nil
	}
	for {
		line, err := s.r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if data.Len() > 0 {
				return parse()
			}
		case strings.HasPrefix(line, ":"):
			// comment or keep-alive
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if err != nil {
			// The last event may lack its trailing blank line.
			if data.Len() > 0 {
				return parse()
			}
			return StreamEvent{}, err
		}
	}
}

func (s *Stream) Close() error { return s.body.Close() }
