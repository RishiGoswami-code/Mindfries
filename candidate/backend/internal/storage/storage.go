// Package storage reads objects from Supabase Storage — the proctoring
// stills, so the vision describer can caption them. It is read-only: it
// fetches an object's bytes from a private bucket using the service-role key,
// and never writes or deletes.
package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxObjectBytes caps a fetched still; a proctoring JPEG is tens of KB, so
// this only guards against a surprise, never a real image.
const maxObjectBytes = 20 << 20 // 20 MB

// Client fetches objects from Supabase Storage buckets.
type Client struct {
	baseURL    string // e.g. https://<ref>.supabase.co (no trailing slash)
	serviceKey string
	http       *http.Client
}

// New returns a Client, or nil when storage isn't configured (no URL or key).
// Callers treat a nil Client as "captions off", not an error — the flags
// still show their stills to the reviewer, just without an auto-description.
func New(baseURL, serviceKey string) *Client {
	if baseURL == "" || serviceKey == "" {
		return nil
	}
	return &Client{baseURL: baseURL, serviceKey: serviceKey, http: &http.Client{Timeout: 30 * time.Second}}
}

// Configured reports whether a usable client was built.
func (c *Client) Configured() bool { return c != nil }

// Fetch downloads one object from a private bucket, returning its bytes and
// the server's content type (e.g. "image/jpeg").
func (c *Client) Fetch(ctx context.Context, bucket, path string) ([]byte, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("storage: not configured")
	}
	url := c.baseURL + "/storage/v1/object/" + bucket + "/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("storage: fetching %s/%s: %w", bucket, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("storage: %s/%s returned %d", bucket, path, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxObjectBytes))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}
