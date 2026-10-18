package stampede_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/stampede"
)

func TestCoalescerCollapsesConcurrentRequests(t *testing.T) {
	coalescer := stampede.NewCoalescer()
	var originInvocations atomic.Int64

	concurrency := 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-startBarrier

			res, _ := coalescer.Do("test_hotspot_key", func() stampede.CoalesceResult {
				originInvocations.Add(1)
				time.Sleep(30 * time.Millisecond) // simulate origin delay
				return stampede.CoalesceResult{
					StatusCode: 200,
					Body:       []byte("coalesced_payload"),
				}
			})

			if res.StatusCode != 200 || string(res.Body) != "coalesced_payload" {
				t.Errorf("unexpected result: %v", res)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	invocations := originInvocations.Load()
	if invocations != 1 {
		t.Fatalf("expected exactly 1 origin invocation due to single-flight collapsing, got %d", invocations)
	}

	stats := coalescer.GetStats()
	if stats.TotalOrigins != 1 {
		t.Fatalf("expected 1 total origin flight, got %d", stats.TotalOrigins)
	}
	if stats.TotalMerged < int64(concurrency-2) {
		t.Fatalf("expected ~%d merged requests, got %d", concurrency-1, stats.TotalMerged)
	}
}
