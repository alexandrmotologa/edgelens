package mockorigin

import (
	"fmt"
	"net/http"
)

func (s *Server) registerDefaultScenarios() {
	// Scenario 1: Vary User-Agent fragmentation disaster
	s.RegisterRoute(MockRoute{
		ID:         "scenario-vary-ua",
		Path:       "/mock/scenarios/vary-user-agent",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "public, max-age=120",
			"Vary":          "User-Agent, Accept-Encoding",
			"Server":        "Origin-Flask/3.0",
		},
		Body:        `{"scenario": "vary-user-agent", "warning": "Cache will fragment by distinct User-Agent strings"}`,
		DelayMs:     50,
		Description: "Simulates accidental Vary: User-Agent which shatters CDN edge cache hit ratio into hundreds of variants.",
	})

	// Scenario 2: Leaky Set-Cookie bypassing edge cache
	s.RegisterRoute(MockRoute{
		ID:         "scenario-leaky-cookie",
		Path:       "/mock/scenarios/leaky-cookie",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "public, max-age=300",
			"Set-Cookie":    "session_id=sess_9948271a; Path=/; HttpOnly; Secure",
			"Surrogate-Key": "auth-profile",
		},
		Body:        `{"scenario": "leaky-cookie", "account_balance": 14250.00, "user_id": "usr_9981"}`,
		DelayMs:     40,
		Description: "Simulates an origin application setting session cookies on cacheable JSON, triggering security risks or CDN cache bypass.",
	})

	// Scenario 3: Stale-While-Revalidate (SWR) zero-downtime refresh
	s.RegisterRoute(MockRoute{
		ID:         "scenario-swr",
		Path:       "/mock/scenarios/stale-while-revalidate",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "public, max-age=4, stale-while-revalidate=40",
			"Surrogate-Key": "live-ticker",
		},
		Body:        `{"scenario": "stale-while-revalidate", "ticker": "BTC/USD", "price": 94820.50, "note": "Instant stale responses while revalidating in background"}`,
		DelayMs:     120,
		Description: "Demonstrates 4s TTL with 40s SWR window. Visitors receive instant stale responses while the edge revalidates in background.",
	})

	// Scenario 4: E-Commerce Multi-Tag Purge (Product + Category + Promo)
	s.RegisterRoute(MockRoute{
		ID:         "scenario-surrogate-product",
		Path:       "/mock/scenarios/product/42",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"Cache-Control":     "public, s-maxage=86400, max-age=3600",
			"Surrogate-Key":     "product:42 category:electronics promo:blackfriday tenant:acme",
			"Surrogate-Control": "max-age=86400, stale-while-revalidate=3600",
			"ETag":              `"prod-42-v1"`,
		},
		Body:        `{"id": 42, "name": "Ultra Retina Display 5K", "price": 1299.99, "stock": 14, "tags": ["product:42", "category:electronics", "promo:blackfriday"]}`,
		DelayMs:     80,
		Description: "Simulates an e-commerce product associated with 4 surrogate keys. Allows testing selective invalidation by single tag or campaign.",
	})

	// Scenario 5: Cache Stampede (Dog-Piling) Hotspot
	s.RegisterRoute(MockRoute{
		ID:         "scenario-stampede",
		Path:       "/mock/scenarios/flash-sale",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "public, max-age=2",
			"Surrogate-Key": "flash-sale",
		},
		Body:        `{"scenario": "stampede", "event": "Flash Sale Midnight Drop", "remaining_items": 5}`,
		DelayMs:     200, // 200ms origin latency to clearly highlight stampede latency spikes
		Description: "Short 2s TTL with 200ms origin delay. Perfect for testing single-flight coalescing under 200 concurrent requests.",
	})

	// Scenario 6: Weak vs Strong ETag Conditional 304 Revalidation
	s.RegisterRoute(MockRoute{
		ID:         "scenario-etag-reval",
		Path:       "/mock/scenarios/conditional-etag",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "public, max-age=5, must-revalidate",
			"ETag":          `W/"hash-rev-7712"`,
			"Last-Modified": "Sun, 18 Oct 2026 20:00:00 GMT",
		},
		Body:        `{"scenario": "conditional-etag", "version": "v1.4.2", "data": "Conditional revalidation payload"}`,
		DelayMs:     60,
		Description: "Demonstrates RFC 9111 conditional requests. Sends 304 Not Modified when client or edge supplies matching ETag.",
	})

	// Scenario 7: Authorization Header Caching with s-maxage
	s.RegisterRoute(MockRoute{
		ID:         "scenario-auth-caching",
		Path:       "/mock/scenarios/authenticated-feed",
		Method:     http.MethodGet,
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "public, s-maxage=180",
			"Vary":          "Accept-Encoding",
		},
		Body:        `{"scenario": "authenticated-feed", "status": "authenticated", "note": "RFC 9111 permits caching authenticated requests when explicitly marked public or s-maxage"}`,
		DelayMs:     70,
		Description: "Demonstrates RFC 9111 Section 3.5: requests with Authorization headers can be cached if origin sends public or s-maxage.",
	})
}

// GetScenarioURL returns the mock scenario URL for a given scenario ID.
func (s *Server) GetScenarioURL(scenarioID string, baseURL string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.routes {
		if r.ID == scenarioID {
			return fmt.Sprintf("%s%s", baseURL, r.Path)
		}
	}
	return ""
}
