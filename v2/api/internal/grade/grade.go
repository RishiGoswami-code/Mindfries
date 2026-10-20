// Package grade runs a candidate's submitted workspace against a task's hidden
// acceptance tests — the automated, objective half of the evidence report. It
// is the same harness that proves the reference solution at authoring time
// (internal/verify), pointed at the candidate's files instead of the clean
// product: overlay the hidden acceptance tests, run the task's own check.
//
// SECURITY: Run executes the candidate's code. It must run in an isolated,
// throwaway environment with the task's toolchain (a sandbox or CI job), NEVER
// in the live engine process — a candidate's submission is untrusted.
package grade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mindfries/v2/api/internal/verify"
)

// Result is the outcome of grading one submission against one task.
type Result struct {
	Passed bool     `json:"passed"`
	Check  []string `json:"check"`  // the command that was run
	Output string   `json:"output"` // combined stdout+stderr of the check
}

// Run grades candidateDir (the candidate's final workspace, e.g. unpacked from
// the submission bundle at head_commit) for the task bankTaskID of the product
// at productDir. It overlays the task's hidden acceptance tests when present and
// runs the task's check command, exactly as verification does for the solution.
func Run(candidateDir, productDir, bankTaskID string) (Result, error) {
	taskDir := filepath.Join(productDir, "_tasks", bankTaskID)
	check, err := readCheck(filepath.Join(taskDir, "task.json"))
	if err != nil {
		return Result{}, err
	}

	var overlays []string
	if acc := filepath.Join(taskDir, "acceptance"); isDir(acc) {
		// Feature tasks hide their tests; a debug task ships the failing test
		// in the product itself, so it simply has no acceptance overlay.
		overlays = append(overlays, acc)
	}

	res, err := verify.Run(verify.Spec{ProductDir: candidateDir, Overlays: overlays, Check: check})
	if err != nil {
		return Result{}, err
	}
	return Result{Passed: res.Passed, Check: check, Output: res.Output}, nil
}

func readCheck(taskJSON string) ([]string, error) {
	b, err := os.ReadFile(taskJSON)
	if err != nil {
		return nil, fmt.Errorf("grade: read task.json: %w", err)
	}
	var tf struct {
		Check []string `json:"check"`
	}
	if err := json.Unmarshal(b, &tf); err != nil {
		return nil, fmt.Errorf("grade: parse task.json: %w", err)
	}
	if len(tf.Check) == 0 {
		return nil, fmt.Errorf("grade: task has no check command: %s", taskJSON)
	}
	return tf.Check, nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
