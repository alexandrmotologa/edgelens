package mockorigin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// MockRoute defines an endpoint configuration on the mock origin server.
type MockRoute struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	StatusCode  int               `json:"statusCode"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	DelayMs     int               `json:"delayMs"`
	Description string            `json:"description"`
	HitCount    atomic.Int64      `json:"-"`
	Hits        int64             `json:"hitCount"`
}

// Server provides an embedded HTTP origin server with controllable caching behaviors.
type Server struct {
	mu     sync.RWMutex
	routes map[string]*MockRoute
}

// NewServer initializes the mock origin server with default scenarios.
func NewServer() *Server {
	s := &Server{
		routes: make(map[string]*MockRoute),
	}
	s.registerDefaultScenarios()
	return s
}

// RegisterRoute adds or updates a mock endpoint.
func (s *Server) RegisterRoute(route MockRoute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%s", route.Method, route.Path)
	s.routes[key] = &route
}

// ListRoutes returns all active mock origin routes and their hit counters.
func (s *Server) ListRoutes() []*MockRoute {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*MockRoute, 0, len(s.routes))
	for _, r := range s.routes {
		r.Hits = r.HitCount.Load()
		out = append(out, r)
	}
	return out
}

// ResetCounters zeroes out hit counters across all routes.
func (s *Server) ResetCounters() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.routes {
		r.HitCount.Store(0)
	}
}

// ServeHTTP handles requests to the mock origin server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := fmt.Sprintf("%s:%s", r.Method, r.URL.Path)

	s.mu.RLock()
	route, ok := s.routes[key]
	s.mu.RUnlock()

	if !ok {
		// Fallback dynamic 200 handler for any unspecified /mock/... path
		s.handleDynamicFallback(w, r)
		return
	}

	route.HitCount.Add(1)

	// Simulate origin processing latency if configured
	if route.DelayMs > 0 {
		time.Sleep(time.Duration(route.DelayMs) * time.Millisecond)
	}

	// Check if simulated failure is requested via query parameter
	if r.URL.Query().Get("fail") == "true" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "Simulated Origin 500 Internal Server Error", "status": 500}`))
		return
	}
	if etag := route.Headers["ETag"]; etag != "" {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Write custom configured headers
	for k, v := range route.Headers {
		w.Header().Set(k, v)
	}

	// Set dynamic Date header
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))

	statusCode := route.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.WriteHeader(statusCode)

	// Inject dynamic variables if body is dynamic JSON
	bodyBytes := []byte(route.Body)
	if len(bodyBytes) == 0 {
		resp := map[string]interface{}{
			"message":   "EdgeLens Mock Origin Response",
			"path":      r.URL.Path,
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"hits":      route.HitCount.Load(),
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	_, _ = w.Write(bodyBytes)
}

func (s *Server) handleDynamicFallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=30")
	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, time.Now().Unix()/60))
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"status":    "mock_fallback",
		"path":      r.URL.Path,
		"method":    r.Method,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"note":      "Default mock response with max-age=60 and SWR=30",
	}
	_ = json.NewEncoder(w).Encode(resp)
}
