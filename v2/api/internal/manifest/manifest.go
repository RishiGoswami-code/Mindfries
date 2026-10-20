// Package manifest is the role-coverage manifest (v2/codebase-engine PLAN task
// 0.3): for a product, which roles it can assess, in which subsystems, which
// competencies/modes each area exercises, and — honestly — which it cannot.
// The manifest drives product selection and lets the report mark "not
// assessed" truthfully instead of inventing a signal (SPEC §19).
package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/mindfries/v2/api/internal/core"
)

// Manifest is stored in v2.products.role_coverage (plus top-level Tags used for
// JD matching, kept here so the whole manifest travels as one JSON blob).
type Manifest struct {
	// Tags are domain keywords the selector matches a JD against
	// (e.g. "support","tickets","helpdesk" for P1). Lowercase.
	Tags []string `json:"tags"`
	// Roles maps each covered role to what the product can assess for it.
	Roles map[core.Role]RoleCoverage `json:"roles"`
}

// RoleCoverage is what one product offers one role.
type RoleCoverage struct {
	// Subsystems are the repo paths/areas this role works in. Required.
	Subsystems []string `json:"subsystems"`
	// Competencies are human-readable skills the area exercises.
	Competencies []string `json:"competencies"`
	// Modes are the assessment modes (SPEC §7) reachable here. Required.
	Modes []core.Mode `json:"modes"`
	// CannotAssess names what this product cannot show for the role, so the
	// report says so instead of guessing.
	CannotAssess []string `json:"cannot_assess"`
}

// Parse decodes and validates a manifest from JSON (as stored in the DB).
func Parse(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: invalid JSON: %w", err)
	}
	return m, m.Validate()
}

// Validate reports every problem it finds (not just the first), so authoring a
// product manifest fails loudly in CI (PLAN task 0.3).
func (m Manifest) Validate() error {
	var probs []string

	if len(m.Roles) == 0 {
		probs = append(probs, "no roles covered: a product must assess at least one role")
	}
	for role, rc := range m.Roles {
		if !core.ValidRole(role) {
			probs = append(probs, fmt.Sprintf("unknown role %q", role))
		}
		if len(rc.Subsystems) == 0 {
			probs = append(probs, fmt.Sprintf("role %q: at least one subsystem path is required", role))
		}
		if len(rc.Modes) == 0 {
			probs = append(probs, fmt.Sprintf("role %q: at least one assessment mode is required", role))
		}
		for _, md := range rc.Modes {
			if !core.ValidMode(md) {
				probs = append(probs, fmt.Sprintf("role %q: unknown mode %q", role, md))
			}
		}
	}

	if len(probs) > 0 {
		return fmt.Errorf("manifest invalid: %v", probs)
	}
	return nil
}

// Covers reports whether the product assesses role.
func (m Manifest) Covers(role core.Role) bool {
	_, ok := m.Roles[role]
	return ok
}
