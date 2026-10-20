package taskgen

import "sync"

// Cache stores resolved task pools keyed by (product, commit, role, jd_hash).
// A JD edit produces a new key, so a stale pool is simply never hit again —
// no explicit invalidation, and sessions pinned to an old task_version are
// unaffected. In production this is backed by v2.product_tasks; MemCache is the
// in-memory implementation used in tests and as a write-through buffer.
type Cache interface {
	GetPool(key string) ([]ResolvedTask, bool)
	PutPool(key string, pool []ResolvedTask)
	// NextVariant returns the next index in [0,n) to serve for key, round-robin,
	// so concurrent candidates on one JD don't all get the identical variant.
	NextVariant(key string, n int) int
}

// MemCache is a process-local Cache.
type MemCache struct {
	mu    sync.Mutex
	pools map[string][]ResolvedTask
	rr    map[string]int
}

// NewMemCache returns an empty in-memory cache.
func NewMemCache() *MemCache {
	return &MemCache{pools: map[string][]ResolvedTask{}, rr: map[string]int{}}
}

func (c *MemCache) GetPool(key string) ([]ResolvedTask, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.pools[key]
	return p, ok
}

func (c *MemCache) PutPool(key string, pool []ResolvedTask) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pools[key] = pool
}

func (c *MemCache) NextVariant(key string, n int) int {
	if n <= 0 {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	i := c.rr[key] % n
	c.rr[key]++
	return i
}
