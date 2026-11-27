// Command wsfuzz sends hostile frames to the realtime gateway (F132-AC-18, SECURITY_ARCHITECTURE §16): malformed
// envelopes, unknown/server-only types, frames above ws_max_frame, a flood above ws_frame_rate. The gateway must
// answer system.command.failed, close with a protocol code (4400, 4429, 1008, 1009…) or drop the frame — and keep
// accepting new connections (liveness after every category). With -metrics (realtime /metrics), the
// eduworld_ws_connections gauge must come back to its value before the run (no connection leak).
//
//	EW_ENV=staging EW_BASE_URL=https://api.staging.<domain>/api/v1 EW_WSFUZZ_USERNAME=hs.qa.fuzz \
//	EW_WSFUZZ_PASSWORD_FILE=/run/secrets/wsfuzz go run ./test/security/wsfuzz -out tmp/security
//
// The student is a synthetic QA account (TEST_STRATEGY §4). Exit 1 on any failed frame, crash or leak.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string) error {
	fs := flag.NewFlagSet("wsfuzz", flag.ContinueOnError)
	maxFrame := fs.Int("max-frame", 16*1024, "ws_max_frame of the gateway (bytes)")
	flood := fs.Int("flood", 200, "frames of the flood case (above the ws_frame_rate burst)")
	window := fs.Duration("window", 3*time.Second, "time to wait for the answer of one frame")
	metrics := fs.String("metrics", getenv("EW_WSFUZZ_METRICS_URL"), "Prometheus text endpoint of realtime (leak check)")
	tolerance := fs.Float64("leak-tolerance", 2, "accepted difference of eduworld_ws_connections (other users)")
	origin := fs.String("origin", getenv("EW_WSFUZZ_ORIGIN"), "Origin header (an origin the gateway allows)")
	out := fs.String("out", "../../tmp/security", "report directory (default: tmp/security of the repository, git-ignored)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	env, err := envguard.FromEnv()
	if err != nil {
		return err
	}
	base := strings.TrimRight(getenv("EW_BASE_URL"), "/")
	if base == "" {
		return fmt.Errorf("wsfuzz: EW_BASE_URL is required")
	}
	api := &client{base: base, http: &http.Client{Timeout: 15 * time.Second}, device: uuid.NewString(), clientVersion: or(getenv("EW_WSFUZZ_CLIENT_VERSION"), "1.0.0")}
	if err := api.authenticate(ctx, getenv); err != nil {
		return err
	}
	// The first ticket also tells the gateway URL when EW_WS_URL is not set (RealtimeTicket.url).
	pending, url, err := api.ticket(ctx)
	if err != nil {
		return err
	}
	wsURL := or(getenv("EW_WS_URL"), url)
	dial := TicketDialer(wsURL, *origin, func(ctx context.Context) (string, error) {
		if pending != "" {
			t := pending
			pending = ""
			return t, nil
		}
		t, _, err := api.ticket(ctx)
		return t, err
	})
	var before float64
	if *metrics != "" {
		if before, err = gauge(ctx, *metrics, "eduworld_ws_connections"); err != nil {
			return err
		}
	}
	rep := Run(ctx, dial, Corpus(*maxFrame, *flood), *window)
	rep.Env, rep.Target = env, hostOf(wsURL)
	if *metrics != "" && !rep.Crashed {
		time.Sleep(10 * time.Second) // let the gateway reap the closed connections
		after, err := gauge(ctx, *metrics, "eduworld_ws_connections")
		if err != nil {
			return err
		}
		d := after - before
		rep.LeakDelta = &d
		if d > *tolerance {
			rep.Failures++
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "wsfuzz.json"), b, 0o644); err != nil {
		return err
	}
	fmt.Printf("wsfuzz (%s): %d frames, %d failures, crashed=%v, liveness=%v\n", env, len(rep.Outcomes), rep.Failures, rep.Crashed, rep.Liveness)
	if rep.Failures > 0 || rep.Crashed {
		return fmt.Errorf("wsfuzz: %d failure(s), crashed=%v", rep.Failures, rep.Crashed)
	}
	return nil
}

type client struct {
	base, token, tenant, session, device, clientVersion string
	http                                                *http.Client
}

func (c *client) post(ctx context.Context, path string, hdr map[string]string, body any, out any) error {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", c.device)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.tenant != "" {
		req.Header.Set("X-Tenant-Id", c.tenant)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("wsfuzz: POST %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

// authenticate uses EW_WSFUZZ_TOKEN + EW_WSFUZZ_TENANT_ID, or logs in with EW_WSFUZZ_USERNAME and the password of
// EW_WSFUZZ_PASSWORD_FILE through /auth/game/login (never a password in an argument).
func (c *client) authenticate(ctx context.Context, getenv func(string) string) error {
	if t := getenv("EW_WSFUZZ_TOKEN"); t != "" {
		c.token, c.tenant = t, getenv("EW_WSFUZZ_TENANT_ID")
		return nil
	}
	user, pwFile := getenv("EW_WSFUZZ_USERNAME"), getenv("EW_WSFUZZ_PASSWORD_FILE")
	if user == "" || pwFile == "" {
		return fmt.Errorf("wsfuzz: EW_WSFUZZ_TOKEN or EW_WSFUZZ_USERNAME + EW_WSFUZZ_PASSWORD_FILE is required")
	}
	pw, err := os.ReadFile(pwFile)
	if err != nil {
		return fmt.Errorf("wsfuzz: read password file: %w", err)
	}
	var out struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
		Bootstrap struct {
			TenantID string `json:"tenant_id"`
		} `json:"bootstrap"`
	}
	if err := c.post(ctx, "/auth/game/login", nil, map[string]string{"username": user, "password": strings.TrimSpace(string(pw)),
		"device_id": c.device, "client_version": c.clientVersion, "platform": "web"}, &out); err != nil {
		return err
	}
	c.token, c.tenant = out.Tokens.AccessToken, out.Bootstrap.TenantID
	return nil
}

// ticket creates the game session once (takeover of a previous one) and a single-use realtime ticket.
func (c *client) ticket(ctx context.Context) (ticket, url string, err error) {
	if c.session == "" {
		key, _ := uuid.NewV7()
		var s struct {
			ID string `json:"game_session_id"`
		}
		if err := c.post(ctx, "/game/sessions", map[string]string{"Idempotency-Key": key.String()},
			map[string]any{"client_platform": "web", "client_version": c.clientVersion, "takeover": true}, &s); err != nil {
			return "", "", err
		}
		c.session = s.ID
	}
	var t struct {
		Ticket string `json:"ticket"`
		URL    string `json:"url"`
	}
	if err := c.post(ctx, "/game/sessions/"+c.session+"/realtime-tickets", nil, map[string]any{}, &t); err != nil {
		return "", "", err
	}
	return t.Ticket, t.URL, nil
}

// gauge sums the samples of metric in a Prometheus text exposition.
func gauge(ctx context.Context, url, metric string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return sumMetric(resp.Body, metric)
}

func sumMetric(r io.Reader, metric string) (float64, error) {
	sum, found := 0.0, false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		l := sc.Text()
		if !strings.HasPrefix(l, metric) || (len(l) > len(metric) && l[len(metric)] != ' ' && l[len(metric)] != '{') {
			continue
		}
		f := strings.Fields(l)
		v, err := strconv.ParseFloat(f[len(f)-1], 64)
		if err != nil {
			continue
		}
		sum, found = sum+v, true
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("wsfuzz: metric %s not exposed", metric)
	}
	return sum, nil
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func hostOf(u string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(u, "wss://"), "ws://")
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	return s
}
