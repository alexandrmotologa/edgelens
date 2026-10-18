package assert

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/explainer"
)

// CheckDetail describes a single assertion evaluated against an HTTP endpoint.
type CheckDetail struct {
	Name         string `json:"name"`
	Expected     string `json:"expected"`
	Actual       string `json:"actual"`
	Passed       bool   `json:"passed"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// AssertionOptions defines criteria for CI/CD pipeline cache assertions.
type AssertionOptions struct {
	URL               string            `json:"url"`
	Method            string            `json:"method"`
	Headers           map[string]string `json:"headers"`
	ExpectStatus      string            `json:"expectStatus"` // e.g. "HIT", "MISS", "STALE"
	MinTTLSeconds     int               `json:"minTtlSeconds"`
	MinGrade          string            `json:"minGrade"` // "A+", "A", "B", "C", "D"
	DisallowedHeaders []string          `json:"disallowedHeaders"`
	RequiredHeaders   []string          `json:"requiredHeaders"`
	Flavor            string            `json:"flavor"` // "cloudflare", "fastly", etc.
	Timeout           time.Duration     `json:"timeout"`
}

// AssertionResult contains the aggregated outcome of the CI/CD test run.
type AssertionResult struct {
	URL           string        `json:"url"`
	Passed        bool          `json:"passed"`
	HTTPStatus    int           `json:"httpStatus"`
	CacheOutcome  string        `json:"cacheOutcome"`
	CalculatedTTL int           `json:"calculatedTtl"`
	AuditGrade    string        `json:"auditGrade"`
	AuditScore    int           `json:"auditScore"`
	Checks        []CheckDetail `json:"checks"`
	DurationMs    int64         `json:"durationMs"`
	Summary       string        `json:"summary"`
}

var gradeRankMap = map[string]int{
	"F":  1,
	"D":  2,
	"C":  3,
	"B":  4,
	"A":  5,
	"A+": 6,
}

// RunAssertion executes an HTTP request and tests all edge cache assertions.
func RunAssertion(opts AssertionOptions) (*AssertionResult, error) {
	if opts.Method == "" {
		opts.Method = "GET"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}

	client := &http.Client{Timeout: opts.Timeout}
	req, err := http.NewRequest(opts.Method, opts.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", opts.URL, err)
	}
	defer resp.Body.Close()

	respDir := cache.ParseResponseDirectives(
		resp.Header.Values("Cache-Control"),
		resp.Header.Values("Surrogate-Control"),
	)
	audit := explainer.AuditHeaders(resp.Header)
	freshnessLifetime := cache.CalculateFreshnessLifetime(respDir, resp.StatusCode, resp.Header, time.Now())
	calculatedTTL := int(freshnessLifetime.Seconds())

	result := &AssertionResult{
		URL:           opts.URL,
		HTTPStatus:    resp.StatusCode,
		CalculatedTTL: calculatedTTL,
		AuditGrade:    audit.Grade,
		AuditScore:    audit.Score,
		DurationMs:    durationMs,
		Passed:        true,
	}

	// Determine cache status from CDN response headers
	detectedStatus := extractCDNStatus(resp.Header)
	result.CacheOutcome = detectedStatus

	// 1. Check ExpectStatus if specified
	if opts.ExpectStatus != "" {
		normExpected := strings.ToUpper(opts.ExpectStatus)
		normActual := strings.ToUpper(detectedStatus)
		passed := normActual == normExpected || (normExpected == "HIT" && strings.Contains(normActual, "HIT"))
		check := CheckDetail{
			Name:     "Cache Status Outcome",
			Expected: opts.ExpectStatus,
			Actual:   detectedStatus,
			Passed:   passed,
		}
		if !passed {
			check.ErrorMessage = fmt.Sprintf("Expected cache status %s, got %s", opts.ExpectStatus, detectedStatus)
			result.Passed = false
		}
		result.Checks = append(result.Checks, check)
	}

	// 2. Check MinTTL if specified
	if opts.MinTTLSeconds > 0 {
		passed := calculatedTTL >= opts.MinTTLSeconds
		check := CheckDetail{
			Name:     "Minimum Edge TTL",
			Expected: fmt.Sprintf(">= %ds", opts.MinTTLSeconds),
			Actual:   fmt.Sprintf("%ds", calculatedTTL),
			Passed:   passed,
		}
		if !passed {
			check.ErrorMessage = fmt.Sprintf("Calculated TTL of %ds is less than required %ds", calculatedTTL, opts.MinTTLSeconds)
			result.Passed = false
		}
		result.Checks = append(result.Checks, check)
	}

	// 3. Check MinGrade if specified
	if opts.MinGrade != "" {
		expectedRank := gradeRankMap[strings.ToUpper(opts.MinGrade)]
		actualRank := gradeRankMap[strings.ToUpper(audit.Grade)]
		passed := actualRank >= expectedRank
		check := CheckDetail{
			Name:     "RFC 9111 Compliance Grade",
			Expected: fmt.Sprintf(">= %s", opts.MinGrade),
			Actual:   fmt.Sprintf("%s (%d/100)", audit.Grade, audit.Score),
			Passed:   passed,
		}
		if !passed {
			check.ErrorMessage = fmt.Sprintf("Compliance grade %s failed minimum threshold of %s", audit.Grade, opts.MinGrade)
			result.Passed = false
		}
		result.Checks = append(result.Checks, check)
	}

	// 4. Check Disallowed Headers
	for _, h := range opts.DisallowedHeaders {
		val := resp.Header.Get(h)
		passed := val == ""
		check := CheckDetail{
			Name:     fmt.Sprintf("Disallow Header [%s]", h),
			Expected: "Absent",
			Actual:   val,
			Passed:   passed,
		}
		if !passed {
			check.Actual = fmt.Sprintf("Present: %s", val)
			check.ErrorMessage = fmt.Sprintf("Security/Caching violation: header '%s' is present in response", h)
			result.Passed = false
		} else {
			check.Actual = "Absent"
		}
		result.Checks = append(result.Checks, check)
	}

	// 5. Check Required Headers
	for _, h := range opts.RequiredHeaders {
		val := resp.Header.Get(h)
		passed := val != ""
		check := CheckDetail{
			Name:     fmt.Sprintf("Require Header [%s]", h),
			Expected: "Present",
			Actual:   val,
			Passed:   passed,
		}
		if !passed {
			check.Actual = "Missing"
			check.ErrorMessage = fmt.Sprintf("Required header '%s' was not found in response", h)
			result.Passed = false
		}
		result.Checks = append(result.Checks, check)
	}

	if result.Passed {
		result.Summary = fmt.Sprintf("All %d edge cache assertions passed successfully.", len(result.Checks))
	} else {
		failedCount := 0
		for _, c := range result.Checks {
			if !c.Passed {
				failedCount++
			}
		}
		result.Summary = fmt.Sprintf("%d of %d edge cache assertions failed.", failedCount, len(result.Checks))
	}

	return result, nil
}

func extractCDNStatus(header http.Header) string {
	if cf := header.Get("CF-Cache-Status"); cf != "" {
		return cf
	}
	if xc := header.Get("X-Cache"); xc != "" {
		return xc
	}
	if vc := header.Get("x-vercel-cache"); vc != "" {
		return vc
	}
	if cs := header.Get("Cache-Status"); cs != "" {
		return cs
	}
	return "UNKNOWN"
}
