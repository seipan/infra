package cache

import (
	"strings"
	"testing"
	"time"
)

func found(body string) Entry {
	return Entry{Body: []byte(body), ContentType: "text/html; charset=utf-8", Found: true}
}

func TestGetReturnsStoredEntry(t *testing.T) {
	c := New(time.Minute, time.Minute, 1<<20)
	c.Put("blog/index.html", found("hello"))

	entry, ok := c.Get("blog/index.html")
	if !ok {
		t.Fatal("expected a hit")
	}
	if string(entry.Body) != "hello" {
		t.Errorf("body = %q, want %q", entry.Body, "hello")
	}
	if !entry.Found {
		t.Error("Found = false, want true")
	}
}

func TestGetMissesUnknownKey(t *testing.T) {
	c := New(time.Minute, time.Minute, 1<<20)
	if _, ok := c.Get("blog/nothing.html"); ok {
		t.Error("expected a miss")
	}
}

func TestNegativeEntryIsCached(t *testing.T) {
	c := New(time.Minute, time.Minute, 1<<20)
	c.Put("blog/wp-login.php", Entry{Found: false})

	entry, ok := c.Get("blog/wp-login.php")
	if !ok {
		t.Fatal("expected a hit")
	}
	if entry.Found {
		t.Error("Found = true, want false")
	}
}

func TestEntriesExpire(t *testing.T) {
	c := New(10*time.Millisecond, time.Minute, 1<<20)
	c.Put("blog/index.html", found("hello"))

	time.Sleep(20 * time.Millisecond)

	if _, ok := c.Get("blog/index.html"); ok {
		t.Error("expected the entry to have expired")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d, want 0; the expired entry should have been dropped", c.Len())
	}
}

func TestNegativeEntriesUseTheirOwnTTL(t *testing.T) {
	c := New(time.Minute, 10*time.Millisecond, 1<<20)
	c.Put("blog/index.html", found("hello"))
	c.Put("blog/missing.html", Entry{Found: false})

	time.Sleep(20 * time.Millisecond)

	if _, ok := c.Get("blog/missing.html"); ok {
		t.Error("negative entry should have expired")
	}
	if _, ok := c.Get("blog/index.html"); !ok {
		t.Error("found entry should still be live")
	}
}

func TestNegativeEntriesSkippedWhenTTLIsZero(t *testing.T) {
	c := New(time.Minute, 0, 1<<20)
	c.Put("blog/missing.html", Entry{Found: false})

	if _, ok := c.Get("blog/missing.html"); ok {
		t.Error("expected negative caching to be off")
	}
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	// Room for two of these entries, not three.
	one := found(strings.Repeat("a", 100))
	c := New(time.Minute, time.Minute, 2*int64(len("key-a")+100+len(one.ContentType)))

	c.Put("key-a", one)
	c.Put("key-b", one)
	// Touch key-a so key-b becomes the least recently used.
	if _, ok := c.Get("key-a"); !ok {
		t.Fatal("key-a should still be cached")
	}
	c.Put("key-c", one)

	if _, ok := c.Get("key-b"); ok {
		t.Error("key-b should have been evicted")
	}
	for _, k := range []string{"key-a", "key-c"} {
		if _, ok := c.Get(k); !ok {
			t.Errorf("%s should still be cached", k)
		}
	}
}

func TestStaysWithinSizeBound(t *testing.T) {
	maxBytes := int64(4096)
	c := New(time.Minute, time.Minute, maxBytes)

	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		c.Put(k, found(strings.Repeat("x", 1000)))
	}

	if c.Bytes() > maxBytes {
		t.Errorf("Bytes = %d, want <= %d", c.Bytes(), maxBytes)
	}
}

func TestOversizedEntryIsNotStored(t *testing.T) {
	c := New(time.Minute, time.Minute, 100)
	c.Put("key", found(strings.Repeat("x", 1000)))

	if _, ok := c.Get("key"); ok {
		t.Error("an entry larger than the cache should not be stored")
	}
	if c.Bytes() != 0 {
		t.Errorf("Bytes = %d, want 0", c.Bytes())
	}
}

func TestReplacingKeyDoesNotDoubleCount(t *testing.T) {
	c := New(time.Minute, time.Minute, 1<<20)
	c.Put("key", found("first"))
	before := c.Bytes()
	c.Put("key", found("first"))

	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1", c.Len())
	}
	if c.Bytes() != before {
		t.Errorf("Bytes = %d, want %d", c.Bytes(), before)
	}
}

func TestDisabledCacheStoresNothing(t *testing.T) {
	for name, c := range map[string]*Cache{
		"zero ttl":      New(0, time.Minute, 1<<20),
		"zero maxBytes": New(time.Minute, time.Minute, 0),
		"nil":           nil,
	} {
		c.Put("key", found("hello"))
		if _, ok := c.Get("key"); ok {
			t.Errorf("%s: expected caching to be off", name)
		}
	}
}
