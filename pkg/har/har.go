package har

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/explainer"
	"github.com/alexandrmotologa/edgelens/pkg/surrogate"
)

// CurlParsedRequest contains extracted parameters from a cURL command.
type CurlParsedRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

// ParseCurl parses a raw cURL command line string into structured HTTP request components.
func ParseCurl(cmd string) (*CurlParsedRequest, error) {
	cmd = strings.TrimSpace(cmd)
	if !strings.HasPrefix(cmd, "curl") {
		return nil, fmt.Errorf("command must start with 'curl'")
	}

	res := &CurlParsedRequest{
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Split command arguments respecting quotes
	args := tokenizeCommand(cmd)

	for i := 1; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-X", "--request":
			if i+1 < len(args) {
				res.Method = strings.ToUpper(args[i+1])
				i++
			}
		case "-H", "--header":
			if i+1 < len(args) {
				headerStr := args[i+1]
				if idx := strings.IndexByte(headerStr, ':'); idx != -1 {
					k := strings.TrimSpace(headerStr[:idx])
					v := strings.TrimSpace(headerStr[idx+1:])
					res.Headers[k] = v
				}
				i++
			}
		case "-A", "--user-agent":
			if i+1 < len(args) {
				res.Headers["User-Agent"] = args[i+1]
				i++
			}
		case "-b", "--cookie":
			if i+1 < len(args) {
				res.Headers["Cookie"] = args[i+1]
				i++
			}
		case "-d", "--data", "--data-raw":
			if i+1 < len(args) {
				res.Body = args[i+1]
				if res.Method == "GET" {
					res.Method = "POST"
				}
				i++
			}
		case "-I", "--head":
			res.Method = "HEAD"
		default:
			if !strings.HasPrefix(arg, "-") && (strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://")) {
				res.URL = arg
			}
		}
	}

	if res.URL == "" {
		return nil, fmt.Errorf("no valid HTTP/HTTPS URL found in curl command")
	}

	return res, nil
}

func tokenizeCommand(cmd string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	var quoteChar rune

	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if inQuotes {
			if r == quoteChar {
				inQuotes = false
			} else {
				current.WriteRune(r)
			}
		} else {
			if r == '"' || r == '\'' {
				inQuotes = true
				quoteChar = r
			} else if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else if r == '\\' && i+1 < len(runes) && (runes[i+1] == '\n' || runes[i+1] == '\r') {
				// line continuation in shell
				i++
			} else {
				current.WriteRune(r)
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// HARLog models the top-level structure of an HTTP Archive (HAR) file.
type HARLog struct {
	Log struct {
		Entries []HAREntry `json:"entries"`
	} `json:"log"`
}

// HAREntry represents a single transaction inside a HAR file.
type HAREntry struct {
	StartedDateTime string `json:"startedDateTime"`
	Time            float64 `json:"time"`
	Request         struct {
		Method  string       `json:"method"`
		URL     string       `json:"url"`
		Headers []HARHeader  `json:"headers"`
	} `json:"request"`
	Response struct {
		Status      int         `json:"status"`
		StatusText  string      `json:"statusText"`
		Headers     []HARHeader `json:"headers"`
		BodySize    int64       `json:"bodySize"`
		Content     struct {
			Size     int64  `json:"size"`
			MimeType string `json:"mimeType"`
		} `json:"content"`
	} `json:"response"`
}

// HARHeader represents an individual name-value header.
type HARHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HAREntryAnalysis is the audited outcome of one entry in a HAR waterfall.
type HAREntryAnalysis struct {
	URL                 string   `json:"url"`
	Method              string   `json:"method"`
	StatusCode          int      `json:"statusCode"`
	MimeType            string   `json:"mimeType"`
	CacheEligible       bool     `json:"cacheEligible"`
	CalculatedTTL       int      `json:"calculatedTtl"`
	CacheControl        string   `json:"cacheControl"`
	Vary                string   `json:"vary"`
	SurrogateTags       []string `json:"surrogateTags"`
	AuditGrade          string   `json:"auditGrade"`
	AuditScore          int      `json:"auditScore"`
	BlockerReason       string   `json:"blockerReason,omitempty"`
	DetectedCDNStatus   string   `json:"detectedCdnStatus,omitempty"`
}

// HARReport aggregates the whole HAR audit.
type HARReport struct {
	TotalRequests     int                `json:"totalRequests"`
	CacheableRequests int                `json:"cacheableRequests"`
	BypassRequests    int                `json:"bypassRequests"`
	CacheabilityRatio float64            `json:"cacheabilityRatio"`
	AverageTTLSeconds int                `json:"averageTtlSeconds"`
	TopBlockers       map[string]int     `json:"topBlockers"`
	Entries           []HAREntryAnalysis `json:"entries"`
}

// ParseHAR analyzes a complete HAR archive and extracts edge caching diagnostics for every asset.
func ParseHAR(data []byte) (*HARReport, error) {
	var har HARLog
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, fmt.Errorf("failed to parse HAR JSON: %w", err)
	}

	report := &HARReport{
		TotalRequests: len(har.Log.Entries),
		TopBlockers:   make(map[string]int),
		Entries:       make([]HAREntryAnalysis, 0, len(har.Log.Entries)),
	}

	totalTTL := 0
	now := time.Now()

	for _, entry := range har.Log.Entries {
		respHeader := make(http.Header)
		for _, h := range entry.Response.Headers {
			respHeader.Add(h.Name, h.Value)
		}

		respDir := cache.ParseResponseDirectives(
			respHeader.Values("Cache-Control"),
			respHeader.Values("Surrogate-Control"),
		)

		audit := explainer.AuditHeaders(respHeader)
		tags := surrogate.ExtractTags(respHeader)
		freshness := cache.CalculateFreshnessLifetime(respDir, entry.Response.Status, respHeader, now)
		ttl := int(freshness.Seconds())

		eligible, blocker := checkHAREligibility(entry.Request.Method, entry.Response.Status, respHeader, respDir)

		analysis := HAREntryAnalysis{
			URL:               entry.Request.URL,
			Method:            entry.Request.Method,
			StatusCode:        entry.Response.Status,
			MimeType:          entry.Response.Content.MimeType,
			CacheEligible:     eligible,
			CalculatedTTL:     ttl,
			CacheControl:      respHeader.Get("Cache-Control"),
			Vary:              respHeader.Get("Vary"),
			SurrogateTags:     tags,
			AuditGrade:        audit.Grade,
			AuditScore:        audit.Score,
			BlockerReason:     blocker,
			DetectedCDNStatus: getCDNHeader(respHeader),
		}

		if eligible {
			report.CacheableRequests++
			totalTTL += ttl
		} else {
			report.BypassRequests++
			if blocker != "" {
				report.TopBlockers[blocker]++
			}
		}

		report.Entries = append(report.Entries, analysis)
	}

	if report.TotalRequests > 0 {
		report.CacheabilityRatio = float64(report.CacheableRequests) / float64(report.TotalRequests) * 100.0
	}
	if report.CacheableRequests > 0 {
		report.AverageTTLSeconds = totalTTL / report.CacheableRequests
	}

	return report, nil
}

func checkHAREligibility(method string, status int, header http.Header, rd cache.ResponseDirectives) (bool, string) {
	if method != "GET" && method != "HEAD" {
		return false, "Non-GET/HEAD method (" + method + ")"
	}
	if status != 200 && status != 203 && status != 204 && status != 206 && status != 300 && status != 301 && status != 308 {
		return false, fmt.Sprintf("HTTP status %d not cacheable", status)
	}
	if rd.NoStore {
		return false, "Cache-Control: no-store"
	}
	if rd.Private {
		return false, "Cache-Control: private"
	}
	if header.Get("Set-Cookie") != "" && !rd.Public && rd.SMaxAge == nil {
		return false, "Origin Set-Cookie header"
	}
	if rd.MaxAge == nil && rd.SMaxAge == nil && header.Get("Expires") == "" {
		return false, "Missing explicit Cache-Control or Expires"
	}
	return true, ""
}

func getCDNHeader(h http.Header) string {
	if cf := h.Get("CF-Cache-Status"); cf != "" {
		return "Cloudflare: " + cf
	}
	if xc := h.Get("X-Cache"); xc != "" {
		return "Fastly/CloudFront: " + xc
	}
	if vc := h.Get("x-vercel-cache"); vc != "" {
		return "Vercel: " + vc
	}
	return ""
}
