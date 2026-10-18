package cache

import (
	"strconv"
	"strings"
)

// ParseResponseDirectives parses origin Cache-Control and Surrogate-Control headers.
func ParseResponseDirectives(cacheControlHeaders []string, surrogateControlHeaders []string) ResponseDirectives {
	var rd ResponseDirectives

	// Parse Surrogate-Control first if present (takes precedence on CDNs like Fastly/Akamai)
	parseHeaderList(surrogateControlHeaders, func(key, val string) {
		if key == "max-age" {
			if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
				rd.SurrogateMaxAge = &secs
				if rd.MaxAge == nil {
					rd.MaxAge = &secs
				}
			}
		} else {
			applyResponseDirective(&rd, key, val)
		}
	})

	// Parse Cache-Control headers
	parseHeaderList(cacheControlHeaders, func(key, val string) {
		applyResponseDirective(&rd, key, val)
	})

	return rd
}

// ParseRequestDirectives parses client-side Cache-Control header directives.
func ParseRequestDirectives(headers []string) RequestDirectives {
	var reqDir RequestDirectives

	parseHeaderList(headers, func(key, val string) {
		switch key {
		case "no-cache":
			reqDir.NoCache = true
		case "no-store":
			reqDir.NoStore = true
		case "no-transform":
			reqDir.NoTransform = true
		case "only-if-cached":
			reqDir.OnlyIfCached = true
		case "max-age":
			if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
				reqDir.MaxAge = &secs
			}
		case "max-stale":
			if val == "" {
				// unspecified max-stale means client accepts any stale duration
				inf := 1<<31 - 1
				reqDir.MaxStale = &inf
			} else if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
				reqDir.MaxStale = &secs
			}
		case "min-fresh":
			if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
				reqDir.MinFresh = &secs
			}
		}
	})

	return reqDir
}

func parseHeaderList(headers []string, handler func(key, val string)) {
	for _, h := range headers {
		parts := strings.Split(h, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			var key, val string
			if idx := strings.IndexByte(part, '='); idx != -1 {
				key = strings.ToLower(strings.TrimSpace(part[:idx]))
				val = strings.Trim(strings.TrimSpace(part[idx+1:]), "\"")
			} else {
				key = strings.ToLower(part)
			}

			handler(key, val)
		}
	}
}

func applyResponseDirective(rd *ResponseDirectives, key, val string) {
	switch key {
	case "public":
		rd.Public = true
	case "private":
		rd.Private = true
	case "no-cache":
		rd.NoCache = true
	case "no-store":
		rd.NoStore = true
	case "no-transform":
		rd.NoTransform = true
	case "must-revalidate":
		rd.MustRevalidate = true
	case "proxy-revalidate":
		rd.ProxyRevalidate = true
	case "must-understand":
		rd.MustUnderstand = true
	case "immutable":
		rd.Immutable = true
	case "max-age":
		if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
			if rd.MaxAge == nil {
				rd.MaxAge = &secs
			}
		}
	case "s-maxage", "s-max-age":
		if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
			if rd.SMaxAge == nil {
				rd.SMaxAge = &secs
			}
		}
	case "stale-while-revalidate":
		if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
			if rd.StaleWhileRevalidate == nil {
				rd.StaleWhileRevalidate = &secs
			}
		}
	case "stale-if-error":
		if secs, err := strconv.Atoi(val); err == nil && secs >= 0 {
			if rd.StaleIfError == nil {
				rd.StaleIfError = &secs
			}
		}
	}
}
