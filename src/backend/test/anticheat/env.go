// Package anticheat is the "modified client" harness of F132 User Story 4 (ANTI_CHEAT §11, ADR-0016, AGENTS R5, R10):
// it logs in like the game, then sends what a tampered client would — duplicate submits, client scores, late answers,
// answers to questions never shown, replayed WebSocket ids, a speed hack — and checks that the server refuses or
// ignores them and records the signal without changing a learning record. It is black-box: only the public REST
// contract (specs/openapi/eduworld-api.v1.yaml) and the realtime protocol (specs/asyncapi/eduworld-realtime.v1.yaml);
// it never imports a module. The scenarios run against local/dev/staging only (tools/envguard) with the QA students of
// a fixture file; credentials come from the environment (GitHub Environment secrets on staging), never from the repo.
// Nothing here logs a token, a password or a response body (AGENTS R14).
package anticheat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/envguard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/login"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
)

// Student is one QA student of the fixture (a seeded test account, never a real student).
type Student struct {
	Username string `yaml:"username"`
	DeviceID string `yaml:"device_id"` // stable per student (login_rate is per device)
}

// Fixtures are the seeded resources of the target environment the scenarios use (EW_ANTICHEAT_FIXTURES, YAML).
type Fixtures struct {
	Primary  Student `yaml:"primary"`
	Opponent Student `yaml:"opponent"` // second student of the same tenant and grade (battle)
	// PracticeActivityID: a published practice activity with choice items that awards XP, assigned to the tenant.
	PracticeActivityID string `yaml:"practice_activity_id"`
	// ExamSessionID: an open exam session assigned to Primary with a short duration (the deadline scenario waits for
	// deadline_at + DeadlineGrace); empty = scenario skipped.
	ExamSessionID string        `yaml:"exam_session_id"`
	DeadlineGrace time.Duration `yaml:"deadline_grace"` // exam_offline_sync_grace of the environment (EXAM_ENGINE §6)
	HubArea       string        `yaml:"hub_area"`       // e.g. main_hub
	ChatChannelID string        `yaml:"chat_channel_id"`
	BattleMode    string        `yaml:"battle_mode"` // createMatchmakingTicket.mode, e.g. ranked_1v1
	BattleSubject string        `yaml:"battle_subject_id"`
	// XPDecay: XP share of the n-th completion of the same content (PROGRESSION_SYSTEM §3.2, AC-CMP-209), in percent.
	XPDecay []int `yaml:"xp_decay"`
}

// Config of a run.
type Config struct {
	Env           string // EW_ENV (envguard)
	BaseURL       string // EW_API_BASE_URL, …/api/v1
	Password      string // EW_ANTICHEAT_PASSWORD (QA students share one secret)
	PrometheusURL string // EW_PROMETHEUS_URL (optional: metric checks are skipped without it)
	ClientVersion string // EW_CLIENT_VERSION
	Fixtures      Fixtures
}

// ErrSkip marks a scenario that cannot run with the given fixtures (reported as skipped, never as passed).
var ErrSkip = errors.New("anticheat: skipped")

// LoadConfig reads the environment and the fixture file; it refuses a non-test environment (F132-AC-07).
func LoadConfig(getenv func(string) string) (Config, error) {
	c := Config{Env: getenv(envguard.Var), BaseURL: strings.TrimRight(getenv("EW_API_BASE_URL"), "/"),
		Password: getenv("EW_ANTICHEAT_PASSWORD"), PrometheusURL: strings.TrimRight(getenv("EW_PROMETHEUS_URL"), "/"),
		ClientVersion: getenv("EW_CLIENT_VERSION")}
	if err := envguard.Check(c.Env); err != nil {
		return c, err
	}
	if c.BaseURL == "" || c.Password == "" {
		return c, errors.New("anticheat: EW_API_BASE_URL and EW_ANTICHEAT_PASSWORD are required")
	}
	if c.ClientVersion == "" {
		c.ClientVersion = "1.0.0"
	}
	path := getenv("EW_ANTICHEAT_FIXTURES")
	if path == "" {
		return c, errors.New("anticheat: EW_ANTICHEAT_FIXTURES (fixture YAML) is required")
	}
	b, err := os.ReadFile(path) // #nosec G304 -- operator-supplied fixture path
	if err != nil {
		return c, fmt.Errorf("anticheat: fixtures: %w", err)
	}
	if err := yaml.Unmarshal(b, &c.Fixtures); err != nil {
		return c, fmt.Errorf("anticheat: fixtures: %w", err)
	}
	c.Fixtures.defaults()
	return c, nil
}

func (f *Fixtures) defaults() {
	if f.HubArea == "" {
		f.HubArea = "main_hub"
	}
	if f.DeadlineGrace <= 0 {
		f.DeadlineGrace = 30 * time.Second
	}
	if len(f.XPDecay) == 0 {
		f.XPDecay = []int{100, 50, 25, 0, 0} // AC-CMP-209
	}
	if f.BattleMode == "" {
		f.BattleMode = "ranked_1v1"
	}
}

// Player is one logged-in QA student: REST identity and, once connected, its realtime connection.
type Player struct {
	Session login.Session
	HTTP    *http.Client
	BaseURL string
	WS      *wsclient.Conn
	cfg     login.Config
	cred    login.Credentials
}

// Env is what a scenario receives.
type Env struct {
	Cfg     Config
	HTTP    *http.Client
	players map[string]*Player
	battle  *battleOutcome // one shared match for AC-CMP-205, 206, 207
	Probe   MetricProbe
	Sleep   func(d time.Duration) // waits (tests replace it)
}

// MetricProbe reads one PromQL scalar (nil = metric checks skipped).
type MetricProbe func(ctx context.Context, query string) (float64, error)

// NewEnv prepares a run (players log in lazily).
func NewEnv(c Config, hc *http.Client) *Env {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	e := &Env{Cfg: c, HTTP: hc, players: map[string]*Player{}, Sleep: func(d time.Duration) { time.Sleep(d) }}
	if c.PrometheusURL != "" {
		e.Probe = PrometheusProbe(hc, c.PrometheusURL)
	}
	return e
}

// Player logs in the fixture student name ("primary" or "opponent") once.
func (e *Env) Player(ctx context.Context, name string) (*Player, error) {
	if p, ok := e.players[name]; ok {
		return p, nil
	}
	st := e.Cfg.Fixtures.Primary
	if name == "opponent" {
		st = e.Cfg.Fixtures.Opponent
	}
	if st.Username == "" {
		return nil, fmt.Errorf("%w: fixture student %q missing", ErrSkip, name)
	}
	if st.DeviceID == "" {
		st.DeviceID = ids.V4()
	}
	lc := login.Config{BaseURL: e.Cfg.BaseURL, ClientVersion: e.Cfg.ClientVersion, MaxAttempts: 10}
	cr := login.Credentials{Username: st.Username, Password: e.Cfg.Password, DeviceID: st.DeviceID}
	s, err := login.Login(ctx, e.HTTP, lc, cr)
	if err != nil {
		return nil, fmt.Errorf("anticheat: login %s: %w", name, err)
	}
	p := &Player{Session: s, HTTP: e.HTTP, BaseURL: e.Cfg.BaseURL, cfg: lc, cred: cr}
	e.players[name] = p
	return p, nil
}

// Connect opens (or reuses) the realtime connection of the player.
func (p *Player) Connect(ctx context.Context) (*wsclient.Conn, error) {
	if p.WS != nil {
		select {
		case <-p.WS.Done():
		default:
			return p.WS, nil
		}
		if err := login.RefreshTicket(ctx, p.HTTP, p.cfg, &p.Session); err != nil {
			return nil, err
		}
	}
	c, err := wsclient.Dial(ctx, p.Session.TicketURL, p.Session.Ticket)
	if err != nil {
		return nil, err
	}
	p.WS = c
	return c, nil
}

// Close ends every realtime connection.
func (e *Env) Close() {
	for _, p := range e.players {
		if p.WS != nil {
			_ = p.WS.Close()
		}
	}
}

// Response of a REST call (the body is kept in memory for the checks, never logged).
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Code is the Problem Details code of an error response ("" when absent).
func (r Response) Code() string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.Body, &p)
	return p.Code
}

// Do calls the public API as the player; writes carry key as Idempotency-Key (a fresh UUIDv7 when empty).
func (p *Player) Do(ctx context.Context, method, path string, body any, key string) (Response, error) {
	var rd io.Reader = http.NoBody
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return Response{}, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.BaseURL+path, rd)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.Session.AccessToken)
	req.Header.Set("X-Tenant-Id", p.Session.TenantID)
	req.Header.Set("Accept-Language", "vi")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		if key == "" {
			key = ids.V7(time.Now())
		}
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("anticheat: %s %s: %w", method, redactPath(path), err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: b}, err
}

// redactPath keeps the route shape of a path for error messages (ids are not PII, but reports stay generic).
func redactPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if len(s) >= 32 {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

// PrometheusProbe queries an instant vector sum through the Prometheus HTTP API.
func PrometheusProbe(hc *http.Client, base string) MetricProbe {
	return func(ctx context.Context, q string) (float64, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/query?query="+url.QueryEscape(q), nil)
		if err != nil {
			return 0, err
		}
		resp, err := hc.Do(req)
		if err != nil {
			return 0, err
		}
		defer func() { _ = resp.Body.Close() }()
		var r struct {
			Data struct {
				Result []struct {
					Value [2]any `json:"value"`
				} `json:"result"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return 0, err
		}
		sum := 0.0
		for _, s := range r.Data.Result {
			var v float64
			if str, ok := s.Value[1].(string); ok {
				_, _ = fmt.Sscan(str, &v)
			}
			sum += v
		}
		return sum, nil
	}
}
