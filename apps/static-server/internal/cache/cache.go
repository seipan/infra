// Package cache provides the in-memory object cache that sits in front of
// MinIO, so that repeatedly requested files (and the readiness probe) do not
// each cost a round trip to the backend.
package cache

import (
	"container/list"
	"sync"
	"time"
)

// Entry is a cached lookup result. Found is false for a negative entry, which
// records that an object is missing so that repeated requests for it - probes
// scanning for /wp-login.php and the like - do not each reach MinIO.
type Entry struct {
	Body        []byte
	ContentType string
	Found       bool
}

// Cache is a size-bounded TTL/LRU cache of object lookups, safe for concurrent
// use. A Cache built with a non-positive ttl or maxBytes caches nothing, which
// is how caching is turned off.
type Cache struct {
	ttl         time.Duration
	negativeTTL time.Duration
	maxBytes    int64

	mu       sync.Mutex
	curBytes int64
	order    *list.List // front is most recently used
	items    map[string]*list.Element
}

type node struct {
	key       string
	entry     Entry
	size      int64
	expiresAt time.Time
}

// New returns a cache holding at most maxBytes of entries. Found entries expire
// after ttl, missing ones after negativeTTL, which is kept shorter so that a
// newly uploaded file does not stay invisible for long.
func New(ttl, negativeTTL time.Duration, maxBytes int64) *Cache {
	return &Cache{
		ttl:         ttl,
		negativeTTL: negativeTTL,
		maxBytes:    maxBytes,
		order:       list.New(),
		items:       make(map[string]*list.Element),
	}
}

func (c *Cache) enabled() bool {
	return c != nil && c.ttl > 0 && c.maxBytes > 0
}

// Get returns the entry stored under key. The second result reports whether
// this was a hit; an expired entry counts as a miss and is dropped.
func (c *Cache) Get(key string) (Entry, bool) {
	if !c.enabled() {
		return Entry{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return Entry{}, false
	}
	n := el.Value.(*node)
	if time.Now().After(n.expiresAt) {
		c.removeElement(el)
		return Entry{}, false
	}
	c.order.MoveToFront(el)
	return n.entry, true
}

// Put stores entry under key, evicting the least recently used entries to stay
// within the size bound. An entry too large to ever fit is dropped instead.
func (c *Cache) Put(key string, entry Entry) {
	if !c.enabled() {
		return
	}

	ttl := c.ttl
	if !entry.Found {
		ttl = c.negativeTTL
	}
	if ttl <= 0 {
		return
	}

	size := int64(len(key) + len(entry.Body) + len(entry.ContentType))
	if size > c.maxBytes {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
	for c.curBytes+size > c.maxBytes {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.removeElement(back)
	}

	c.items[key] = c.order.PushFront(&node{
		key:       key,
		entry:     entry,
		size:      size,
		expiresAt: time.Now().Add(ttl),
	})
	c.curBytes += size
}

// removeElement drops el from both the list and the index. Callers must hold
// c.mu.
func (c *Cache) removeElement(el *list.Element) {
	n := el.Value.(*node)
	c.order.Remove(el)
	delete(c.items, n.key)
	c.curBytes -= n.size
}

// Len reports how many entries are currently held.
func (c *Cache) Len() int {
	if !c.enabled() {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Bytes reports the approximate size of the entries currently held.
func (c *Cache) Bytes() int64 {
	if !c.enabled() {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.curBytes
}
