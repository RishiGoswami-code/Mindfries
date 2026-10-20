package taskgen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/engine"
)

// Provider adapts a task brief to a job description. A real implementation
// calls a model server-side; it is optional — a nil provider (or one that
// errors) leaves the authored brief untouched. It never produces task code.
type Provider interface {
	Adapt(ctx context.Context, brief, jd string) (string, error)
}

// ResolvedTask is what the engine hands the session: which authored task to run
// (by bank id, so the overlay/acceptance/solution come from verified files),
// the brief to show, and a stable task_version a running session pins to.
type ResolvedTask struct {
	TaskVersion string    `json:"task_version"`
	ProductSlug string    `json:"product_slug"`
	Role        core.Role `json:"role"`
	Kind        core.Kind `json:"kind"`
	Title       string    `json:"title"`
	BankTaskID  string    `json:"bank_task_id"`
	Brief       string    `json:"brief"`
	VariantNo   int       `json:"variant_no"`
}

// Generator resolves tasks from a bank, caching pools by JD.
type Generator struct {
	bank     []BankTask
	provider Provider // optional
	cache    Cache
	poolSize int
}

// New builds a Generator. poolSize < 1 defaults to 3. provider may be nil.
func New(bank []BankTask, cache Cache, provider Provider, poolSize int) *Generator {
	if poolSize < 1 {
		poolSize = 3
	}
	if cache == nil {
		cache = NewMemCache()
	}
	return &Generator{bank: bank, provider: provider, cache: cache, poolSize: poolSize}
}

// Resolve returns the task for (product, role, jd). It serves a cached pool
// when the JD (by hash) has been seen, otherwise builds and caches one. The
// returned variant rotates across calls so candidates on one JD differ.
func (g *Generator) Resolve(ctx context.Context, productSlug string, role core.Role, jd string) (ResolvedTask, error) {
	key := cacheKey(productSlug, role, engine.HashJD(jd))

	if pool, ok := g.cache.GetPool(key); ok && len(pool) > 0 {
		return pool[g.cache.NextVariant(key, len(pool))], nil
	}

	ranked := g.rank(productSlug, role, jd)
	if len(ranked) == 0 {
		return ResolvedTask{}, fmt.Errorf("taskgen: no bank task for product %q role %q", productSlug, role)
	}
	n := g.poolSize
	if n > len(ranked) {
		n = len(ranked)
	}

	pool := make([]ResolvedTask, 0, n)
	for i := 0; i < n; i++ {
		bt := ranked[i]
		brief := bt.Brief
		if g.provider != nil {
			if adapted, err := g.provider.Adapt(ctx, bt.Brief, jd); err == nil && adapted != "" {
				brief = adapted // graceful: keep authored brief on error/empty
			}
		}
		pool = append(pool, ResolvedTask{
			TaskVersion: taskVersion(key, i, bt.ID),
			ProductSlug: productSlug,
			Role:        role,
			Kind:        bt.Kind,
			Title:       bt.Title,
			BankTaskID:  bt.ID,
			Brief:       brief,
			VariantNo:   i,
		})
	}
	g.cache.PutPool(key, pool)
	return pool[g.cache.NextVariant(key, len(pool))], nil
}

// rank returns the product+role bank tasks, best JD match first, ties by id.
func (g *Generator) rank(productSlug string, role core.Role, jd string) []BankTask {
	type scored struct {
		bt    BankTask
		score int
	}
	var ss []scored
	for _, bt := range g.bank {
		if bt.ProductSlug != productSlug || bt.Role != role {
			continue
		}
		score, _ := engine.TagScore(jd, bt.Tags)
		ss = append(ss, scored{bt, score})
	}
	sort.SliceStable(ss, func(i, j int) bool {
		if ss[i].score != ss[j].score {
			return ss[i].score > ss[j].score
		}
		return ss[i].bt.ID < ss[j].bt.ID
	})
	out := make([]BankTask, len(ss))
	for i, s := range ss {
		out[i] = s.bt
	}
	return out
}

func cacheKey(productSlug string, role core.Role, jdHash string) string {
	return productSlug + "|" + string(role) + "|" + jdHash
}

// taskVersion is a stable id a running session pins to, derived from the cache
// key, the variant, and the authored task — so the same JD always yields the
// same versions and a session's task never shifts under it.
func taskVersion(key string, variant int, bankTaskID string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|v%d|%s", key, variant, bankTaskID)))
	return "task_" + hex.EncodeToString(sum[:])[:16]
}
