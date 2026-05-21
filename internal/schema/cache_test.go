package schema

import (
	"testing"
	"time"
)

func TestTTLCacheRoundTrip(t *testing.T) {
	c := newTTLCache(50 * time.Millisecond)
	c.set("k", "v")
	got, ok := c.get("k")
	if !ok || got.(string) != "v" {
		t.Fatalf("expected hit with v, got ok=%v got=%v", ok, got)
	}
}

func TestTTLCacheExpiry(t *testing.T) {
	c := newTTLCache(10 * time.Millisecond)
	c.set("k", "v")
	time.Sleep(20 * time.Millisecond)
	if _, ok := c.get("k"); ok {
		t.Fatal("expected expired entry to miss")
	}
}

func TestTTLCacheInvalidatePrefix(t *testing.T) {
	c := newTTLCache(1 * time.Hour)
	c.set("tables:conn-A:ks1", 1)
	c.set("tables:conn-A:ks2", 2)
	c.set("tables:conn-B:ks1", 3)
	c.set("keyspaces:conn-A", 4)

	c.invalidatePrefix("tables:conn-A:")

	if _, ok := c.get("tables:conn-A:ks1"); ok {
		t.Fatal("conn-A:ks1 should be evicted")
	}
	if _, ok := c.get("tables:conn-A:ks2"); ok {
		t.Fatal("conn-A:ks2 should be evicted")
	}
	if _, ok := c.get("tables:conn-B:ks1"); !ok {
		t.Fatal("conn-B:ks1 should remain")
	}
	if _, ok := c.get("keyspaces:conn-A"); !ok {
		t.Fatal("keyspaces:conn-A should remain (different prefix)")
	}
}

func TestCachePrefixesCoverAllOurKeys(t *testing.T) {
	prefixes := cachePrefixes("CONN")
	for _, key := range []string{
		"cluster:CONN",
		"keyspaces:CONN",
		"tables:CONN:ks",
		"table:CONN:ks:t",
		"types:CONN:ks",
	} {
		matched := false
		for _, p := range prefixes {
			if len(key) >= len(p) && key[:len(p)] == p {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("no cachePrefix covers %q", key)
		}
	}
}
