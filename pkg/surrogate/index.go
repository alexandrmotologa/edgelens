package surrogate

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// TagSummary provides telemetry on an active surrogate key / cache tag.
type TagSummary struct {
	Tag          string    `json:"tag"`
	EntriesCount int       `json:"entriesCount"`
	SampleKeys   []string  `json:"sampleKeys"`
	LastSeen     time.Time `json:"lastSeen"`
}

// Index maintains thread-safe bidirectional mappings between tags and cache keys.
type Index struct {
	mu        sync.RWMutex
	tagToKeys map[string]map[string]bool
	keyToTags map[string]map[string]bool
	tagTimes  map[string]time.Time
}

// NewIndex initializes an empty surrogate key inverted index.
func NewIndex() *Index {
	return &Index{
		tagToKeys: make(map[string]map[string]bool),
		keyToTags: make(map[string]map[string]bool),
		tagTimes:  make(map[string]time.Time),
	}
}

// ExtractTags parses tags from origin headers (Surrogate-Key, Cache-Tag, Edge-Cache-Tag).
func ExtractTags(h http.Header) []string {
	var tags []string
	tagMap := make(map[string]bool)

	addTokens := func(val string, sep string) {
		tokens := strings.Split(val, sep)
		for _, t := range tokens {
			clean := strings.TrimSpace(t)
			if clean != "" && !tagMap[clean] {
				tagMap[clean] = true
				tags = append(tags, clean)
			}
		}
	}

	// Surrogate-Key (Fastly: space-delimited)
	for _, v := range h.Values("Surrogate-Key") {
		addTokens(v, " ")
	}

	// Cache-Tag (Cloudflare: comma-delimited)
	for _, v := range h.Values("Cache-Tag") {
		addTokens(v, ",")
	}

	// Edge-Cache-Tag (Akamai: comma-delimited)
	for _, v := range h.Values("Edge-Cache-Tag") {
		addTokens(v, ",")
	}

	return tags
}

// IndexEntry registers a cache key under the specified tags.
func (idx *Index) IndexEntry(key string, tags []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear previous mappings for this key if re-indexing
	if existingTags, ok := idx.keyToTags[key]; ok {
		for t := range existingTags {
			if keys, found := idx.tagToKeys[t]; found {
				delete(keys, key)
				if len(keys) == 0 {
					delete(idx.tagToKeys, t)
				}
			}
		}
	}

	if len(tags) == 0 {
		delete(idx.keyToTags, key)
		return
	}

	tagSet := make(map[string]bool, len(tags))
	now := time.Now()

	for _, t := range tags {
		tagSet[t] = true
		if idx.tagToKeys[t] == nil {
			idx.tagToKeys[t] = make(map[string]bool)
		}
		idx.tagToKeys[t][key] = true
		idx.tagTimes[t] = now
	}

	idx.keyToTags[key] = tagSet
}

// RemoveEntry removes a cache key from all tag indexes.
func (idx *Index) RemoveEntry(key string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tags, ok := idx.keyToTags[key]
	if !ok {
		return
	}

	for t := range tags {
		if keys, found := idx.tagToKeys[t]; found {
			delete(keys, key)
			if len(keys) == 0 {
				delete(idx.tagToKeys, t)
				delete(idx.tagTimes, t)
			}
		}
	}

	delete(idx.keyToTags, key)
}

// GetKeysForTag returns all cache keys associated with a surrogate key / tag.
func (idx *Index) GetKeysForTag(tag string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	keyMap, ok := idx.tagToKeys[tag]
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(keyMap))
	for k := range keyMap {
		keys = append(keys, k)
	}
	return keys
}

// ListTags returns all active tags with their counts and sample keys.
func (idx *Index) ListTags() []TagSummary {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	summaries := make([]TagSummary, 0, len(idx.tagToKeys))
	for tag, keys := range idx.tagToKeys {
		sample := make([]string, 0, 3)
		for k := range keys {
			sample = append(sample, k)
			if len(sample) >= 3 {
				break
			}
		}

		summaries = append(summaries, TagSummary{
			Tag:          tag,
			EntriesCount: len(keys),
			SampleKeys:   sample,
			LastSeen:     idx.tagTimes[tag],
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].EntriesCount > summaries[j].EntriesCount
	})

	return summaries
}
