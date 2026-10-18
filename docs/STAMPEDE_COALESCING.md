# Cache Stampede and Single-Flight Request Collapsing

A cache stampede (also called dog-piling or the thundering herd problem) occurs when a heavily requested resource expires in cache. Hundreds of concurrent requests notice the expired TTL in the same millisecond and simultaneously connect to the origin server, exhausting database connection pools and causing cascading outages.

---

## 1. Single-Flight Request Collapsing

EdgeLens implements in-flight deduplication using a single-flight coalescing coordinator:

1. When request A arrives for an uncached or expired key, it registers a `call` structure inside the coalescer and initiates the network connection to the origin.
2. When requests B through Z arrive for the exact same cache key while request A is in-flight, they detect the active flight.
3. Instead of creating redundant origin connections, requests B through Z increment a shared counter and block on a synchronized `sync.WaitGroup`.
4. When request A finishes, it populates the response body and headers, then unblocks all waiting goroutines.
5. All concurrent callers receive the same response payload. Exactly one network transaction reached the origin.

---

## 2. Stampede Benchmarking Lab

EdgeLens includes an interactive concurrency simulator to measure origin collapse and test single-flight protection:

- **Concurrency Range**: 10 to 500 simultaneous requests.
- **Barrier Release**: Uses a Go channel closure barrier (`close(startBarrier)`), ensuring all simulated goroutines hit the edge proxy in the exact same millisecond.
- **Toggling Collapsing**:
  - `Coalescing = false`: All concurrent requests pass through to the origin. On a 100-request test with 200ms origin delay, 100 origin connections execute in parallel.
  - `Coalescing = true`: Exactly 1 request reaches the origin. The remaining 99 requests are absorbed at the edge.

---

## 3. Telemetry and Percentile Latencies

The simulator computes the following distribution metrics:

- **Origin Penetration Rate**: `(Origin Hits / Total Requests) * 100`.
- **Latency Percentiles**: P50, P90, P95, P99, Min, and Max.
- **Resilience Score**: 0 to 100 rating based on origin protection under load.
