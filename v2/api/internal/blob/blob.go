// Package blob keeps the bytes of evidence (camera stills, screenshots,
// microphone clips) outside the database. Production uses a private Supabase
// Storage bucket, written only with the service key; tests use Mem. Nothing
// here is ever public: a reviewer gets a short-lived signed address.
package blob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned for a path that holds nothing.
var ErrNotFound = errors.New("blob: not found")

// Store is where evidence bytes live.
type Store interface {
	Put(ctx context.Context, path, contentType string, data []byte) error
	// SignedURL is an address that serves the object for ttl and then stops.
	SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error)
	// Delete removes objects; a path that's already gone isn't an error.
	Delete(ctx context.Context, paths []string) error
}

// Mem is an in-memory Store for tests.
type Mem struct {
	mu      sync.Mutex
	Objects map[string][]byte
	Types   map[string]string
}

func NewMem() *Mem { return &Mem{Objects: map[string][]byte{}, Types: map[string]string{}} }

func (m *Mem) Put(_ context.Context, path, ct string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Objects[path] = append([]byte(nil), data...)
	m.Types[path] = ct
	return nil
}

func (m *Mem) SignedURL(_ context.Context, path string, ttl time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Objects[path]; !ok {
		return "", ErrNotFound
	}
	return "mem://" + path + "?ttl=" + ttl.String(), nil
}

func (m *Mem) Delete(_ context.Context, paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range paths {
		delete(m.Objects, p)
		delete(m.Types, p)
	}
	return nil
}

// Count is how many objects are held.
func (m *Mem) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Objects)
}

// Supabase stores objects in a private bucket through Storage's REST API.
type Supabase struct {
	BaseURL string // https://<project>.supabase.co
	Key     string // the service role key
	Bucket  string
	HTTP    *http.Client
}

func (s Supabase) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (s Supabase) object(path string) string {
	segs := strings.Split(path, "/")
	for i, p := range segs {
		segs[i] = url.PathEscape(p)
	}
	return strings.TrimRight(s.BaseURL, "/") + "/storage/v1/object/" + url.PathEscape(s.Bucket) + "/" + strings.Join(segs, "/")
}

func (s Supabase) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+s.Key)
	req.Header.Set("apikey", s.Key)
	return s.client().Do(req)
}

func (s Supabase) Put(ctx context.Context, path, contentType string, data []byte) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.object(path), bytes.NewReader(data))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")
	res, err := s.do(req)
	if err != nil {
		return fmt.Errorf("blob: put: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("blob: put %s: %d %s", path, res.StatusCode, b)
	}
	return nil
}

func (s Supabase) SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	body, _ := json.Marshal(map[string]int{"expiresIn": int(ttl.Seconds())})
	signURL := strings.Replace(s.object(path), "/object/", "/object/sign/", 1)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, signURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := s.do(req)
	if err != nil {
		return "", fmt.Errorf("blob: sign: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("blob: sign %s: %d", path, res.StatusCode)
	}
	var out struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&out); err != nil || out.SignedURL == "" {
		return "", errors.New("blob: sign: unreadable answer")
	}
	if strings.HasPrefix(out.SignedURL, "/") {
		return strings.TrimRight(s.BaseURL, "/") + "/storage/v1" + out.SignedURL, nil
	}
	return out.SignedURL, nil
}

func (s Supabase) Delete(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string][]string{"prefixes": paths})
	u := strings.TrimRight(s.BaseURL, "/") + "/storage/v1/object/" + url.PathEscape(s.Bucket)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := s.do(req)
	if err != nil {
		return fmt.Errorf("blob: delete: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("blob: delete: %d", res.StatusCode)
	}
	return nil
}
