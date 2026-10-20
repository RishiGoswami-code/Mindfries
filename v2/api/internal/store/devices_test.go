package store

import (
	"context"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/testdb"
)

func openTest(t *testing.T) (*DB, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	db, err := Open(ctx, testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db, ctx
}

func TestDeviceSignInLifecycle(t *testing.T) {
	db, ctx := openTest(t)
	now := time.Now().UTC().Truncate(time.Microsecond) // Postgres keeps microseconds
	secret := auth.NewSecret()
	dev := auth.Device{CodeHash: auth.Hash(secret), UserCode: auth.NewUserCode(), ExpiresAt: now.Add(10 * time.Minute), Interval: 5}
	if err := db.CreateDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}

	got, err := db.DeviceByHash(ctx, auth.Hash(secret))
	if err != nil || got.Status != "pending" || got.UserCode != dev.UserCode {
		t.Fatalf("%+v %v", got, err)
	}
	if prev, err := db.TouchDevice(ctx, got.CodeHash, now); err != nil || prev != nil {
		t.Fatalf("the first poll has no previous: %v %v", prev, err)
	}
	if prev, err := db.TouchDevice(ctx, got.CodeHash, now.Add(time.Second)); err != nil || prev == nil || !prev.Equal(now) {
		t.Fatalf("the second poll sees the first: %v %v", prev, err)
	}

	cand := auth.NewUUID()
	grant := session.Grant{Role: "backend", JD: "support saas", AssessmentTitle: "Backend", DurationMinutes: 60,
		Links: session.Links{CandidateID: cand}, AssessmentID: auth.NewUUID()}
	if ok, err := db.ApproveDevice(ctx, dev.UserCode, cand, grant, now); !ok || err != nil {
		t.Fatalf("approve: %v %v", ok, err)
	}
	if ok, _ := db.ApproveDevice(ctx, dev.UserCode, cand, grant, now); ok {
		t.Fatal("a second approval must not work")
	}
	if ok, _ := db.DenyDevice(ctx, dev.UserCode, now); ok {
		t.Fatal("an approved sign-in can't then be denied")
	}
	got, _ = db.DeviceByUserCode(ctx, dev.UserCode)
	if got.Status != "approved" || got.Grant == nil || got.Grant.Role != "backend" || got.CandidateID != cand {
		t.Fatalf("%+v", got)
	}

	// Seed a session to attach, then spend the code exactly once.
	sess := session.Session{ID: auth.NewID("ses", 6), Token: auth.NewSecret(), Role: "backend", ProductSlug: "p1-support-saas",
		BankTaskID: "t", TaskVersion: "v1", Status: "ready", StartedAt: now, EndsAt: now.Add(time.Hour), DurationMinutes: 60}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.UseDevice(ctx, got.CodeHash, sess.ID); !ok || err != nil {
		t.Fatalf("use: %v %v", ok, err)
	}
	if ok, _ := db.UseDevice(ctx, got.CodeHash, sess.ID); ok {
		t.Fatal("a device code is spent once")
	}
}

func TestDeviceExpiredCannotBeApprovedOrDenied(t *testing.T) {
	db, ctx := openTest(t)
	now := time.Now().UTC()
	dev := auth.Device{CodeHash: auth.Hash(auth.NewSecret()), UserCode: auth.NewUserCode(), ExpiresAt: now.Add(-time.Minute), Interval: 5}
	if err := db.CreateDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.ApproveDevice(ctx, dev.UserCode, auth.NewUUID(), session.Grant{}, now); ok {
		t.Fatal("an expired code can't be approved")
	}
	if ok, _ := db.DenyDevice(ctx, dev.UserCode, now); ok {
		t.Fatal("an expired code can't be denied")
	}
	if n, err := db.PurgeDevices(ctx, now); err != nil || n < 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	if _, err := db.DeviceByUserCode(ctx, dev.UserCode); err != ErrDeviceNotFound {
		t.Fatalf("purged: %v", err)
	}
}

func TestSessionTokensAreHashedAndRotate(t *testing.T) {
	db, ctx := openTest(t)
	now := time.Now().UTC()
	cand, asmt := auth.NewUUID(), auth.NewUUID()
	s := session.Session{ID: auth.NewID("ses", 6), Token: auth.NewSecret(), Role: "backend", ProductSlug: "p1-support-saas",
		BankTaskID: "t", TaskVersion: "v1", Status: "ready", StartedAt: now, EndsAt: now.Add(time.Hour), DurationMinutes: 60,
		Links: session.Links{CandidateID: cand}, AssessmentID: asmt}
	oldToken := s.Token
	if err := db.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}

	var plain *string
	if err := db.pool.QueryRow(ctx, `select token from v2.sessions where id = $1`, s.ID).Scan(&plain); err != nil || plain != nil {
		t.Fatalf("the plain token must not be stored: %v %v", plain, err)
	}
	got, err := db.SessionByToken(ctx, oldToken)
	if err != nil || got.ID != s.ID || got.Links.CandidateID != cand {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := db.SessionByToken(ctx, "not-a-token"); err != session.ErrNotFound {
		t.Fatalf("unknown token: %v", err)
	}

	live, ok, err := db.LiveSessionFor(ctx, cand, asmt)
	if err != nil || !ok || live.ID != s.ID {
		t.Fatalf("live session: %+v %v %v", live, ok, err)
	}
	newToken := auth.NewSecret()
	if err := db.RotateToken(ctx, s.ID, newToken); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SessionByToken(ctx, oldToken); err != session.ErrNotFound {
		t.Fatal("the old token must stop working")
	}
	if got, err := db.SessionByToken(ctx, newToken); err != nil || got.ID != s.ID {
		t.Fatalf("new token: %v", err)
	}

	if err := db.SetStatus(ctx, s.ID, "submitted"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.LiveSessionFor(ctx, cand, asmt); ok {
		t.Fatal("a submitted session isn't live")
	}
}

func TestAssessmentConfigRoundTrip(t *testing.T) {
	db, ctx := openTest(t)
	id := auth.NewUUID()
	c, found, err := db.AssessmentConfig(ctx, id)
	if err != nil || found || c.DurationMinutes != 90 || c.AssistantMode != policy.ExplainOnly {
		t.Fatalf("unconfigured assessments get the defaults: %+v %v %v", c, found, err)
	}
	c.DurationMinutes = 120
	c.AssistantMode = policy.CodeAllowed
	c.Reviewers = []string{"@Zed", "alice", "ALICE"}
	c.RequireAudio = false
	if err := db.SaveAssessmentConfig(ctx, c, "tester"); err != nil {
		t.Fatal(err)
	}
	got, found, err := db.AssessmentConfig(ctx, id)
	if err != nil || !found || got.DurationMinutes != 120 || got.AssistantMode != policy.CodeAllowed || got.RequireAudio {
		t.Fatalf("%+v %v %v", got, found, err)
	}
	if len(got.Reviewers) != 2 || got.Reviewers[0] != "alice" || got.Reviewers[1] != "Zed" {
		t.Fatalf("reviewers are normalised on save: %v", got.Reviewers)
	}
	got.Reviewers = []string{"new-person"}
	if err := db.SaveAssessmentConfig(ctx, got, "tester"); err != nil {
		t.Fatal(err)
	}
	again, _, _ := db.AssessmentConfig(ctx, id)
	if len(again.Reviewers) != 1 || again.Reviewers[0] != "new-person" {
		t.Fatalf("reviewers can change mid-session: %v", again.Reviewers)
	}
}
