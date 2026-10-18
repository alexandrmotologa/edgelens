package assert

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunAssertionSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("ETag", `"v123"`)
		w.Header().Set("CF-Cache-Status", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	opts := AssertionOptions{
		URL:               ts.URL,
		ExpectStatus:      "HIT",
		MinTTLSeconds:     600,
		MinGrade:          "A",
		DisallowedHeaders: []string{"Set-Cookie"},
		RequiredHeaders:   []string{"ETag"},
	}

	result, err := RunAssertion(opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Passed {
		t.Fatalf("Expected assertion to pass, but failed: %s", result.Summary)
	}

	if len(result.Checks) != 5 {
		t.Errorf("Expected 5 checks, got %d", len(result.Checks))
	}
}

func TestRunAssertionFailureOnDisallowedHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("Set-Cookie", "session=evil; Path=/")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	opts := AssertionOptions{
		URL:               ts.URL,
		MinTTLSeconds:     600,
		DisallowedHeaders: []string{"Set-Cookie"},
	}

	result, err := RunAssertion(opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Passed {
		t.Errorf("Expected assertion to fail due to Set-Cookie, but passed")
	}
}
