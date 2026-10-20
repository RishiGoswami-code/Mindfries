package assistant

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ErrDomain is returned when a page isn't on the assessment's research list.
var ErrDomain = errors.New("that site isn't an allowed research source")

// Researcher fetches a page for the assistant, on the server. It exists so a
// candidate can have the assistant look things up without the assessment
// opening the whole web: only listed sites, only https, never an address
// inside our network, a size cap, and every redirect checked against the list.
type Researcher struct {
	Allowed []string // empty: DefaultResearch
	// AllowPrivate lets tests reach a local server. Never set in production.
	AllowPrivate bool
	HTTP         *http.Client
}

const (
	maxPage    = 300 << 10
	maxExcerpt = 8 << 10
)

var (
	scriptRe = regexp.MustCompile(`(?is)<(script|style|noscript|svg|nav|footer|header)[^>]*>.*?</(script|style|noscript|svg|nav|footer|header)>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRe  = regexp.MustCompile(`[ \t\r\f\v]+`)
	nlRe     = regexp.MustCompile(`\n{3,}`)
	titleRe  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

func (r Researcher) client() *http.Client {
	if r.HTTP != nil {
		return r.HTTP
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, ip := range ips {
					if !r.AllowPrivate && !publicIP(ip.IP) {
						return nil, fmt.Errorf("refusing to connect to %s", ip.IP)
					}
				}
				if len(ips) == 0 {
					return nil, errors.New("no address")
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
			},
			TLSHandshakeTimeout: 5 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 3 {
				return errors.New("too many redirects")
			}
			if err := r.check(req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

// publicIP is false for loopback, private, link-local, multicast and the
// unspecified address: nothing the assistant researches may point inside.
func publicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified())
}

func (r Researcher) check(u *url.URL) error {
	if u.Scheme != "https" && !(r.AllowPrivate && u.Scheme == "http") {
		return fmt.Errorf("only https pages: %w", ErrDomain)
	}
	if u.User != nil || !AllowedDomain(u.Hostname(), r.Allowed) {
		return ErrDomain
	}
	return nil
}

// Fetch returns the page's title and a plain-text excerpt.
func (r Researcher) Fetch(ctx context.Context, raw string) (title, text string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", errors.New("not a web address")
	}
	if err := r.check(u); err != nil {
		return "", "", err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "MindfriesAssessment/1 (documentation lookup)")
	req.Header.Set("Accept", "text/html,text/plain;q=0.9")
	res, err := r.client().Do(req)
	if err != nil {
		if errors.Is(err, ErrDomain) {
			return "", "", ErrDomain
		}
		return "", "", fmt.Errorf("couldn't fetch the page: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("the site answered %d", res.StatusCode)
	}
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	if ct != "" && !strings.HasPrefix(ct, "text/html") && !strings.HasPrefix(ct, "text/plain") && !strings.HasPrefix(ct, "application/xhtml") {
		return "", "", errors.New("that isn't a readable page")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxPage))
	if err != nil {
		return "", "", err
	}
	page := string(body)
	if m := titleRe.FindStringSubmatch(page); m != nil {
		title = strings.TrimSpace(html.UnescapeString(spaceRe.ReplaceAllString(m[1], " ")))
	}
	page = scriptRe.ReplaceAllString(page, " ")
	page = tagRe.ReplaceAllString(page, "\n")
	page = html.UnescapeString(page)
	page = spaceRe.ReplaceAllString(page, " ")
	lines := strings.Split(page, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	page = nlRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	page = strings.TrimSpace(page)
	if len(page) > maxExcerpt {
		cut := maxExcerpt
		for cut > 0 && !isBoundary(page, cut) {
			cut--
		}
		page = page[:cut] + "\n…"
	}
	if title == "" {
		title = u.Hostname()
	}
	return title, page, nil
}
