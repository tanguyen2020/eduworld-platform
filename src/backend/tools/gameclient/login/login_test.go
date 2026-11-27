package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGame is a TEST STAND-IN of the public game API (gameLogin, createGameSession, createRealtimeTicket), shaped as
// specs/openapi/eduworld-api.v1.yaml. It is never used outside these tests (AGENTS R17).
type fakeGame struct {
	busy        int32 // number of LOGIN_QUEUE_BUSY answers before success
	rateLimited int32 // number of RATE_LIMITED answers before success
	errors5xx   int32
	retryAfter  string
	conflict    bool // createGameSession answers 409 once (another active session) → the bot takes over
	logins      atomic.Int32
	mu          sync.Mutex
	seen        []http.Header
	bodies      []map[string]any
}

func (f *fakeGame) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/game/login", func(w http.ResponseWriter, r *http.Request) {
		n := f.logins.Add(1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.bodies = append(f.bodies, body)
		f.mu.Unlock()
		switch {
		case n <= f.errors5xx:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		case n <= f.errors5xx+f.busy:
			problem(w, f.retryAfter, "LOGIN_QUEUE_BUSY")
			return
		case n <= f.errors5xx+f.busy+f.rateLimited:
			problem(w, f.retryAfter, "RATE_LIMITED")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tokens": map[string]any{"access_token": "tok", "refresh_token": "r", "token_type": "Bearer", "expires_in": 300, "refresh_expires_in": 3600},
			"bootstrap": map[string]any{"tenant_id": "11111111-1111-4111-8111-111111111111", "user": map[string]any{"user_id": "u"},
				"student": map[string]any{"student_id": "s", "school_id": "sc", "grade_level": 3}},
		})
	})
	conflictOnce := f.conflict
	mux.HandleFunc("POST /api/v1/game/sessions", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.seen = append(f.seen, r.Header.Clone())
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.bodies = append(f.bodies, body)
		c := conflictOnce
		conflictOnce = false
		f.mu.Unlock()
		if c {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"GAME_SESSION_ACTIVE_ELSEWHERE","status":409}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"game_session_id":"22222222-2222-4222-8222-222222222222","started_at":"x","login_at":"x"}`))
	})
	mux.HandleFunc("POST /api/v1/game/sessions/{id}/realtime-tickets", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "22222222-2222-4222-8222-222222222222" || r.Header.Get("X-Tenant-Id") == "" {
			t.Errorf("ticket request %s %v", r.PathValue("id"), r.Header)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ticket":"tk","expires_at":"x","url":"wss://rt.example/realtime/v1","protocol_version":1}`))
	})
	return mux
}

func problem(w http.ResponseWriter, retryAfter, code string) {
	if retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"type":"about:blank","status":429,"code":"` + code + `"}`))
}

// recorder captures every wait of the state machine instead of sleeping.
type recorder struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *recorder) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	r.waits = append(r.waits, d)
	r.mu.Unlock()
	return nil
}

func newConfig(url string, rec *recorder) Config {
	return Config{BaseURL: url + "/api/v1", ClientVersion: "1.0.0", Platform: "web", MaxAttempts: 20, MaxWait: time.Minute,
		JitterMax: 500 * time.Millisecond, Jitter: func(max time.Duration) time.Duration { return max / 2 }, Sleep: rec.sleep}
}

var cred = Credentials{Username: "perf.001.01.00001", Password: "secret", DeviceID: "33333333-3333-4333-8333-333333333333"}

// F132-AC-03 (TC-AUTH-044, TC-PERF-016): on 429 LOGIN_QUEUE_BUSY the bot waits Retry-After plus jitter, never a
// tight retry, and gets in after the peak.
func TestQueueBusyWaitsRetryAfterPlusJitter(t *testing.T) {
	f := &fakeGame{busy: 3, retryAfter: "2"}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	rec := &recorder{}
	s, err := Login(context.Background(), srv.Client(), newConfig(srv.URL, rec), cred)
	if err != nil {
		t.Fatal(err)
	}
	if s.QueueBusy != 3 || s.Attempts != 4 || len(rec.waits) != 3 {
		t.Fatalf("session %+v waits %v", s, rec.waits)
	}
	for _, w := range rec.waits {
		if w != 2*time.Second+250*time.Millisecond {
			t.Fatalf("wait %v, want Retry-After 2 s + jitter 250 ms", w)
		}
	}
	if s.Ticket != "tk" || s.TicketURL != "wss://rt.example/realtime/v1" || s.TenantID == "" || s.GameSessionID == "" || s.GradeLevel != 3 {
		t.Fatalf("session %+v", s)
	}
}

// A missing or invalid Retry-After never produces a tight retry: the bot waits at least one second plus jitter, and
// an absurd Retry-After is capped at MaxWait.
func TestRetryAfterFallbackAndCap(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   time.Duration
	}{{"", time.Second}, {"abc", time.Second}, {"0", time.Second}, {"3600", time.Minute}} {
		f := &fakeGame{busy: 1, retryAfter: tc.header}
		srv := httptest.NewServer(f.handler(t))
		rec := &recorder{}
		if _, err := Login(context.Background(), srv.Client(), newConfig(srv.URL, rec), cred); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		if len(rec.waits) != 1 || rec.waits[0] != tc.want+250*time.Millisecond {
			t.Fatalf("Retry-After %q: waits %v, want %v + jitter", tc.header, rec.waits, tc.want)
		}
	}
}

// RATE_LIMITED (10/min per device or username, login_rate) is counted apart from the queue: it means the bot data
// shares a device_id, which datagen must prevent (spec edge case). The bot still honours Retry-After.
func TestRateLimitedPerDeviceIsCountedApart(t *testing.T) {
	f := &fakeGame{rateLimited: 2, retryAfter: "5"}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	rec := &recorder{}
	s, err := Login(context.Background(), srv.Client(), newConfig(srv.URL, rec), cred)
	if err != nil {
		t.Fatal(err)
	}
	if s.RateLimited != 2 || s.QueueBusy != 0 || rec.waits[0] < 5*time.Second {
		t.Fatalf("session %+v waits %v", s, rec.waits)
	}
}

// 5xx are counted (they must be 0 in TC-PERF-016) and retried with a growing backoff, never in a tight loop.
func TestServerErrorsBackOff(t *testing.T) {
	f := &fakeGame{errors5xx: 3}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	rec := &recorder{}
	s, err := Login(context.Background(), srv.Client(), newConfig(srv.URL, rec), cred)
	if err != nil {
		t.Fatal(err)
	}
	if s.ServerErrors != 3 || len(rec.waits) != 3 || rec.waits[0] >= rec.waits[1] || rec.waits[1] >= rec.waits[2] {
		t.Fatalf("session %+v waits %v", s, rec.waits)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	f := &fakeGame{busy: 100, retryAfter: "1"}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	cfg := newConfig(srv.URL, &recorder{})
	cfg.MaxAttempts = 4
	_, err := Login(context.Background(), srv.Client(), cfg, cred)
	if !errors.Is(err, ErrGaveUp) || f.logins.Load() != 4 {
		t.Fatalf("err = %v after %d logins", err, f.logins.Load())
	}
}

// The login body and the game session request follow the contract: device_id per bot, X-Device-Id,
// Idempotency-Key (UUIDv7), X-Tenant-Id from the bootstrap; a 409 (session active elsewhere) is taken over once.
func TestContractFieldsAndTakeover(t *testing.T) {
	f := &fakeGame{conflict: true}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	s, err := Login(context.Background(), srv.Client(), newConfig(srv.URL, &recorder{}), cred)
	if err != nil {
		t.Fatal(err)
	}
	if s.Takeover != true || len(f.seen) != 2 {
		t.Fatalf("session %+v, %d game session calls", s, len(f.seen))
	}
	login := f.bodies[0]
	for _, k := range []string{"username", "password", "device_id", "client_version", "platform"} {
		if login[k] == nil {
			t.Fatalf("login body lacks %s: %v", k, login)
		}
	}
	h := f.seen[1]
	if h.Get("X-Device-Id") != cred.DeviceID || h.Get("Authorization") != "Bearer tok" || !strings.HasPrefix(h.Get("Idempotency-Key")[14:15], "7") {
		t.Fatalf("headers %v", h)
	}
	if f.bodies[2]["takeover"] != true {
		t.Fatalf("second createGameSession body %v, want takeover", f.bodies[2])
	}
	if f.seen[0].Get("Idempotency-Key") == h.Get("Idempotency-Key") {
		t.Fatal("takeover must use a new Idempotency-Key (another request body)")
	}
}

// Wrong credentials are terminal: no retry storm on 401.
func TestUnauthorizedIsTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	rec := &recorder{}
	_, err := Login(context.Background(), srv.Client(), newConfig(srv.URL, rec), cred)
	if !errors.Is(err, ErrRejected) || len(rec.waits) != 0 {
		t.Fatalf("err = %v waits %v", err, rec.waits)
	}
}
