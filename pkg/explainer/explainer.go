package explainer

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
)

// DecisionPhase represents a discrete step in RFC 9111 edge processing.
type DecisionPhase struct {
	Name        string   `json:"name"`
	Passed      bool     `json:"passed"`
	Description string   `json:"description"`
	Details     []string `json:"details"`
}

// Recommendation highlights optimizations and potential bugs in caching headers.
type Recommendation struct {
	Level   string `json:"level"` // "critical", "warning", "info"
	Title   string `json:"title"`
	Message string `json:"message"`
}

// Explanation is the complete diagnostic report for an HTTP transaction.
type Explanation struct {
	ID              string           `json:"id"`
	Timestamp       time.Time        `json:"timestamp"`
	URL             string           `json:"url"`
	Method          string           `json:"method"`
	FinalStatus     cache.CacheStatus `json:"finalStatus"`
	StatusCode      int              `json:"statusCode"`
	LatencyMs       int64            `json:"latencyMs"`
	OriginLatencyMs int64            `json:"originLatencyMs"`
	CDNFlavor       cdn.Flavor       `json:"cdnFlavor"`
	PrimaryKey      string           `json:"primaryKey"`
	VaryHash        string           `json:"varyHash,omitempty"`
	VaryHeaders     []string         `json:"varyHeaders,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	CurrentAge      int              `json:"currentAgeSeconds"`
	FreshnessTTL    int              `json:"freshnessTtlSeconds"`
	Phases          []DecisionPhase  `json:"phases"`
	Recommendations []Recommendation `json:"recommendations"`
	RequestHeaders  http.Header      `json:"requestHeaders"`
	ResponseHeaders http.Header      `json:"responseHeaders"`
}

// BuildExplanation generates a granular 5-phase explanation of the caching decision.
func BuildExplanation(
	id string,
	req *http.Request,
	reqDir cache.RequestDirectives,
	respHeaders http.Header,
	respDir cache.ResponseDirectives,
	primaryKey string,
	varyHash string,
	varyHeaders []string,
	cachedEntry *cache.Entry,
	status cache.CacheStatus,
	latencyMs int64,
	originLatencyMs int64,
	cdnFlavor cdn.Flavor,
	now time.Time,
) *Explanation {
	exp := &Explanation{
		ID:              id,
		Timestamp:       now,
		URL:             req.URL.String(),
		Method:          req.Method,
		FinalStatus:     status,
		LatencyMs:       latencyMs,
		OriginLatencyMs: originLatencyMs,
		CDNFlavor:       cdnFlavor,
		PrimaryKey:      primaryKey,
		VaryHash:        varyHash,
		VaryHeaders:     varyHeaders,
		RequestHeaders:  req.Header.Clone(),
		ResponseHeaders: respHeaders.Clone(),
	}

	if cachedEntry != nil {
		exp.Tags = cachedEntry.Tags
		curAge := cache.CalculateCurrentAge(cachedEntry, now)
		exp.CurrentAge = int(curAge.Seconds())
		exp.FreshnessTTL = int(cachedEntry.FreshnessLifetime.Seconds())
	}

	// Phase 1: Request Ingestion & Directive Parsing
	p1 := DecisionPhase{
		Name:   "1. Request Ingestion & Directive Evaluation",
		Passed: true,
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		p1.Passed = false
		p1.Description = fmt.Sprintf("HTTP method %s is non-idempotent; request automatically bypasses cache.", req.Method)
		p1.Details = append(p1.Details, "RFC 9111 Section 4.1 specifies that only GET and HEAD methods are safely cacheable by default.")
	} else if reqDir.NoStore {
		p1.Passed = false
		p1.Description = "Client sent Cache-Control: no-store; response cannot be served from or stored in cache."
		p1.Details = append(p1.Details, "Bypassing edge cache due to client directive.")
	} else if reqDir.NoCache {
		p1.Description = "Client sent Cache-Control: no-cache; forcing upstream revalidation before serving."
		p1.Details = append(p1.Details, "Client requested immediate validation with the origin server.")
	} else if auth := req.Header.Get("Authorization"); auth != "" && respDir.SMaxAge == nil && !respDir.Public {
		p1.Description = "Request includes Authorization header without public or s-maxage directives."
		p1.Details = append(p1.Details, "Shared edge caches must not store authenticated responses unless explicitly marked public or s-maxage.")
	} else {
		p1.Description = "Request method and client headers permit edge cache evaluation."
		p1.Details = append(p1.Details, fmt.Sprintf("Evaluated client Cache-Control: max-age=%v, min-fresh=%v, max-stale=%v",
			formatIntPtr(reqDir.MaxAge), formatIntPtr(reqDir.MinFresh), formatIntPtr(reqDir.MaxStale)))
	}
	exp.Phases = append(exp.Phases, p1)

	// Phase 2: Cache Key Generation & Vary Evaluation
	p2 := DecisionPhase{
		Name:        "2. Cache Key & Vary Matrix Evaluation",
		Passed:      true,
		Description: fmt.Sprintf("Normalized cache key: %s", primaryKey),
	}
	if len(varyHeaders) > 0 {
		p2.Details = append(p2.Details, fmt.Sprintf("Origin specifies Vary: %s", strings.Join(varyHeaders, ", ")))
		p2.Details = append(p2.Details, fmt.Sprintf("Computed secondary Vary hash: %s", varyHash))
	} else {
		p2.Details = append(p2.Details, "No Vary dimensions active for this endpoint. Single global variant.")
	}
	exp.Phases = append(exp.Phases, p2)

	// Phase 3: Cache Storage Lookup
	p3 := DecisionPhase{
		Name: "3. Cache Storage & Freshness Computation",
	}
	if cachedEntry == nil {
		p3.Passed = false
		p3.Description = "Cache MISS: No cached entry found for this key in storage."
		p3.Details = append(p3.Details, "Forwarding request to upstream origin server.")
	} else {
		eval := cache.EvaluateFreshness(cachedEntry, reqDir, respDir, now)
		p3.Passed = eval.IsFresh || eval.IsInSWR
		p3.Description = fmt.Sprintf("Entry found. Current Age: %ds, Freshness Lifetime: %ds.", int(eval.CurrentAge.Seconds()), int(eval.FreshnessLifetime.Seconds()))
		p3.Details = append(p3.Details, fmt.Sprintf("Evaluation reason: %s", eval.Reason))
		if eval.IsSoftPurged {
			p3.Details = append(p3.Details, "Entry was previously soft-purged by surrogate key.")
		}
		if eval.IsInSWR {
			p3.Details = append(p3.Details, fmt.Sprintf("Entry is stale but inside SWR window (%ds remaining). Serving stale while triggering background revalidation.", int(cachedEntry.SWRDuration.Seconds())))
		}
	}
	exp.Phases = append(exp.Phases, p3)

	// Phase 4: Upstream Revalidation
	p4 := DecisionPhase{
		Name: "4. Upstream Revalidation & Origin Exchange",
	}
	if status == cache.StatusHit {
		p4.Passed = true
		p4.Description = "No upstream revalidation required; served fresh from edge storage."
		p4.Details = append(p4.Details, fmt.Sprintf("Edge latency: %dms (0ms origin latency)", latencyMs))
	} else if status == cache.StatusRevalidated {
		p4.Passed = true
		p4.Description = "Origin validated entry with 304 Not Modified."
		p4.Details = append(p4.Details, fmt.Sprintf("Upstream conditional revalidation completed in %dms. Cached entry refreshed.", originLatencyMs))
	} else if status == cache.StatusStale {
		p4.Passed = true
		p4.Description = "Served STALE response instantaneously under Stale-While-Revalidate."
		p4.Details = append(p4.Details, "Background asynchronous revalidation task dispatched to origin.")
	} else {
		p4.Passed = false
		p4.Description = fmt.Sprintf("Fetched full fresh response from origin in %dms.", originLatencyMs)
		p4.Details = append(p4.Details, fmt.Sprintf("Origin returned HTTP status %s.", respHeaders.Get("Status")))
	}
	exp.Phases = append(exp.Phases, p4)

	// Phase 5: Storage Decision & Edge Header Injection
	p5 := DecisionPhase{
		Name: "5. Storage Decision & CDN Header Injection",
	}
	p5.Description = fmt.Sprintf("Final edge status: %s (Emulating %s)", status, cdnFlavor)
	if respDir.NoStore {
		p5.Passed = false
		p5.Details = append(p5.Details, "Response will NOT be stored in cache: Origin specified Cache-Control: no-store.")
	} else if respDir.Private {
		p5.Passed = false
		p5.Details = append(p5.Details, "Response will NOT be stored in shared edge cache: Origin specified Cache-Control: private.")
	} else if sc := respHeaders.Get("Set-Cookie"); sc != "" && respDir.SMaxAge == nil && !respDir.Public {
		p5.Passed = false
		p5.Details = append(p5.Details, "Origin returned Set-Cookie header without explicit public or s-maxage directive.")
	} else {
		p5.Passed = true
		p5.Details = append(p5.Details, "Response successfully validated as cacheable by edge proxy.")
	}
	exp.Phases = append(exp.Phases, p5)

	// Collect Recommendations and Audits
	exp.Recommendations = generateRecommendations(req, reqDir, respHeaders, respDir, varyHeaders, status)

	return exp
}

func generateRecommendations(
	req *http.Request,
	reqDir cache.RequestDirectives,
	respHeaders http.Header,
	respDir cache.ResponseDirectives,
	varyHeaders []string,
	status cache.CacheStatus,
) []Recommendation {
	var recs []Recommendation

	// 1. Set-Cookie on cacheable response
	if sc := respHeaders.Get("Set-Cookie"); sc != "" && (status == cache.StatusHit || status == cache.StatusMiss) {
		recs = append(recs, Recommendation{
			Level:   "critical",
			Title:   "Leaky Set-Cookie Header on Cacheable Response",
			Message: "The origin response includes a Set-Cookie header. If cached, one user's session cookie could be served to other visitors. CDNs typically bypass cache or strip this cookie.",
		})
	}

	// 2. Vary User-Agent fragmentation
	for _, vh := range varyHeaders {
		if strings.EqualFold(vh, "User-Agent") {
			recs = append(recs, Recommendation{
				Level:   "critical",
				Title:   "Vary: User-Agent Cache Fragmentation",
				Message: "Varying on User-Agent fragments the edge cache into hundreds of thousands of distinct device strings, effectively dropping the edge hit ratio to near zero. Normalize client hints or device types instead.",
			})
		}
	}

	// 3. Static asset missing immutable
	urlPath := req.URL.Path
	if (strings.HasSuffix(urlPath, ".js") || strings.HasSuffix(urlPath, ".css") || strings.HasSuffix(urlPath, ".png") || strings.HasSuffix(urlPath, ".svg")) &&
		(strings.Contains(urlPath, "-") || strings.Contains(urlPath, ".")) {
		if !respDir.Immutable && respDir.MaxAge != nil && *respDir.MaxAge >= 86400 {
			recs = append(recs, Recommendation{
				Level:   "info",
				Title:   "Missing Immutable Directive on Static Asset",
				Message: "This URL appears to contain a content hash. Adding 'immutable' to Cache-Control prevents browsers from sending unnecessary 304 conditional revalidation requests on reload.",
			})
		}
	}

	// 4. Missing stale-while-revalidate on dynamic feed
	if respDir.MaxAge != nil && *respDir.MaxAge > 0 && *respDir.MaxAge <= 300 && respDir.StaleWhileRevalidate == nil {
		recs = append(recs, Recommendation{
			Level:   "warning",
			Title:   "Short TTL Without Stale-While-Revalidate",
			Message: "This endpoint has a short TTL (<= 5 min). Adding 'stale-while-revalidate=60' eliminates origin latency spikes for users arriving right when the cache expires.",
		})
	}

	// 5. Cache-Control: max-age=0, must-revalidate vs no-cache
	if respDir.MaxAge != nil && *respDir.MaxAge == 0 && !respDir.NoCache {
		recs = append(recs, Recommendation{
			Level:   "info",
			Title:   "max-age=0 Equivalent to no-cache",
			Message: "Origin specified max-age=0. Consider using 'no-cache' for clarity, which explicitly requires revalidation before reuse while still permitting caching.",
		})
	}

	return recs
}

func formatIntPtr(i *int) string {
	if i == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *i)
}
