package har

import (
	"testing"
)

func TestParseCurl(t *testing.T) {
	cmd := `curl -X GET "https://api.example.com/v1/products?limit=10" -H "Accept: application/json" -H "Cache-Control: no-cache"`
	req, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("Failed to parse curl: %v", err)
	}

	if req.Method != "GET" {
		t.Errorf("Expected GET, got %s", req.Method)
	}
	if req.URL != "https://api.example.com/v1/products?limit=10" {
		t.Errorf("Expected URL, got %s", req.URL)
	}
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("Expected Accept header, got %s", req.Headers["Accept"])
	}
}

func TestParseHAR(t *testing.T) {
	harJSON := `{
		"log": {
			"entries": [
				{
					"request": {
						"method": "GET",
						"url": "https://example.com/styles.css",
						"headers": []
					},
					"response": {
						"status": 200,
						"content": {"size": 1200, "mimeType": "text/css"},
						"headers": [
							{"name": "Cache-Control", "value": "public, max-age=86400"},
							{"name": "ETag", "value": "\"abc\""}
						]
					}
				},
				{
					"request": {
						"method": "GET",
						"url": "https://example.com/api/user",
						"headers": []
					},
					"response": {
						"status": 200,
						"content": {"size": 400, "mimeType": "application/json"},
						"headers": [
							{"name": "Cache-Control", "value": "private, no-store"}
						]
					}
				}
			]
		}
	}`

	report, err := ParseHAR([]byte(harJSON))
	if err != nil {
		t.Fatalf("Failed to parse HAR: %v", err)
	}

	if report.TotalRequests != 2 {
		t.Errorf("Expected 2 requests, got %d", report.TotalRequests)
	}
	if report.CacheableRequests != 1 {
		t.Errorf("Expected 1 cacheable, got %d", report.CacheableRequests)
	}
	if report.BypassRequests != 1 {
		t.Errorf("Expected 1 bypass, got %d", report.BypassRequests)
	}
}
