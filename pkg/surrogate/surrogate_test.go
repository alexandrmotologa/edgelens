package surrogate_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/surrogate"
)

func TestExtractTags(t *testing.T) {
	h := make(http.Header)
	h.Add("Surrogate-Key", "prod-101 cat-tech promo-x")
	h.Add("Cache-Tag", "tenant-alpha, global")
	h.Add("Edge-Cache-Tag", "edge-london")

	tags := surrogate.ExtractTags(h)
	expected := []string{"prod-101", "cat-tech", "promo-x", "tenant-alpha", "global", "edge-london"}

	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}
}

func TestIndexAndPurge(t *testing.T) {
	store := cache.NewStore()
	idx := surrogate.NewIndex()
	pe := surrogate.NewPurgeEngine(store, idx)

	// Register 2 entries with overlapping tags
	key1 := "GET://api.example.com/item/1"
	key2 := "GET://api.example.com/item/2"

	entry1 := &cache.Entry{
		Key:               key1,
		URL:               "https://api.example.com/item/1",
		FreshnessLifetime: 300 * time.Second,
		StoredAt:          time.Now(),
		ResponseTime:      time.Now(),
	}
	entry2 := &cache.Entry{
		Key:               key2,
		URL:               "https://api.example.com/item/2",
		FreshnessLifetime: 300 * time.Second,
		StoredAt:          time.Now(),
		ResponseTime:      time.Now(),
	}

	store.Put(entry1, key1, nil)
	store.Put(entry2, key2, nil)

	idx.IndexEntry(key1, []string{"category:electronics", "brand:apple"})
	idx.IndexEntry(key2, []string{"category:electronics", "brand:sony"})

	// Verify index mapping
	keysForCat := idx.GetKeysForTag("category:electronics")
	if len(keysForCat) != 2 {
		t.Fatalf("expected 2 keys for category:electronics, got %d", len(keysForCat))
	}

	// Purge single brand:sony
	res := pe.PurgeTag("brand:sony", false)
	if res.AffectedCount != 1 || res.AffectedKeys[0] != key2 {
		t.Fatalf("expected 1 key purged for brand:sony, got %v", res)
	}

	// Verify key2 was deleted from store
	if _, ok := store.Get(key2); ok {
		t.Fatalf("key2 should have been deleted from cache")
	}

	// Verify key1 is still present
	if _, ok := store.Get(key1); !ok {
		t.Fatalf("key1 should still exist in cache")
	}

	// Test soft purge on remaining category
	resSoft := pe.PurgeTag("category:electronics", true)
	if resSoft.AffectedCount != 1 || !resSoft.IsSoft {
		t.Fatalf("expected 1 soft-purged entry, got %v", resSoft)
	}

	e1, ok := store.Get(key1)
	if !ok || !e1.IsSoftPurged {
		t.Fatalf("key1 should still exist in store but marked soft purged")
	}
}
