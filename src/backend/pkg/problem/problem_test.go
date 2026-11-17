package problem_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F002-AC-20: known errors map to their catalog status; Problem Details JSON with code and trace_id.
func TestWriteKnownError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	problem.Write(rec, req, problem.New(problem.TenantMismatch, "tenant does not match"), "trace-123")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type = %q", ct)
	}
	var p problem.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != problem.TenantMismatch || p.Status != 403 || p.TraceID != "trace-123" || p.Title == "" || p.Type == "" {
		t.Fatalf("unexpected problem: %+v", p)
	}
}

// F002-AC-20: an unknown error becomes 500 INTERNAL without leaking its message.
func TestWriteUnknownErrorHidesDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	secret := errors.New("pq: password authentication failed for user identity_app at 10.1.2.3")
	problem.Write(rec, httptest.NewRequest(http.MethodGet, "/", nil), fmt.Errorf("load: %w", secret), "t1")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"INTERNAL"`) || strings.Contains(body, "password") || strings.Contains(body, "10.1.2.3") {
		t.Fatalf("internal error leaked or wrong code: %s", body)
	}
}

func TestWrappedProblemKeepsCode(t *testing.T) {
	err := fmt.Errorf("resolve: %w", problem.New(problem.Unavailable, "identity unavailable").Retry())
	p := problem.From(err, "t")
	if p.Code != problem.Unavailable || p.Status != 503 || p.Retryable == nil || !*p.Retryable {
		t.Fatalf("unexpected: %+v", p)
	}
}

func TestInvalidFields(t *testing.T) {
	p := problem.From(problem.Invalid("title", problem.SubMaxLength, "max 255"), "t")
	if p.Code != problem.ValidationFailed || p.Status != 422 || len(p.Errors) != 1 || p.Errors[0].Field != "title" || p.Errors[0].Code != "MAX_LENGTH" {
		t.Fatalf("unexpected: %+v", p)
	}
}

func TestExtensionFieldsSerialized(t *testing.T) {
	rec := httptest.NewRecorder()
	problem.Write(rec, httptest.NewRequest(http.MethodGet, "/", nil),
		problem.New(problem.FeatureDisabled, "action disabled").With("feature_key", "action.school_lms.classes.view"), "t")
	if !strings.Contains(rec.Body.String(), `"feature_key":"action.school_lms.classes.view"`) {
		t.Fatalf("extension missing: %s", rec.Body.String())
	}
}

func TestEveryCodeHasStatus(t *testing.T) {
	if len(problem.AllCodes()) < 100 {
		t.Fatalf("codes not generated: %d", len(problem.AllCodes()))
	}
	for _, c := range problem.AllCodes() {
		// Most codes are 4xx/5xx; a few are informational outcomes (e.g. CHAT_CONTENT_HELD 202).
		if s := problem.Status(c); s < 200 || s > 599 {
			t.Fatalf("code %s has status %d", c, s)
		}
	}
}
