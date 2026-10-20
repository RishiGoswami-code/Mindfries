// Command grade is the out-of-process grading runner (GRADE step). It finds
// verified-but-ungraded submissions, unpacks each candidate's git bundle at its
// head commit, runs the task's hidden acceptance tests (package grade), and
// stores the pass/fail result the admin review surface reads.
//
// SECURITY: this EXECUTES candidate code (npm test / go test / unittest). Run it
// only in an isolated, throwaway environment that has the task toolchains (git,
// plus Node/Go/Python as the tasks need) — never alongside anything sensitive,
// and never inside the live engine process.
//
// Usage:
//
//	V2_DATABASE_URL=postgres://... V2_PRODUCTS_ROOT=../products go run ./cmd/grade
//	go run ./cmd/grade -submission sub_abc123   # grade just one
//	go run ./cmd/grade -n 10                      # cap how many this run
//	go run ./cmd/grade -watch                     # keep running, grading new submissions as they arrive
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mindfries/v2/api/internal/grade"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
)

const maxStoredOutput = 20000 // keep the check log readable in the DB

func main() {
	var one string
	var n int
	var watch bool
	var every time.Duration
	flag.BoolVar(&watch, "watch", false, "keep running: look for new submissions every -every")
	flag.DurationVar(&every, "every", time.Minute, "how often -watch looks")
	flag.StringVar(&one, "submission", "", "grade only this submission id")
	flag.IntVar(&n, "n", 50, "max submissions to grade this run")
	flag.Parse()

	url := os.Getenv("V2_DATABASE_URL")
	if url == "" {
		log.Fatal("V2_DATABASE_URL is required")
	}
	productsRoot := os.Getenv("V2_PRODUCTS_ROOT")
	if productsRoot == "" {
		productsRoot = "../products"
	}

	ctx := context.Background()
	db, err := store.Open(ctx, url)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if !watch {
		pass(ctx, db, productsRoot, one, n, false)
		return
	}
	log.Printf("watching for submissions every %s", every)
	for {
		pass(ctx, db, productsRoot, "", n, true)
		time.Sleep(every)
	}
}

const maxAttempts = 3

var attempts = map[string]int{}

// pass grades what's waiting (or just `one`) and logs the result.
func pass(ctx context.Context, db *store.DB, productsRoot, one string, n int, quiet bool) {
	var ids []string
	var err error
	if one != "" {
		ids = []string{one}
	} else if ids, err = db.UngradedSubmissions(ctx, n); err != nil {
		log.Printf("list ungraded: %v", err)
		return
	}
	// A submission that errors (a broken bundle, say) is tried a few times, then
	// left for a person, so a watching runner doesn't retry it every minute for ever.
	var todo []string
	for _, id := range ids {
		if attempts[id] < maxAttempts {
			todo = append(todo, id)
		}
	}
	ids = todo
	if len(ids) == 0 {
		if !quiet {
			log.Println("nothing to grade")
		}
		return
	}
	log.Printf("grading %d submission(s)", len(ids))

	ok, fail, errs := 0, 0, 0
	for _, id := range ids {
		passed, err := gradeOne(ctx, db, productsRoot, id)
		switch {
		case err != nil:
			errs++
			attempts[id]++
			log.Printf("  %s: ERROR %v", id, err)
		case passed:
			ok++
			log.Printf("  %s: PASSED", id)
		default:
			fail++
			log.Printf("  %s: FAILED", id)
		}
	}
	log.Printf("done: %d passed, %d failed, %d errored", ok, fail, errs)
}

func gradeOne(ctx context.Context, db *store.DB, productsRoot, id string) (bool, error) {
	sub, err := db.SubmissionByID(ctx, id)
	if err != nil {
		return false, fmt.Errorf("load submission: %w", err)
	}
	sess, err := db.SessionByID(ctx, sub.SessionID)
	if err != nil {
		return false, fmt.Errorf("load session: %w", err)
	}
	bundle, err := db.SubmissionBundle(ctx, id)
	if err != nil {
		return false, fmt.Errorf("load bundle: %w", err)
	}
	if len(bundle) == 0 {
		return false, fmt.Errorf("no bundle stored (submission not completed?)")
	}

	dir, cleanup, err := unpackBundle(bundle, sub.HeadCommit)
	defer cleanup()
	if err != nil {
		return false, err
	}

	res, err := grade.Run(dir, filepath.Join(productsRoot, sess.ProductSlug), sess.BankTaskID)
	if err != nil {
		return false, fmt.Errorf("run grade: %w", err)
	}

	out := res.Output
	if len(out) > maxStoredOutput {
		out = out[:maxStoredOutput] + "\n…(truncated)"
	}
	g := session.Grade{SubmissionID: id, Passed: res.Passed, Check: res.Check, Output: out, GradedAt: time.Now().UTC()}
	if err := db.SaveGrade(ctx, g); err != nil {
		return res.Passed, fmt.Errorf("save grade: %w", err)
	}
	return res.Passed, nil
}

// unpackBundle writes the git bundle to disk, clones it, and checks out head.
// The returned dir is the candidate's workspace at the submitted commit.
func unpackBundle(bundle []byte, head string) (string, func(), error) {
	tmp, err := os.MkdirTemp("", "grade-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	bf := filepath.Join(tmp, "submission.bundle")
	if err := os.WriteFile(bf, bundle, 0o600); err != nil {
		return "", cleanup, err
	}
	work := filepath.Join(tmp, "work")
	if out, err := exec.Command("git", "clone", "--quiet", bf, work).CombinedOutput(); err != nil {
		return "", cleanup, fmt.Errorf("git clone bundle: %w: %s", err, out)
	}
	if head != "" {
		// Clone already checks out the bundle's HEAD; pin to head to be exact.
		if out, err := exec.Command("git", "-C", work, "checkout", "--quiet", head).CombinedOutput(); err != nil {
			log.Printf("  checkout %s failed (%s); using clone HEAD", head, out)
		}
	}
	return work, cleanup, nil
}
