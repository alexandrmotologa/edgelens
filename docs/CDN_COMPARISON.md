# CDN Header Conventions and Emulation Profiles

Different Content Delivery Networks use distinct headers to expose cache status, debugging metadata, and origin routing choices. EdgeLens allows switching between these profiles during local development.

---

## 1. Comparison Matrix

| CDN Profile | Primary Status Header | Stale Status Value | Surrogate Header Used | Tag Stripping on Egress |
|---|---|---|---|---|
| **Cloudflare** | `CF-Cache-Status` | `STALE` | `Cache-Tag` | Yes |
| **Fastly** | `X-Cache` | `HIT` | `Surrogate-Control`, `Surrogate-Key` | Yes |
| **AWS CloudFront** | `X-Cache` | `RefreshHit from cloudfront` | N/A | No |
| **Vercel Edge** | `x-vercel-cache` | `STALE` | `Surrogate-Key` | Yes |
| **RFC 9111 Generic** | `Cache-Status`, `Age` | `hit; stale` | `Surrogate-Key` | Configurable |

---

## 2. Platform Details

### Cloudflare
- Injects `CF-Cache-Status: HIT | MISS | EXPIRED | STALE | BYPASS | REVALIDATED | DYNAMIC`.
- Injects `CF-Ray` with edge Point of Presence location (for example, `...-LHR`).
- Automatically strips `Set-Cookie` on cache hits to protect against user session leakage.

### Fastly
- Injects `X-Cache: HIT | MISS`.
- Injects `X-Cache-Hits: <count>`, exposing how many times an entry has been served from memory.
- `Surrogate-Control` headers from the origin take precedence over public `Cache-Control` headers.

### AWS CloudFront
- Injects `X-Cache: Hit from cloudfront` or `Miss from cloudfront`.
- Injects `Via: 2.0 edgelens.cloudfront.net (CloudFront)` and `X-Amz-Cf-Pop`.

### Vercel Edge
- Injects `x-vercel-cache: HIT | MISS | STALE`.
- Injects `x-vercel-id: <region>::<hash>`.
- Prioritizes background stale-while-revalidate for edge-rendered React Server Components and Next.js ISR.
