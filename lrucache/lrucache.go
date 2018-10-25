package lrucache

import (
	"fmt"
	"sync"
	"time"
)

// LRUCache stores a bounded number of key value pairs. If the cache exceeds its
// maximum capacity, the least recently used entry is evicted.
type LRUCache interface {
	// Put adds a new key value pair to the cache. If the key is already in the
	// cache, the previous entry is evicted. If the cache is at maximum capacity,
	// the least recently used entry is evicted.
	Put(key, value string)

	// Get returns the value of the given key in the cache. The returned cache
	// entry is marked as recently used. If the key does not exist in the cache,
	// ok is false.
	Get(key string) (value string, ok bool)

	// Delete removes an entry from the cache. If the given key does not exist,
	// ok is false.
	Delete(key string) (ok bool)

	// Len returns the count of entries in the cache.
	Len() int
}

type lruCacheEntry struct {
	key, value       string
	expiry           time.Time
	lruPrev, lruNext *lruCacheEntry
	expPrev, expNext *lruCacheEntry
}

func (e *lruCacheEntry) IsExpired() bool {
	if e.expiry.IsZero() {
		return false
	}
	return time.Now().After(e.expiry)
}

func (e *lruCacheEntry) String() string {
	if e == nil {
		return "<nil>"
	}
	key := e.key
	if key == "" {
		key = "<nil>"
	}
	value := e.value
	if value == "" {
		value = "<nil>"
	}
	if e.expiry.IsZero() {
		return fmt.Sprintf("%s=%s", key, value)
	}
	return fmt.Sprintf("%s=%v (expires: %v)", key, value, e.expiry)
}

type lruCache struct {
	sync.Mutex

	maxSize    int
	ttl        time.Duration
	head, tail lruCacheEntry
	m          map[string]*lruCacheEntry
}

// New returns a new LRUCache. If maxSize is greater than zero, when new entries
// are added beyond this limit, the least recently used entry is evicted. If the
// given ttl is greater than zero, when entries are retrieved that are older
// than this limit, the entries are evicted and not returned.
func New(maxSize int, ttl time.Duration) LRUCache {
	if maxSize < 0 {
		panic("invalid max size")
	}
	if ttl < 0 {
		panic("invalid ttl")
	}
	initSize := 64
	if maxSize > 0 && maxSize < initSize {
		initSize = maxSize
	}
	c := &lruCache{
		maxSize: maxSize,
		ttl:     ttl,
		m:       make(map[string]*lruCacheEntry, initSize),
	}
	c.head.lruNext = &c.tail
	c.head.expNext = &c.tail
	c.tail.lruPrev = &c.head
	c.tail.expPrev = &c.head
	return c
}

func (c *lruCache) Put(key, value string) {
	c.Lock()
	defer c.Unlock()

	e, ok := c.m[key]
	if ok {
		c.delete(e)
	}

	e = &lruCacheEntry{
		key:     key,
		value:   value,
		lruPrev: &c.head,
		lruNext: c.head.lruNext,
		expPrev: &c.head,
		expNext: c.head.expNext,
	}
	if c.ttl > 0 {
		e.expiry = time.Now().Add(c.ttl)
	}
	c.m[key] = e
	c.head.lruNext.lruPrev = e
	c.head.lruNext = e
	c.head.expNext.expPrev = e
	c.head.expNext = e
	c.trim()
}

func (c *lruCache) Get(key string) (value string, ok bool) {
	c.Lock()
	defer c.Unlock()

	var e *lruCacheEntry
	e, ok = c.m[key]
	if !ok {
		return
	}

	if e.IsExpired() {
		ok = false
		c.delete(e)
		return
	}

	value = e.value
	e.lruPrev.lruNext = e.lruNext
	e.lruNext.lruPrev = e.lruPrev
	e.lruPrev = &c.head
	e.lruNext = c.head.lruNext
	c.head.lruNext.lruPrev = e
	c.head.lruNext = e
	return
}

func (c *lruCache) Delete(key string) (ok bool) {
	c.Lock()
	defer c.Unlock()

	var e *lruCacheEntry
	e, ok = c.m[key]
	if !ok {
		return
	}
	c.delete(e)
	return
}

// Len returns the number of entries in the LRUCache. The returned value may
// include expired cache entries.
func (c *lruCache) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.m)
}

// delist removes the given entry from both the LRU and Expiry lists.
func (c *lruCache) delist(e *lruCacheEntry) {
	if e == nil || e == &c.head || e == &c.tail {
		panic("cannot delist nil, head or tail")
	}
	e.lruPrev.lruNext = e.lruNext
	e.lruNext.lruPrev = e.lruPrev
	e.expPrev.expNext = e.expNext
	e.expNext.expPrev = e.expPrev
}

// delete removes an entry from the cache.
func (c *lruCache) delete(e *lruCacheEntry) {
	c.delist(e)
	delete(c.m, e.key)
}

// trim evicts a single entry if the cache has exceeded maxSize. If the oldest
// entry has expired, it is evicted. Otherwise the least recently used entry is
// evicted.
func (c *lruCache) trim() {
	if c.maxSize <= 0 || len(c.m) <= c.maxSize {
		return
	}
	if c.tail.expPrev.IsExpired() {
		c.delete(c.tail.expPrev)
		return
	}
	c.delete(c.tail.lruPrev)
}
