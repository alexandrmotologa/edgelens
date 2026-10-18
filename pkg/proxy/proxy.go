package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
	"github.com/alexandrmotologa/edgelens/pkg/explainer"
	"github.com/alexandrmotologa/edgelens/pkg/stampede"
	"github.com/alexandrmotologa/edgelens/pkg/surrogate"
)

// TransactionListener is a callback invoked on every completed transaction for telemetry/SSE.
type TransactionListener func(exp *explainer.Explanation)

// Proxy handles HTTP caching, edge emulation, revalidation, and single-flight coalescing.
type Proxy struct {
	store          *cache.Store
	index          *surrogate.Index
	purgeEngine    *surrogate.PurgeEngine
	cdnModifier    *cdn.HeaderModifier
	coalescer      *stampede.Coalescer
	httpClient     *http.Client
	defaultOrigin  string
	mu             sync.RWMutex
	listeners      []TransactionListener
	recentExps     []*explainer.Explanation
	maxRecentCount int
}

// Config defines initialization parameters for the edge proxy.
type Config struct {
	DefaultOrigin string
	CDNConfig     cdn.Config
}

// NewProxy creates an EdgeLens proxy engine.
func NewProxy(cfg Config, store *cache.Store, index *surrogate.Index, purgeEngine *surrogate.PurgeEngine) *Proxy {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Proxy{
		store:          store,
		index:          index,
		purgeEngine:    purgeEngine,
		cdnModifier:    cdn.NewHeaderModifier(cfg.CDNConfig),
		coalescer:      stampede.NewCoalescer(),
		httpClient:     &http.Client{Transport: transport, Timeout: 30 * time.Second},
		defaultOrigin:  cfg.DefaultOrigin,
		recentExps:     make([]*explainer.Explanation, 0, 100),
		maxRecentCount: 100,
	}
}

// AddListener registers an event consumer for real-time telemetry streaming (SSE).
func (p *Proxy) AddListener(listener TransactionListener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listeners = append(p.listeners, listener)
}

// GetRecentExplanations returns a list of recent transactions.
func (p *Proxy) GetRecentExplanations() []*explainer.Explanation {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make([]*explainer.Explanation, len(p.recentExps))
	copy(out, p.recentExps)
	return out
}

// GetExplanationByID retrieves a specific transaction explanation by ID.
func (p *Proxy) GetExplanationByID(id string) (*explainer.Explanation, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, e := range p.recentExps {
		if e.ID == id {
			return e, true
		}
	}
	return nil, false
}

// SetCDNFlavor switches active CDN emulation profile.
func (p *Proxy) SetCDNFlavor(f cdn.Flavor) {
	p.cdnModifier.SetFlavor(f)
}

// GetCDNConfig returns current CDN profile.
func (p *Proxy) GetCDNConfig() cdn.Config {
	return p.cdnModifier.GetConfig()
}

// ServeHTTP handles incoming client requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	txID := fmt.Sprintf("tx_%d_%s", startTime.UnixNano(), randomHex(4))

	// Resolve target URL
	targetURL := p.resolveTargetURL(r)
	if targetURL == "" {
		http.Error(w, "EdgeLens: Missing or invalid target URL. Use /proxy?url=<target> or /proxy/<url>", http.StatusBadRequest)
		return
	}

	// Parse client request directives
	reqDir := cache.ParseRequestDirectives(r.Header.Values("Cache-Control"))
	primaryKey := cache.GeneratePrimaryKey(r.Method, targetURL)

	// Check method cacheability
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		p.handleBypass(w, r, targetURL, primaryKey, reqDir, txID, startTime, "Non-idempotent HTTP method")
		return
	}

	// Check client no-store
	if reqDir.NoStore {
		p.handleBypass(w, r, targetURL, primaryKey, reqDir, txID, startTime, "Client Cache-Control: no-store")
		return
	}

	// Lookup in cache store matching Vary matrix
	cachedEntry, found := p.store.LookupVariant(primaryKey, r.Header)
	now := time.Now()

	if found {
		respDir := cache.ParseResponseDirectives(
			cachedEntry.Headers.Values("Cache-Control"),
			cachedEntry.Headers.Values("Surrogate-Control"),
		)
		eval := cache.EvaluateFreshness(cachedEntry, reqDir, respDir, now)

		if eval.IsFresh {
			// Cache HIT
			p.serveHit(w, r, cachedEntry, reqDir, respDir, primaryKey, txID, startTime, eval.CurrentAge, cache.StatusHit)
			return
		}

		if eval.IsInSWR {
			// STALE-WHILE-REVALIDATE HIT: Serve stale response immediately, revalidate in background
			p.serveHit(w, r, cachedEntry, reqDir, respDir, primaryKey, txID, startTime, eval.CurrentAge, cache.StatusStale)
			go p.revalidateInBackground(targetURL, primaryKey, cachedEntry, r.Header)
			return
		}

		// Expired or needs revalidation: perform conditional upstream request
		p.revalidateSynchronously(w, r, targetURL, primaryKey, cachedEntry, reqDir, respDir, txID, startTime)
		return
	}

	// Cache MISS: Fetch from origin (with single-flight coalescing)
	p.fetchAndCache(w, r, targetURL, primaryKey, reqDir, txID, startTime)
}

func (p *Proxy) resolveTargetURL(r *http.Request) string {
	// 1. Check query parameter: /proxy?url=...
	if queryURL := r.URL.Query().Get("url"); queryURL != "" {
		return queryURL
	}

	// 2. Check path prefix: /proxy/http://... or /proxy/https://...
	path := r.URL.Path
	if strings.HasPrefix(path, "/proxy/") {
		raw := strings.TrimPrefix(path, "/proxy/")
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			if r.URL.RawQuery != "" {
				raw += "?" + r.URL.RawQuery
			}
			return raw
		}
	}

	// 3. Fallback to default origin if configured
	if p.defaultOrigin != "" {
		return fmt.Sprintf("%s%s", strings.TrimSuffix(p.defaultOrigin, "/"), r.URL.RequestURI())
	}

	return ""
}

func (p *Proxy) serveHit(
	w http.ResponseWriter,
	r *http.Request,
	entry *cache.Entry,
	reqDir cache.RequestDirectives,
	respDir cache.ResponseDirectives,
	primaryKey string,
	txID string,
	startTime time.Time,
	currentAge time.Duration,
	status cache.CacheStatus,
) {
	latencyMs := time.Since(startTime).Milliseconds()

	// Copy cached headers
	for k, vv := range entry.Headers {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Apply CDN headers
	p.cdnModifier.ApplyHeaders(w.Header(), status, entry, currentAge, entry.Hits)

	w.WriteHeader(entry.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = w.Write(entry.Body)
	}

	// Build and publish explanation
	varyHeaders := entry.Headers.Values("Vary")
	varyHash, _ := cache.ComputeVaryHash(varyHeaders, r.Header)

	exp := explainer.BuildExplanation(
		txID,
		r,
		reqDir,
		w.Header(),
		respDir,
		primaryKey,
		varyHash,
		varyHeaders,
		entry,
		status,
		latencyMs,
		0,
		p.cdnModifier.GetConfig().ActiveFlavor,
		time.Now(),
	)
	p.recordExplanation(exp)
}

func (p *Proxy) fetchAndCache(
	w http.ResponseWriter,
	r *http.Request,
	targetURL string,
	primaryKey string,
	reqDir cache.RequestDirectives,
	txID string,
	startTime time.Time,
) {
	originStart := time.Now()

	// Check if coalescing is requested or disabled (via simulation header)
	enableCoalescing := true
	if r.Header.Get("X-EdgeLens-Coalesce") == "false" {
		enableCoalescing = false
	}

	fetchFn := func() stampede.CoalesceResult {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
		if err != nil {
			return stampede.CoalesceResult{Err: err}
		}

		// Forward headers (excluding hop-by-hop)
		copyHeaders(r.Header, req.Header)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return stampede.CoalesceResult{Err: err}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return stampede.CoalesceResult{Err: err}
		}

		return stampede.CoalesceResult{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       body,
		}
	}

	var result stampede.CoalesceResult
	var coalesced bool

	if enableCoalescing {
		result, coalesced = p.coalescer.Do(primaryKey, fetchFn)
	} else {
		result = fetchFn()
	}

	originLatencyMs := time.Since(originStart).Milliseconds()
	totalLatencyMs := time.Since(startTime).Milliseconds()

	if result.Err != nil {
		http.Error(w, fmt.Sprintf("EdgeLens Upstream Error: %v", result.Err), http.StatusBadGateway)
		return
	}

	respDir := cache.ParseResponseDirectives(
		result.Header.Values("Cache-Control"),
		result.Header.Values("Surrogate-Control"),
	)
	tags := surrogate.ExtractTags(result.Header)
	varyHeaders := result.Header.Values("Vary")
	varyHash, varyCaptured := cache.ComputeVaryHash(varyHeaders, r.Header)
	fullKey := cache.GenerateFullKey(primaryKey, varyHash)

	// Evaluate if response can be stored in edge cache
	canCache := p.isResponseCacheable(r, reqDir, result.StatusCode, result.Header, respDir)

	var entry *cache.Entry
	if canCache {
		now := time.Now()
		freshnessLifetime := cache.CalculateFreshnessLifetime(respDir, result.StatusCode, result.Header, now)

		var swrDuration time.Duration
		if respDir.StaleWhileRevalidate != nil {
			swrDuration = time.Duration(*respDir.StaleWhileRevalidate) * time.Second
		}
		var sieDuration time.Duration
		if respDir.StaleIfError != nil {
			sieDuration = time.Duration(*respDir.StaleIfError) * time.Second
		}

		entry = &cache.Entry{
			Key:               fullKey,
			Method:            r.Method,
			URL:               targetURL,
			StatusCode:        result.StatusCode,
			Headers:           result.Header.Clone(),
			Body:              result.Body,
			BodySize:          int64(len(result.Body)),
			VaryHeaders:       varyCaptured,
			Tags:              tags,
			StoredAt:          now,
			ResponseTime:      now,
			RequestTime:       originStart,
			FreshnessLifetime: freshnessLifetime,
			SWRDuration:       swrDuration,
			SIEDuration:       sieDuration,
			ETag:              result.Header.Get("ETag"),
			LastModified:      result.Header.Get("LastModified"),
			Hits:              1,
		}

		p.store.Put(entry, primaryKey, varyHeaders)
		p.index.IndexEntry(fullKey, tags)
	}

	status := cache.StatusMiss
	if coalesced {
		status = cache.StatusHit
	}

	// Write response headers
	for k, vv := range result.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	p.cdnModifier.ApplyHeaders(w.Header(), status, entry, 0, 1)

	w.WriteHeader(result.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = w.Write(result.Body)
	}

	exp := explainer.BuildExplanation(
		txID,
		r,
		reqDir,
		w.Header(),
		respDir,
		primaryKey,
		varyHash,
		varyHeaders,
		entry,
		status,
		totalLatencyMs,
		originLatencyMs,
		p.cdnModifier.GetConfig().ActiveFlavor,
		time.Now(),
	)
	p.recordExplanation(exp)
}

func (p *Proxy) revalidateSynchronously(
	w http.ResponseWriter,
	r *http.Request,
	targetURL string,
	primaryKey string,
	cachedEntry *cache.Entry,
	reqDir cache.RequestDirectives,
	respDir cache.ResponseDirectives,
	txID string,
	startTime time.Time,
) {
	originStart := time.Now()
	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copyHeaders(r.Header, req.Header)

	// Add conditional headers
	if cachedEntry.ETag != "" {
		req.Header.Set("If-None-Match", cachedEntry.ETag)
	}
	if cachedEntry.LastModified != "" {
		req.Header.Set("If-Modified-Since", cachedEntry.LastModified)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		// If origin errors out and cached entry has stale-if-error, serve stale!
		if cachedEntry.SIEDuration > 0 {
			p.serveHit(w, r, cachedEntry, reqDir, respDir, primaryKey, txID, startTime, cache.CalculateCurrentAge(cachedEntry, time.Now()), cache.StatusStale)
			return
		}
		http.Error(w, fmt.Sprintf("EdgeLens Revalidation Upstream Error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	originLatencyMs := time.Since(originStart).Milliseconds()
	totalLatencyMs := time.Since(startTime).Milliseconds()

	if resp.StatusCode == http.StatusNotModified {
		// 304 NOT MODIFIED: Refresh existing cached entry
		now := time.Now()
		cachedEntry.StoredAt = now
		cachedEntry.ResponseTime = now
		cachedEntry.IsSoftPurged = false

		// Merge headers from origin if updated
		for k, vv := range resp.Header {
			for _, v := range vv {
				cachedEntry.Headers.Set(k, v)
			}
		}

		for k, vv := range cachedEntry.Headers {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}

		p.cdnModifier.ApplyHeaders(w.Header(), cache.StatusRevalidated, cachedEntry, 0, cachedEntry.Hits)
		w.WriteHeader(cachedEntry.StatusCode)
		if r.Method != http.MethodHead {
			_, _ = w.Write(cachedEntry.Body)
		}

		varyHeaders := cachedEntry.Headers.Values("Vary")
		varyHash, _ := cache.ComputeVaryHash(varyHeaders, r.Header)

		exp := explainer.BuildExplanation(
			txID,
			r,
			reqDir,
			w.Header(),
			respDir,
			primaryKey,
			varyHash,
			varyHeaders,
			cachedEntry,
			cache.StatusRevalidated,
			totalLatencyMs,
			originLatencyMs,
			p.cdnModifier.GetConfig().ActiveFlavor,
			now,
		)
		p.recordExplanation(exp)
		return
	}

	// 200 OK: Full refresh
	body, _ := io.ReadAll(resp.Body)
	newRespDir := cache.ParseResponseDirectives(
		resp.Header.Values("Cache-Control"),
		resp.Header.Values("Surrogate-Control"),
	)
	tags := surrogate.ExtractTags(resp.Header)
	varyHeaders := resp.Header.Values("Vary")
	varyHash, varyCaptured := cache.ComputeVaryHash(varyHeaders, r.Header)
	fullKey := cache.GenerateFullKey(primaryKey, varyHash)

	now := time.Now()
	freshnessLifetime := cache.CalculateFreshnessLifetime(newRespDir, resp.StatusCode, resp.Header, now)

	cachedEntry.Headers = resp.Header.Clone()
	cachedEntry.Body = body
	cachedEntry.BodySize = int64(len(body))
	cachedEntry.StoredAt = now
	cachedEntry.ResponseTime = now
	cachedEntry.FreshnessLifetime = freshnessLifetime
	cachedEntry.Tags = tags
	cachedEntry.VaryHeaders = varyCaptured
	cachedEntry.ETag = resp.Header.Get("ETag")
	cachedEntry.LastModified = resp.Header.Get("Last-Modified")
	cachedEntry.IsSoftPurged = false

	p.store.Put(cachedEntry, primaryKey, varyHeaders)
	p.index.IndexEntry(fullKey, tags)

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	p.cdnModifier.ApplyHeaders(w.Header(), cache.StatusExpired, cachedEntry, 0, cachedEntry.Hits)
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}

	exp := explainer.BuildExplanation(
		txID,
		r,
		reqDir,
		w.Header(),
		newRespDir,
		primaryKey,
		varyHash,
		varyHeaders,
		cachedEntry,
		cache.StatusExpired,
		totalLatencyMs,
		originLatencyMs,
		p.cdnModifier.GetConfig().ActiveFlavor,
		now,
	)
	p.recordExplanation(exp)
}

func (p *Proxy) revalidateInBackground(targetURL, primaryKey string, cachedEntry *cache.Entry, clientHeaders http.Header) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return
	}

	copyHeaders(clientHeaders, req.Header)
	if cachedEntry.ETag != "" {
		req.Header.Set("If-None-Match", cachedEntry.ETag)
	}
	if cachedEntry.LastModified != "" {
		req.Header.Set("If-Modified-Since", cachedEntry.LastModified)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	now := time.Now()
	if resp.StatusCode == http.StatusNotModified {
		cachedEntry.StoredAt = now
		cachedEntry.ResponseTime = now
		cachedEntry.IsSoftPurged = false
		return
	}

	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}

		newRespDir := cache.ParseResponseDirectives(
			resp.Header.Values("Cache-Control"),
			resp.Header.Values("Surrogate-Control"),
		)
		tags := surrogate.ExtractTags(resp.Header)
		freshnessLifetime := cache.CalculateFreshnessLifetime(newRespDir, resp.StatusCode, resp.Header, now)

		cachedEntry.Headers = resp.Header.Clone()
		cachedEntry.Body = body
		cachedEntry.BodySize = int64(len(body))
		cachedEntry.StoredAt = now
		cachedEntry.ResponseTime = now
		cachedEntry.FreshnessLifetime = freshnessLifetime
		cachedEntry.Tags = tags
		cachedEntry.ETag = resp.Header.Get("ETag")
		cachedEntry.LastModified = resp.Header.Get("Last-Modified")
		cachedEntry.IsSoftPurged = false
	}
}

func (p *Proxy) handleBypass(
	w http.ResponseWriter,
	r *http.Request,
	targetURL string,
	primaryKey string,
	reqDir cache.RequestDirectives,
	txID string,
	startTime time.Time,
	reason string,
) {
	originStart := time.Now()

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	copyHeaders(r.Header, req.Header)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("EdgeLens Upstream Bypass Error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	originLatencyMs := time.Since(originStart).Milliseconds()
	totalLatencyMs := time.Since(startTime).Milliseconds()

	// If method is POST, PUT, DELETE, PATCH, RFC 9111 mandates cache invalidation of the target URI!
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
		p.store.DeleteByPrimaryKey(primaryKey)
	}
	p.store.RecordBypass()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	p.cdnModifier.ApplyHeaders(w.Header(), cache.StatusBypass, nil, 0, 0)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)

	respDir := cache.ParseResponseDirectives(resp.Header.Values("Cache-Control"), nil)
	exp := explainer.BuildExplanation(
		txID,
		r,
		reqDir,
		w.Header(),
		respDir,
		primaryKey,
		"",
		nil,
		nil,
		cache.StatusBypass,
		totalLatencyMs,
		originLatencyMs,
		p.cdnModifier.GetConfig().ActiveFlavor,
		time.Now(),
	)
	p.recordExplanation(exp)
}

func (p *Proxy) isResponseCacheable(
	r *http.Request,
	reqDir cache.RequestDirectives,
	statusCode int,
	header http.Header,
	rd cache.ResponseDirectives,
) bool {
	// Status code must be cacheable
	if statusCode != 200 && statusCode != 203 && statusCode != 204 && statusCode != 206 &&
		statusCode != 300 && statusCode != 301 && statusCode != 308 && statusCode != 404 &&
		statusCode != 405 && statusCode != 410 && statusCode != 414 && statusCode != 501 {
		return false
	}

	// Origin no-store prohibits caching
	if rd.NoStore {
		return false
	}

	// Shared proxy must not store private responses
	if rd.Private {
		return false
	}

	// If Authorization is present, only cache if public or s-maxage
	if r.Header.Get("Authorization") != "" && !rd.Public && rd.SMaxAge == nil {
		return false
	}

	// If Set-Cookie is present, shared proxy avoids caching unless explicit public or s-maxage
	if header.Get("Set-Cookie") != "" && !rd.Public && rd.SMaxAge == nil {
		return false
	}

	return true
}

func (p *Proxy) recordExplanation(exp *explainer.Explanation) {
	p.mu.Lock()
	p.recentExps = append([]*explainer.Explanation{exp}, p.recentExps...)
	if len(p.recentExps) > p.maxRecentCount {
		p.recentExps = p.recentExps[:p.maxRecentCount]
	}
	listeners := make([]TransactionListener, len(p.listeners))
	copy(listeners, p.listeners)
	p.mu.Unlock()

	for _, l := range listeners {
		l(exp)
	}
}

func copyHeaders(src, dst http.Header) {
	for k, vv := range src {
		// Skip hop-by-hop headers
		if strings.EqualFold(k, "Connection") || strings.EqualFold(k, "Keep-Alive") ||
			strings.EqualFold(k, "Proxy-Authenticate") || strings.EqualFold(k, "Proxy-Authorization") ||
			strings.EqualFold(k, "TE") || strings.EqualFold(k, "Trailers") ||
			strings.EqualFold(k, "Transfer-Encoding") || strings.EqualFold(k, "Upgrade") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func randomHex(bytesLen int) string {
	return fmt.Sprintf("%x", bytes.Repeat([]byte{0x7a}, bytesLen))
}
