package rules

import (
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RuleType defines the kind of edge transform rule.
type RuleType string

const (
	RuleStripQueryParams RuleType = "strip_query_params"
	RuleStripCookies     RuleType = "strip_cookies"
	RuleOverrideTTL      RuleType = "override_ttl"
	RuleAddSurrogateKeys RuleType = "add_surrogate_keys"
	RuleBypassCache      RuleType = "bypass_cache"
)

// Rule represents a single configurable edge transformation.
type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        RuleType `json:"type"`
	Enabled     bool     `json:"enabled"`
	PathPattern string   `json:"pathPattern"` // regex or glob
	// Type-specific parameters
	StripParams  []string `json:"stripParams,omitempty"`  // for strip_query_params (e.g. utm_*, fbclid)
	StripAll     bool     `json:"stripAll,omitempty"`     // strip all query params
	TTLSeconds   int      `json:"ttlSeconds,omitempty"`   // for override_ttl
	SurrogateKey string   `json:"surrogateKey,omitempty"` // for add_surrogate_keys
	StatusCodes  []int    `json:"statusCodes,omitempty"`  // status codes for override_ttl
}

// Engine manages and executes edge transformation rules.
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewEngine initializes an edge rules engine with pre-configured recommended rules.
func NewEngine() *Engine {
	e := &Engine{
		rules: []Rule{
			{
				ID:          "rule-marketing-params",
				Name:        "Strip Marketing Query Parameters",
				Type:        RuleStripQueryParams,
				Enabled:     true,
				PathPattern: ".*",
				StripParams: []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "fbclid", "gclid", "_ga"},
			},
			{
				ID:          "rule-static-cookies",
				Name:        "Strip Cookies on Static Assets",
				Type:        RuleStripCookies,
				Enabled:     true,
				PathPattern: `\.(?:css|js|png|jpg|jpeg|gif|webp|svg|woff2?|ico)$`,
			},
		},
	}
	return e
}

// GetRules returns a copy of current active rules.
func (e *Engine) GetRules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// SetRules replaces the ruleset.
func (e *Engine) SetRules(rules []Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// AddRule appends a new rule.
func (e *Engine) AddRule(rule Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if rule.ID == "" {
		rule.ID = "rule-" + time.Now().Format("20060102150405")
	}
	e.rules = append(e.rules, rule)
}

// ToggleRule enables or disables a rule by ID.
func (e *Engine) ToggleRule(id string, enabled bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.rules {
		if e.rules[i].ID == id {
			e.rules[i].Enabled = enabled
			return true
		}
	}
	return false
}

// DeleteRule removes a rule by ID.
func (e *Engine) DeleteRule(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.rules {
		if e.rules[i].ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return true
		}
	}
	return false
}

// TransformResult contains the outcome of applying edge rules to a request or response.
type TransformResult struct {
	ModifiedURL     string
	BypassCache     bool
	BypassReason    string
	StrippedCookies bool
	InjectedTags    []string
	OverrideTTL     *int
	AppliedRuleIDs  []string
}

// ApplyRequestRules runs enabled rules over an incoming request and target URL.
func (e *Engine) ApplyRequestRules(r *http.Request, rawURL string) TransformResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	res := TransformResult{
		ModifiedURL: rawURL,
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return res
	}

	path := parsed.Path

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		if !matchPath(rule.PathPattern, path) {
			continue
		}

		switch rule.Type {
		case RuleStripQueryParams:
			if rule.StripAll {
				parsed.RawQuery = ""
				res.ModifiedURL = parsed.String()
				res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
			} else if len(rule.StripParams) > 0 {
				q := parsed.Query()
				modified := false
				for _, param := range rule.StripParams {
					param = strings.TrimSpace(param)
					if strings.HasSuffix(param, "*") {
						prefix := strings.TrimSuffix(param, "*")
						for k := range q {
							if strings.HasPrefix(strings.ToLower(k), strings.ToLower(prefix)) {
								q.Del(k)
								modified = true
							}
						}
					} else {
						if _, exists := q[param]; exists {
							q.Del(param)
							modified = true
						}
					}
				}
				if modified {
					parsed.RawQuery = q.Encode()
					res.ModifiedURL = parsed.String()
					res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
				}
			}

		case RuleStripCookies:
			if r.Header.Get("Cookie") != "" {
				r.Header.Del("Cookie")
				res.StrippedCookies = true
				res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
			}

		case RuleBypassCache:
			res.BypassCache = true
			res.BypassReason = "Edge rule bypass: " + rule.Name
			res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
		}
	}

	return res
}

// ApplyResponseRules processes origin response headers and injects tags or overrides TTL.
func (e *Engine) ApplyResponseRules(rawURL string, statusCode int, header http.Header) TransformResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	res := TransformResult{}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return res
	}
	path := parsed.Path

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		if !matchPath(rule.PathPattern, path) {
			continue
		}

		switch rule.Type {
		case RuleStripCookies:
			if header.Get("Set-Cookie") != "" {
				header.Del("Set-Cookie")
				res.StrippedCookies = true
				res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
			}

		case RuleOverrideTTL:
			statusMatch := len(rule.StatusCodes) == 0
			for _, sc := range rule.StatusCodes {
				if sc == statusCode {
					statusMatch = true
					break
				}
			}
			if statusMatch && rule.TTLSeconds > 0 {
				ttl := rule.TTLSeconds
				res.OverrideTTL = &ttl
				res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
			}

		case RuleAddSurrogateKeys:
			if rule.SurrogateKey != "" {
				existing := header.Get("Surrogate-Key")
				if existing == "" {
					header.Set("Surrogate-Key", rule.SurrogateKey)
				} else {
					header.Set("Surrogate-Key", existing+" "+rule.SurrogateKey)
				}
				keys := strings.Fields(rule.SurrogateKey)
				res.InjectedTags = append(res.InjectedTags, keys...)
				res.AppliedRuleIDs = append(res.AppliedRuleIDs, rule.ID)
			}
		}
	}

	return res
}

func matchPath(pattern, path string) bool {
	if pattern == "" || pattern == "*" || pattern == ".*" {
		return true
	}

	// Try regex match first
	if rx, err := regexp.Compile(pattern); err == nil {
		if rx.MatchString(path) {
			return true
		}
	}

	// Try glob match
	if matched, err := filepath.Match(pattern, path); err == nil && matched {
		return true
	}

	return false
}
