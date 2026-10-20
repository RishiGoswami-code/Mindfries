// Package core holds the small, dependency-free vocabulary the whole engine
// shares: the roles we assess, the assessment modes (SPEC §7), and the task
// kinds. Keeping these in a leaf package lets both manifest and engine import
// them without a cycle. The role and kind sets are mirrored by the CHECK
// constraints in v2/db/migrations/0003_product_tasks.sql — keep them in step.
package core

// Role is one of the eight roles the product set covers
// (v2/codebase-engine/ANALYSIS.md).
type Role string

const (
	Backend      Role = "backend"
	Frontend     Role = "frontend"
	FullStack    Role = "fullstack"
	FDE          Role = "fde"
	Quant        Role = "quant"
	DataAnalyst  Role = "data_analyst"
	MLResearcher Role = "ml_researcher"
	AIEngineer   Role = "ai_engineer"
)

// Roles lists every valid role.
var Roles = []Role{Backend, Frontend, FullStack, FDE, Quant, DataAnalyst, MLResearcher, AIEngineer}

// ValidRole reports whether r is a known role.
func ValidRole(r Role) bool {
	for _, x := range Roles {
		if x == r {
			return true
		}
	}
	return false
}

// Mode is an assessment mode — what a task reveals (SPEC §7, step 2).
type Mode string

const (
	Implementation Mode = "implementation"
	SystemDesign   Mode = "system_design"
	Research       Mode = "research"
	Debugging      Mode = "debugging"
	Comprehension  Mode = "comprehension"
	Testing        Mode = "testing"
	Delivery       Mode = "delivery"
	Communication  Mode = "communication"
)

// Modes lists every valid mode.
var Modes = []Mode{Implementation, SystemDesign, Research, Debugging, Comprehension, Testing, Delivery, Communication}

// ValidMode reports whether m is a known mode.
func ValidMode(m Mode) bool {
	for _, x := range Modes {
		if x == m {
			return true
		}
	}
	return false
}

// Kind is the shape of a task on a product.
type Kind string

const (
	// Feature: implement new behaviour in a subsystem, verified against an
	// authored reference solution's acceptance tests.
	Feature Kind = "feature"
	// Debug: a planted bug on the green golden commit; the fix greens the
	// intended failing tests without breaking the suite.
	Debug Kind = "debug"
	// Review: comprehension / code review of real subsystem code.
	Review Kind = "review"
	// Design: a system-design prompt grounded in the product.
	Design Kind = "design"
)

// Kinds lists every valid task kind.
var Kinds = []Kind{Feature, Debug, Review, Design}

// ValidKind reports whether k is a known kind.
func ValidKind(k Kind) bool {
	for _, x := range Kinds {
		if x == k {
			return true
		}
	}
	return false
}
