package varymatrix

import (
	"testing"
)

func TestAnalyzeVaryHeader(t *testing.T) {
	report := AnalyzeVaryHeader("Accept-Encoding, User-Agent, Accept-Language")

	if len(report.Dimensions) != 3 {
		t.Fatalf("Expected 3 dimensions, got %d", len(report.Dimensions))
	}

	if report.OriginPenetrationRisk != RiskCritical {
		t.Errorf("Expected RiskCritical due to User-Agent, got %v", report.OriginPenetrationRisk)
	}

	if report.EstimatedPermutations < 5000 {
		t.Errorf("Expected >= 5000 permutations, got %d", report.EstimatedPermutations)
	}

	if len(report.Recommendations) == 0 {
		t.Errorf("Expected recommendations, got none")
	}
}
