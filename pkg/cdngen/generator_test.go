package cdngen

import (
	"strings"
	"testing"
)

func TestGenerateAll(t *testing.T) {
	opts := DefaultOptions()
	configs := GenerateAll(opts)

	if !strings.Contains(configs.NginxConf, "proxy_cache edgelens_zone;") {
		t.Errorf("Nginx config missing proxy_cache directive")
	}

	if !strings.Contains(configs.FastlyVCL, "beresp.stale_while_revalidate") {
		t.Errorf("Fastly VCL missing stale_while_revalidate")
	}

	if !strings.Contains(configs.CloudflareTerraform, "cloudflare_ruleset") {
		t.Errorf("Cloudflare Terraform missing cloudflare_ruleset")
	}

	if !strings.Contains(configs.CloudFrontTerraform, "aws_cloudfront_cache_policy") {
		t.Errorf("CloudFront Terraform missing aws_cloudfront_cache_policy")
	}
}
