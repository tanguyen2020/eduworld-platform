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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
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
