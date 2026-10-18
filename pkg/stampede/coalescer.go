package stampede

import (
	"net/http"
	"sync"
	"sync/atomic"
)

// CoalesceResult captures the shared response from a coalesced origin fetch.
type CoalesceResult struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Err        error
}

type call struct {
	wg     sync.WaitGroup
	res    CoalesceResult
	shared atomic.Int64 // number of callers who shared this single flight
}

// Coalescer coordinates single-flight request collapsing across concurrent requests.
type Coalescer struct {
	mu           sync.Mutex
	calls        map[string]*call
	totalMerged  atomic.Int64
	totalOrigins atomic.Int64
}

// NewCoalescer initializes a single-flight coalescer.
func NewCoalescer() *Coalescer {
	return &Coalescer{
		calls: make(map[string]*call),
	}
}

// Do executes or merges an upstream origin request by key.
// If another goroutine is already fetching for the same key, this call waits and shares the result.
func (c *Coalescer) Do(key string, fn func() CoalesceResult) (CoalesceResult, bool) {
	c.mu.Lock()
	if cl, ok := c.calls[key]; ok {
		c.mu.Unlock()
		cl.shared.Add(1)
		c.totalMerged.Add(1)
		cl.wg.Wait()
		return cl.res, true // true indicates request was coalesced / merged
	}

	cl := &call{}
	cl.wg.Add(1)
	c.calls[key] = cl
	c.mu.Unlock()

	c.totalOrigins.Add(1)

	cl.res = fn()
	cl.wg.Done()

	c.mu.Lock()
	delete(c.calls, key)
	c.mu.Unlock()

	return cl.res, false // false indicates this was the flight leader that contacted origin
}

// Stats returns coalescing efficiency metrics.
type CoalescerStats struct {
	TotalMerged  int64 `json:"totalMerged"`
	TotalOrigins int64 `json:"totalOrigins"`
}

// GetStats retrieves coalescer telemetry.
func (c *Coalescer) GetStats() CoalescerStats {
	return CoalescerStats{
		TotalMerged:  c.totalMerged.Load(),
		TotalOrigins: c.totalOrigins.Load(),
	}
}
