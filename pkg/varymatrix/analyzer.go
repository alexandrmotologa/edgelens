package varymatrix

import (
	"fmt"
	"strings"
)

// RiskLevel classifies the danger of cache fragmentation.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// VaryDimension details one header used in cache variation.
type VaryDimension struct {
	HeaderName         string    `json:"headerName"`
	TypicalCardinality int       `json:"typicalCardinality"`
	Risk               RiskLevel `json:"risk"`
	Description        string    `json:"description"`
	EdgeRemediation    string    `json:"edgeRemediation"`
	SampleValues       []string  `json:"sampleValues"`
}

// SampleVariant shows a concrete slice in the cache.
type SampleVariant struct {
	Headers          map[string]string `json:"headers"`
	CacheKeySuffix   string            `json:"cacheKeySuffix"`
	CollapsesWithFix bool              `json:"collapsesWithFix"`
}

// AnalysisReport contains the complete fragmentation audit.
type AnalysisReport struct {
	RawVaryHeader              string          `json:"rawVaryHeader"`
	Dimensions                 []VaryDimension `json:"dimensions"`
	EstimatedPermutations      int64           `json:"estimatedPermutations"`
	OriginPenetrationRisk      RiskLevel       `json:"originPenetrationRisk"`
	EstimatedMemoryMultiplier  float64         `json:"estimatedMemoryMultiplier"`
	NormalizedVaryProposal     string          `json:"normalizedVaryProposal"`
	Recommendations            []string        `json:"recommendations"`
	SampleVariants             []SampleVariant `json:"sampleVariants"`
}

// AnalyzeVaryHeader parses a Vary header value and produces an exhaustive fragmentation audit.
func AnalyzeVaryHeader(rawVary string) AnalysisReport {
	report := AnalysisReport{
		RawVaryHeader: rawVary,
		Dimensions:    make([]VaryDimension, 0),
	}

	parts := strings.Split(rawVary, ",")
	var cleanedHeaders []string
	var permutations int64 = 1

	for _, p := range parts {
		h := strings.TrimSpace(p)
		if h == "" {
			continue
		}
		cleanedHeaders = append(cleanedHeaders, h)
		dim := evaluateDimension(h)
		report.Dimensions = append(report.Dimensions, dim)
		permutations *= int64(dim.TypicalCardinality)
	}

	if len(report.Dimensions) == 0 {
		report.EstimatedPermutations = 1
		report.OriginPenetrationRisk = RiskLow
		report.EstimatedMemoryMultiplier = 1.0
		report.NormalizedVaryProposal = "None"
		report.Recommendations = []string{"No Vary header present. Responses are cached uniformly across all client variations."}
		return report
	}

	report.EstimatedPermutations = permutations
	report.EstimatedMemoryMultiplier = float64(permutations)

	// Determine overall risk
	hasCritical := false
	hasHigh := false
	for _, d := range report.Dimensions {
		if d.Risk == RiskCritical {
			hasCritical = true
		} else if d.Risk == RiskHigh {
			hasHigh = true
		}
	}

	if hasCritical || permutations > 500 {
		report.OriginPenetrationRisk = RiskCritical
	} else if hasHigh || permutations > 50 {
		report.OriginPenetrationRisk = RiskHigh
	} else if permutations > 4 {
		report.OriginPenetrationRisk = RiskMedium
	} else {
		report.OriginPenetrationRisk = RiskLow
	}

	// Generate proposal and recommendations
	report.Recommendations, report.NormalizedVaryProposal = generateRecommendations(report.Dimensions)

	// Generate sample variant slices
	report.SampleVariants = generateSampleVariants(report.Dimensions)

	return report
}

func evaluateDimension(header string) VaryDimension {
	switch strings.ToLower(header) {
	case "accept-encoding":
		return VaryDimension{
			HeaderName:         "Accept-Encoding",
			TypicalCardinality: 4,
			Risk:               RiskLow,
			Description:        "Compression encoding requested by browser (gzip, br, zstd, identity).",
			EdgeRemediation:    "Normalize at the CDN edge: convert all variations into either 'br', 'gzip', or none.",
			SampleValues:       []string{"gzip, deflate, br", "gzip, deflate", "br", "identity"},
		}
	case "user-agent":
		return VaryDimension{
			HeaderName:         "User-Agent",
			TypicalCardinality: 5000,
			Risk:               RiskCritical,
			Description:        "Full browser user-agent string. Completely fragments cache into thousands of micro-slices.",
			EdgeRemediation:    "REMOVE from Vary. Use Client Hints (Sec-CH-UA-Mobile) or Edge Device Detection to bucket into Desktop vs Mobile.",
			SampleValues:       []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130...", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X)...", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)..."},
		}
	case "accept-language":
		return VaryDimension{
			HeaderName:         "Accept-Language",
			TypicalCardinality: 40,
			Risk:               RiskHigh,
			Description:        "Client language and regional weights (q-factors). Creates dozens of redundant combinations.",
			EdgeRemediation:    "Bucket on CDN edge to primary ISO language code ('en', 'ro', 'fr') or redirect to localized subpaths (/en/, /ro/).",
			SampleValues:       []string{"en-US,en;q=0.9", "ro-RO,ro;q=0.8,en;q=0.6", "fr-FR,fr;q=0.9", "de-DE,de;q=0.9"},
		}
	case "cookie":
		return VaryDimension{
			HeaderName:         "Cookie",
			TypicalCardinality: 10000,
			Risk:               RiskCritical,
			Description:        "HTTP cookie string. Effectively disables edge caching by creating a unique variant per logged-in session.",
			EdgeRemediation:    "Never include full Cookie in Vary. Split static/anonymous content from personalized API endpoints or use Cookie whitelisting.",
			SampleValues:       []string{"session_id=abc; preferences=dark", "session_id=def; preferences=light", "analytics_id=987"},
		}
	case "origin":
		return VaryDimension{
			HeaderName:         "Origin",
			TypicalCardinality: 3,
			Risk:               RiskLow,
			Description:        "CORS request origin. Necessary when serving cross-origin authenticated assets.",
			EdgeRemediation:    "Safe to vary if CORS headers depend on Origin; otherwise consider wildcard 'Access-Control-Allow-Origin: *'.",
			SampleValues:       []string{"https://app.example.com", "https://staging.example.com", "null"},
		}
	default:
		return VaryDimension{
			HeaderName:         header,
			TypicalCardinality: 10,
			Risk:               RiskMedium,
			Description:        fmt.Sprintf("Custom header variation (%s).", header),
			EdgeRemediation:    fmt.Sprintf("Verify that %s contains a bounded set of discrete values before including in edge cache key.", header),
			SampleValues:       []string{"value-1", "value-2", "value-3"},
		}
	}
}

func generateRecommendations(dims []VaryDimension) ([]string, string) {
	var recs []string
	var keptHeaders []string

	for _, d := range dims {
		switch strings.ToLower(d.HeaderName) {
		case "user-agent":
			recs = append(recs, "CRITICAL: Remove 'User-Agent' from Vary header immediately. It destroys cache efficiency and causes 98% cache misses.")
		case "cookie":
			recs = append(recs, "CRITICAL: Remove 'Cookie' from Vary. Use a dedicated public endpoint for shared content and separate private requests.")
		case "accept-language":
			recs = append(recs, "HIGH: Normalize Accept-Language on edge CDN to top supported languages (e.g. en, ro) instead of passing raw locale string.")
			keptHeaders = append(keptHeaders, "Accept-Language (Normalized)")
		case "accept-encoding":
			recs = append(recs, "BEST PRACTICE: Keep 'Vary: Accept-Encoding' and let the edge CDN compress and normalize encoding automatically.")
			keptHeaders = append(keptHeaders, "Accept-Encoding")
		case "origin":
			keptHeaders = append(keptHeaders, "Origin")
		default:
			keptHeaders = append(keptHeaders, d.HeaderName)
		}
	}

	proposal := "None"
	if len(keptHeaders) > 0 {
		proposal = strings.Join(keptHeaders, ", ")
	}

	return recs, proposal
}

func generateSampleVariants(dims []VaryDimension) []SampleVariant {
	var variants []SampleVariant

	if len(dims) == 0 {
		return variants
	}

	// Produce 3-4 realistic combinations
	v1 := SampleVariant{
		Headers:          make(map[string]string),
		CacheKeySuffix:   "#variant_1",
		CollapsesWithFix: false,
	}
	v2 := SampleVariant{
		Headers:          make(map[string]string),
		CacheKeySuffix:   "#variant_2",
		CollapsesWithFix: true,
	}
	v3 := SampleVariant{
		Headers:          make(map[string]string),
		CacheKeySuffix:   "#variant_3",
		CollapsesWithFix: true,
	}

	for _, d := range dims {
		if len(d.SampleValues) > 0 {
			v1.Headers[d.HeaderName] = d.SampleValues[0]
		}
		if len(d.SampleValues) > 1 {
			v2.Headers[d.HeaderName] = d.SampleValues[1]
		} else if len(d.SampleValues) > 0 {
			v2.Headers[d.HeaderName] = d.SampleValues[0]
		}
		if len(d.SampleValues) > 2 {
			v3.Headers[d.HeaderName] = d.SampleValues[2]
		} else if len(d.SampleValues) > 0 {
			v3.Headers[d.HeaderName] = d.SampleValues[0]
		}
	}

	variants = append(variants, v1, v2, v3)
	return variants
}
