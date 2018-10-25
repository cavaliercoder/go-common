package lrucache

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var testKeys = []string{
	"alcorns", "attwaters", "bairds", "big", "bottas", "bullers", "camas",
	"central-texas", "cherries", "chiriqui", "darien", "desert", "geomys",
	"giant", "goldmans", "hispid", "idaho", "knox-jones", "mazama", "merriams",
	"mountain", "nicaraguan", "northern", "oaxacan", "oriental-basin",
	"pappogeomys", "plains", "smoky", "southeastern", "southern", "texas",
	"thaelers", "thomomys", "townsends", "tropical", "underwoods", "variable",
	"wyoming", "yellow-faced",
}

func init() {
	rand.Seed(time.Now().Unix())
}

func randKV() (key, value string) {
	key = testKeys[rand.Intn(len(testKeys))]
	value = testKeys[rand.Intn(len(testKeys))]
	return
}

type lruCacheLogger struct {
	t *testing.T
	c LRUCache
}

func WithLogging(t *testing.T, c LRUCache) LRUCache {
	return &lruCacheLogger{t: t, c: c}
}

func (c *lruCacheLogger) Put(key, value string) {
	c.c.Put(key, value)
	c.t.Logf("Put(\"%s\", \"%s\")", key, value)
}

func (c *lruCacheLogger) Get(key string) (value string, ok bool) {
	value, ok = c.c.Get(key)
	c.t.Logf("Get(\"%s\") → (\"%s\", %v)", key, value, ok)
	return
}

func (c *lruCacheLogger) Delete(key string) (ok bool) {
	ok = c.c.Delete(key)
	c.t.Logf("Delete(\"%s\") → %v", key, ok)
	return

}

func (c *lruCacheLogger) Len() (n int) {
	n = c.c.Len()
	c.t.Logf("Len() → %d", n)
	return
}

func TestCRUD(t *testing.T) {
	key, value := randKV()
	c := WithLogging(t, New(0, 0))
	assertLen(t, c, 0)
	assertGetMissing(t, c, key)
	for i := 0; i < 1024; i++ {
		assertPut(t, c, key, value)
		assertLen(t, c, 1)
		_, value = randKV()
	}
	assertDelete(t, c, key)
	assertLen(t, c, 0)
}

func TestMaxSize(t *testing.T) {
	for maxSize := 1; maxSize <= len(testKeys); maxSize++ {
		t.Run(fmt.Sprintf("%d", maxSize), func(t *testing.T) {
			c := WithLogging(t, New(maxSize, 0))
			for i := 0; i < len(testKeys); i++ {
				c.Put(testKeys[i], testKeys[i])

				// len should never exceed maxSize
				expectLen := i + 1
				if expectLen > maxSize {
					expectLen = maxSize
				}
				assertLen(t, c, expectLen)

				// oldest keys should be evicted
				for j := 0; j < i; j++ {
					if j <= i-maxSize {
						assertGetMissing(t, c, testKeys[j])
					}
				}
			}
		})
	}
}

func TestLRU(t *testing.T) {
	// seed cache with half of all keys
	t.Log("Seeding cache")
	n := len(testKeys) / 2
	c := WithLogging(t, New(n, 0))
	for i := 0; i < n; i++ {
		key := testKeys[i]
		assertPut(t, c, key, key)
		assertLen(t, c, i+1)
	}

	// randomize usage order
	t.Log("Accessing keys in random order")
	useOrder := rand.Perm(n)
	for i := 0; i < n; i++ {
		key := testKeys[useOrder[i]]
		assertGet(t, c, key, key)
	}

	// expect eviction in correct order
	t.Log("Evicting keys one at a time")
	l := c.Len()
	for i := 0; i < n; i++ {
		key := testKeys[n+i]
		c.Put(key, key)
		assertLen(t, c, l)
		evictKey := testKeys[useOrder[i]]
		assertGetMissing(t, c, evictKey)
	}
}

func TestExpiry(t *testing.T) {
	// create an expiring cache with capacity for all keys except one
	n := len(testKeys) - 1
	ttl := 100 * time.Millisecond
	firstKey, lastKey := testKeys[0], testKeys[n]
	c := WithLogging(t, New(n, ttl))
	for i := 0; i < n; i++ {
		key := testKeys[i]
		assertPut(t, c, key, key)
		assertLen(t, c, i+1)

		// ensure first key is oldest by some margin, but is never the least
		// recently used
		if i == 0 {
			time.Sleep(ttl / 2)
		}
		assertGet(t, c, firstKey, firstKey)
	}

	// force an eviction by inserting last key
	time.Sleep(ttl / 2)
	assertLen(t, c, n)
	assertPut(t, c, lastKey, lastKey)
	assertLen(t, c, n)

	// every key except firstKey should still be present
	for i := 0; i < n; i++ {
		key := testKeys[i+1]
		assertGet(t, c, key, key)
	}

	// ensure all keys expire
	time.Sleep(ttl)
	for i := 0; i <= n; i++ {
		key := testKeys[i]
		assertGetMissing(t, c, key)
	}
	assertLen(t, c, 0)
}

func BenchmarkPut(b *testing.B) {
	c := New(3, 0)
	for i := 0; i < b.N; i++ {
		c.Put(randKV())
	}
}

func BenchmarkGet(b *testing.B) {
	c := New(0, 0)
	for i := 0; i < len(testKeys); i++ {
		c.Put(testKeys[i], testKeys[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(testKeys[i%len(testKeys)])
	}
}

func BenchmarkCRUD(b *testing.B) {
	c := New(0, 0)
	for i := 0; i < b.N; i++ {
		key, value := randKV()
		c.Put(key, value)
		value, _ = c.Get(key)
		c.Put(key, value)
		c.Delete(key)
	}
}

func assertGet(t *testing.T, c LRUCache, key, value string) {
	actual, ok := c.Get(key)
	if actual != value || !ok {
		t.Errorf("expected: Get(\"%s\") → (\"%s\", %v), got: (\"%s\", %v)",
			key,
			value,
			true,
			actual,
			ok)
	}
}

func assertGetMissing(t *testing.T, c LRUCache, key string) {
	expectValue, expectOk := "", false
	actualValue, actualOk := c.Get(key)
	if actualOk != expectOk || actualValue != expectValue {
		t.Errorf("expected: Get(\"%s\") → (\"%s\", %v), got: (\"%s\", %v)",
			key,
			expectValue,
			expectOk,
			actualValue,
			actualOk)
		return
	}
}

func assertPut(t *testing.T, c LRUCache, key, value string) {
	c.Put(key, value)
	assertGet(t, c, key, value)
}

func assertDelete(t *testing.T, c LRUCache, key string) {
	_, ok := c.Get(key)
	if !ok {
		panic(fmt.Sprintf("tried to delete non-existing key: %s", key))
	}

	expectOk := true
	actualOk := c.Delete(key)
	if actualOk != expectOk {
		t.Errorf("expected: Delete(\"%s\") → %v, got: %v", key, expectOk, actualOk)
	}
	assertGetMissing(t, c, key)
}

func assertLen(t *testing.T, c LRUCache, n int) {
	actual := c.Len()
	if actual != n {
		t.Errorf("expected: Len() → %d, got: %d", n, actual)
	}
}
