//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring/wiringtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// F001-AC-05 edge case: worker --profile=chat checks only eduworld_chat (plus Valkey, NATS);
// --profile=events checks every module database.
func TestWorkerProfileChecks(t *testing.T) {
	ctx := context.Background()
	cluster := testkit.StartCluster(ctx, t, true)
	_, vkAddr := testkit.StartValkey(ctx, t)
	_, natsURL := testkit.StartNATS(ctx, t)

	cases := map[string][]string{
		"chat":   {"atlas:chat", "db:chat", "nats", "valkey"},
		"events": {"atlas:chat", "atlas:competition", "atlas:game", "atlas:identity", "atlas:learning", "db:chat", "db:competition", "db:game", "db:identity", "db:learning", "nats", "valkey"},
	}
	for profile, want := range cases {
		t.Run(profile, func(t *testing.T) {
			// The complete configuration of the profile (OQ F010-2: events runs the relays and consumers of every module).
			env := wiringtest.Env(wiringtest.Options{RepoRoot: testkit.RepoRoot(t), DSN: cluster.ModuleDSN, Issuer: "http://127.0.0.1:1/realms/eduworld",
				ValkeyAddr: vkAddr, NATSURL: natsURL, SecretsDir: t.TempDir()})
			if profile == "chat" { // other module databases are not even configured
				for k := range env {
					if strings.HasPrefix(k, "EW_DB_") && !strings.HasPrefix(k, "EW_DB_CHAT_") {
						delete(env, k)
					}
				}
			}
			cctx, cancel := context.WithCancel(ctx)
			addrs := make(chan string, 1)
			done := make(chan error, 1)
			go func() {
				done <- run(cctx, profile, func(k string) string { return env[k] }, func(a, _ string) { addrs <- a })
			}()
			var admin string
			select {
			case admin = <-addrs:
			case err := <-done:
				t.Fatalf("worker exited: %v", err)
			case <-time.After(30 * time.Second):
				t.Fatal("worker did not start")
			}
			defer func() { cancel(); <-done }()

			var body struct {
				Status string                    `json:"status"`
				Checks map[string]map[string]any `json:"checks"`
			}
			deadline := time.Now().Add(30 * time.Second)
			for {
				resp, err := http.Get("http://" + admin + "/readyz")
				if err != nil {
					t.Fatal(err)
				}
				_ = json.NewDecoder(resp.Body).Decode(&body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK || time.Now().After(deadline) {
					if resp.StatusCode != http.StatusOK {
						t.Fatalf("readyz = %d %+v", resp.StatusCode, body)
					}
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			got := make([]string, 0, len(body.Checks))
			for name := range body.Checks {
				got = append(got, name)
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("profile %s checks = %v, want %v", profile, got, want)
			}
		})
	}
}
