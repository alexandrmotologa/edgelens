package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Store provides high-performance thread-safe storage for cached responses with Vary matrix support.
type Store struct {
	mu           sync.RWMutex
	entries      map[string]*Entry
	varyMap      map[string][]string // primaryKey -> list of Vary header names
	variants     map[string][]string // primaryKey -> list of full cache keys
	totalHits    atomic.Int64
	totalMisses  atomic.Int64
	totalBypases atomic.Int64
}

// NewStore initializes an in-memory cache store.
func NewStore() *Store {
	return &Store{
		entries:  make(map[string]*Entry),
		varyMap:  make(map[string][]string),
		variants: make(map[string][]string),
	}
}

// GeneratePrimaryKey creates a canonical URI-based primary key.
func GeneratePrimaryKey(method, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToUpper(method) + ":" + rawURL
	}

	// Canonicalize query parameters (sort keys for cache key consistency)
	q := parsed.Query()
	canonicalQuery := q.Encode()

	host := strings.ToLower(parsed.Host)
	path := parsed.Path
	if path == "" {
		path = "/"
	}

	key := strings.ToUpper(method) + "://" + host + path
	if canonicalQuery != "" {
		key += "?" + canonicalQuery
	}
	return key
}

// ComputeVaryHash creates a deterministic hash of the request headers specified by Vary.
func ComputeVaryHash(varyHeaders []string, reqHeaders http.Header) (string, map[string]string) {
	if len(varyHeaders) == 0 {
		return "", nil
	}

	captured := make(map[string]string)
	var pairs []string

	for _, vh := range varyHeaders {
		vhClean := strings.TrimSpace(vh)
		if vhClean == "" || vhClean == "*" {
			continue
		}
		val := reqHeaders.Get(vhClean)
		captured[strings.ToLower(vhClean)] = val
		pairs = append(pairs, strings.ToLower(vhClean)+"="+val)
	}

	if len(pairs) == 0 {
		return "", captured
	}

	sort.Strings(pairs)
	h := sha256.New()
	h.Write([]byte(strings.Join(pairs, "&")))
	return hex.EncodeToString(h.Sum(nil))[:12], captured
}

// GenerateFullKey builds the exact storage key including the Vary hash.
func GenerateFullKey(primaryKey, varyHash string) string {
	if varyHash == "" {
		return primaryKey
	}
	return primaryKey + "#vary=" + varyHash
}

// Get retrieves an entry by its full key.
func (s *Store) Get(fullKey string) (*Entry, bool) {
	s.mu.RLock()
	entry, ok := s.entries[fullKey]
	s.mu.RUnlock()

	if ok {
		s.totalHits.Add(1)
		atomic.AddInt64(&entry.Hits, 1)
		return entry, true
	}

	s.totalMisses.Add(1)
	return nil, false
}

// LookupVariant finds a cached entry matching the primary URI key and the request headers against Vary dimensions.
func (s *Store) LookupVariant(primaryKey string, reqHeaders http.Header) (*Entry, bool) {
	s.mu.RLock()
	varyHeaders, hasVary := s.varyMap[primaryKey]
	s.mu.RUnlock()

	if !hasVary || len(varyHeaders) == 0 {
		return s.Get(primaryKey)
	}

	varyHash, _ := ComputeVaryHash(varyHeaders, reqHeaders)
	fullKey := GenerateFullKey(primaryKey, varyHash)
	return s.Get(fullKey)
}

// Put stores an entry into the cache, updating Vary and variant index maps.
func (s *Store) Put(entry *Entry, primaryKey string, varyHeaders []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[entry.Key] = entry

	if len(varyHeaders) > 0 {
		s.varyMap[primaryKey] = varyHeaders
	}

	// Track variants for primary key
	existing := s.variants[primaryKey]
	found := false
	for _, k := range existing {
		if k == entry.Key {
			found = true
			break
		}
	}
	if !found {
		s.variants[primaryKey] = append(existing, entry.Key)
	}
}

// Delete removes a specific cache key and cleans up variant mappings.
func (s *Store) Delete(fullKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[fullKey]; !exists {
		return false
	}

	delete(s.entries, fullKey)

	// Clean up variants map
	for pKey, variantList := range s.variants {
		for i, k := range variantList {
			if k == fullKey {
				s.variants[pKey] = append(variantList[:i], variantList[i+1:]...)
				break
			}
		}
		if len(s.variants[pKey]) == 0 {
			delete(s.variants, pKey)
			delete(s.varyMap, pKey)
		}
	}

	return true
}

// DeleteByPrimaryKey removes all variants matching a primary URI.
func (s *Store) DeleteByPrimaryKey(primaryKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	if keys, ok := s.variants[primaryKey]; ok {
		for _, k := range keys {
			delete(s.entries, k)
			count++
		}
		delete(s.variants, primaryKey)
		delete(s.varyMap, primaryKey)
	} else if _, ok := s.entries[primaryKey]; ok {
		delete(s.entries, primaryKey)
		count++
	}

	return count
}

// MarkSoftPurged flags an entry as soft-purged without immediately deleting it.
func (s *Store) MarkSoftPurged(fullKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[fullKey]; ok {
		entry.IsSoftPurged = true
		entry.SoftPurgedAt = time.Now()
		return true
	}
	return false
}

// PurgeAll flushes the entire cache.
func (s *Store) PurgeAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.entries)
	s.entries = make(map[string]*Entry)
	s.varyMap = make(map[string][]string)
	s.variants = make(map[string][]string)
	return count
}

// RecordBypass increments bypass counter.
func (s *Store) RecordBypass() {
	s.totalBypases.Add(1)
}

// Stats returns snapshot telemetry for the cache store.
type Stats struct {
	TotalEntries int64 `json:"totalEntries"`
	TotalHits    int64 `json:"totalHits"`
	TotalMisses  int64 `json:"totalMisses"`
	TotalBypasses int64 `json:"totalBypasses"`
	TotalBytes   int64 `json:"totalBytes"`
	HitRatio     float64 `json:"hitRatio"`
}

// GetStats computes telemetry metrics.
func (s *Store) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalBytes int64
	for _, e := range s.entries {
		totalBytes += e.BodySize
	}

	hits := s.totalHits.Load()
	misses := s.totalMisses.Load()
	bypasses := s.totalBypases.Load()
	totalReqs := hits + misses

	var hitRatio float64
	if totalReqs > 0 {
		hitRatio = float64(hits) / float64(totalReqs) * 100.0
	}

	return Stats{
		TotalEntries:  int64(len(s.entries)),
		TotalHits:     hits,
		TotalMisses:   misses,
		TotalBypasses: bypasses,
		TotalBytes:    totalBytes,
		HitRatio:      hitRatio,
	}
}

// ListSummaries returns light metadata for all cached entries.
func (s *Store) ListSummaries() []EntrySummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	summaries := make([]EntrySummary, 0, len(s.entries))

	for _, e := range s.entries {
		currentAge := CalculateCurrentAge(e, now)
		isFresh := !e.IsSoftPurged && (currentAge < e.FreshnessLifetime)
		isInSWR := !isFresh && e.SWRDuration > 0 && (currentAge <= e.FreshnessLifetime+e.SWRDuration)

		summaries = append(summaries, EntrySummary{
			Key:               e.Key,
			Method:            e.Method,
			URL:               e.URL,
			StatusCode:        e.StatusCode,
			BodySize:          e.BodySize,
			Tags:              e.Tags,
			StoredAt:          e.StoredAt,
			FreshnessLifetime: int(e.FreshnessLifetime.Seconds()),
			CurrentAge:        int(currentAge.Seconds()),
			IsFresh:           isFresh,
			IsInSWR:           isInSWR,
			IsSoftPurged:      e.IsSoftPurged,
			Hits:              e.Hits,
			ETag:              e.ETag,
			VaryHeaders:       e.VaryHeaders,
		})
	}

	// Sort newest stored first
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StoredAt.After(summaries[j].StoredAt)
	})

	return summaries
}
