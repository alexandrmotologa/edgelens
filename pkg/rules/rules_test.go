package rules

import (
	"net/http"
	"testing"
)

func TestStripMarketingQueryParams(t *testing.T) {
	engine := NewEngine()

	req, _ := http.NewRequest("GET", "https://example.com/products?id=42&utm_source=twitter&fbclid=xyz", nil)
	result := engine.ApplyRequestRules(req, req.URL.String())

	expected := "https://example.com/products?id=42"
	if result.ModifiedURL != expected {
		t.Errorf("Expected %q, got %q", expected, result.ModifiedURL)
	}

	if len(result.AppliedRuleIDs) == 0 {
		t.Errorf("Expected applied rule IDs, got none")
	}
}

func TestStripCookiesOnStaticAssets(t *testing.T) {
	engine := NewEngine()

	req, _ := http.NewRequest("GET", "https://example.com/assets/app.css", nil)
	req.Header.Set("Cookie", "session=123")

	result := engine.ApplyRequestRules(req, req.URL.String())
	if !result.StrippedCookies {
		t.Errorf("Expected StrippedCookies to be true")
	}
	if req.Header.Get("Cookie") != "" {
		t.Errorf("Expected Cookie header to be stripped, got %q", req.Header.Get("Cookie"))
	}
}

func TestOverrideTTLAndSurrogateKeys(t *testing.T) {
	engine := NewEngine()

	engine.AddRule(Rule{
		ID:          "rule-override-api",
		Name:        "Override API TTL",
		Type:        RuleOverrideTTL,
		Enabled:     true,
		PathPattern: `^/api/.*`,
		TTLSeconds:  7200,
		StatusCodes: []int{200},
	})

	engine.AddRule(Rule{
		ID:           "rule-add-tags",
		Name:         "Inject Catalog Tag",
		Type:         RuleAddSurrogateKeys,
		Enabled:      true,
		PathPattern:  `^/api/catalog`,
		SurrogateKey: "catalog v1",
	})

	header := http.Header{}
	res := engine.ApplyResponseRules("https://example.com/api/catalog", 200, header)

	if res.OverrideTTL == nil || *res.OverrideTTL != 7200 {
		t.Errorf("Expected OverrideTTL=7200, got %v", res.OverrideTTL)
	}

	surrogateKey := header.Get("Surrogate-Key")
	if surrogateKey != "catalog v1" {
		t.Errorf("Expected 'catalog v1', got %q", surrogateKey)
	}
}
