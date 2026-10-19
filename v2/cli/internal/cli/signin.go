package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/state"
)

// Browser sign-in: what `mindfries start` does when there's no session on
// this computer and no setup code was given. The CLI asks for a sign-in,
// opens the browser at the Mindfries page with a short code, and polls
// until the candidate signs in there, checks the code and picks the
// assessment they were invited to. The CLI only ever receives that one
// session's token: no account credential, nothing in shell history.

// How long to keep retrying while the API doesn't answer at all: a server
// that has been asleep (the hosted fake API) can take a minute to wake.
var (
	wakeFor   = 2 * time.Minute
	wakeEvery = 3 * time.Second
)

// waking runs call, retrying network failures and 5xx answers for up to
// wakeFor, and says once that it is still trying.
func (a *App) waking(ctx context.Context, call func() error) error {
	deadline := time.Now().Add(wakeFor)
	told := false
	for {
		err := call()
		if err == nil || !api.IsRetryable(err) || time.Now().After(deadline) {
			return err
		}
		if !told {
			fmt.Fprintln(a.Stdout, "Mindfries is slow to answer (a server that has been idle can take a minute to wake up). Still trying...")
			told = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wakeEvery):
		}
	}
}

// signIn runs the browser sign-in against apiURL and records the session.
func (a *App) signIn(ctx context.Context, apiURL string, noBrowser bool) (*state.Local, *api.Client, error) {
	c := api.New(apiURL, buildinfo.Version)
	var da *api.DeviceAuthorization
	err := a.waking(ctx, func() (err error) {
		da, err = c.StartSignIn(ctx, api.DeviceRequest{Device: a.device()})
		return err
	})
	if err != nil {
		if p, ok := api.AsProblem(err); ok && p.Code == api.CodeCLITooOld {
			return nil, nil, tooOld(p)
		}
		return nil, nil, fmt.Errorf("couldn't reach Mindfries to sign in: %w\nCheck your connection and run 'mindfries start' again", err)
	}
	link := da.VerificationURIComplete
	if link == "" {
		link = da.VerificationURI
	}
	if !webLink(link) {
		return nil, nil, fmt.Errorf("Mindfries sent a sign-in address that isn't a web page (%q); not opening it", link)
	}

	fmt.Fprintln(a.Stdout, "\nSign in to connect this computer to your assessment.")
	opened := !noBrowser && a.Getenv("MINDFRIES_NO_BROWSER") != "1" && a.openBrowser(link)
	if opened {
		fmt.Fprintf(a.Stdout, "  Opening %s in your browser.\n  If it doesn't open, go to that address yourself.\n", link)
	} else {
		fmt.Fprintf(a.Stdout, "  Open this address in your browser:\n    %s\n", link)
	}
	fmt.Fprintf(a.Stdout, "  Check the page shows this code: %s\n\nWaiting for you to confirm in the browser (Ctrl+C to cancel)...\n", da.UserCode)

	interval := time.Duration(da.IntervalSeconds) * time.Second
	if interval < time.Second {
		interval = 5 * time.Second
	}
	if a.tunePoll != nil {
		interval = a.tunePoll(interval)
	}
	// The server says when the code has expired; the local limit only
	// matters if it stops answering.
	deadline := time.Now().Add(time.Until(da.ExpiresAt) + time.Minute)
	if d := time.Now().Add(15 * time.Minute); da.ExpiresAt.IsZero() || deadline.After(d) {
		deadline = d
	}
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(interval):
		}
		res, err := c.PollSignIn(ctx, da.DeviceCode)
		if err == nil {
			fmt.Fprintln(a.Stdout, "Signed in.")
			return a.remember(apiURL, c, res)
		}
		p, isProblem := api.AsProblem(err)
		switch {
		case isProblem && p.Code == api.CodeAuthorizationPending:
		case isProblem && p.Code == api.CodeSlowDown:
			interval += 5 * time.Second
		case isProblem && p.Code == api.CodeAccessDenied:
			return nil, nil, errors.New("the sign-in was refused in the browser, so nothing was connected.\nRun 'mindfries start' to try again")
		case isProblem && (p.Code == api.CodeDeviceCodeExpired || p.Code == api.CodeDeviceCodeInvalid || p.Code == api.CodeDeviceCodeUsed):
			return nil, nil, errors.New("the sign-in wasn't confirmed in time (it lasts about 10 minutes).\nRun 'mindfries start' to get a new one")
		case isProblem && p.Code == api.CodeCLITooOld:
			return nil, nil, tooOld(p)
		case api.IsRetryable(err):
			// A dropped connection or a waking server: keep waiting.
		default:
			return nil, nil, fmt.Errorf("the sign-in failed: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, nil, errors.New("the sign-in wasn't confirmed in time.\nRun 'mindfries start' to get a new one")
		}
	}
}

// webLink reports whether link is an http(s) address, the only kind the
// CLI will hand to the browser.
func webLink(link string) bool {
	u, err := url.Parse(link)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// openBrowser opens link in the default browser and reports whether it
// managed to start one.
func (a *App) openBrowser(link string) bool {
	s := a.sys()
	var err error
	switch s.GOOS {
	case "windows":
		// Not "cmd /c start", which would reinterpret & and ^ in the link.
		err = s.Start("rundll32", "url.dll,FileProtocolHandler", link)
	case "darwin":
		err = s.Start("open", link)
	default:
		if s.Getenv("DISPLAY") == "" && s.Getenv("WAYLAND_DISPLAY") == "" {
			return false // over SSH, or no desktop
		}
		if _, lerr := s.LookPath("xdg-open"); lerr != nil {
			return false
		}
		err = s.Start("xdg-open", link)
	}
	return err == nil
}
