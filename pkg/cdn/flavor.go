package cdn

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/alexandrmotologa/edgelens/pkg/cache"
)

// Flavor defines supported CDN edge emulation profiles.
type Flavor string

const (
	FlavorCloudflare Flavor = "cloudflare"
	FlavorFastly     Flavor = "fastly"
	FlavorCloudFront Flavor = "cloudfront"
	FlavorVercel     Flavor = "vercel"
	FlavorRFC9111    Flavor = "rfc9111"
)

// SupportedFlavors returns the list of all available CDN profiles.
func SupportedFlavors() []Flavor {
	return []Flavor{
		FlavorCloudflare,
		FlavorFastly,
		FlavorCloudFront,
		FlavorVercel,
		FlavorRFC9111,
	}
}

// Config controls CDN-specific caching rules and header manipulation.
type Config struct {
	ActiveFlavor        Flavor `json:"activeFlavor"`
	StripSurrogateKeys  bool   `json:"stripSurrogateKeys"`
	StripSetCookieOnHit bool   `json:"stripSetCookieOnHit"`
	EnableStaleIfError  bool   `json:"enableStaleIfError"`
}

// DefaultConfig returns default CDN settings.
func DefaultConfig() Config {
	return Config{
		ActiveFlavor:        FlavorCloudflare,
		StripSurrogateKeys:  true,
		StripSetCookieOnHit: true,
		EnableStaleIfError:  true,
	}
}

// HeaderModifier applies CDN-specific response headers.
type HeaderModifier struct {
	config Config
}

// NewHeaderModifier creates a new CDN header modifier.
func NewHeaderModifier(cfg Config) *HeaderModifier {
	return &HeaderModifier{config: cfg}
}

// SetFlavor changes the current active CDN profile.
func (hm *HeaderModifier) SetFlavor(f Flavor) {
	hm.config.ActiveFlavor = f
}

// GetConfig returns current CDN configuration.
func (hm *HeaderModifier) GetConfig() Config {
	return hm.config
}

// ApplyHeaders injects CDN headers into the downstream client response.
func (hm *HeaderModifier) ApplyHeaders(
	w http.Header,
	status cache.CacheStatus,
	entry *cache.Entry,
	age time.Duration,
	hits int64,
) {
	ageSecs := int(age.Seconds())
	if ageSecs < 0 {
		ageSecs = 0
	}

	// Always inject standard Age header if response came from edge cache or was stale
	if status == cache.StatusHit || status == cache.StatusStale || status == cache.StatusExpired {
		w.Set("Age", strconv.Itoa(ageSecs))
	}

	switch hm.config.ActiveFlavor {
	case FlavorCloudflare:
		applyCloudflareHeaders(w, status)
	case FlavorFastly:
		applyFastlyHeaders(w, status, hits)
	case FlavorCloudFront:
		applyCloudFrontHeaders(w, status)
	case FlavorVercel:
		applyVercelHeaders(w, status)
	case FlavorRFC9111:
		applyRFCHeaders(w, status, ageSecs)
	}

	// Strip origin surrogate keys to emulate edge CDNs protecting internal tags
	if hm.config.StripSurrogateKeys {
		w.Del("Surrogate-Control")
		w.Del("Surrogate-Key")
		w.Del("Cache-Tag")
		w.Del("Edge-Cache-Tag")
	}

	// If serving from cache (HIT or STALE), strip origin Set-Cookie to prevent session leaks
	if hm.config.StripSetCookieOnHit && (status == cache.StatusHit || status == cache.StatusStale) {
		w.Del("Set-Cookie")
	}
}

func applyCloudflareHeaders(h http.Header, status cache.CacheStatus) {
	var cfStatus string
	switch status {
	case cache.StatusHit:
		cfStatus = "HIT"
	case cache.StatusMiss:
		cfStatus = "MISS"
	case cache.StatusExpired:
		cfStatus = "EXPIRED"
	case cache.StatusStale:
		cfStatus = "STALE"
	case cache.StatusBypass:
		cfStatus = "BYPASS"
	case cache.StatusRevalidated:
		cfStatus = "REVALIDATED"
	case cache.StatusDynamic:
		cfStatus = "DYNAMIC"
	default:
		cfStatus = "DYNAMIC"
	}

	h.Set("CF-Cache-Status", cfStatus)
	h.Set("CF-Ray", randomHex(16)+"-LHR")
	h.Set("Server", "cloudflare")
}

func applyFastlyHeaders(h http.Header, status cache.CacheStatus, hits int64) {
	switch status {
	case cache.StatusHit, cache.StatusStale:
		h.Set("X-Cache", "HIT")
		h.Set("X-Cache-Hits", strconv.FormatInt(hits, 10))
	case cache.StatusRevalidated:
		h.Set("X-Cache", "HIT, HIT")
		h.Set("X-Cache-Hits", strconv.FormatInt(hits, 10))
	case cache.StatusMiss, cache.StatusExpired, cache.StatusDynamic:
		h.Set("X-Cache", "MISS")
		h.Set("X-Cache-Hits", "0")
	case cache.StatusBypass:
		h.Set("X-Cache", "PASS")
		h.Set("X-Cache-Hits", "0")
	}

	h.Set("X-Served-By", "cache-lhr-edgelens")
}

func applyCloudFrontHeaders(h http.Header, status cache.CacheStatus) {
	var cfStatus string
	switch status {
	case cache.StatusHit, cache.StatusStale:
		cfStatus = "Hit from cloudfront"
	case cache.StatusRevalidated:
		cfStatus = "RefreshHit from cloudfront"
	case cache.StatusMiss, cache.StatusExpired, cache.StatusDynamic:
		cfStatus = "Miss from cloudfront"
	case cache.StatusBypass:
		cfStatus = "Miss from cloudfront"
	default:
		cfStatus = "Miss from cloudfront"
	}

	h.Set("X-Cache", cfStatus)
	h.Set("Via", "2.0 edgelens.cloudfront.net (CloudFront)")
	h.Set("X-Amz-Cf-Pop", "LHR50-C1")
	h.Set("X-Amz-Cf-Id", randomHex(16))
}

func applyVercelHeaders(h http.Header, status cache.CacheStatus) {
	var vStatus string
	switch status {
	case cache.StatusHit:
		vStatus = "HIT"
	case cache.StatusStale:
		vStatus = "STALE"
	case cache.StatusRevalidated:
		vStatus = "REVALIDATED"
	case cache.StatusMiss, cache.StatusExpired, cache.StatusDynamic:
		vStatus = "MISS"
	case cache.StatusBypass:
		vStatus = "BYPASS"
	default:
		vStatus = "MISS"
	}

	h.Set("x-vercel-cache", vStatus)
	h.Set("x-vercel-id", fmt.Sprintf("lhr1::%s", randomHex(12)))
	h.Set("Server", "Vercel")
}

func applyRFCHeaders(h http.Header, status cache.CacheStatus, ageSecs int) {
	var statusParam string
	switch status {
	case cache.StatusHit:
		statusParam = fmt.Sprintf("EdgeLens; hit; ttl=%d", ageSecs)
	case cache.StatusStale:
		statusParam = "EdgeLens; hit; stale"
		h.Set("Warning", `110 - "Response is Stale"`)
	case cache.StatusRevalidated:
		statusParam = "EdgeLens; hit; revalidated"
	case cache.StatusMiss:
		statusParam = "EdgeLens; fwd=uri-miss"
	case cache.StatusBypass:
		statusParam = "EdgeLens; fwd=bypass"
	default:
		statusParam = "EdgeLens; fwd=request"
	}

	h.Set("Cache-Status", statusParam)
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
