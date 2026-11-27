// Package login is the login state machine of a load bot and of the anti-cheat runner (F132-AC-03, TC-PERF-016, FR-AUTH-027): it behaves like
// the game client (F061) — gameLogin → createGameSession → createRealtimeTicket — and on overload it honours the
// login queue: 429 LOGIN_QUEUE_BUSY waits Retry-After plus a random jitter, never a tight retry; 429 RATE_LIMITED
// (per device/username, login_rate) is counted apart because it means two bots share a device_id; 5xx are counted
// (must stay 0) and retried with a growing backoff. Tokens and passwords are never logged (AGENTS R14).
package login

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
)

var (
	// ErrGaveUp is returned after MaxAttempts logins without success.
	ErrGaveUp = errors.New("login: gave up")
	// ErrRejected is a terminal answer (401, 403, 422, 426): retrying cannot help.
	ErrRejected = errors.New("login: rejected")
)

// Doer sends HTTP requests (*http.Client).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Config tunes the state machine. Zero values get the defaults of New.
type Config struct {
	BaseURL       string // …/api/v1
	ClientVersion string
	Platform      string // web | android | ios (client_platform)
	MaxAttempts   int
	MaxWait       time.Duration // cap of one wait (an absurd Retry-After never parks a bot forever)
	JitterMax     time.Duration // random extra wait added to Retry-After, like the game client
	Jitter        func(max time.Duration) time.Duration
	Sleep         func(ctx context.Context, d time.Duration) error
	Now           func() time.Time
}

// Credentials of one bot; DeviceID is unique per bot (datagen).
type Credentials struct {
	Username string
	Password string
	DeviceID string
}

// Session is the outcome of a successful login, with the counters of the attempts.
type Session struct {
	AccessToken   string
	TenantID      string
	StudentID     string
	GradeLevel    int
	GameSessionID string
	Ticket        string
	TicketURL     string
	Attempts      int
	QueueBusy     int
	RateLimited   int
	ServerErrors  int
	Takeover      bool
	UntilIn       time.Duration // first gameLogin → realtime ticket (login_slo "đăng nhập → vào hub")
	LoginLatency  time.Duration // last (successful) gameLogin call
}

func (c *Config) defaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 30
	}
	if c.MaxWait <= 0 {
		c.MaxWait = 2 * time.Minute
	}
	if c.JitterMax <= 0 {
		c.JitterMax = time.Second
	}
	if c.Jitter == nil {
		c.Jitter = func(max time.Duration) time.Duration { return time.Duration(rand.Int64N(int64(max) + 1)) } // #nosec G404 -- jitter, not a secret
	}
	if c.Sleep == nil {
		c.Sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Platform == "" {
		c.Platform = "web"
	}
}

type problemBody struct {
	Code string `json:"code"`
}

type loginResponse struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
	Bootstrap struct {
		TenantID string `json:"tenant_id"`
		Student  struct {
			StudentID  string `json:"student_id"`
			GradeLevel int    `json:"grade_level"`
		} `json:"student"`
	} `json:"bootstrap"`
}

// Login runs the whole sequence for one bot.
func Login(ctx context.Context, hc Doer, c Config, cr Credentials) (Session, error) {
	c.defaults()
	var s Session
	start := c.Now()
	backoff := time.Second
	for {
		if s.Attempts >= c.MaxAttempts {
			return s, fmt.Errorf("%w after %d attempts", ErrGaveUp, s.Attempts)
		}
		s.Attempts++
		t0 := c.Now()
		status, hdr, body, err := post(ctx, hc, c.BaseURL+"/auth/game/login", nil, map[string]any{"username": cr.Username, "password": cr.Password,
			"device_id": cr.DeviceID, "client_version": c.ClientVersion, "platform": c.Platform})
		if err != nil {
			if ctx.Err() != nil {
				return s, ctx.Err()
			}
			s.ServerErrors++ // transport error: same treatment as a 5xx
			if err := c.Sleep(ctx, c.Jitter(backoff)+backoff); err != nil {
				return s, err
			}
			backoff = min(backoff*2, c.MaxWait)
			continue
		}
		switch {
		case status == http.StatusOK:
			var lr loginResponse
			if err := json.Unmarshal(body, &lr); err != nil || lr.Tokens.AccessToken == "" || lr.Bootstrap.TenantID == "" {
				return s, fmt.Errorf("login: unexpected body: %w", err)
			}
			s.LoginLatency = c.Now().Sub(t0)
			s.AccessToken, s.TenantID = lr.Tokens.AccessToken, lr.Bootstrap.TenantID
			s.StudentID, s.GradeLevel = lr.Bootstrap.Student.StudentID, lr.Bootstrap.Student.GradeLevel
			if err := gameSession(ctx, hc, c, cr, &s); err != nil {
				return s, err
			}
			s.UntilIn = c.Now().Sub(start)
			return s, nil
		case status == http.StatusTooManyRequests:
			var p problemBody
			_ = json.Unmarshal(body, &p)
			if p.Code == "RATE_LIMITED" {
				s.RateLimited++
			} else {
				s.QueueBusy++ // LOGIN_QUEUE_BUSY (or an untyped 429 of the gateway)
			}
			if err := c.Sleep(ctx, c.wait(hdr)); err != nil {
				return s, err
			}
		case status >= 500:
			s.ServerErrors++
			if err := c.Sleep(ctx, backoff+c.Jitter(backoff)); err != nil {
				return s, err
			}
			backoff = min(backoff*2, c.MaxWait)
		default:
			return s, fmt.Errorf("%w: gameLogin %d", ErrRejected, status)
		}
	}
}

// wait is Retry-After (seconds; missing, invalid or < 1 → 1 s; capped at MaxWait) plus jitter.
func (c *Config) wait(h http.Header) time.Duration {
	d := time.Second
	if n, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After"))); err == nil && n >= 1 {
		d = time.Duration(n) * time.Second
	}
	return min(d, c.MaxWait) + c.Jitter(c.JitterMax)
}

func gameSession(ctx context.Context, hc Doer, c Config, cr Credentials, s *Session) error {
	auth := map[string]string{"Authorization": "Bearer " + s.AccessToken, "X-Tenant-Id": s.TenantID}
	for i := 0; ; i++ {
		h := map[string]string{"Idempotency-Key": ids.V7(c.Now()), "X-Device-Id": cr.DeviceID}
		for k, v := range auth {
			h[k] = v
		}
		body := map[string]any{"client_platform": c.Platform, "client_version": c.ClientVersion}
		if s.Takeover {
			body["takeover"] = true
		}
		status, _, b, err := post(ctx, hc, c.BaseURL+"/game/sessions", h, body)
		if err != nil {
			return fmt.Errorf("createGameSession: %w", err)
		}
		if status == http.StatusConflict && i == 0 {
			s.Takeover = true // another active session of this account (a previous run): take it over, like the game asks
			continue
		}
		if status != http.StatusCreated {
			return fmt.Errorf("createGameSession: %d", status)
		}
		var gs struct {
			ID string `json:"game_session_id"`
		}
		if err := json.Unmarshal(b, &gs); err != nil || gs.ID == "" {
			return fmt.Errorf("createGameSession: unexpected body: %w", err)
		}
		s.GameSessionID = gs.ID
		break
	}
	status, _, b, err := post(ctx, hc, c.BaseURL+"/game/sessions/"+s.GameSessionID+"/realtime-tickets", auth, nil)
	if err != nil {
		return fmt.Errorf("createRealtimeTicket: %w", err)
	}
	if status != http.StatusCreated {
		return fmt.Errorf("createRealtimeTicket: %d", status)
	}
	var tk struct {
		Ticket string `json:"ticket"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(b, &tk); err != nil || tk.Ticket == "" {
		return fmt.Errorf("createRealtimeTicket: unexpected body: %w", err)
	}
	s.Ticket, s.TicketURL = tk.Ticket, tk.URL
	return nil
}

// RefreshTicket asks a new single-use realtime ticket (reconnect after a drop, TTL 60 s).
func RefreshTicket(ctx context.Context, hc Doer, c Config, s *Session) error {
	c.defaults()
	auth := map[string]string{"Authorization": "Bearer " + s.AccessToken, "X-Tenant-Id": s.TenantID}
	status, _, b, err := post(ctx, hc, c.BaseURL+"/game/sessions/"+s.GameSessionID+"/realtime-tickets", auth, nil)
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return fmt.Errorf("createRealtimeTicket: %d", status)
	}
	var tk struct {
		Ticket string `json:"ticket"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(b, &tk); err != nil {
		return err
	}
	s.Ticket, s.TicketURL = tk.Ticket, tk.URL
	return nil
}

func post(ctx context.Context, hc Doer, url string, headers map[string]string, body any) (int, http.Header, []byte, error) {
	var rd io.Reader = http.NoBody
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, rd)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "vi")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, resp.Header, b, err
}
