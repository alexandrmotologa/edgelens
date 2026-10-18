# RFC 9111 HTTP Caching Specification in EdgeLens

RFC 9111 obsoletes RFC 7234 and defines the core mechanics of HTTP caching across clients, gateways, and intermediate proxies. EdgeLens implements an in-memory shared caching proxy following RFC 9111 rules.

---

## 1. Freshness Lifetime Evaluation

Freshness lifetime determines how long a stored response can be reused without contacting the origin server. EdgeLens checks directives in the following order:

1. `s-maxage` response directive:
   Takes highest priority in shared caches like CDN edge nodes. If `s-maxage=86400` is present, the entry is fresh for 24 hours at the edge, even if `max-age` specifies a lower duration for browser caches.
2. `max-age` response directive:
   Applied if `s-maxage` is absent.
3. `Expires` header:
   Calculated as `Expires - Date`. If the format is invalid or `Date` is absent, the header is ignored.
4. Heuristic Freshness (RFC 9111 Section 4.2.2):
   Applied if status code is heuristically cacheable (200, 203, 204, 206, 300, 301, 308, 404, 405, 410, 414, 501) and `Last-Modified` is present.
   EdgeLens calculates heuristic freshness as 10% of the elapsed time between `Date` and `Last-Modified`, capped at 24 hours.

---

## 2. Age Calculation

Current age is calculated using the algorithm from RFC 9111 Section 4.2.3:

- `apparent_age = max(0, response_time - date_value)`
- `response_delay = response_time - request_time`
- `corrected_age_value = age_value + response_delay`
- `corrected_initial_age = max(apparent_age, corrected_age_value)`
- `resident_time = now - response_time`
- `current_age = corrected_initial_age + resident_time`

When serving hits or stale responses, EdgeLens updates the outgoing `Age` header with `current_age` in whole seconds.

---

## 3. Stale Responses and Background Revalidation

EdgeLens supports RFC 5861 and RFC 9111 stale directives:

- `stale-while-revalidate=N`:
  If an entry has expired but `current_age <= freshness_lifetime + N`, EdgeLens immediately serves the cached body with a STALE status code, then launches a non-blocking background goroutine to revalidate with the origin server.
- `stale-if-error=N`:
  If the upstream origin returns a 5xx status code or network timeout and `current_age <= freshness_lifetime + N`, EdgeLens falls back to serving the cached entry rather than propagating the upstream error.

---

## 4. Vary Matrix Evaluation

When the origin sends a `Vary` header, EdgeLens creates a secondary variant key:

1. It extracts the request header names listed in `Vary` (for example, `Accept-Encoding`, `Accept`).
2. It normalizes values by trimming whitespace and sorting keys.
3. It computes a SHA-256 hash across the name-value pairs.
4. Storage key format: `METHOD://host/path?query#vary=<hash>`.

This ensures that compressed responses (`gzip`, `br`) and format negotiations (`application/json`, `text/html`) remain separated without overwriting each other.
