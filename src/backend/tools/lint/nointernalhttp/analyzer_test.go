package nointernalhttp_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/lint/nointernalhttp"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), nointernalhttp.Analyzer, "example.com/app")
}
