package github

import (
	"container/list"
	"net/http"
	"sync"
)

const maxCacheEntries = 256

type cacheEntry struct {
	key    string
	etag   string
	body   []byte
	header http.Header
}

type responseCache struct {
	mu    sync.Mutex
	max   int
	order *list.List
	items map[string]*list.Element
}

func newResponseCache(max int) *responseCache {
	return &responseCache{max: max, order: list.New(), items: make(map[string]*list.Element)}
}

func (c *responseCache) get(key string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return cacheEntry{}, false
	}
	c.order.MoveToFront(el)
	return *el.Value.(*cacheEntry), true
}

func (c *responseCache) put(entry cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[entry.key]; ok {
		*el.Value.(*cacheEntry) = entry
		c.order.MoveToFront(el)
		return
	}
	c.items[entry.key] = c.order.PushFront(&entry)
	for c.order.Len() > c.max {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*cacheEntry).key)
	}
}

func (c *responseCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
