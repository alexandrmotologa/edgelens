package comparator

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
)

// FlavorComparison contains evaluated edge behavior for one CDN flavor.
type FlavorComparison struct {
	Flavor              cdn.Flavor        `json:"flavor"`
	FlavorName          string            `json:"flavorName"`
	StatusHeader        string            `json:"statusHeader"`
	StatusValue         string            `json:"statusValue"`
	CacheOutcome        cache.CacheStatus `json:"cacheOutcome"`
	EffectiveTTLSeconds int               `json:"effectiveTtlSeconds"`
	SurrogatePrecedence string            `json:"surrogatePrecedence"`
	CookieHandling      string            `json:"cookieHandling"`
	QueryStringBehavior string            `json:"queryStringBehavior"`
	InjectedHeaders     map[string]string `json:"injectedHeaders"`
	StrippedHeaders     []string          `json:"strippedHeaders"`
	KeyNotes            []string          `json:"keyNotes"`
}

// DiffItem highlights where CDN flavors diverge in their caching decision.
type DiffItem struct {
	Field       string            `json:"field"`
	Description string            `json:"description"`
	Values      map[string]string `json:"values"`
	Impact      string            `json:"impact"`
}

// ComparisonReport is the full side-by-side diagnostic outcome.
type ComparisonReport struct {
	URL             string                      `json:"url"`
	Method          string                      `json:"method"`
	StatusCode      int                         `json:"statusCode"`
	Flavors         map[string]FlavorComparison `json:"flavors"`
	Differences     []DiffItem                  `json:"differences"`
	Recommendations []string                    `json:"recommendations"`
	EvaluatedAt     time.Time                   `json:"evaluatedAt"`
}

// CompareRequestResponse evaluates a given HTTP transaction across all 5 supported CDN profiles.
func CompareRequestResponse(
	method string,
	targetURL string,
	reqHeaders http.Header,
	statusCode int,
	respHeaders http.Header,
	simulatedAgeSecs int,
) ComparisonReport {
	if method == "" {
		method = "GET"
	}
	now := time.Now()
	report := ComparisonReport{
		URL:         targetURL,
		Method:      method,
		StatusCode:  statusCode,
		Flavors:     make(map[string]FlavorComparison),
		EvaluatedAt: now,
	}

	reqDir := cache.ParseRequestDirectives(reqHeaders.Values("Cache-Control"))
	cacheControl := respHeaders.Values("Cache-Control")
	surrogateControl := respHeaders.Values("Surrogate-Control")

	hasSetCookie := respHeaders.Get("Set-Cookie") != ""
	hasAuth := reqHeaders.Get("Authorization") != ""

	// 1. Evaluate for Cloudflare
	cf := evaluateCloudflare(method, targetURL, statusCode, cacheControl, surrogateControl, hasSetCookie, hasAuth, simulatedAgeSecs)
	report.Flavors["cloudflare"] = cf

	// 2. Evaluate for Fastly
	fastly := evaluateFastly(method, targetURL, statusCode, cacheControl, surrogateControl, hasSetCookie, hasAuth, simulatedAgeSecs)
	report.Flavors["fastly"] = fastly

	// 3. Evaluate for AWS CloudFront
	cfFront := evaluateCloudFront(method, targetURL, statusCode, cacheControl, surrogateControl, hasSetCookie, hasAuth, simulatedAgeSecs)
	report.Flavors["cloudfront"] = cfFront

	// 4. Evaluate for Vercel Edge
	vercel := evaluateVercel(method, targetURL, statusCode, cacheControl, surrogateControl, hasSetCookie, hasAuth, simulatedAgeSecs)
	report.Flavors["vercel"] = vercel

	// 5. Evaluate for RFC 9111 / 9211 Standard
	rfc := evaluateRFC(method, targetURL, statusCode, cacheControl, surrogateControl, reqDir, hasSetCookie, hasAuth, simulatedAgeSecs)
	report.Flavors["rfc9111"] = rfc

	// Calculate differences
	report.Differences = computeDifferences(report.Flavors)

	// Build recommendations
	report.Recommendations = generateComparatorRecommendations(report)

	return report
}

func evaluateCloudflare(
	method string,
	targetURL string,
	statusCode int,
	cacheControl []string,
	surrogateControl []string,
	hasSetCookie, hasAuth bool,
	simulatedAge int,
) FlavorComparison {
	fc := FlavorComparison{
		Flavor:              cdn.FlavorCloudflare,
		FlavorName:          "Cloudflare Edge",
		StatusHeader:        "CF-Cache-Status",
		SurrogatePrecedence: "Uses Cache-Control (Surrogate-Control ignored without Cloudflare Enterprise)",
		CookieHandling:      "Bypasses cache if Set-Cookie is present unless overridden by Cache Rules",
		QueryStringBehavior: "Query string included in cache key by default (Standard setting)",
		InjectedHeaders:     make(map[string]string),
		StrippedHeaders:     []string{"Set-Cookie (on HIT)", "Surrogate-Key", "Cache-Tag"},
		KeyNotes:            []string{},
	}

	respDir := cache.ParseResponseDirectives(cacheControl, nil)

	// Determine cacheability
	if method != "GET" && method != "HEAD" {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "BYPASS"
		fc.KeyNotes = append(fc.KeyNotes, "Cloudflare does not cache non-GET/HEAD methods")
		return fc
	}

	if respDir.NoStore || respDir.Private {
		fc.CacheOutcome = cache.StatusDynamic
		fc.StatusValue = "DYNAMIC"
		fc.KeyNotes = append(fc.KeyNotes, "Directives no-store/private force CF-Cache-Status: DYNAMIC")
		return fc
	}

	if hasSetCookie && !respDir.Public && respDir.SMaxAge == nil {
		fc.CacheOutcome = cache.StatusDynamic
		fc.StatusValue = "DYNAMIC"
		fc.KeyNotes = append(fc.KeyNotes, "Origin Set-Cookie prevents caching (triggers DYNAMIC)")
		return fc
	}

	// Calculate TTL: Cloudflare prefers s-maxage, then max-age
	ttl := 0
	if respDir.SMaxAge != nil {
		ttl = *respDir.SMaxAge
		fc.KeyNotes = append(fc.KeyNotes, fmt.Sprintf("Using s-maxage=%d as edge TTL", ttl))
	} else if respDir.MaxAge != nil {
		ttl = *respDir.MaxAge
		fc.KeyNotes = append(fc.KeyNotes, fmt.Sprintf("Using max-age=%d as edge TTL", ttl))
	} else if isStaticExtension(targetURL) {
		ttl = 7200 // default heuristic for static extensions on Cloudflare
		fc.KeyNotes = append(fc.KeyNotes, "Cached by file extension heuristic (2 hours)")
	} else {
		fc.CacheOutcome = cache.StatusDynamic
		fc.StatusValue = "DYNAMIC"
		fc.KeyNotes = append(fc.KeyNotes, "Cloudflare does not cache HTML or APIs without explicit Cache-Control or Page Rules")
		return fc
	}

	fc.EffectiveTTLSeconds = ttl
	if simulatedAge > 0 && simulatedAge < ttl {
		fc.CacheOutcome = cache.StatusHit
		fc.StatusValue = "HIT"
		fc.InjectedHeaders["Age"] = strconv.Itoa(simulatedAge)
	} else if simulatedAge >= ttl && respDir.StaleWhileRevalidate != nil && simulatedAge <= (ttl+*respDir.StaleWhileRevalidate) {
		fc.CacheOutcome = cache.StatusStale
		fc.StatusValue = "STALE"
	} else {
		fc.CacheOutcome = cache.StatusMiss
		fc.StatusValue = "MISS"
	}

	fc.InjectedHeaders["CF-Cache-Status"] = fc.StatusValue
	fc.InjectedHeaders["Server"] = "cloudflare"
	return fc
}

func evaluateFastly(
	method string,
	targetURL string,
	statusCode int,
	cacheControl []string,
	surrogateControl []string,
	hasSetCookie, hasAuth bool,
	simulatedAge int,
) FlavorComparison {
	fc := FlavorComparison{
		Flavor:              cdn.FlavorFastly,
		FlavorName:          "Fastly Varnish Edge",
		StatusHeader:        "X-Cache",
		SurrogatePrecedence: "Surrogate-Control takes absolute precedence over Cache-Control",
		CookieHandling:      "Default VCL bypasses if Cookie or Set-Cookie is present; strips Surrogate-Key to client",
		QueryStringBehavior: "Query string is included in hash_data by default",
		InjectedHeaders:     make(map[string]string),
		StrippedHeaders:     []string{"Surrogate-Control", "Surrogate-Key"},
		KeyNotes:            []string{},
	}

	respDir := cache.ParseResponseDirectives(cacheControl, surrogateControl)

	if method != "GET" && method != "HEAD" {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "PASS"
		return fc
	}

	if respDir.NoStore || respDir.Private {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "PASS"
		fc.KeyNotes = append(fc.KeyNotes, "Fastly default VCL passes private or no-store responses")
		return fc
	}

	if hasSetCookie && !respDir.Public && respDir.SMaxAge == nil {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "PASS"
		fc.KeyNotes = append(fc.KeyNotes, "Fastly passes response because origin set a cookie")
		return fc
	}

	ttl := 0
	if len(surrogateControl) > 0 {
		fc.KeyNotes = append(fc.KeyNotes, "Surrogate-Control directives override standard Cache-Control for Fastly")
	}

	if respDir.SurrogateMaxAge != nil {
		ttl = *respDir.SurrogateMaxAge
	} else if respDir.SMaxAge != nil {
		ttl = *respDir.SMaxAge
	} else if respDir.MaxAge != nil {
		ttl = *respDir.MaxAge
	} else {
		ttl = 120 // Fastly default TTL (2 mins) if cacheable status
		fc.KeyNotes = append(fc.KeyNotes, "Fastly fallback default TTL is 120 seconds")
	}

	fc.EffectiveTTLSeconds = ttl
	if simulatedAge > 0 && simulatedAge < ttl {
		fc.CacheOutcome = cache.StatusHit
		fc.StatusValue = "HIT"
		fc.InjectedHeaders["X-Cache"] = "HIT"
		fc.InjectedHeaders["X-Cache-Hits"] = "1"
	} else {
		fc.CacheOutcome = cache.StatusMiss
		fc.StatusValue = "MISS"
		fc.InjectedHeaders["X-Cache"] = "MISS"
		fc.InjectedHeaders["X-Cache-Hits"] = "0"
	}

	fc.InjectedHeaders["X-Served-By"] = "cache-lhr-fastly"
	return fc
}

func evaluateCloudFront(
	method string,
	targetURL string,
	statusCode int,
	cacheControl []string,
	surrogateControl []string,
	hasSetCookie, hasAuth bool,
	simulatedAge int,
) FlavorComparison {
	fc := FlavorComparison{
		Flavor:              cdn.FlavorCloudFront,
		FlavorName:          "AWS CloudFront",
		StatusHeader:        "X-Cache",
		SurrogatePrecedence: "Ignores Surrogate-Control unless mapped via Lambda@Edge or CloudFront Functions",
		CookieHandling:      "Depends on CachePolicy (default Managed-CachingOptimized strips cookies from cache key)",
		QueryStringBehavior: "Managed policy strips query strings; CachingOptimized forwards query strings",
		InjectedHeaders:     make(map[string]string),
		StrippedHeaders:     []string{"Set-Cookie (on HIT)"},
		KeyNotes:            []string{},
	}

	respDir := cache.ParseResponseDirectives(cacheControl, nil)

	if method != "GET" && method != "HEAD" {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "Miss from cloudfront"
		return fc
	}

	if respDir.NoStore || respDir.Private {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "Miss from cloudfront"
		fc.KeyNotes = append(fc.KeyNotes, "CloudFront does not cache no-store or private responses")
		return fc
	}

	ttl := 0
	if respDir.SMaxAge != nil {
		ttl = *respDir.SMaxAge
	} else if respDir.MaxAge != nil {
		ttl = *respDir.MaxAge
	} else {
		ttl = 86400 // CloudFront default TTL (24h) if no directives present in CachingOptimized
		fc.KeyNotes = append(fc.KeyNotes, "Default TTL is 86400 seconds (24h) under standard CloudFront distribution")
	}

	fc.EffectiveTTLSeconds = ttl
	if simulatedAge > 0 && simulatedAge < ttl {
		fc.CacheOutcome = cache.StatusHit
		fc.StatusValue = "Hit from cloudfront"
	} else {
		fc.CacheOutcome = cache.StatusMiss
		fc.StatusValue = "Miss from cloudfront"
	}

	fc.InjectedHeaders["X-Cache"] = fc.StatusValue
	fc.InjectedHeaders["Via"] = "2.0 edgelens.cloudfront.net (CloudFront)"
	return fc
}

func evaluateVercel(
	method string,
	targetURL string,
	statusCode int,
	cacheControl []string,
	surrogateControl []string,
	hasSetCookie, hasAuth bool,
	simulatedAge int,
) FlavorComparison {
	fc := FlavorComparison{
		Flavor:              cdn.FlavorVercel,
		FlavorName:          "Vercel Edge Network",
		StatusHeader:        "x-vercel-cache",
		SurrogatePrecedence: "Reads s-maxage for edge TTL, max-age for browser TTL",
		CookieHandling:      "Bypasses cache if Authorization or Set-Cookie is present without explicit s-maxage",
		QueryStringBehavior: "Query string is included in the cache key",
		InjectedHeaders:     make(map[string]string),
		StrippedHeaders:     []string{"Set-Cookie (on HIT)"},
		KeyNotes:            []string{},
	}

	respDir := cache.ParseResponseDirectives(cacheControl, nil)

	if method != "GET" && method != "HEAD" {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "BYPASS"
		return fc
	}

	if respDir.NoStore || respDir.Private {
		fc.CacheOutcome = cache.StatusBypass
		fc.StatusValue = "BYPASS"
		return fc
	}

	ttl := 0
	if respDir.SMaxAge != nil {
		ttl = *respDir.SMaxAge
		fc.KeyNotes = append(fc.KeyNotes, "Vercel uses s-maxage strictly for edge routing cache")
	} else if respDir.MaxAge != nil {
		ttl = *respDir.MaxAge
	}

	fc.EffectiveTTLSeconds = ttl
	if simulatedAge > 0 && simulatedAge < ttl {
		fc.CacheOutcome = cache.StatusHit
		fc.StatusValue = "HIT"
	} else if simulatedAge >= ttl && respDir.StaleWhileRevalidate != nil && simulatedAge <= (ttl+*respDir.StaleWhileRevalidate) {
		fc.CacheOutcome = cache.StatusStale
		fc.StatusValue = "STALE"
	} else {
		fc.CacheOutcome = cache.StatusMiss
		fc.StatusValue = "MISS"
	}

	fc.InjectedHeaders["x-vercel-cache"] = fc.StatusValue
	fc.InjectedHeaders["Server"] = "Vercel"
	return fc
}

func evaluateRFC(
	method string,
	targetURL string,
	statusCode int,
	cacheControl []string,
	surrogateControl []string,
	reqDir cache.RequestDirectives,
	hasSetCookie, hasAuth bool,
	simulatedAge int,
) FlavorComparison {
	fc := FlavorComparison{
		Flavor:              cdn.FlavorRFC9111,
		FlavorName:          "RFC 9111 & 9211 Standard",
		StatusHeader:        "Cache-Status",
		SurrogatePrecedence: "Surrogate-Control not part of base RFC 9111 (s-maxage is the standard shared cache directive)",
		CookieHandling:      "Shared cache must not store private responses; Set-Cookie requires explicit public or s-maxage",
		QueryStringBehavior: "Full URI (including query parameters) forms the primary cache key",
		InjectedHeaders:     make(map[string]string),
		StrippedHeaders:     []string{},
		KeyNotes:            []string{},
	}

	respDir := cache.ParseResponseDirectives(cacheControl, nil)
	ttl := 0
	if respDir.SMaxAge != nil {
		ttl = *respDir.SMaxAge
	} else if respDir.MaxAge != nil {
		ttl = *respDir.MaxAge
	}

	fc.EffectiveTTLSeconds = ttl
	if simulatedAge > 0 && simulatedAge < ttl {
		fc.CacheOutcome = cache.StatusHit
		fc.StatusValue = fmt.Sprintf("EdgeLens; hit; ttl=%d", ttl-simulatedAge)
	} else if simulatedAge >= ttl && respDir.StaleWhileRevalidate != nil && simulatedAge <= (ttl+*respDir.StaleWhileRevalidate) {
		fc.CacheOutcome = cache.StatusStale
		fc.StatusValue = "EdgeLens; hit; stale"
		fc.InjectedHeaders["Warning"] = `110 - "Response is Stale"`
	} else {
		fc.CacheOutcome = cache.StatusMiss
		fc.StatusValue = "EdgeLens; fwd=uri-miss"
	}

	fc.InjectedHeaders["Cache-Status"] = fc.StatusValue
	return fc
}

func computeDifferences(flavors map[string]FlavorComparison) []DiffItem {
	var diffs []DiffItem

	// 1. Check TTL differences
	ttlMap := make(map[string]string)
	for k, f := range flavors {
		ttlMap[k] = strconv.Itoa(f.EffectiveTTLSeconds) + "s"
	}
	allSameTTL := true
	var firstTTL string
	for _, v := range ttlMap {
		if firstTTL == "" {
			firstTTL = v
		} else if firstTTL != v {
			allSameTTL = false
			break
		}
	}
	if !allSameTTL {
		diffs = append(diffs, DiffItem{
			Field:       "Effective Edge TTL",
			Description: "CDNs calculate different retention windows depending on s-maxage, Surrogate-Control, and default heuristics.",
			Values:      ttlMap,
			Impact:      "High - Inconsistent cache invalidation schedules across edge POPs.",
		})
	}

	// 2. Check Cache Outcome
	outcomeMap := make(map[string]string)
	for k, f := range flavors {
		outcomeMap[k] = string(f.CacheOutcome)
	}
	allSameOutcome := true
	var firstOutcome string
	for _, v := range outcomeMap {
		if firstOutcome == "" {
			firstOutcome = v
		} else if firstOutcome != v {
			allSameOutcome = false
			break
		}
	}
	if !allSameOutcome {
		diffs = append(diffs, DiffItem{
			Field:       "Cache Eligibility / Status",
			Description: "Some CDNs (e.g. Cloudflare) require explicit Cache-Control or file extensions, while others (Fastly/CloudFront) apply default TTLs.",
			Values:      outcomeMap,
			Impact:      "Critical - Endpoints may receive 0% cache hits on one provider while caching on another.",
		})
	}

	// 3. Surrogate Key Precedence
	surrMap := make(map[string]string)
	for k, f := range flavors {
		surrMap[k] = f.SurrogatePrecedence
	}
	diffs = append(diffs, DiffItem{
		Field:       "Surrogate-Control Precedence",
		Description: "Fastly honors Surrogate-Control over Cache-Control, while Cloudflare and CloudFront ignore it by default.",
		Values:      surrMap,
		Impact:      "Medium - Surrogate-Control based edge routing will fail if migrating to Cloudflare without Enterprise.",
	})

	return diffs
}

func generateComparatorRecommendations(rep ComparisonReport) []string {
	var recs []string

	// Check if any flavor returned DYNAMIC or BYPASS
	hasDynamic := false
	for _, f := range rep.Flavors {
		if f.CacheOutcome == cache.StatusDynamic || f.CacheOutcome == cache.StatusBypass {
			hasDynamic = true
			break
		}
	}

	if hasDynamic {
		recs = append(recs, "Provide an explicit 's-maxage=3600' or 'max-age=3600, public' to ensure uniform caching across Cloudflare, Fastly, and CloudFront.")
	}

	recs = append(recs, "Strip 'Set-Cookie' on cacheable responses at origin to avoid accidental cache bypass or session leaks.")
	recs = append(recs, "Normalize or strip non-cache query parameters (like utm_*, gclid) across all CDN providers using Edge Rules.")
	return recs
}

func isStaticExtension(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	exts := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp", ".woff", ".woff2", ".ttf"}
	for _, ext := range exts {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}
