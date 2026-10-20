package store

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/taskgen"
)

// TaskCache is a taskgen.Cache backed by v2.product_tasks: a resolved pool for
// a (product, role, jd_hash) is persisted so it is reused across candidates and
// across restarts, and regenerated only when the JD (hence the key) changes.
//
// The bank task id is stored in overlay_ref (it identifies the _tasks/<id> dir
// the overlay/solution/acceptance come from). NextVariant is a per-process
// round-robin counter — good enough for spreading variants across candidates;
// ponytail: make it a DB counter if you run many server instances.
type TaskCache struct {
	db *DB
	mu sync.Mutex
	rr map[string]int
}

// NewTaskCache returns a DB-backed cache.
func NewTaskCache(db *DB) *TaskCache {
	return &TaskCache{db: db, rr: map[string]int{}}
}

// key is "slug|role|jdHash" (see taskgen.cacheKey).
func splitKey(key string) (slug, role, jd string) {
	parts := strings.SplitN(key, "|", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

// GetPool reads the cached variant pool for the key, newest golden commit.
func (c *TaskCache) GetPool(key string) ([]taskgen.ResolvedTask, bool) {
	slug, role, jd := splitKey(key)
	if slug == "" {
		return nil, false
	}
	rows, err := c.db.pool.Query(context.Background(),
		`select pt.task_version, pt.variant_no, pt.kind, pt.brief_md, coalesce(pt.overlay_ref, '')
		   from v2.product_tasks pt
		   join v2.products p on p.id = pt.product_id
		  where p.slug = $1 and pt.role = $2 and pt.jd_hash = $3
		    and pt.golden_commit = coalesce(p.golden_commit, '')
		  order by pt.variant_no`, slug, role, jd)
	if err != nil {
		log.Printf("taskcache: GetPool query: %v", err)
		return nil, false
	}
	defer rows.Close()

	var pool []taskgen.ResolvedTask
	for rows.Next() {
		var tv, kind, brief, overlay string
		var vno int
		if err := rows.Scan(&tv, &vno, &kind, &brief, &overlay); err != nil {
			log.Printf("taskcache: GetPool scan: %v", err)
			return nil, false
		}
		pool = append(pool, taskgen.ResolvedTask{
			TaskVersion: tv, VariantNo: vno, Kind: core.Kind(kind),
			Brief: brief, BankTaskID: overlay, Role: core.Role(role), ProductSlug: slug,
		})
	}
	if len(pool) == 0 {
		return nil, false
	}
	return pool, true
}

// PutPool persists a freshly generated variant pool.
func (c *TaskCache) PutPool(key string, pool []taskgen.ResolvedTask) {
	slug, role, jd := splitKey(key)
	if slug == "" {
		return
	}
	ctx := context.Background()
	var pid, commit string
	if err := c.db.pool.QueryRow(ctx,
		`select id::text, coalesce(golden_commit, '') from v2.products where slug = $1`, slug,
	).Scan(&pid, &commit); err != nil {
		log.Printf("taskcache: PutPool lookup product %q: %v", slug, err)
		return
	}
	for _, rt := range pool {
		if _, err := c.db.pool.Exec(ctx,
			`insert into v2.product_tasks
			   (product_id, golden_commit, role, jd_hash, variant_no, task_version, kind, brief_md, overlay_ref)
			 values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 on conflict (product_id, golden_commit, role, jd_hash, variant_no) do update
			   set task_version = excluded.task_version, kind = excluded.kind,
			       brief_md = excluded.brief_md, overlay_ref = excluded.overlay_ref`,
			pid, commit, role, jd, rt.VariantNo, rt.TaskVersion, string(rt.Kind), rt.Brief, rt.BankTaskID,
		); err != nil {
			log.Printf("taskcache: PutPool insert: %v", err)
		}
	}
}

// NextVariant round-robins across the pool (per process).
func (c *TaskCache) NextVariant(key string, n int) int {
	if n <= 0 {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	i := c.rr[key] % n
	c.rr[key]++
	return i
}
