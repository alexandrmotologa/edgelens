package cache

import (
	"net/http"
	"time"
)

// CalculateFreshnessLifetime determines the freshness lifetime of a response per RFC 9111 Section 4.2.1.
func CalculateFreshnessLifetime(rd ResponseDirectives, statusCode int, header http.Header, now time.Time) time.Duration {
	// 0. Surrogate-Control max-age takes absolute precedence on edge CDNs (Fastly/Surrogate architecture)
	if rd.SurrogateMaxAge != nil {
		return time.Duration(*rd.SurrogateMaxAge) * time.Second
	}

	// 1. s-maxage takes precedence on shared caches (edge CDN proxy)
	if rd.SMaxAge != nil {
		return time.Duration(*rd.SMaxAge) * time.Second
	}

	// 2. max-age
	if rd.MaxAge != nil {
		return time.Duration(*rd.MaxAge) * time.Second
	}

	// 3. Expires header
	if expiresStr := header.Get("Expires"); expiresStr != "" {
		if expiresTime, err := http.ParseTime(expiresStr); err == nil {
			dateStr := header.Get("Date")
			dateTime := now
			if dateStr != "" {
				if dt, err := http.ParseTime(dateStr); err == nil {
					dateTime = dt
				}
			}
			if expiresTime.After(dateTime) {
				return expiresTime.Sub(dateTime)
			}
			return 0
		}
	}

	// 4. Heuristic Freshness (RFC 9111 Section 4.2.2)
	if isHeuristicallyCacheableStatus(statusCode) {
		if lastModStr := header.Get("Last-Modified"); lastModStr != "" {
			if lastModTime, err := http.ParseTime(lastModStr); err == nil {
				dateStr := header.Get("Date")
				dateTime := now
				if dateStr != "" {
					if dt, err := http.ParseTime(dateStr); err == nil {
						dateTime = dt
					}
				}
				if dateTime.After(lastModTime) {
					// 10% of the time since last modification
					heuristic := dateTime.Sub(lastModTime) / 10
					// Cap at 24 hours per standard CDN heuristics
					if heuristic > 24*time.Hour {
						heuristic = 24 * time.Hour
					}
					return heuristic
				}
			}
		}
	}

	return 0
}

func isHeuristicallyCacheableStatus(code int) bool {
	switch code {
	case 200, 203, 204, 206, 300, 301, 308, 404, 405, 410, 414, 501:
		return true
	default:
		return false
	}
}

// CalculateCurrentAge calculates the current age per RFC 9111 Section 4.2.3.
func CalculateCurrentAge(entry *Entry, now time.Time) time.Duration {
	// Parse Date header or fallback to responseTime
	dateVal := entry.ResponseTime
	if dateHeader := entry.Headers.Get("Date"); dateHeader != "" {
		if dt, err := http.ParseTime(dateHeader); err == nil {
			dateVal = dt
		}
	}

	// apparent_age = max(0, response_time - date_value)
	var apparentAge time.Duration
	if entry.ResponseTime.After(dateVal) {
		apparentAge = entry.ResponseTime.Sub(dateVal)
	}

	// response_delay = response_time - request_time
	var responseDelay time.Duration
	if entry.ResponseTime.After(entry.RequestTime) {
		responseDelay = entry.ResponseTime.Sub(entry.RequestTime)
	}

	// age_value from origin Age header if present
	var ageVal time.Duration
	if ageHeader := entry.Headers.Get("Age"); ageHeader != "" {
		if sec, err := time.ParseDuration(ageHeader + "s"); err == nil {
			ageVal = sec
		}
	}

	// corrected_age_value = age_value + response_delay
	correctedAgeVal := ageVal + responseDelay

	// corrected_initial_age = max(apparent_age, corrected_age_value)
	correctedInitialAge := apparentAge
	if correctedAgeVal > correctedInitialAge {
		correctedInitialAge = correctedAgeVal
	}

	// resident_time = now - response_time
	var residentTime time.Duration
	if now.After(entry.ResponseTime) {
		residentTime = now.Sub(entry.ResponseTime)
	}

	return correctedInitialAge + residentTime
}

// FreshnessEvaluation captures detailed freshness state for an entry.
type FreshnessEvaluation struct {
	IsFresh            bool
	CurrentAge         time.Duration
	FreshnessLifetime  time.Duration
	RemainingFreshness time.Duration
	IsInSWR            bool
	IsInSIE            bool
	IsSoftPurged       bool
	RequiresReval      bool
	Reason             string
}

// EvaluateFreshness computes whether the cached response can be served fresh, stale, or needs revalidation.
func EvaluateFreshness(entry *Entry, reqDir RequestDirectives, respDir ResponseDirectives, now time.Time) FreshnessEvaluation {
	currentAge := CalculateCurrentAge(entry, now)
	lifetime := entry.FreshnessLifetime

	eval := FreshnessEvaluation{
		CurrentAge:        currentAge,
		FreshnessLifetime: lifetime,
		IsSoftPurged:      entry.IsSoftPurged,
	}

	if lifetime > currentAge {
		eval.RemainingFreshness = lifetime - currentAge
	} else {
		eval.RemainingFreshness = 0
	}

	// Check soft purge first
	if entry.IsSoftPurged {
		eval.IsFresh = false
		eval.RequiresReval = true
		eval.Reason = "Entry was soft-purged via surrogate key or tag"
		// Soft-purged entries can be served as STALE during SWR if SWR is available
		if entry.SWRDuration > 0 {
			eval.IsInSWR = true
		}
		return eval
	}

	// Origin no-cache or must-revalidate
	if respDir.NoCache {
		eval.IsFresh = false
		eval.RequiresReval = true
		eval.Reason = "Origin response contains Cache-Control: no-cache"
		return eval
	}

	// Client request directives
	if reqDir.NoCache {
		eval.IsFresh = false
		eval.RequiresReval = true
		eval.Reason = "Client request contains Cache-Control: no-cache"
		return eval
	}

	// Client max-age constraint
	if reqDir.MaxAge != nil {
		clientMaxAge := time.Duration(*reqDir.MaxAge) * time.Second
		if currentAge > clientMaxAge {
			eval.IsFresh = false
			eval.RequiresReval = true
			eval.Reason = "Entry exceeds client Cache-Control: max-age constraint"
			return eval
		}
	}

	// Client min-fresh constraint
	if reqDir.MinFresh != nil {
		clientMinFresh := time.Duration(*reqDir.MinFresh) * time.Second
		if eval.RemainingFreshness < clientMinFresh {
			eval.IsFresh = false
			eval.RequiresReval = true
			eval.Reason = "Entry does not satisfy client Cache-Control: min-fresh constraint"
			return eval
		}
	}

	// Natural freshness check
	if currentAge < lifetime {
		eval.IsFresh = true
		eval.RequiresReval = false
		eval.Reason = "Entry is within freshness lifetime"
		return eval
	}

	// Stale while revalidate window check (RFC 5861 / RFC 9111)
	if entry.SWRDuration > 0 {
		swrEnd := lifetime + entry.SWRDuration
		if currentAge <= swrEnd {
			eval.IsFresh = false
			eval.IsInSWR = true
			eval.RequiresReval = true
			eval.Reason = "Entry is stale but inside stale-while-revalidate window"
			return eval
		}
	}

	// Client max-stale check
	if reqDir.MaxStale != nil {
		clientMaxStale := time.Duration(*reqDir.MaxStale) * time.Second
		staleDuration := currentAge - lifetime
		if staleDuration <= clientMaxStale {
			eval.IsFresh = false
			eval.RequiresReval = false
			eval.Reason = "Entry is stale but accepted by client Cache-Control: max-stale"
			return eval
		}
	}

	// Stale if error window
	if entry.SIEDuration > 0 {
		sieEnd := lifetime + entry.SIEDuration
		if currentAge <= sieEnd {
			eval.IsInSIE = true
		}
	}

	eval.IsFresh = false
	eval.RequiresReval = true
	eval.Reason = "Entry freshness lifetime has expired"
	return eval
}
