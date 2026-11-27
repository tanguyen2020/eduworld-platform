// Command loadbot drives realtime load like real game clients (F132 US1, TEST_STRATEGY §8.1, TC-PERF-003/004/010/
// 011/013/014/016): every bot logs in like the game (gameLogin → createGameSession → createRealtimeTicket, honouring
// the login queue with Retry-After + jitter), opens the WebSocket, joins the hub and then plays its role of the mix:
// hub walker (default 70%), match player (20%) or learner (10%). Bots come from bots.json of datagen (one device_id
// each); the password is read from the environment variable named by -password-env (a secret of GitHub Environment
// staging, never a flag, never logged). Metrics eduworld_loadbot_* are served on -admin; a JSON summary is written at
// the end. It refuses to run unless EW_ENV is local, dev or staging (F132-AC-07).
//
//	EW_ENV=staging EW_LOADBOT_PASSWORD=… loadbot -base-url https://staging.example/api/v1 -bots bots.json \
//	  -scenario ccu -bots-max 4000 -ramp 10m -duration 60m -activity-id <uuid> -subject-id <uuid> -summary out.json
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/login"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/chat"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

// Bot is one entry of bots.json (datagen).
type Bot struct {
	Username string `json:"username"`
	DeviceID string `json:"device_id"`
}

// Mix is the share of each role (TEST_STRATEGY §8.1 / TC-PERF-014: 70% hub, 20% match/co-op, 10% learning).
type Mix struct{ Hub, Match, Learning float64 }

// config of a run.
type config struct {
	BaseURL       string
	BotsFile      string
	PasswordEnv   string
	Scenario      string
	Max           int
	Ramp          time.Duration
	Duration      time.Duration
	Mix           Mix
	Area          string
	ActivityID    string
	SubjectID     string
	Mode          string
	ChatInterval  time.Duration
	ThinkTime     time.Duration
	BattleEvery   time.Duration
	ReadEvery     time.Duration
	LeaderboardID string
	ClientVersion string
	Platform      string
	Admin         string
	Summary       string
	Seed          uint64
}

func main() {
	cfg, err := parse(os.Args[1:])
	if err == nil {
		_, err = envguard.FromEnv()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loadbot:", err)
		if errors.Is(err, envguard.ErrRefused) {
			os.Exit(3)
		}
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, os.Getenv(cfg.PasswordEnv)); err != nil {
		fmt.Fprintln(os.Stderr, "loadbot:", err)
		os.Exit(1)
	}
}

// presets are the scenarios of tests/load/scenarios.yaml served by loadbot; flags given explicitly win.
var presets = map[string]Mix{
	"ccu":        {Hub: 0.7, Match: 0.2, Learning: 0.1}, // TC-PERF-014
	"hub":        {Hub: 1},                              // TC-PERF-003
	"battle":     {Match: 1},                            // TC-PERF-004 matchmaking + battle
	"attempt":    {Learning: 1},                         // TC-PERF-013 learning side
	"chat":       {Hub: 1},                              // TC-PERF-010 (hub walkers that chat every -chat-interval)
	"chat_burst": {Hub: 1},                              // TC-PERF-011
	"login":      {},                                    // TC-PERF-016: log in, open the WebSocket, stay connected
}

func parse(args []string) (config, error) {
	fs := flag.NewFlagSet("loadbot", flag.ContinueOnError)
	var c config
	var mix string
	fs.StringVar(&c.BaseURL, "base-url", os.Getenv("EW_LOADBOT_BASE_URL"), "public API base URL (…/api/v1)")
	fs.StringVar(&c.BotsFile, "bots", "bots.json", "bots.json of datagen")
	fs.StringVar(&c.PasswordEnv, "password-env", "EW_LOADBOT_PASSWORD", "environment variable holding the bot password")
	fs.StringVar(&c.Scenario, "scenario", "ccu", "ccu|hub|battle|attempt|chat|chat_burst|login")
	fs.IntVar(&c.Max, "bots-max", 0, "number of bots (0 = every bot of the file)")
	fs.DurationVar(&c.Ramp, "ramp", time.Minute, "spread of the bot starts")
	fs.DurationVar(&c.Duration, "duration", 5*time.Minute, "how long each bot stays after its start")
	fs.StringVar(&mix, "mix", "", "role shares hub=0.7,match=0.2,learning=0.1 (default: preset of -scenario)")
	fs.StringVar(&c.Area, "area", "main_hub", "hub area joined by every bot")
	fs.StringVar(&c.ActivityID, "activity-id", "", "published activity of the perf tenants (learners)")
	fs.StringVar(&c.SubjectID, "subject-id", "", "subject of the matchmaking tickets (match players)")
	fs.StringVar(&c.Mode, "mode", "ranked_1v1", "matchmaking mode")
	fs.DurationVar(&c.ChatInterval, "chat-interval", 0, "one chat message per interval (0 = preset: ccu 1m, chat 10s, chat_burst 1s; hub/battle/attempt off)")
	fs.DurationVar(&c.ThinkTime, "think", 20*time.Second, "time between two answers of a learner (answer_rate ≥ 1 s/question)")
	fs.DurationVar(&c.BattleEvery, "battle-every", 15*time.Minute, "a match player starts a match this often (1 match / 2 sessions of 30 min)")
	fs.DurationVar(&c.ReadEvery, "read-every", 6*time.Minute, "profile/leaderboard read interval per bot (~5 per 30-min session; 0 = off)")
	fs.StringVar(&c.LeaderboardID, "leaderboard-id", "", "leaderboard read by the bots (getLeaderboard)")
	fs.StringVar(&c.ClientVersion, "client-version", "1.0.0", "client_version sent at login")
	fs.StringVar(&c.Platform, "platform", "web", "client platform web|android|ios")
	fs.StringVar(&c.Admin, "admin", "127.0.0.1:9300", "admin address serving /metrics and /healthz")
	fs.StringVar(&c.Summary, "summary", "", "write the JSON summary to this file (default stdout)")
	fs.Uint64Var(&c.Seed, "seed", 1, "seed of the behaviour randomness")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	p, ok := presets[c.Scenario]
	if !ok {
		return c, fmt.Errorf("unknown scenario %q", c.Scenario)
	}
	c.Mix = p
	if mix != "" {
		m, err := parseMix(mix)
		if err != nil {
			return c, err
		}
		c.Mix = m
	}
	if c.ChatInterval == 0 {
		c.ChatInterval = map[string]time.Duration{"ccu": time.Minute, "chat": 10 * time.Second, "chat_burst": time.Second}[c.Scenario]
	}
	if c.BaseURL == "" {
		return c, errors.New("-base-url (or EW_LOADBOT_BASE_URL) is required")
	}
	return c, nil
}

func parseMix(s string) (Mix, error) {
	var m Mix
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		f, err := strconv.ParseFloat(v, 64)
		if !ok || err != nil || f < 0 {
			return m, fmt.Errorf("bad mix entry %q", part)
		}
		switch k {
		case "hub":
			m.Hub = f
		case "match":
			m.Match = f
		case "learning":
			m.Learning = f
		default:
			return m, fmt.Errorf("unknown role %q", k)
		}
	}
	if t := m.Hub + m.Match + m.Learning; t < 0.999 || t > 1.001 {
		return m, fmt.Errorf("mix must sum to 1, got %.3f", t)
	}
	return m, nil
}

// roleOf assigns roles deterministically so the shares hold at every bot count.
func roleOf(i, n int, m Mix) string {
	f := (float64(i) + 0.5) / float64(n)
	switch {
	case f < m.Hub:
		return "hub"
	case f < m.Hub+m.Match:
		return "match"
	case m.Learning > 0:
		return "learning"
	}
	return "idle"
}

func loadBots(path string, max int) ([]Bot, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- operator-chosen input file
	if err != nil {
		return nil, err
	}
	var bots []Bot
	if err := json.Unmarshal(b, &bots); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if max > 0 && max < len(bots) {
		bots = bots[:max]
	}
	if len(bots) == 0 {
		return nil, fmt.Errorf("%s: no bot", path)
	}
	return bots, nil
}

func run(ctx context.Context, c config, password string) error {
	if password == "" {
		return fmt.Errorf("%s is empty: the bot password comes from the environment", c.PasswordEnv)
	}
	bots, err := loadBots(c.BotsFile, c.Max)
	if err != nil {
		return err
	}
	m := report.New(c.Scenario)
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	ln, err := net.Listen("tcp", c.Admin)
	if err != nil {
		return err
	}
	admin := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = admin.Serve(ln) }()
	defer func() { _ = admin.Close() }()
	slog.Info("loadbot: start", "scenario", c.Scenario, "bots", len(bots), "ramp", c.Ramp, "duration", c.Duration, "admin", ln.Addr().String())

	hc := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 1024, IdleConnTimeout: 90 * time.Second}}
	started := time.Now()
	var wg sync.WaitGroup
	for i, b := range bots {
		delay := time.Duration(0)
		if len(bots) > 1 {
			delay = c.Ramp * time.Duration(i) / time.Duration(len(bots)-1)
		}
		wg.Add(1)
		go func(i int, b Bot) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			bctx, cancel := context.WithTimeout(ctx, c.Duration)
			defer cancel()
			runBot(bctx, c, hc, m, b, password, roleOf(i, len(bots), c.Mix), rand.New(rand.NewPCG(c.Seed, uint64(i)))) // #nosec G404 -- behaviour randomness
		}(i, b)
	}
	wg.Wait()
	s := m.Summary(started, time.Now())
	out := os.Stdout
	if c.Summary != "" {
		f, err := os.Create(c.Summary)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		out = f
	}
	return s.WriteJSON(out)
}

func runBot(ctx context.Context, c config, hc *http.Client, m *report.Metrics, b Bot, password, role string, r *rand.Rand) {
	lc := login.Config{BaseURL: c.BaseURL, ClientVersion: c.ClientVersion, Platform: c.Platform}
	s, err := login.Login(ctx, hc, lc, login.Credentials{Username: b.Username, Password: password, DeviceID: b.DeviceID})
	m.LoginRetries(s.QueueBusy, s.RateLimited, s.ServerErrors)
	if err != nil {
		if ctx.Err() == nil {
			m.Login("failed", 0)
		}
		return
	}
	m.Login("ok", s.UntilIn)
	m.HTTP("gameLogin", http.StatusOK, s.LoginLatency)
	rc := &restcall.Client{HTTP: hc, BaseURL: c.BaseURL, Token: s.AccessToken, TenantID: s.TenantID, Metrics: m}
	for ctx.Err() == nil {
		conn, err := wsclient.Dial(ctx, s.TicketURL, s.Ticket)
		if err != nil {
			m.Error("ws_dial")
			return
		}
		m.Connected(1)
		playRole(ctx, c, rc, conn, m, role, r)
		m.Connected(-1)
		_ = conn.Close()
		if ctx.Err() != nil {
			return
		}
		m.Error("ws_reconnect") // dropped by the server: new single-use ticket, then reconnect like the client
		if err := login.RefreshTicket(ctx, hc, lc, &s); err != nil {
			m.Error("ticket_refresh")
			return
		}
	}
}

// reads are the profile/leaderboard reads of a session (~5 per 30 min, TEST_STRATEGY §8.1).
func reads(ctx context.Context, c config, rc *restcall.Client, r *rand.Rand) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(float64(c.ReadEvery) * (0.5 + r.Float64()))):
		}
		if c.LeaderboardID != "" && r.IntN(2) == 0 {
			_, _, _ = rc.Do(ctx, "getLeaderboard", http.MethodGet, "/leaderboards/"+c.LeaderboardID+"?limit=20", nil, "")
			continue
		}
		_, _, _ = rc.Do(ctx, "getGameProfile", http.MethodGet, "/game/profile", nil, "")
	}
}

func playRole(ctx context.Context, c config, rc *restcall.Client, conn *wsclient.Conn, m *report.Metrics, role string, r *rand.Rand) {
	if c.Scenario == "login" { // stay connected (heartbeats only) until the end of the run
		select {
		case <-ctx.Done():
		case <-conn.Done():
		}
		return
	}
	in, err := hub.Join(ctx, conn, m, c.Area, 10*time.Second)
	if err != nil {
		m.Error("hub_join")
		return
	}
	if c.ReadEvery > 0 {
		go reads(ctx, c, rc, rand.New(rand.NewPCG(r.Uint64(), 1))) // #nosec G404 -- behaviour randomness
	}
	if c.ChatInterval > 0 && role == "hub" {
		cr := rand.New(rand.NewPCG(r.Uint64(), r.Uint64())) // #nosec G404 -- behaviour randomness (own source: r is not goroutine-safe)
		go func() {
			ch, err := chat.PickChannel(ctx, rc)
			if err != nil {
				m.Error("chat_no_channel")
				return
			}
			_ = chat.Send(ctx, conn, m, ch, c.ChatInterval, cr)
		}()
	}
	switch role {
	case "match":
		for ctx.Err() == nil {
			wctx, cancel := context.WithTimeout(ctx, time.Duration(float64(c.BattleEvery)*(0.5+r.Float64())))
			_ = hub.Walk(wctx, conn, m, in, hub.Options{}, r)
			cancel()
			if ctx.Err() != nil || c.SubjectID == "" {
				continue
			}
			if _, err := battle.Play(ctx, rc, conn, m, battle.Options{Mode: c.Mode, SubjectID: c.SubjectID,
				Think: func() time.Duration { return time.Second + time.Duration(rand.Int64N(int64(4*time.Second))) }}); err != nil && ctx.Err() == nil {
				m.Error("battle")
			}
		}
	case "learning":
		wr := rand.New(rand.NewPCG(r.Uint64(), 0)) // #nosec G404 -- behaviour randomness
		go func() {
			_ = hub.Walk(ctx, conn, m, in, hub.Options{WalkShare: 0.1}, wr)
		}() // #nosec G404
		for ctx.Err() == nil && c.ActivityID != "" {
			if _, err := attempt.Run(ctx, rc, m, attempt.Options{ActivityID: c.ActivityID,
				Think: func() time.Duration { return c.ThinkTime/2 + time.Duration(r.Int64N(int64(c.ThinkTime))) }}); err != nil && ctx.Err() == nil {
				m.Error("attempt")
				time.Sleep(5 * time.Second)
			}
		}
		<-ctx.Done()
	default:
		_ = hub.Walk(ctx, conn, m, in, hub.Options{}, r)
	}
}
