package cache

import (
	"net/http"
	"time"
)

// CacheStatus represents the edge caching outcome.
type CacheStatus string

const (
	StatusHit         CacheStatus = "HIT"
	StatusMiss        CacheStatus = "MISS"
	StatusExpired     CacheStatus = "EXPIRED"
	StatusStale       CacheStatus = "STALE"
	StatusBypass      CacheStatus = "BYPASS"
	StatusRevalidated CacheStatus = "REVALIDATED"
	StatusDynamic     CacheStatus = "DYNAMIC"
)

// ResponseDirectives represents parsed Cache-Control directives from origin.
type ResponseDirectives struct {
	Public               bool
	Private              bool
	NoCache              bool
	NoStore              bool
	NoTransform          bool
	MustRevalidate       bool
	ProxyRevalidate      bool
	MustUnderstand       bool
	Immutable            bool
	MaxAge               *int
	SMaxAge              *int
	SurrogateMaxAge      *int
	StaleWhileRevalidate *int
	StaleIfError         *int
}

// RequestDirectives represents parsed Cache-Control directives from client.
type RequestDirectives struct {
	NoCache      bool
	NoStore      bool
	NoTransform  bool
	OnlyIfCached bool
	MaxAge       *int
	MaxStale     *int
	MinFresh     *int
}

// Entry represents a cached HTTP response in memory.
type Entry struct {
	Key               string            `json:"key"`
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	StatusCode        int               `json:"statusCode"`
	Headers           http.Header       `json:"headers"`
	Body              []byte            `json:"-"`
	BodySize          int64             `json:"bodySize"`
	VaryHeaders       map[string]string `json:"varyHeaders"`
	Tags              []string          `json:"tags"`
	StoredAt          time.Time         `json:"storedAt"`
	ResponseTime      time.Time         `json:"responseTime"`
	RequestTime       time.Time         `json:"requestTime"`
	FreshnessLifetime time.Duration     `json:"freshnessLifetime"`
	SWRDuration       time.Duration     `json:"swrDuration"`
	SIEDuration       time.Duration     `json:"sieDuration"`
	ETag              string            `json:"etag"`
	LastModified      string            `json:"lastModified"`
	IsSoftPurged      bool              `json:"isSoftPurged"`
	SoftPurgedAt      time.Time         `json:"softPurgedAt,omitempty"`
	Hits              int64             `json:"hits"`
}

// EntrySummary provides light telemetry for UI and listings.
type EntrySummary struct {
	Key               string            `json:"key"`
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	StatusCode        int               `json:"statusCode"`
	BodySize          int64             `json:"bodySize"`
	Tags              []string          `json:"tags"`
	StoredAt          time.Time         `json:"storedAt"`
	FreshnessLifetime int               `json:"freshnessLifetimeSeconds"`
	CurrentAge        int               `json:"currentAgeSeconds"`
	IsFresh           bool              `json:"isFresh"`
	IsInSWR           bool              `json:"isInSWR"`
	IsSoftPurged      bool              `json:"isSoftPurged"`
	Hits              int64             `json:"hits"`
	ETag              string            `json:"etag"`
	VaryHeaders       map[string]string `json:"varyHeaders"`
}
