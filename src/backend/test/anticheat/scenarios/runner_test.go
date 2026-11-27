//go:build anticheat

// Package scenarios runs the 10 anti-cheat scenarios of ANTI_CHEAT §11 against a live environment (F132 User Story 4):
//
//	make anticheat ENV=staging   # EW_API_BASE_URL, EW_ANTICHEAT_PASSWORD, EW_ANTICHEAT_FIXTURES, [EW_PROMETHEUS_URL]
//
// The build tag keeps them out of `go test ./...` (they need a running platform and seeded QA students). TestMain
// refuses any EW_ENV other than local/dev/staging and writes the report to EW_ANTICHEAT_REPORT (Markdown + .json).
package scenarios

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/anticheat"
)

var (
	env     *anticheat.Env
	mu      sync.Mutex
	results []anticheat.Result
)

func TestMain(m *testing.M) {
	cfg, err := anticheat.LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	env = anticheat.NewEnv(cfg, nil)
	code := m.Run()
	env.Close()
	if err := writeReport(cfg.Env); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	if p, n, ok := anticheat.Summary(results); !ok {
		fmt.Fprintf(os.Stderr, "anticheat: %d/%d passed (SC-005 needs %d/%d)\n", p, n, len(anticheat.Scenarios), len(anticheat.Scenarios))
		if code == 0 && os.Getenv("EW_ANTICHEAT_PARTIAL") == "" { // a -run subset is allowed with EW_ANTICHEAT_PARTIAL=1
			code = 1
		}
	}
	os.Exit(code)
}

// run executes the scenario id as a subtest; skipped scenarios are t.Skip (and never counted as passed).
func run(t *testing.T, id string) {
	t.Helper()
	for _, s := range anticheat.Scenarios {
		if s.ID != id {
			continue
		}
		r := anticheat.RunOne(context.Background(), env, s, 10*time.Minute)
		mu.Lock()
		results = append(results, r)
		mu.Unlock()
		switch r.Status {
		case anticheat.Skipped:
			t.Skip(r.Reason)
		case anticheat.Failed:
			t.Fatalf("%s (%s): %s", s.ID, s.AC, r.Reason)
		}
		return
	}
	t.Fatalf("unknown scenario %s", id)
}

func writeReport(envName string) error {
	out := os.Getenv("EW_ANTICHEAT_REPORT")
	if out == "" {
		return nil
	}
	at := time.Now()
	if err := os.WriteFile(out, []byte(anticheat.Markdown(envName, at, results)), 0o600); err != nil {
		return err
	}
	b, err := anticheat.JSON(envName, at, results)
	if err != nil {
		return err
	}
	return os.WriteFile(strings.TrimSuffix(out, ".md")+".json", b, 0o600)
}
