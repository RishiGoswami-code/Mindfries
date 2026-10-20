// Package taskgen is the JD-keyed task engine (v2/codebase-engine ARCHITECTURE
// part 3, PLAN tasks 2.2–2.3). Given a product, a role, and a job description,
// it resolves the task a candidate works on.
//
// The reliable core is selection + adaptation, not fresh code generation: the
// task's code (overlay / acceptance / solution) always comes from the
// product's pre-verified reference bank, so the engine never ships unverified
// generated code. A model, when configured, only adapts the brief wording to
// the JD (and degrades gracefully to the authored brief if it fails). Tasks
// are cached per (product, commit, role, jd_hash) with a pool of variants; a
// JD edit changes the hash and so yields a fresh pool, while in-flight sessions
// keep their pinned task_version.
package taskgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mindfries/v2/api/internal/core"
)

// BankTask is one authored, pre-verified reference task for a product.
type BankTask struct {
	ID          string
	ProductSlug string
	Role        core.Role
	Kind        core.Kind
	Title       string
	Tags        []string
	Brief       string   // TASK.md contents
	Dir         string   // task directory (holds overlay/ acceptance/ solution/)
	Check       []string // check command, e.g. ["go","test","./..."]
}

// taskFile is the on-disk task.json shape.
type taskFile struct {
	ID    string    `json:"id"`
	Kind  core.Kind `json:"kind"`
	Role  core.Role `json:"role"`
	Title string    `json:"title"`
	Tags  []string  `json:"tags"`
	Check []string  `json:"check"`
}

// LoadBank reads every tasks/<id>/task.json under a product directory and
// returns the reference task bank. productSlug labels the tasks' product.
func LoadBank(productDir, productSlug string) ([]BankTask, error) {
	tasksDir := filepath.Join(productDir, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("taskgen: read tasks dir: %w", err)
	}
	var bank []BankTask
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(tasksDir, e.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "task.json"))
		if err != nil {
			return nil, fmt.Errorf("taskgen: %s: %w", e.Name(), err)
		}
		var tf taskFile
		if err := json.Unmarshal(raw, &tf); err != nil {
			return nil, fmt.Errorf("taskgen: %s/task.json: %w", e.Name(), err)
		}
		if !core.ValidRole(tf.Role) {
			return nil, fmt.Errorf("taskgen: %s: unknown role %q", e.Name(), tf.Role)
		}
		if !core.ValidKind(tf.Kind) {
			return nil, fmt.Errorf("taskgen: %s: unknown kind %q", e.Name(), tf.Kind)
		}
		brief, err := os.ReadFile(filepath.Join(dir, "TASK.md"))
		if err != nil {
			return nil, fmt.Errorf("taskgen: %s: TASK.md: %w", e.Name(), err)
		}
		bank = append(bank, BankTask{
			ID: tf.ID, ProductSlug: productSlug, Role: tf.Role, Kind: tf.Kind,
			Title: tf.Title, Tags: tf.Tags, Brief: string(brief), Dir: dir, Check: tf.Check,
		})
	}
	return bank, nil
}
