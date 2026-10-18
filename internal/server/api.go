package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/assert"
	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
	"github.com/alexandrmotologa/edgelens/pkg/cdngen"
	"github.com/alexandrmotologa/edgelens/pkg/comparator"
	"github.com/alexandrmotologa/edgelens/pkg/explainer"
	"github.com/alexandrmotologa/edgelens/pkg/har"
	"github.com/alexandrmotologa/edgelens/pkg/mockorigin"
	"github.com/alexandrmotologa/edgelens/pkg/proxy"
	"github.com/alexandrmotologa/edgelens/pkg/rules"
	"github.com/alexandrmotologa/edgelens/pkg/stampede"
	"github.com/alexandrmotologa/edgelens/pkg/surrogate"
	"github.com/alexandrmotologa/edgelens/pkg/varymatrix"
)

// APIHandler dispatches REST endpoints for the web studio.
type APIHandler struct {
	store       *cache.Store
	index       *surrogate.Index
	purgeEngine *surrogate.PurgeEngine
	proxy       *proxy.Proxy
	mockOrigin  *mockorigin.Server
	broker      *SSEBroker
	httpClient  *http.Client
}

// NewAPIHandler constructs the API multiplexer.
func NewAPIHandler(
	store *cache.Store,
	index *surrogate.Index,
	purgeEngine *surrogate.PurgeEngine,
	p *proxy.Proxy,
	mockOrigin *mockorigin.Server,
	broker *SSEBroker,
) *APIHandler {
	return &APIHandler{
		store:       store,
		index:       index,
		purgeEngine: purgeEngine,
		proxy:       p,
		mockOrigin:  mockOrigin,
		broker:      broker,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// RegisterRoutes registers all REST and SSE endpoints on a multiplexer.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/stream", h.broker.HandleSSE)
	mux.HandleFunc("/api/traffic", h.handleTraffic)
	mux.HandleFunc("/api/traffic/", h.handleTrafficDetail)
	mux.HandleFunc("/api/cache/entries", h.handleCacheEntries)
	mux.HandleFunc("/api/cache/purge", h.handleCachePurge)
	mux.HandleFunc("/api/cache/tags", h.handleCacheTags)
	mux.HandleFunc("/api/cache/stats", h.handleCacheStats)
	mux.HandleFunc("/api/cache/history", h.handlePurgeHistory)
	mux.HandleFunc("/api/cdn/config", h.handleCDNConfig)
	mux.HandleFunc("/api/stampede/run", h.handleStampedeRun)
	mux.HandleFunc("/api/mock/routes", h.handleMockRoutes)
	mux.HandleFunc("/api/audit", h.handleAudit)

	// New feature endpoints
	mux.HandleFunc("/api/rules", h.handleRules)
	mux.HandleFunc("/api/rules/", h.handleRuleItem)
	mux.HandleFunc("/api/compare", h.handleCompare)
	mux.HandleFunc("/api/vary-matrix", h.handleVaryMatrix)
	mux.HandleFunc("/api/export-cdn", h.handleExportCDN)
	mux.HandleFunc("/api/assert", h.handleAssert)
	mux.HandleFunc("/api/parse-curl", h.handleParseCurl)
	mux.HandleFunc("/api/parse-har", h.handleParseHAR)
}

func (h *APIHandler) handleTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	explanations := h.proxy.GetRecentExplanations()
	statusFilter := r.URL.Query().Get("status")

	if statusFilter != "" {
		filtered := make([]*explainer.Explanation, 0)
		for _, e := range explanations {
			if strings.EqualFold(string(e.FinalStatus), statusFilter) {
				filtered = append(filtered, e)
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	writeJSON(w, http.StatusOK, explanations)
}

func (h *APIHandler) handleTrafficDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/traffic/")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	exp, found := h.proxy.GetExplanationByID(id)
	if !found {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, exp)
}

func (h *APIHandler) handleCacheEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summaries := h.store.ListSummaries()
	writeJSON(w, http.StatusOK, summaries)
}

type purgeRequest struct {
	Type     string `json:"type"` // "tag", "url", "all"
	Target   string `json:"target"`
	IsSoft   bool   `json:"isSoft"`
	Wildcard bool   `json:"wildcard"`
}

func (h *APIHandler) handleCachePurge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req purgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	var res surrogate.PurgeResult
	switch req.Type {
	case "tag":
		res = h.purgeEngine.PurgeTag(req.Target, req.IsSoft)
	case "url":
		res = h.purgeEngine.PurgeURL(req.Target, req.Wildcard, req.IsSoft)
	case "all":
		res = h.purgeEngine.PurgeAll(req.IsSoft)
	default:
		http.Error(w, "Invalid purge type. Expected 'tag', 'url', or 'all'", http.StatusBadRequest)
		return
	}

	// Broadcast purge event via SSE
	h.broker.Broadcast("purge", res)
	writeJSON(w, http.StatusOK, res)
}

func (h *APIHandler) handleCacheTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tags := h.index.ListTags()
	writeJSON(w, http.StatusOK, tags)
}

func (h *APIHandler) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.store.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *APIHandler) handlePurgeHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := h.purgeEngine.GetHistory()
	writeJSON(w, http.StatusOK, history)
}

type cdnConfigRequest struct {
	Flavor string `json:"flavor"`
}

func (h *APIHandler) handleCDNConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg := h.proxy.GetCDNConfig()
		resp := map[string]interface{}{
			"activeFlavor":     cfg.ActiveFlavor,
			"supportedFlavors": cdn.SupportedFlavors(),
			"config":           cfg,
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if r.Method == http.MethodPost {
		var req cdnConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		f := cdn.Flavor(strings.ToLower(req.Flavor))
		h.proxy.SetCDNFlavor(f)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":       "updated",
			"activeFlavor": f,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *APIHandler) handleStampedeRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var opts stampede.SimulatorOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if opts.TargetURL == "" {
		http.Error(w, "Missing targetUrl parameter", http.StatusBadRequest)
		return
	}

	report, err := stampede.RunSimulation(r.Context(), h.httpClient, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleMockRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		routes := h.mockOrigin.ListRoutes()
		writeJSON(w, http.StatusOK, routes)
		return
	}

	if r.Method == http.MethodPost {
		var route mockorigin.MockRoute
		if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		h.mockOrigin.RegisterRoute(route)
		writeJSON(w, http.StatusCreated, route)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type auditRequest struct {
	URL     string              `json:"url,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
}

func (h *APIHandler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req auditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	headersToAudit := req.Headers
	if headersToAudit == nil && req.URL != "" {
		// Fetch headers from live URL
		fetchReq, err := http.NewRequestWithContext(r.Context(), http.MethodHead, req.URL, nil)
		if err != nil {
			http.Error(w, "Invalid target URL: "+err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := h.httpClient.Do(fetchReq)
		if err != nil {
			http.Error(w, "Failed to connect to target URL: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		headersToAudit = resp.Header
	}

	if headersToAudit == nil {
		headersToAudit = make(map[string][]string)
	}

	report := explainer.AuditHeaders(headersToAudit)
	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleRules(w http.ResponseWriter, r *http.Request) {
	rulesEngine := h.proxy.GetRulesEngine()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rulesEngine.GetRules())
	case http.MethodPost:
		var rule rules.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		rulesEngine.AddRule(rule)
		writeJSON(w, http.StatusCreated, rulesEngine.GetRules())
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleRuleItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rules/")
	rulesEngine := h.proxy.GetRulesEngine()

	if strings.HasSuffix(path, "/toggle") && r.Method == http.MethodPut {
		id := strings.TrimSuffix(path, "/toggle")
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if ok := rulesEngine.ToggleRule(id, payload.Enabled); !ok {
			http.Error(w, "Rule not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, rulesEngine.GetRules())
		return
	}

	if r.Method == http.MethodDelete {
		if ok := rulesEngine.DeleteRule(path); !ok {
			http.Error(w, "Rule not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, rulesEngine.GetRules())
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type compareRequest struct {
	Method           string              `json:"method"`
	URL              string              `json:"url"`
	Headers          map[string][]string `json:"headers"`
	StatusCode       int                 `json:"statusCode"`
	ResponseHeaders  map[string][]string `json:"responseHeaders"`
	SimulatedAgeSecs int                 `json:"simulatedAgeSecs"`
}

func (h *APIHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req compareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.Method == "" {
		req.Method = "GET"
	}
	if req.StatusCode == 0 {
		req.StatusCode = 200
	}

	reqH := http.Header(req.Headers)
	respH := http.Header(req.ResponseHeaders)

	// If URL is provided and response headers are empty, fetch from live URL
	if req.URL != "" && len(respH) == 0 {
		fetchReq, err := http.NewRequestWithContext(r.Context(), req.Method, req.URL, nil)
		if err == nil {
			for k, vv := range reqH {
				for _, v := range vv {
					fetchReq.Header.Add(k, v)
				}
			}
			resp, err := h.httpClient.Do(fetchReq)
			if err == nil {
				defer resp.Body.Close()
				req.StatusCode = resp.StatusCode
				respH = resp.Header.Clone()
			}
		}
	}

	report := comparator.CompareRequestResponse(req.Method, req.URL, reqH, req.StatusCode, respH, req.SimulatedAgeSecs)
	writeJSON(w, http.StatusOK, report)
}

type varyMatrixRequest struct {
	VaryHeader string `json:"varyHeader"`
}

func (h *APIHandler) handleVaryMatrix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req varyMatrixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	report := varymatrix.AnalyzeVaryHeader(req.VaryHeader)
	writeJSON(w, http.StatusOK, report)
}

func (h *APIHandler) handleExportCDN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var opts cdngen.ConfigOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	configs := cdngen.GenerateAll(opts)
	writeJSON(w, http.StatusOK, configs)
}

func (h *APIHandler) handleAssert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var opts assert.AssertionOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if opts.URL == "" {
		http.Error(w, "Missing target URL", http.StatusBadRequest)
		return
	}

	result, err := assert.RunAssertion(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type parseCurlRequest struct {
	Command string `json:"command"`
}

func (h *APIHandler) handleParseCurl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req parseCurlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	parsed, err := har.ParseCurl(req.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, parsed)
}

func (h *APIHandler) handleParseHAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	report, err := har.ParseHAR(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
