package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

const (
	// Synthetic token split across literals so secret scanners do not flag the fixture itself.
	jwtLike = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImsxIn0" + "." + "eyJzdWIiOiJrYy0xIiwiYXpwIjoiZWR1d29ybGQtcG9ydGFsIn0" + "." + "c2lnbmF0dXJlLXZhbHVlLWhlcmU"
	email   = "hoc.sinh@truong.edu.vn"
)

func newLogger(buf *bytes.Buffer) *slog.Logger {
	return observability.NewLogger(buf, observability.LoggerOptions{Binary: "core-api", Profile: "", Env: "test", Version: "v0.0.0", Level: slog.LevelDebug})
}

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		out = append(out, m)
	}
	return out
}

// F002-AC-23 (TC-AUTH-060 backend part): secrets and PII never reach the log.
func TestLoggerRedacts(t *testing.T) {
	var buf bytes.Buffer
	log := newLogger(&buf)
	log.Info("login attempt",
		"authorization", "Bearer "+jwtLike,
		"password", "S3cret!pass",
		"refresh_token", "opaque-refresh",
		"cookie", "__Secure-ew_portal_rt=abc",
		"email", email,
		"full_name", "Nguyễn Văn A",
		"answer", "42",
		"chat_content", "xin chào",
		"ip", "203.113.1.25",
		"detail", "user "+email+" sent "+jwtLike,
		slog.Group("req", "headers", map[string]string{"Authorization": "Bearer " + jwtLike}),
		"user_id", "0193f0a2-7c1e-7000-8000-000000000001",
	)
	out := buf.String()
	for _, leak := range []string{jwtLike, "S3cret!pass", "opaque-refresh", "__Secure-ew_portal_rt=abc", email, "Nguyễn Văn A",
		"xin chào", "203.113.1.25", `"answer":"42"`} {
		if strings.Contains(out, leak) {
			t.Errorf("log leaks %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "0193f0a2-7c1e-7000-8000-000000000001") {
		t.Fatal("user_id (internal UUID) must stay")
	}
	m := lines(t, &buf)[0]
	for _, k := range []string{"ts", "level", "msg", "binary", "env", "version"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("field %s missing: %v", k, m)
		}
	}
	if m["binary"] != "core-api" {
		t.Fatalf("binary = %v", m["binary"])
	}
}

func TestLoggerErrorsAndModule(t *testing.T) {
	var buf bytes.Buffer
	log := newLogger(&buf).With("module", "learning")
	log.Error("failed", "error", errors.New("dial postgres://learning_app:pw@db/learning: refused"))
	m := lines(t, &buf)[0]
	if m["module"] != "learning" {
		t.Fatalf("module = %v", m["module"])
	}
	if strings.Contains(buf.String(), ":pw@") {
		t.Fatalf("DSN credentials leaked: %s", buf.String())
	}
}

// The HTTP middleware logs one access line with request fields, never headers or query strings.
func TestAccessLogFields(t *testing.T) {
	var buf bytes.Buffer
	obs := observability.NewForTest(newLogger(&buf))
	h := obs.HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := observability.InfoFrom(r.Context())
		info.SetRoute("/api/v1/lms/classes", "listClasses", "learning")
		info.SetPrincipal("0193f0a2-7c1e-7000-8000-000000000001", "0193f0a2-7c1e-7000-8000-0000000000aa")
		info.SetCode("PERMISSION_DENIED")
		w.WriteHeader(http.StatusForbidden)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/lms/classes?ticket=secret-ticket", nil)
	req.Header.Set("Authorization", "Bearer "+jwtLike)
	req.Header.Set("X-Request-Id", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(context.Background()))
	out := buf.String()
	if strings.Contains(out, "secret-ticket") || strings.Contains(out, jwtLike) {
		t.Fatalf("access log leaks: %s", out)
	}
	var access map[string]any
	for _, m := range lines(t, &buf) {
		if m["msg"] == "http request" {
			access = m
		}
	}
	if access == nil {
		t.Fatalf("no access log: %s", out)
	}
	for k, want := range map[string]any{"route": "/api/v1/lms/classes", "status": float64(403), "error.code": "PERMISSION_DENIED",
		"module": "learning", "user_id": "0193f0a2-7c1e-7000-8000-000000000001", "tenant_id": "0193f0a2-7c1e-7000-8000-0000000000aa", "method": "GET"} {
		if access[k] != want {
			t.Fatalf("%s = %v, want %v (%v)", k, access[k], want, access)
		}
	}
	rid, _ := access["request_id"].(string)
	if len(rid) != 36 || rid == "not-a-uuid" || rec.Header().Get("X-Request-Id") != rid {
		t.Fatalf("request_id = %q (invalid incoming id must be replaced)", rid)
	}
	if _, ok := access["duration_ms"]; !ok {
		t.Fatal("duration_ms missing")
	}
}

// F013 T002: the game login fields never reach the log (F013-AC-16).
func TestLoggerRedactsGameLoginFields(t *testing.T) {
	var buf bytes.Buffer
	newLogger(&buf).Info("game login", "username", "hs.an.3a", "password", "Mat-khau-1", "login_ticket", "tk-123456789",
		"refresh_token", "rt-abc", "setup_ticket", "st-1", "new_password", "Moi-1")
	for _, leak := range []string{"hs.an.3a", "Mat-khau-1", "tk-123456789", "rt-abc", "st-1", "Moi-1"} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("log leaks %q: %s", leak, buf.String())
		}
	}
}
