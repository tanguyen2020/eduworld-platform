//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// panicking is a TEST DOUBLE of the game session use cases whose login panics after receiving the body.
type panicking struct{ command.GameSessionHandler }

func (panicking) Login(context.Context, dto.GameLogin) (dto.GameLoginResult, error) {
	panic("simulated failure in gameLogin")
}

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// F013-AC-16 (TC-AUTH-060): a unique sample password, the username, refresh tokens and login tickets never
// appear in the logs or spans of core-api — on success, wrong password, Keycloak 5xx and a panic.
func TestGameLoginNeverLogsCredentials(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	a := seedTenant(ctx, t, g.platform, "pgd-secu")
	g.student(ctx, t, a, "hs.bao.mat")

	var logs syncBuffer
	logger := observability.NewLogger(&logs, observability.LoggerOptions{Binary: "core-api", Env: "test", Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(prev) })
	rec := tracetest.NewSpanRecorder()
	obs, err := observability.New(observability.Options{Logger: logger, SpanProcessor: rec, Binary: "core-api"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(obs.Shutdown)
	srv := httptest.NewServer(obs.HTTP(g.handler))
	t.Cleanup(srv.Close)
	ps, err := httpx.NewServer(g.deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := identityhttp.RegisterGameAuth(ps, identityhttp.GameAuthHandlers{Session: panicking{g.auth}}); err != nil {
		t.Fatal(err)
	}
	psrv := httptest.NewServer(obs.HTTP(ps))
	t.Cleanup(psrv.Close)
	g.srv = srv

	const sample = "Mau-Bao-Mat-7f3a9c" // unique: any occurrence is a leak
	g.kc.mu.Lock()
	g.kc.users["hs.bao.mat"] = fakeKCUser{password: sample, student: true}
	g.kc.mu.Unlock()
	body := func() map[string]any {
		b := gameLogin("hs.bao.mat", uuid.New())
		b["password"] = sample
		return b
	}
	var secrets []string
	r := g.post(t, "/auth/game/login", body(), nil) // success
	if r.code != 200 {
		t.Fatalf("login: %d %v", r.code, r.body)
	}
	secrets = append(secrets, tokensOf(t, r)["refresh_token"].(string))
	wrong := body()
	wrong["password"] = sample + "-sai"
	_ = g.post(t, "/auth/game/login", wrong, nil) // 401
	g.kc.mu.Lock()
	g.kc.down = true
	g.kc.mu.Unlock()
	if r := g.post(t, "/auth/game/login", body(), nil); r.code != 503 { // 5xx path
		t.Fatalf("down: %d", r.code)
	}
	g.srv = psrv
	if r := g.post(t, "/auth/game/login", body(), nil); r.code != http.StatusInternalServerError { // panic recovered
		t.Fatalf("panic: %d", r.code)
	}

	var spans strings.Builder
	for _, s := range rec.Ended() {
		fmt.Fprintf(&spans, "%s %v %s\n", s.Name(), s.Attributes(), s.Status().Description)
		for _, e := range s.Events() {
			fmt.Fprintf(&spans, "%s %v\n", e.Name, e.Attributes)
		}
	}
	if spans.Len() == 0 || !strings.Contains(logs.String(), "identity.command.GameLogin") {
		t.Fatal("nothing captured: the test would prove nothing")
	}
	for _, leak := range append(secrets, sample, "hs.bao.mat") {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("log leaks %q", leak)
		}
		if strings.Contains(spans.String(), leak) {
			t.Errorf("span leaks %q", leak)
		}
	}
}
