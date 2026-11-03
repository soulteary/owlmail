package api

import (
	"container/list"
	"sync"
)

// Claim caching only avoids repeated binding work. Proxy verification and the
// authoritative personal-mailbox lookup still run on every request. Eviction
// causes a safe, idempotent rebind; it never changes ownership or authorizes a
// request. Keep the cache bounded even when upstream asserts many subjects.
const maxCachedClaims = 4096

// claimCache is a zero-value-ready LRU. Keys and evidence are already bounded by
// identity.Proxy; entries are process-local hints, not durable identity state.
type claimCache struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	recent  list.List
}

type cachedClaim struct{ key, evidence string }

func (c *claimCache) Load(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.entries[key]
	if !ok {
		return "", false
	}
	c.recent.MoveToFront(item)
	return item.Value.(cachedClaim).evidence, true
}

func (c *claimCache) Store(key, evidence string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.entries[key]; ok {
		item.Value = cachedClaim{key, evidence}
		c.recent.MoveToFront(item)
		return
	}
	if c.entries == nil {
		c.entries = make(map[string]*list.Element)
	}
	if len(c.entries) == maxCachedClaims {
		oldest := c.recent.Back()
		delete(c.entries, oldest.Value.(cachedClaim).key)
		c.recent.Remove(oldest)
	}
	c.entries[key] = c.recent.PushFront(cachedClaim{key, evidence})
}
