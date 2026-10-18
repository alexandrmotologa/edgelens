# Surrogate Keys and Cache Tag Invalidation

Surrogate keys allow applications to assign logical labels to cached HTTP responses. When underlying data changes, the application sends a single purge command specifying the affected tag, immediately invalidating every cached document associated with that tag.

---

## 1. Supported Origin Headers

EdgeLens parses surrogate tags across commercial CDN formats:

- `Surrogate-Key` (Fastly convention): Space-delimited tags, for example `product:42 category:laptops promo:blackfriday`.
- `Cache-Tag` (Cloudflare convention): Comma-delimited tags, for example `product:42, category:laptops, promo:blackfriday`.
- `Edge-Cache-Tag` (Akamai convention): Comma-delimited tags.

On response egress, EdgeLens strips these headers before sending the payload to downstream clients, preventing internal application tag topologies from leaking publicly.

---

## 2. Inverted Index Architecture

EdgeLens maintains an in-memory inverted index:

- `tagToKeys`: Maps each tag string to a hash set of full cache keys.
- `keyToTags`: Maps each cache key to its assigned tags for clean garbage collection.

When a response is cached or updated, EdgeLens indexes all extracted tags in constant time O(T), where T is the number of tags on that response.

---

## 3. Hard Purge vs Soft Purge

### Hard Purge
- Removes matching cache keys directly from memory and deletes index references.
- The next request for any affected resource results in an edge MISS, blocking until the origin responds.

### Soft Purge
- Flags matching cache keys as expired by setting `IsSoftPurged = true`.
- If the endpoint configured `stale-while-revalidate`, future visitors receive the existing response with `STALE` status in sub-millisecond time.
- A single background flight connects to the origin to fetch fresh content, avoiding traffic spikes on origin databases.

---

## 4. API and CLI Usage

### Purge via HTTP API
```bash
# Purge by tag (Hard Purge)
curl -X POST http://localhost:8080/api/cache/purge \
  -H "Content-Type: application/json" \
  -d '{"type": "tag", "target": "product:42", "isSoft": false}'

# Purge by tag (Soft Purge)
curl -X POST http://localhost:8080/api/cache/purge \
  -H "Content-Type: application/json" \
  -d '{"type": "tag", "target": "promo:blackfriday", "isSoft": true}'

# Purge by URL prefix
curl -X POST http://localhost:8080/api/cache/purge \
  -H "Content-Type: application/json" \
  -d '{"type": "url", "target": "http://localhost:8080/api/products/*", "wildcard": true}'
```

### Purge via CLI
```bash
# Hard purge
edgelens purge --tag promo:blackfriday

# Soft purge
edgelens purge --tag category:electronics --soft

# Purge all entries
edgelens purge --all
```
