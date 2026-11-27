package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// The presigned GET example of the AWS S3 documentation ("Authenticating Requests: Using Query Parameters").
func TestPresignAWSVector(t *testing.T) {
	cfg := Config{Region: "us-east-1", AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}
	at := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	q := presign(cfg, http.MethodGet, "examplebucket.s3.amazonaws.com", "/test.txt", nil, at, 86400*time.Second)
	const want = "aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
	if !strings.HasSuffix(q, "X-Amz-Signature="+want) {
		t.Fatalf("query = %s", q)
	}
}

func TestPresignPutSignsContentLength(t *testing.T) {
	s, err := New(Config{Endpoint: "http://floci:4566", AccessKey: "test", SecretKey: "test", Bucket: "ew-sis-imports"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.PresignPut(context.Background(), "sis-imports/t/a b.csv", 1234, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "http://floci:4566/ew-sis-imports/sis-imports/t/a%20b.csv?") || !strings.Contains(u, "X-Amz-SignedHeaders=content-length%3Bhost") ||
		!strings.Contains(u, "X-Amz-Expires=900") {
		t.Fatalf("url = %s", u)
	}
	if _, err := New(Config{Endpoint: "http://x"}, nil); err == nil {
		t.Fatal("missing credentials accepted")
	}
}

func TestHeadOpenRetryAndNotFound(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test/") || r.Header.Get("x-amz-date") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/ew-sis-imports/flaky.csv":
			if calls.Load() == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = io.WriteString(w, "a,b\n")
		case "/ew-sis-imports/ok.csv":
			w.Header().Set("Content-Length", "4")
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, "a,b\n")
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	s, err := New(Config{Endpoint: srv.URL, AccessKey: "test", SecretKey: "test", Bucket: "ew-sis-imports"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if n, err := s.Head(ctx, "ok.csv"); err != nil || n != 4 {
		t.Fatalf("head = %d %v", n, err)
	}
	if _, err := s.Head(ctx, "missing.csv"); !errors.Is(err, port.ErrObjectNotFound) {
		t.Fatalf("missing = %v", err)
	}
	rc, err := s.Open(ctx, "flaky.csv") // 503 then 200: retried
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "a,b\n" {
		t.Fatalf("body = %q", b)
	}
}

// MONITORING §3: one eduworld_external_client_requests_total per logical call (a not-found answer is "ok", the outcome
// is the availability of the storage) and one eduworld_external_client_retries_total per retry.
func TestExternalClientMetrics(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ew-sis-imports/flaky.csv":
			if calls.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = io.WriteString(w, "a,b\n")
		case "/ew-sis-imports/down.csv":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	s, err := New(Config{Endpoint: srv.URL, AccessKey: "test", SecretKey: "test", Bucket: "ew-sis-imports"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	req := func(o string) float64 {
		return testutil.ToFloat64(observability.ExternalRequests().WithLabelValues("identity", observability.IntegrationS3, o))
	}
	retries := func() float64 {
		return testutil.ToFloat64(observability.ExternalRetries().WithLabelValues("identity", observability.IntegrationS3))
	}
	ok0, err0, r0 := req(observability.OutcomeOK), req(observability.OutcomeError), retries()
	ctx := context.Background()
	rc, err := s.Open(ctx, "flaky.csv") // 503 then 200: one call, one retry
	if err != nil {
		t.Fatal(err)
	}
	_ = rc.Close()
	if _, err := s.Head(ctx, "missing.csv"); !errors.Is(err, port.ErrObjectNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if _, err := s.Head(ctx, "down.csv"); err == nil { // 500 three times: one failed call, two retries
		t.Fatal("down.csv must fail")
	}
	if d := req(observability.OutcomeOK) - ok0; d != 2 {
		t.Fatalf("ok += %v, want 2 (flaky + not found)", d)
	}
	if d := req(observability.OutcomeError) - err0; d != 1 {
		t.Fatalf("error += %v, want 1", d)
	}
	if d := retries() - r0; d != 3 {
		t.Fatalf("retries += %v, want 3 (1 flaky + 2 down)", d)
	}
}
