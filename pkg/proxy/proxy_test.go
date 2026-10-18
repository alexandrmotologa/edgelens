package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
	"github.com/alexandrmotologa/edgelens/pkg/mockorigin"
	"github.com/alexandrmotologa/edgelens/pkg/proxy"
	"github.com/alexandrmotologa/edgelens/pkg/surrogate"
)

func TestProxyEndToEnd(t *testing.T) {
	// 1. Setup mock origin
	mockOrigin := mockorigin.NewServer()
	originServer := httptest.NewServer(mockOrigin)
	defer originServer.Close()

	// 2. Setup EdgeLens proxy
	store := cache.NewStore()
	idx := surrogate.NewIndex()
	pe := surrogate.NewPurgeEngine(store, idx)

	p := proxy.NewProxy(proxy.Config{
		DefaultOrigin: originServer.URL,
		CDNConfig:     cdn.DefaultConfig(),
	}, store, idx, pe)

	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// Request 1 to SWR scenario: Expect MISS
	req1, _ := http.NewRequest(http.MethodGet, proxyServer.URL+"/mock/scenarios/stale-while-revalidate", nil)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	defer resp1.Body.Close()
	body1, _ := io.ReadAll(resp1.Body)

	if resp1.Header.Get("CF-Cache-Status") != "MISS" {
		t.Fatalf("expected CF-Cache-Status MISS on request 1, got %s", resp1.Header.Get("CF-Cache-Status"))
	}
	if len(body1) == 0 {
		t.Fatalf("expected non-empty body on request 1")
	}

	// Request 2 immediately: Expect HIT
	req2, _ := http.NewRequest(http.MethodGet, proxyServer.URL+"/mock/scenarios/stale-while-revalidate", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.Header.Get("CF-Cache-Status") != "HIT" {
		t.Fatalf("expected CF-Cache-Status HIT on request 2, got %s", resp2.Header.Get("CF-Cache-Status"))
	}

	// Request 3 to E-Commerce product with surrogate keys
	req3, _ := http.NewRequest(http.MethodGet, proxyServer.URL+"/mock/scenarios/product/42", nil)
	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("request 3 failed: %v", err)
	}
	defer resp3.Body.Close()

	// Verify surrogate key index registered product:42
	keys := idx.GetKeysForTag("product:42")
	if len(keys) == 0 {
		t.Fatalf("expected product:42 to be indexed in surrogate index")
	}

	// Purge by tag "promo:blackfriday"
	purgeRes := pe.PurgeTag("promo:blackfriday", false)
	if purgeRes.AffectedCount != 1 {
		t.Fatalf("expected 1 entry purged by promo:blackfriday tag, got %d", purgeRes.AffectedCount)
	}

	// Request 4 to product 42 after purge: Expect MISS again!
	req4, _ := http.NewRequest(http.MethodGet, proxyServer.URL+"/mock/scenarios/product/42", nil)
	resp4, err := client.Do(req4)
	if err != nil {
		t.Fatalf("request 4 failed: %v", err)
	}
	defer resp4.Body.Close()

	if resp4.Header.Get("CF-Cache-Status") != "MISS" {
		t.Fatalf("expected CF-Cache-Status MISS on request 4 after purge, got %s", resp4.Header.Get("CF-Cache-Status"))
	}
}
