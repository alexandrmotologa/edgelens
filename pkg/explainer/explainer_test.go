package explainer_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
	"github.com/alexandrmotologa/edgelens/pkg/explainer"
)

func TestBuildExplanation(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/items", nil)
	reqDir := cache.RequestDirectives{}

	respHeaders := make(http.Header)
	respHeaders.Set("Cache-Control", "public, max-age=3600")
	respHeaders.Set("ETag", `"v1.0"`)
	maxAge := 3600
	respDir := cache.ResponseDirectives{Public: true, MaxAge: &maxAge}

	exp := explainer.BuildExplanation(
		"tx_test_1",
		req,
		reqDir,
		respHeaders,
		respDir,
		"GET://api.example.com/items",
		"",
		nil,
		nil,
		cache.StatusMiss,
		45,
		44,
		cdn.FlavorCloudflare,
		time.Now(),
	)

	if exp.ID != "tx_test_1" {
		t.Errorf("expected ID tx_test_1, got %s", exp.ID)
	}
	if len(exp.Phases) != 5 {
		t.Errorf("expected 5 decision phases, got %d", len(exp.Phases))
	}
	if exp.FinalStatus != cache.StatusMiss {
		t.Errorf("expected StatusMiss, got %s", exp.FinalStatus)
	}
}

func TestAuditHeaders(t *testing.T) {
	// Case 1: Leaky cookie critical vulnerability
	badHeaders := map[string][]string{
		"Cache-Control": {"public, max-age=3600"},
		"Set-Cookie":    {"session_id=12345"},
	}
	report1 := explainer.AuditHeaders(badHeaders)
	if report1.Grade != "D" && report1.Grade != "F" {
		t.Errorf("expected low grade for Set-Cookie with public caching, got %s (score %d)", report1.Grade, report1.Score)
	}

	// Case 2: Ideal caching
	goodHeaders := map[string][]string{
		"Cache-Control": {"public, max-age=86400, stale-while-revalidate=3600, immutable"},
		"ETag":          {`"asset-hash-99"`},
		"Surrogate-Key": {"bundle-app static-assets"},
	}
	report2 := explainer.AuditHeaders(goodHeaders)
	if report2.Grade != "A+" && report2.Grade != "A" {
		t.Errorf("expected grade A or A+ for well-tuned headers, got %s (score %d)", report2.Grade, report2.Score)
	}
}
