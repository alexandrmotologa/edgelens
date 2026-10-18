package cache_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
)

func TestParseResponseDirectives(t *testing.T) {
	ccHeaders := []string{"public, max-age=3600, stale-while-revalidate=60, immutable"}
	surrogateHeaders := []string{"max-age=86400"}

	rd := cache.ParseResponseDirectives(ccHeaders, surrogateHeaders)

	if !rd.Public {
		t.Errorf("expected Public=true")
	}
	if !rd.Immutable {
		t.Errorf("expected Immutable=true")
	}
	// Surrogate max-age=86400 was parsed first
	if rd.MaxAge == nil || *rd.MaxAge != 86400 {
		t.Errorf("expected MaxAge=86400 from Surrogate-Control, got %v", rd.MaxAge)
	}
	if rd.StaleWhileRevalidate == nil || *rd.StaleWhileRevalidate != 60 {
		t.Errorf("expected SWR=60, got %v", rd.StaleWhileRevalidate)
	}
}

func TestCalculateFreshnessLifetime(t *testing.T) {
	now := time.Now()
	header := make(http.Header)

	// Test 1: s-maxage priority
	sMaxAge := 7200
	maxAge := 3600
	rd1 := cache.ResponseDirectives{SMaxAge: &sMaxAge, MaxAge: &maxAge}
	lifetime1 := cache.CalculateFreshnessLifetime(rd1, 200, header, now)
	if lifetime1 != 7200*time.Second {
		t.Fatalf("expected s-maxage 7200s, got %v", lifetime1)
	}

	// Test 2: max-age
	rd2 := cache.ResponseDirectives{MaxAge: &maxAge}
	lifetime2 := cache.CalculateFreshnessLifetime(rd2, 200, header, now)
	if lifetime2 != 3600*time.Second {
		t.Fatalf("expected max-age 3600s, got %v", lifetime2)
	}

	// Test 3: Heuristic freshness (10% of Date - Last-Modified)
	header.Set("Date", now.UTC().Format(http.TimeFormat))
	lastMod := now.Add(-100 * time.Minute)
	header.Set("Last-Modified", lastMod.UTC().Format(http.TimeFormat))
	rd3 := cache.ResponseDirectives{}
	lifetime3 := cache.CalculateFreshnessLifetime(rd3, 200, header, now)
	expectedHeuristic := 10 * time.Minute
	if lifetime3 < expectedHeuristic-time.Second || lifetime3 > expectedHeuristic+time.Second {
		t.Fatalf("expected heuristic freshness ~10m, got %v", lifetime3)
	}
}

func TestStoreAndVaryMatrix(t *testing.T) {
	store := cache.NewStore()

	primaryKey := cache.GeneratePrimaryKey("GET", "https://api.example.com/products?sort=asc")
	varyHeaders := []string{"Accept-Encoding", "Accept"}

	// Variant 1: gzip, json
	reqHeaders1 := make(http.Header)
	reqHeaders1.Set("Accept-Encoding", "gzip")
	reqHeaders1.Set("Accept", "application/json")
	hash1, _ := cache.ComputeVaryHash(varyHeaders, reqHeaders1)
	key1 := cache.GenerateFullKey(primaryKey, hash1)

	entry1 := &cache.Entry{
		Key:               key1,
		Method:            "GET",
		URL:               "https://api.example.com/products?sort=asc",
		StatusCode:        200,
		Headers:           make(http.Header),
		Body:              []byte(`{"variant":"gzip"}`),
		BodySize:          int64(len(`{"variant":"gzip"}`)),
		FreshnessLifetime: 60 * time.Second,
		StoredAt:          time.Now(),
		ResponseTime:      time.Now(),
		RequestTime:       time.Now().Add(-10 * time.Millisecond),
	}
	store.Put(entry1, primaryKey, varyHeaders)

	// Variant 2: br, json
	reqHeaders2 := make(http.Header)
	reqHeaders2.Set("Accept-Encoding", "br")
	reqHeaders2.Set("Accept", "application/json")
	hash2, _ := cache.ComputeVaryHash(varyHeaders, reqHeaders2)
	key2 := cache.GenerateFullKey(primaryKey, hash2)

	entry2 := &cache.Entry{
		Key:               key2,
		Method:            "GET",
		URL:               "https://api.example.com/products?sort=asc",
		StatusCode:        200,
		Headers:           make(http.Header),
		Body:              []byte(`{"variant":"br"}`),
		BodySize:          int64(len(`{"variant":"br"}`)),
		FreshnessLifetime: 60 * time.Second,
		StoredAt:          time.Now(),
		ResponseTime:      time.Now(),
		RequestTime:       time.Now().Add(-10 * time.Millisecond),
	}
	store.Put(entry2, primaryKey, varyHeaders)

	// Lookup variant 1
	found1, ok1 := store.LookupVariant(primaryKey, reqHeaders1)
	if !ok1 || string(found1.Body) != `{"variant":"gzip"}` {
		t.Fatalf("expected variant 1 with gzip body, got %v", found1)
	}

	// Lookup variant 2
	found2, ok2 := store.LookupVariant(primaryKey, reqHeaders2)
	if !ok2 || string(found2.Body) != `{"variant":"br"}` {
		t.Fatalf("expected variant 2 with br body, got %v", found2)
	}

	// Test soft purge
	store.MarkSoftPurged(key1)
	eval := cache.EvaluateFreshness(entry1, cache.RequestDirectives{}, cache.ResponseDirectives{}, time.Now())
	if eval.IsFresh {
		t.Fatalf("expected soft-purged entry to NOT be fresh")
	}

	// Stats verification
	stats := store.GetStats()
	if stats.TotalEntries != 2 {
		t.Fatalf("expected 2 entries, got %d", stats.TotalEntries)
	}
}
