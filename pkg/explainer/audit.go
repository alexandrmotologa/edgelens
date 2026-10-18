package explainer

import (
	"strings"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
)

// AuditFinding describes a single issue found during header analysis.
type AuditFinding struct {
	Severity    string `json:"severity"` // "critical", "high", "medium", "low", "good"
	Title       string `json:"title"`
	Description string `json:"description"`
	RuleID      string `json:"ruleId"`
	Suggestion  string `json:"suggestion"`
}

// AuditReport aggregates score, grade, and findings for an HTTP response header set.
type AuditReport struct {
	Score        int            `json:"score"`
	Grade        string         `json:"grade"`
	Findings     []AuditFinding `json:"findings"`
	CacheControl string         `json:"cacheControl"`
	Vary         string         `json:"vary"`
	ETag         string         `json:"etag"`
	LastModified string         `json:"lastModified"`
	SurrogateKey string         `json:"surrogateKey"`
}

// AuditHeaders inspects HTTP response headers against RFC 9111 and edge best practices.
func AuditHeaders(headers map[string][]string) AuditReport {
	cacheControlVal := getHeader(headers, "Cache-Control")
	surrogateControlVal := getHeader(headers, "Surrogate-Control")
	varyVal := getHeader(headers, "Vary")
	etagVal := getHeader(headers, "ETag")
	lastModVal := getHeader(headers, "Last-Modified")
	setCookieVal := getHeader(headers, "Set-Cookie")
	surrogateKeyVal := getHeader(headers, "Surrogate-Key")
	if surrogateKeyVal == "" {
		surrogateKeyVal = getHeader(headers, "Cache-Tag")
	}

	rd := cache.ParseResponseDirectives([]string{cacheControlVal}, []string{surrogateControlVal})

	score := 100
	var findings []AuditFinding

	// Rule 1: Set-Cookie with public caching
	if setCookieVal != "" && (rd.Public || rd.SMaxAge != nil) {
		score -= 35
		findings = append(findings, AuditFinding{
			Severity:    "critical",
			RuleID:      "SEC-001",
			Title:       "Set-Cookie Present with Public Shared Caching",
			Description: "The response instructs shared edge caches to store the response while setting user cookies. This creates a severe session leak vulnerability where user A's cookie is served to user B.",
			Suggestion:  "Remove Set-Cookie on cacheable assets, or mark the response 'Cache-Control: private, no-store'.",
		})
	}

	// Rule 2: Unbounded Vary: User-Agent
	if strings.Contains(strings.ToLower(varyVal), "user-agent") {
		score -= 25
		findings = append(findings, AuditFinding{
			Severity:    "high",
			RuleID:      "PERF-001",
			Title:       "Vary: User-Agent Cache Shattering",
			Description: "Varying by User-Agent generates a unique cache key variant for every browser version, operating system, and build, dropping cache hit ratio to single digits.",
			Suggestion:  "Remove User-Agent from Vary. Use responsive CSS, Client Hints (Sec-CH-UA-Mobile), or perform device detection at origin without cache fragmentation.",
		})
	}

	// Rule 3: Missing Freshness Directive
	if cacheControlVal == "" {
		score -= 20
		findings = append(findings, AuditFinding{
			Severity:    "medium",
			RuleID:      "RFC-001",
			Title:       "Missing Cache-Control Header",
			Description: "No Cache-Control header was provided. The edge proxy and browsers are forced to use heuristic freshness or bypass caching entirely.",
			Suggestion:  "Explicitly define Cache-Control (e.g. 'public, max-age=3600, stale-while-revalidate=60').",
		})
	}

	// Rule 4: Missing Revalidation Validator (ETag and Last-Modified)
	if etagVal == "" && lastModVal == "" && !rd.NoStore {
		score -= 15
		findings = append(findings, AuditFinding{
			Severity:    "medium",
			RuleID:      "PERF-002",
			Title:       "Missing ETag and Last-Modified Validators",
			Description: "Without an ETag or Last-Modified header, the edge cache cannot perform conditional 304 revalidation and must download the full body on every expiration.",
			Suggestion:  "Add a strong ETag or Last-Modified header at origin to enable 304 Not Modified exchanges.",
		})
	}

	// Rule 5: Short TTL Without Stale-While-Revalidate
	if rd.MaxAge != nil && *rd.MaxAge > 0 && *rd.MaxAge <= 300 && rd.StaleWhileRevalidate == nil {
		score -= 10
		findings = append(findings, AuditFinding{
			Severity:    "low",
			RuleID:      "PERF-003",
			Title:       "Short TTL Without Stale-While-Revalidate",
			Description: "The resource expires frequently (TTL <= 5 minutes) without an SWR window, causing latency spikes for visitors arriving during revalidation.",
			Suggestion:  "Add 'stale-while-revalidate=60' or higher to serve instant stale hits while revalidating asynchronously.",
		})
	}

	// Rule 6: Positive Validation Check
	if rd.Immutable {
		findings = append(findings, AuditFinding{
			Severity:    "good",
			RuleID:      "PERF-004",
			Title:       "Immutable Directive Configured",
			Description: "The response includes 'immutable', eliminating redundant conditional revalidations on browser reloads.",
			Suggestion:  "Keep this setting for content-hashed assets.",
		})
	}

	if surrogateKeyVal != "" {
		findings = append(findings, AuditFinding{
			Severity:    "good",
			RuleID:      "EDGE-001",
			Title:       "Surrogate Keys Configured",
			Description: "The origin provides Surrogate-Key / Cache-Tag headers for targeted distributed edge invalidation.",
			Suggestion:  "Group related entities (e.g. product, collection, tenant) to enable granular purges.",
		})
	}

	if score < 0 {
		score = 0
	}

	var grade string
	switch {
	case score >= 95:
		grade = "A+"
	case score >= 90:
		grade = "A"
	case score >= 80:
		grade = "B"
	case score >= 70:
		grade = "C"
	case score >= 60:
		grade = "D"
	default:
		grade = "F"
	}

	return AuditReport{
		Score:        score,
		Grade:        grade,
		Findings:     findings,
		CacheControl: cacheControlVal,
		Vary:         varyVal,
		ETag:         etagVal,
		LastModified: lastModVal,
		SurrogateKey: surrogateKeyVal,
	}
}

func getHeader(headers map[string][]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}
