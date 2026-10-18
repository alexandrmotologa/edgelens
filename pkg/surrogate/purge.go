package surrogate

import (
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
)

// PurgeResult records telemetry for an invalidation operation.
type PurgeResult struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"` // "tag", "url", "all"
	Target        string    `json:"target"`
	IsSoft        bool      `json:"isSoft"`
	AffectedCount int       `json:"affectedCount"`
	AffectedKeys  []string  `json:"affectedKeys"`
	Timestamp     time.Time `json:"timestamp"`
	DurationMs    int64     `json:"durationMs"`
}

// PurgeEngine coordinates invalidation between the index and cache store.
type PurgeEngine struct {
	store   *cache.Store
	index   *Index
	mu      sync.RWMutex
	history []PurgeResult
}

// NewPurgeEngine creates a purge engine with history buffer.
func NewPurgeEngine(store *cache.Store, index *Index) *PurgeEngine {
	return &PurgeEngine{
		store:   store,
		index:   index,
		history: make([]PurgeResult, 0, 50),
	}
}

// PurgeTag invalidates all cache entries mapped to a surrogate key or tag.
func (pe *PurgeEngine) PurgeTag(tag string, soft bool) PurgeResult {
	start := time.Now()
	keys := pe.index.GetKeysForTag(tag)

	affected := make([]string, 0, len(keys))
	for _, k := range keys {
		if soft {
			if pe.store.MarkSoftPurged(k) {
				affected = append(affected, k)
			}
		} else {
			if pe.store.Delete(k) {
				pe.index.RemoveEntry(k)
				affected = append(affected, k)
			}
		}
	}

	res := PurgeResult{
		ID:            time.Now().Format("20060102150405.000"),
		Type:          "tag",
		Target:        tag,
		IsSoft:        soft,
		AffectedCount: len(affected),
		AffectedKeys:  affected,
		Timestamp:     start,
		DurationMs:    time.Since(start).Milliseconds(),
	}

	pe.recordHistory(res)
	return res
}

// PurgeURL invalidates cache entries matching a specific URL or wildcard pattern.
func (pe *PurgeEngine) PurgeURL(targetURL string, wildcard bool, soft bool) PurgeResult {
	start := time.Now()
	summaries := pe.store.ListSummaries()

	cleanTarget := strings.TrimSpace(targetURL)
	var affected []string

	for _, s := range summaries {
		match := false
		if wildcard {
			prefix := strings.TrimSuffix(cleanTarget, "*")
			if strings.HasPrefix(s.URL, prefix) {
				match = true
			}
		} else {
			if s.URL == cleanTarget {
				match = true
			}
		}

		if match {
			if soft {
				if pe.store.MarkSoftPurged(s.Key) {
					affected = append(affected, s.Key)
				}
			} else {
				if pe.store.Delete(s.Key) {
					pe.index.RemoveEntry(s.Key)
					affected = append(affected, s.Key)
				}
			}
		}
	}

	res := PurgeResult{
		ID:            time.Now().Format("20060102150405.000"),
		Type:          "url",
		Target:        targetURL,
		IsSoft:        soft,
		AffectedCount: len(affected),
		AffectedKeys:  affected,
		Timestamp:     start,
		DurationMs:    time.Since(start).Milliseconds(),
	}

	pe.recordHistory(res)
	return res
}

// PurgeAll flushes or soft-purges the entire cache store.
func (pe *PurgeEngine) PurgeAll(soft bool) PurgeResult {
	start := time.Now()
	var count int
	var affected []string

	if soft {
		summaries := pe.store.ListSummaries()
		for _, s := range summaries {
			if pe.store.MarkSoftPurged(s.Key) {
				affected = append(affected, s.Key)
			}
		}
		count = len(affected)
	} else {
		summaries := pe.store.ListSummaries()
		for _, s := range summaries {
			affected = append(affected, s.Key)
			pe.index.RemoveEntry(s.Key)
		}
		count = pe.store.PurgeAll()
	}

	res := PurgeResult{
		ID:            time.Now().Format("20060102150405.000"),
		Type:          "all",
		Target:        "*",
		IsSoft:        soft,
		AffectedCount: count,
		AffectedKeys:  affected,
		Timestamp:     start,
		DurationMs:    time.Since(start).Milliseconds(),
	}

	pe.recordHistory(res)
	return res
}

func (pe *PurgeEngine) recordHistory(res PurgeResult) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Prepend to history, keep max 50 entries
	pe.history = append([]PurgeResult{res}, pe.history...)
	if len(pe.history) > 50 {
		pe.history = pe.history[:50]
	}
}

// GetHistory returns recent invalidation operations.
func (pe *PurgeEngine) GetHistory() []PurgeResult {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	out := make([]PurgeResult, len(pe.history))
	copy(out, pe.history)
	return out
}
