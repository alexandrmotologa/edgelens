package comparator

import (
	"net/http"
	"testing"
)

func TestCompareRequestResponse(t *testing.T) {
	reqH := http.Header{}
	respH := http.Header{}
	respH.Set("Cache-Control", "public, s-maxage=600, max-age=300")
	respH.Set("Surrogate-Control", "max-age=1800")

	report := CompareRequestResponse("GET", "https://api.example.com/v1/catalog", reqH, 200, respH, 50)

	if len(report.Flavors) != 5 {
		t.Fatalf("Expected 5 flavors, got %d", len(report.Flavors))
	}

	cf := report.Flavors["cloudflare"]
	if cf.EffectiveTTLSeconds != 600 {
		t.Errorf("Expected Cloudflare TTL=600 (s-maxage), got %d", cf.EffectiveTTLSeconds)
	}

	fastly := report.Flavors["fastly"]
	if fastly.EffectiveTTLSeconds != 1800 {
		t.Errorf("Expected Fastly TTL=1800 (Surrogate-Control), got %d", fastly.EffectiveTTLSeconds)
	}

	if len(report.Differences) == 0 {
		t.Errorf("Expected differences between Cloudflare and Fastly TTL, found none")
	}
}
