# CI/CD Cache Assertion Runner

EdgeLens provides an automated cache assertion runner designed for continuous integration pipelines (such as GitHub Actions, GitLab CI, and deployment smoke tests). It validates that edge endpoints, staging servers, and production CDNs adhere to strict caching policies before and after deployments.

---

## Command Line Usage

The `edgelens assert` command executes an HTTP request against a target URL, evaluates the response headers and RFC 9111 rules, checks user-defined assertions, and exits with a deterministic status code:
- **Exit Code 0**: All assertions passed.
- **Exit Code 1**: One or more assertions failed.

### Basic Syntax

```bash
edgelens assert --url <URL> [flags]
```

### Available Flags

| Flag | Default | Description |
|---|---|---|
| `--url` | *required* | Target endpoint to inspect and test |
| `--expect-status` | `""` | Expected cache status (`HIT`, `MISS`, `BYPASS`, `STALE`, etc.) |
| `--min-ttl` | `0s` | Minimum freshness TTL requirement (e.g. `60s`, `5m`, `1h`) |
| `--min-grade` | `""` | Minimum acceptable edge compliance grade (`A+`, `A`, `B`, `C`, `D`) |
| `--disallow-headers` | `""` | Comma-separated list of prohibited headers (e.g. `Set-Cookie,X-Powered-By`) |
| `--require-headers` | `""` | Comma-separated list of required headers (e.g. `Cache-Control,ETag`) |
| `--cdn` | `rfc` | CDN flavor convention (`rfc`, `cloudflare`, `fastly`, `cloudfront`, `vercel`) |
| `--json` | `false` | Output results in machine-readable JSON format for scripts |

---

## Examples

### 1. Enforce Cacheable Assets Never Leak Session Cookies

Verify that public static assets or API endpoints have a minimum TTL of 60 seconds and do not leak `Set-Cookie` headers:

```bash
edgelens assert \
  --url https://api.example.com/v1/catalog \
  --min-ttl 60s \
  --disallow-headers "Set-Cookie" \
  --min-grade B
```

Sample Terminal Output:
```text
Running EdgeLens Cache Assertions on: https://api.example.com/v1/catalog
CDN Flavor: rfc | Timestamp: 2026-10-18T21:20:00Z

[PASS] Disallowed Headers: none of the disallowed headers are present (Set-Cookie)
[PASS] Cache Grade: received 'A' (meets or exceeds minimum 'B')
[PASS] Minimum TTL: 300s (meets or exceeds minimum 60s)

Summary: All 3 assertions passed.
```

### 2. Verify Edge Cache Hit on Staging Deployment

Ensure an endpoint warms up and delivers cache hits:

```bash
# First request to warm the edge
curl -s -o /dev/null https://staging.example.com/assets/app.js

# Assert that subsequent requests result in a HIT
edgelens assert \
  --url https://staging.example.com/assets/app.js \
  --expect-status HIT \
  --require-headers "Cache-Control,ETag"
```

### 3. Machine-Readable JSON Output for CI Pipelines

Using the `--json` flag prints a structured JSON payload:

```bash
edgelens assert --url https://api.example.com/v1/catalog --min-grade A --json
```

Output:
```json
{
  "passed": true,
  "url": "https://api.example.com/v1/catalog",
  "status_code": 200,
  "cache_status": "HIT",
  "ttl_seconds": 300,
  "grade": "A",
  "score": 95,
  "checks": [
    {
      "name": "Cache Grade",
      "passed": true,
      "message": "received 'A' (meets or exceeds minimum 'A')",
      "severity": "fail"
    }
  ]
}
```

---

## GitHub Actions Workflow Example

Integrate EdgeLens directly into your GitHub Actions release workflow:

```yaml
name: Edge Cache Policy Verification

on:
  push:
    branches: [main]
  deployment_status:

jobs:
  cache-smoke-test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Build EdgeLens
        run: go build -o /usr/local/bin/edgelens ./cmd/edgelens

      - name: Test Critical Catalog Endpoint
        run: |
          edgelens assert \
            --url "https://my-app.example.com/api/products" \
            --min-ttl 300s \
            --disallow-headers "Set-Cookie,Server" \
            --min-grade B

      - name: Test Static Assets
        run: |
          edgelens assert \
            --url "https://my-app.example.com/assets/bundle.js" \
            --require-headers "ETag,Cache-Control" \
            --min-ttl 86400s \
            --min-grade A
```

---

## Programmatic REST API

In addition to the CLI, EdgeLens exposes the assertion engine via HTTP POST for remote automation:

```http
POST /api/assert
Content-Type: application/json

{
  "url": "https://api.example.com/data",
  "expect_status": "HIT",
  "min_ttl_seconds": 120,
  "min_grade": "B",
  "disallowed_headers": ["Set-Cookie"],
  "required_headers": ["Cache-Control"]
}
```

The response includes the execution duration, computed grade, individual check status, and boolean `passed` flag.
