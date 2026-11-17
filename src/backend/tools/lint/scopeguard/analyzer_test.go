package scopeguard_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/lint/scopeguard"
)

// F002-AC-11: only package authz creates scopes; authz itself is not reported.
func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), scopeguard.Analyzer, "example.com/app",
		"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz")
}
