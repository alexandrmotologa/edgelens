package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alexandrmotologa/edgelens/internal/server"
	"github.com/alexandrmotologa/edgelens/pkg/cache"
	"github.com/alexandrmotologa/edgelens/pkg/cdn"
	"github.com/alexandrmotologa/edgelens/pkg/explainer"
	"github.com/alexandrmotologa/edgelens/pkg/mockorigin"
	"github.com/alexandrmotologa/edgelens/pkg/proxy"
	"github.com/alexandrmotologa/edgelens/pkg/stampede"
	"github.com/alexandrmotologa/edgelens/pkg/surrogate"
	"github.com/alexandrmotologa/edgelens/web"
)

const banner = `
  ███████╗██████╗  ██████╗ ███████╗██╗     ███████╗███╗   ██╗███████╗
  ██╔════╝██╔══██╗██╔════╝ ██╔════╝██║     ██╔════╝████╗  ██║██╔════╝
  █████╗  ██║  ██║██║  ███╗█████╗  ██║     █████╗  ██╔██╗ ██║███████╗
  ██╔══╝  ██║  ██║██║   ██║██╔══╝  ██║     ██╔══╝  ██║╚██╗██║╚════██║
  ███████╗██████╔╝╚██████╔╝███████╗███████╗███████╗██║ ╚████║███████║
  ╚══════╝╚═════╝  ╚═════╝ ╚══════╝╚══════╝╚══════╝╚═╝  ╚═══╝╚══════╝
  RFC 9111 HTTP Caching Engine & CDN Edge Invalidation Workbench
`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]
	switch command {
	case "serve":
		cmdServe(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "purge":
		cmdPurge(os.Args[2:])
	case "stampede":
		cmdStampede(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("EdgeLens v1.0.0 (RFC 9111 HTTP Caching Engine)")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command '%s'\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(banner)
	fmt.Println("Usage: edgelens <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve      Start the EdgeLens proxy, mock origin, and visual studio")
	fmt.Println("  inspect    Audit an HTTP endpoint and view RFC 9111 decision tree")
	fmt.Println("  purge      Invalidate cache entries by surrogate tag or URL")
	fmt.Println("  stampede   Run a concurrent thundering herd benchmark against an endpoint")
	fmt.Println("  version    Display EdgeLens version")
	fmt.Println()
	fmt.Println("Run 'edgelens <command> --help' for details on each command.")
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "Port for EdgeLens proxy and web studio")
	origin := fs.String("origin", "", "Default upstream origin URL (defaults to embedded mock origin)")
	cdnFlavor := fs.String("cdn", "cloudflare", "Active CDN profile: cloudflare | fastly | cloudfront | vercel | rfc9111")
	_ = fs.Parse(args)

	fmt.Print(banner)

	store := cache.NewStore()
	index := surrogate.NewIndex()
	purgeEngine := surrogate.NewPurgeEngine(store, index)
	mockOrigin := mockorigin.NewServer()
	broker := server.NewSSEBroker()

	cdnCfg := cdn.DefaultConfig()
	cdnCfg.ActiveFlavor = cdn.Flavor(strings.ToLower(*cdnFlavor))

	p := proxy.NewProxy(proxy.Config{
		DefaultOrigin: *origin,
		CDNConfig:     cdnCfg,
	}, store, index, purgeEngine)

	p.AddListener(func(exp *explainer.Explanation) {
		broker.BroadcastExplanation(exp)
	})

	apiHandler := server.NewAPIHandler(store, index, purgeEngine, p, mockOrigin, broker)

	// Top level multiplexer
	mux := http.NewServeMux()

	// Register API endpoints
	apiHandler.RegisterRoutes(mux)

	// Register Mock Origin routes
	mux.Handle("/mock/", mockOrigin)

	// Register Edge Proxy routes
	mux.Handle("/proxy", p)
	mux.Handle("/proxy/", p)

	// Register Embedded React SPA Static Assets
	spaFS := web.GetFileSystem()
	fileServer := http.FileServer(spaFS)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/mock/") || strings.HasPrefix(r.URL.Path, "/proxy") {
			return
		}
		// Try file, fallback to index.html for SPA client-side routing
		f, err := spaFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		indexFile, err := spaFS.Open("index.html")
		if err == nil {
			defer indexFile.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, indexFile)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("✓ EdgeLens Server running on http://localhost:%d\n", *port)
	fmt.Printf("✓ Visual Workbench Studio: http://localhost:%d\n", *port)
	fmt.Printf("✓ Edge Reverse Proxy:    http://localhost:%d/proxy?url=<target>\n", *port)
	fmt.Printf("✓ Embedded Mock Origin:  http://localhost:%d/mock/scenarios/...\n", *port)
	fmt.Printf("✓ CDN Profile Emulation: %s\n\n", cdnCfg.ActiveFlavor)
	fmt.Println("Press Ctrl+C to stop.")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdInspect(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: edgelens inspect <url>")
		os.Exit(1)
	}
	targetURL := args[0]
	fmt.Printf("Auditing headers for: %s\n\n", targetURL)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	resp, err := client.Do(req)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		fmt.Printf("Error fetching URL: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	report := explainer.AuditHeaders(resp.Header)

	fmt.Printf("HTTP Status:   %d %s (Latency: %dms)\n", resp.StatusCode, http.StatusText(resp.StatusCode), dur)
	fmt.Printf("Cache Grade:   %s (Score: %d/100)\n\n", report.Grade, report.Score)

	fmt.Println("Relevant Cache Headers:")
	fmt.Printf("  Cache-Control: %s\n", resp.Header.Get("Cache-Control"))
	fmt.Printf("  Vary:          %s\n", resp.Header.Get("Vary"))
	fmt.Printf("  ETag:          %s\n", resp.Header.Get("ETag"))
	fmt.Printf("  Last-Modified: %s\n", resp.Header.Get("Last-Modified"))
	fmt.Printf("  Surrogate-Key: %s\n", resp.Header.Get("Surrogate-Key"))
	fmt.Printf("  Age:           %s\n\n", resp.Header.Get("Age"))

	fmt.Println("Audit Findings:")
	if len(report.Findings) == 0 {
		fmt.Println("  ✓ No caching issues identified.")
	} else {
		for _, f := range report.Findings {
			fmt.Printf("  [%s] %s: %s\n", strings.ToUpper(f.Severity), f.Title, f.Description)
			fmt.Printf("         Suggestion: %s\n", f.Suggestion)
		}
	}
}

func cmdPurge(args []string) {
	fs := flag.NewFlagSet("purge", flag.ExitOnError)
	tag := fs.String("tag", "", "Surrogate tag to purge")
	rawURL := fs.String("url", "", "URL to purge")
	all := fs.Bool("all", false, "Purge all cache entries")
	soft := fs.Bool("soft", false, "Perform soft purge (marks stale instead of hard delete)")
	apiURL := fs.String("api-url", "http://localhost:8080", "EdgeLens server API base URL")
	_ = fs.Parse(args)

	payload := map[string]interface{}{
		"isSoft": *soft,
	}

	if *all {
		payload["type"] = "all"
		payload["target"] = "*"
	} else if *tag != "" {
		payload["type"] = "tag"
		payload["target"] = *tag
	} else if *rawURL != "" {
		payload["type"] = "url"
		payload["target"] = *rawURL
	} else {
		fmt.Println("Specify --tag <tag>, --url <url>, or --all")
		os.Exit(1)
	}

	bodyBytes, _ := json.Marshal(payload)
	resp, err := http.Post(*apiURL+"/api/cache/purge", "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		fmt.Printf("Failed to contact EdgeLens server at %s: %v\n", *apiURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result surrogate.PurgeResult
	_ = json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("✓ Purge Completed (Type: %s, Soft: %v)\n", result.Type, result.IsSoft)
	fmt.Printf("  Target:        %s\n", result.Target)
	fmt.Printf("  Affected Keys: %d\n", result.AffectedCount)
	fmt.Printf("  Duration:      %dms\n", result.DurationMs)
}

func cmdStampede(args []string) {
	fs := flag.NewFlagSet("stampede", flag.ExitOnError)
	targetURL := fs.String("url", "http://localhost:8080/mock/scenarios/flash-sale", "Target URL to stress")
	concurrency := fs.Int("c", 100, "Number of concurrent requests")
	coalesce := fs.Bool("coalesce", true, "Enable single-flight request collapsing")
	_ = fs.Parse(args)

	fmt.Printf("Running Stampede Simulation on: %s\n", *targetURL)
	fmt.Printf("Concurrency: %d, Request Collapsing: %v\n\n", *concurrency, *coalesce)

	client := &http.Client{Timeout: 15 * time.Second}
	report, err := stampede.RunSimulation(context.Background(), client, stampede.SimulatorOptions{
		TargetURL:        *targetURL,
		Concurrency:      *concurrency,
		EnableCoalescing: *coalesce,
	})
	if err != nil {
		fmt.Printf("Simulation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Stampede Completed in %dms\n", report.TotalDurationMs)
	fmt.Printf("  Origin Hits (Penetration): %d / %d (%.1f%%)\n", report.OriginHits, report.Concurrency, report.PenetrationRate)
	fmt.Printf("  Edge Hits (Absorbed):      %d\n", report.EdgeHits)
	fmt.Printf("  Resilience Score:          %d / 100\n\n", report.ResilienceScore)

	fmt.Println("Latency Distribution:")
	fmt.Printf("  Min: %dms | P50: %dms | P90: %dms | P95: %dms | P99: %dms | Max: %dms\n\n",
		report.Percentiles.MinMs, report.Percentiles.P50Ms, report.Percentiles.P90Ms,
		report.Percentiles.P95Ms, report.Percentiles.P99Ms, report.Percentiles.MaxMs)

	fmt.Printf("Summary Note: %s\n", report.RecommendationNotes)
}
