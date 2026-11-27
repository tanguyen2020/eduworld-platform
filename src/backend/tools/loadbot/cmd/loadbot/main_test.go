package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/fakert"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
)

func TestRoleSharesFollowTheMix(t *testing.T) {
	n := 1000
	got := map[string]int{}
	for i := 0; i < n; i++ {
		got[roleOf(i, n, presets["ccu"])]++
	}
	if got["hub"] != 700 || got["match"] != 200 || got["learning"] != 100 {
		t.Fatalf("roles %v, want 70/20/10 (TC-PERF-014)", got)
	}
}

func TestParse(t *testing.T) {
	if _, err := parse([]string{"-base-url", "http://x", "-mix", "hub=0.5,match=0.2"}); err == nil {
		t.Fatal("mix not summing to 1 accepted")
	}
	if _, err := parse([]string{"-base-url", "http://x", "-scenario", "nope"}); err == nil {
		t.Fatal("unknown scenario accepted")
	}
	c, err := parse([]string{"-base-url", "http://x", "-scenario", "chat_burst"})
	if err != nil || c.ChatInterval != time.Second {
		t.Fatalf("%+v %v", c, err)
	}
}

// End to end against TEST STAND-INS (fake public API + fake gateway, AGENTS R17): two bots log in through one
// LOGIN_QUEUE_BUSY, connect and stay connected in scenario login; the summary counts them.
func TestRunLoginScenarioAgainstStandIns(t *testing.T) {
	rt := fakert.New("tk")
	defer rt.Close()
	var busy atomic.Bool
	busy.Store(true)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/auth/game/login":
			if busy.CompareAndSwap(true, false) {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":"LOGIN_QUEUE_BUSY"}`))
				return
			}
			_, _ = w.Write([]byte(`{"tokens":{"access_token":"tok"},"bootstrap":{"tenant_id":"t1","student":{"grade_level":3}}}`))
		case r.URL.Path == "/api/v1/game/sessions":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"game_session_id":"g1"}`))
		case strings.HasSuffix(r.URL.Path, "/realtime-tickets"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ticket":"tk","url":"` + rt.URL() + `"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()
	dir := t.TempDir()
	botsFile := filepath.Join(dir, "bots.json")
	_ = os.WriteFile(botsFile, []byte(`[{"username":"a","device_id":"d1"},{"username":"b","device_id":"d2"}]`), 0o600)
	summary := filepath.Join(dir, "summary.json")
	c, err := parse([]string{"-base-url", api.URL + "/api/v1", "-bots", botsFile, "-scenario", "login", "-ramp", "10ms",
		"-duration", "3s", "-admin", "127.0.0.1:0", "-summary", summary})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), c, "secret"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(summary)
	var s report.Summary
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if s.Counts["login_ok"] != 2 || s.Counts["login_queue_busy"] != 1 {
		t.Fatalf("counts %v", s.Counts)
	}
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "tok") {
		t.Fatal("summary leaks a credential")
	}
}

func TestRunNeedsPassword(t *testing.T) {
	if err := run(context.Background(), config{PasswordEnv: "EW_LOADBOT_PASSWORD"}, ""); err == nil {
		t.Fatal("empty password accepted")
	}
}
