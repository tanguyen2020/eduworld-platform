// Command ewlint runs the EduWorld analyzers (bypassreason, scopeguard, nointernalhttp) over packages:
//
//	go run ./tools/lint/ewlint ./...
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/lint/bypassreason"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/lint/nointernalhttp"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/lint/scopeguard"
)

func main() { multichecker.Main(bypassreason.Analyzer, scopeguard.Analyzer, nointernalhttp.Analyzer) }
