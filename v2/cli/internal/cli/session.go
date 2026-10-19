package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/secret"
	"github.com/mindfries/v2/cli/internal/state"
)

func (a *App) home() (string, error) { return state.Home(a.Getenv) }

var codeRe = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$`)

// normalizeCode accepts the code however it was pasted.
func normalizeCode(c string) (string, bool) {
	c = strings.ToUpper(strings.Join(strings.Fields(c), ""))
	if len(c) == 12 && !strings.Contains(c, "-") {
		c = c[0:4] + "-" + c[4:8] + "-" + c[8:12]
	}
	return c, codeRe.MatchString(c)
}

// connect trades the setup code for a session token and records the
// session locally. The token goes to the keychain (or the owner-only file)
// and is never printed.
func (a *App) connect(ctx context.Context, apiURL, code string) (*state.Local, *api.Client, error) {
	home, err := a.home()
	if err != nil {
		return nil, nil, err
	}
	c := api.New(apiURL, buildinfo.Version)
	s := a.sys()
	res, err := c.Exchange(ctx, api.ExchangeRequest{SetupCode: code, Device: api.Device{OS: s.GOOS, Arch: s.GOARCH}})
	if err != nil {
		return nil, nil, exchangeError(err)
	}
	store, err := secret.Save(secret.Choose(home, a.Getenv), res.Session.ID, res.SessionToken)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't store the session token safely: %w", err)
	}
	l := &state.Local{
		APIURL:       apiURL,
		SessionID:    res.Session.ID,
		AssessmentID: res.Session.AssessmentID,
		Title:        res.Session.AssessmentTitle,
		EndsAt:       res.Session.EndsAt,
		TokenStore:   store,
		Policy:       res.Session.Policy,
		Intervals:    res.Session.Intervals,
		Limits:       res.Session.Limits,
		ConnectedAt:  time.Now().UTC(),
	}
	if err := state.Save(home, l); err != nil {
		_ = secret.Open(home, store).Delete(l.SessionID)
		return nil, nil, err
	}
	c.Token = res.SessionToken
	return l, c, nil
}

func exchangeError(err error) error {
	p, ok := api.AsProblem(err)
	if !ok {
		return fmt.Errorf("couldn't reach Mindfries: %w\nCheck your connection and run the setup command again", err)
	}
	switch p.Code {
	case api.CodeSetupCodeUsed:
		return errors.New("this setup code has already been used.\n" +
			"If you started this assessment on this computer, run 'mindfries start' (without a code) to carry on.\n" +
			"Otherwise get a fresh setup command from your Mindfries dashboard")
	case api.CodeSetupCodeExpired:
		return errors.New("this setup code has expired (codes last about 10 minutes).\nGet a fresh setup command from your Mindfries dashboard")
	case api.CodeSetupCodeInvalid:
		return errors.New("this setup code isn't recognised.\nCopy the whole setup command from your Mindfries dashboard again")
	case api.CodeCLITooOld:
		return fmt.Errorf("this mindfries is too old (need %s or newer).\nRun the setup command from your dashboard again; it installs the latest", p.MinimumVersion)
	}
	return p
}

// errNoSession is what every command says when nothing is set up.
var errNoSession = errors.New("there's no active assessment on this computer.\nStart one from your Mindfries dashboard and run the setup command it shows")

// active loads the session and a client carrying its token.
func (a *App) active() (*state.Local, *api.Client, error) {
	home, err := a.home()
	if err != nil {
		return nil, nil, err
	}
	l, err := state.Load(home)
	if errors.Is(err, state.ErrNone) {
		return nil, nil, errNoSession
	}
	if err != nil {
		return nil, nil, err
	}
	tok, err := secret.Open(home, l.TokenStore).Get(l.SessionID)
	if err != nil {
		return l, nil, fmt.Errorf("this computer's session token is missing (%v).\nRun 'mindfries stop', then get a fresh setup command from your dashboard", err)
	}
	c := api.New(l.APIURL, buildinfo.Version)
	c.Token = tok
	return l, c, nil
}

// sessionEnded explains an auth failure: the token is no longer valid.
func sessionEnded(err error) error {
	p, _ := api.AsProblem(err)
	why := "has ended"
	if p != nil {
		switch p.Code {
		case api.CodeTokenRevoked:
			why = "was ended"
		case api.CodeTokenExpired:
			why = "has run out of time"
		}
	}
	return fmt.Errorf("this assessment session %s. Your code is still in the workspace folder", why)
}
