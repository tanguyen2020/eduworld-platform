package bypassreason_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/lint/bypassreason"
)

// F002-AC-30: a reason outside the closed list (ADR-0089 §3) fails lint.
func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), bypassreason.Analyzer, "example.com/app")
}
