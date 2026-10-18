# EdgeLens: Architecture and Execution Master Plan

EdgeLens is an RFC 9111 HTTP caching engine, CDN edge proxy emulator, surrogate key invalidator, and cache stampede simulator. It gives frontend, backend, and infrastructure engineers an interactive workbench to diagnose edge caching behaviors, test cache tags and surrogate keys locally, simulate origin stampedes, and verify multi-CDN status headers without deploying to production edge networks.

---

## 1. Core Engineering Requirements

### 1.1 RFC 9111 HTTP Caching Engine
- Full compliance with RFC 9111 (HTTP Caching specification).
- Freshness Lifetime Calculation:
  - Evaluation priority: `s-maxage` > `max-age` > `Expires` > Heuristic Freshness.
  - Heuristic freshness: 10% of `Date - Last-Modified` (RFC 9111 Section 4.2.2).
- Age and Expiration Tracking:
  - `current_age = apparent_age + response_delay + corrected_age + resident_time` (Section 4.2.3).
  - Accurate updates to outgoing `Age` headers on hits and stale responses.
- Directive Evaluation:
  - Response directives: `public`, `private`, `no-cache`, `no-store`, `max-age`, `s-maxage`, `must-revalidate`, `proxy-revalidate`, `immutable`, `stale-while-revalidate`, `stale-if-error`.
  - Request directives: `no-cache`, `no-store`, `max-age`, `min-fresh`, `max-stale`, `only-if-cached`.
- Conditional Upstream Revalidation:
  - Strong and weak ETag support (`If-None-Match`).
  - Modification timestamp validation (`If-Modified-Since`).
  - Seamless 304 Not Modified upstream response handling with cache metadata updating.
- Multi-Variant Caching (`Vary` Header Matrix):
  - Normalized extraction and hashing of request headers specified in `Vary`.
  - Multi-variant tree persistence per URI path.
  - Detection of over-fragmented cache keys (e.g., uncontrolled `User-Agent` variations).
- Bypass Triggers:
  - `Authorization` header without `public` or `s-maxage`.
  - `Set-Cookie` in origin response causing edge bypass (with configurable strip or bypass policies).
  - Non-idempotent HTTP methods (POST, PUT, DELETE, PATCH) bypassing cache and invalidating target cache keys.

### 1.2 Multi-CDN Flavor Emulation
- EdgeLens emulates specific commercial CDN header conventions and routing decisions:
  - **Cloudflare**: Injects `cf-cache-status` (`HIT`, `MISS`, `EXPIRED`, `STALE`, `BYPASS`, `REVALIDATED`, `DYNAMIC`). Emulates Cloudflare standard file extension rules and edge TTL defaults.
  - **Fastly**: Injects `X-Cache` (`HIT`, `MISS`), parses `Surrogate-Control` (which takes precedence over `Cache-Control`), and handles `Surrogate-Key` indexing.
  - **AWS CloudFront**: Injects `X-Cache` (`Hit from cloudfront`, `Miss from cloudfront`, `RefreshHit from cloudfront`), query parameter forwarding rules, and cookie whitelists.
  - **Vercel Edge**: Injects `x-vercel-cache` (`HIT`, `MISS`, `STALE`, `PRERENDER`, `BYPASS`), background stale-while-revalidate triggers, and `__prerender_bypass` cookie recognition.
  - **RFC 9111 Generic**: Injects standard `Age`, `Cache-Status` (RFC 9211), and `Warning` headers.
- Switchable per-request or global runtime CDN emulation mode.

### 1.3 Step-by-Step Cache Decision Explainer
- For every HTTP transaction passing through the proxy, EdgeLens computes an audit trail across 5 structured phases:
  1. **Phase 1: Request Ingestion & Directive Parsing**: Analyzes method, authorization headers, cookies, and client caching directives.
  2. **Phase 2: Cache Key Generation**: Normalizes URI, sorts query arguments, and builds the Vary variant hash.
  3. **Phase 3: Cache Storage Lookup**: Inspects storage for existing entry, calculates freshness lifetime, current age, and verifies if the entry is fresh, in the SWR window, or expired.
  4. **Phase 4: Upstream Revalidation**: Dispatches conditional requests (`If-None-Match`, `If-Modified-Since`) if expired. Records 304 vs 200 outcomes and origin latency.
  5. **Phase 5: Storage Decision & Edge Header Injection**: Formulates the cache status decision, records reasons for non-cacheability, updates tag indexes, and writes CDN response headers.
- Actionable diagnostics: flags critical performance bottlenecks, unintended bypasses, sensitive cookie leaks, and missing `immutable` directives.

### 1.4 Surrogate Key and Cache-Tag Invalidation Engine
- Multi-header tag extraction: `Surrogate-Key` (Fastly), `Cache-Tag` (Cloudflare), `Edge-Cache-Tag` (Akamai).
- High-performance inverted index in memory linking tags to active cache keys.
- Invalidation mechanisms:
  - **Hard Purge**: Immediate removal of matching entries from memory and index.
  - **Soft Purge**: Instantly expires matching entries, marking them as stale. Future requests will serve stale content while triggering non-blocking background revalidation.
  - **URL Purge**: Exact URI and wildcard prefix purges (`/products/*`).
- Tag Explorer with blast-radius calculation, listing all associated cache keys and live invalidation telemetry.

### 1.5 Cache Stampede (Dog-Piling / Thundering Herd) Coalescing Lab
- **Single-Flight Request Collapsing**:
  - Implements upstream request deduplication. When hundreds of concurrent requests arrive for an expired or missing cache key, only 1 request connects to the origin. The remaining concurrent requests await the response on synchronized channels.
- **Interactive Stampede Benchmarking Simulator**:
  - Configurable load generator running 10 to 500 concurrent goroutines against any target URL at the exact moment of TTL expiry.
  - Toggles Single-Flight Coalescing on and off to demonstrate the direct impact on origin load.
  - Collects and renders metrics: origin hit count, edge hit count, p50, p95, p99 latencies, and thundering herd penetration ratio.

### 1.6 Built-in Mock Origin Sandbox
- Embedded HTTP mock server providing fully configurable endpoints to test complex edge caching scenarios without external servers.
- Includes 7 pre-configured diagnostic scenarios:
  1. **Vary User-Agent Fragmentation**: Shows how un-normalized Vary headers shatter cache hit rates into distinct variants.
  2. **Leaky Set-Cookie Bypass**: Simulates origin frameworks setting session cookies on static responses, triggering edge cache bypass.
  3. **Stale-While-Revalidate (SWR)**: Demonstrates zero-downtime background revalidation with instantaneous stale responses.
  4. **Multi-Tag E-Commerce Purge**: Tests granular purges across `product:101`, `category:hardware`, and `campaign:blackfriday`.
  5. **Thundering Herd on 3s TTL**: Exposes origin collapse under high concurrency without request coalescing.
  6. **ETag Conditional 304 Validation**: Tests strong vs weak ETag revalidation cycles.
  7. **Private vs Public Authorization**: Shows how `s-maxage` and `public` allow caching of authenticated endpoints.
- Route editor: allows adding dynamic endpoints with customizable status codes, headers, delay latencies, and JSON/HTML bodies.

### 1.7 Edge Header Linter & Security Auditor
- Audits any URL or header block against RFC 9111 best practices and edge security guidelines.
- Computes an overall grade (A+ through F) with severity-ranked findings:
  - Critical: `Set-Cookie` present in public shared cache responses.
  - High: Missing cache control on static assets, unconstrained `Vary` headers.
  - Medium: Absence of `stale-while-revalidate` on dynamic data feeds.
  - Low: Missing `immutable` directive on hashed asset URLs.

---

## 2. Technical Stack and Architecture

- **Backend**: Go 1.27 (`net/http`, `sync`, `net/url`, `crypto/sha256`). Zero heavyweight frameworks for minimal overhead and single-binary deployment.
- **Frontend**: React 19, Vite, Tailwind CSS, Lucide icons, Recharts for stampede latency visualization.
- **Packaging**: Embedded static files via Go `embed.FS`. The web studio and API compile into a standalone `edgelens.exe` binary.
- **CLI Commands**:
  - `edgelens serve`: Starts the edge proxy, mock origin, and visual studio.
  - `edgelens inspect <url>`: CLI terminal output of RFC 9111 decision tree and header audit.
  - `edgelens purge`: CLI trigger for tag or URL invalidation.
  - `edgelens stampede`: CLI load generator testing request coalescing.

---

## 3. Implementation Roadmap

- [x] Phase 0: GitHub repository creation, topic tagging, and portfolio registry (Phase 1).
- [x] Phase 1: Core RFC 9111 caching engine, data models, age calculation, and Vary key generator.
- [x] Phase 2: CDN flavor emulators (Cloudflare, Fastly, CloudFront, Vercel, RFC).
- [x] Phase 3: Cache decision explainer with 5-phase breakdown and recommendation engine.
- [x] Phase 4: Surrogate key and cache-tag inverted index, hard and soft purge engine.
- [x] Phase 5: Cache stampede single-flight coalescer and concurrent load generator.
- [x] Phase 6: Built-in mock origin server and 8 pre-configured scenarios.
- [x] Phase 7: Edge proxy middleware, REST APIs, SSE real-time traffic feed, and CLI commands.
- [x] Phase 8: Modern React 19 visual studio frontend with 11 operational workspaces.
- [x] Phase 9: Automated tests across all caching scenarios, stampede coalescing, and surrogate key invalidation.
- [x] Phase 10: Brand visual assets (procedural SVG logo, screenshots, demo GIF), humanized README, and portfolio sync (Phase 2).
- [x] Phase 11: Production extensions (Multi-CDN Comparator, Vary Matrix visualizer, Edge Rules engine, CDN Config Exporter, CI/CD Assertions runner, cURL/HAR importer, interactive SWR scrubber, and standalone exportable diagnostic reports).
