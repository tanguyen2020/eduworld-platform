// Package storage is the S3-compatible object storage adapter of module identity (SIS import files,
// ADR-0075: floci locally, managed S3-compatible storage in staging/production through EW_S3_ENDPOINT).
// Requests are signed with AWS Signature V4 (path-style URLs); every call has a timeout, HEAD/GET are
// retried on transient failures and a circuit breaker protects the storage (AGENTS R29).
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Config of the adapter (EW_S3_ENDPOINT, EW_S3_REGION, EW_S3_ACCESS_KEY, EW_S3_SECRET_KEY, bucket name).
type Config struct {
	Endpoint  string // e.g. http://floci:4566
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string // ew-sis-imports
	Timeout   time.Duration
}

// S3 implements port.ObjectStorage.
type S3 struct {
	cfg   Config
	base  *url.URL
	http  *http.Client
	exec  failsafe.Executor[result]
	clock func() time.Time
}

var _ port.ObjectStorage = (*S3)(nil)

var errTransient = errors.New("s3: transient")

// New builds the adapter.
func New(cfg Config, client *http.Client) (*S3, error) {
	u, err := url.Parse(strings.TrimRight(cfg.Endpoint, "/"))
	if err != nil || u.Host == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3: endpoint, bucket and credentials are required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if client == nil {
		client = &http.Client{}
	}
	transient := func(_ result, err error) bool { return errors.Is(err, errTransient) }
	cb := observability.ObserveCircuit(circuitbreaker.NewBuilder[result](), "identity", observability.IntegrationS3).HandleIf(transient).WithFailureThresholdRatio(5, 10).
		WithDelay(30 * time.Second).Build()
	rp := retrypolicy.NewBuilder[result]().HandleIf(transient).WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, time.Second).ReturnLastFailure().Build()
	return &S3{cfg: cfg, base: u, http: client, exec: failsafe.With[result](rp, cb), clock: time.Now}, nil
}

func (s *S3) objectURI(key string) string {
	return s.base.Path + "/" + uriEncode(s.cfg.Bucket, false) + "/" + uriEncode(key, true)
}

// PresignPut signs a PUT whose Content-Length must equal size (the size limit is enforced by the signature).
func (s *S3) PresignPut(_ context.Context, key string, size int64, ttl time.Duration) (string, error) {
	uri := s.objectURI(key)
	q := presign(s.cfg, http.MethodPut, s.base.Host, uri, map[string]string{"content-length": strconv.FormatInt(size, 10)}, s.clock().UTC(), ttl)
	return s.base.Scheme + "://" + s.base.Host + uri + "?" + q, nil
}

// Head returns the object size.
func (s *S3) Head(ctx context.Context, key string) (int64, error) {
	resp, err := s.do(ctx, http.MethodHead, key)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.ContentLength, nil
}

// Open streams the object; the caller closes it.
func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.do(ctx, http.MethodGet, key)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// result wraps the response of one attempt (the body is closed by the caller of do).
type result struct{ resp *http.Response }

func (s *S3) do(ctx context.Context, method, key string) (*http.Response, error) {
	r, err := s.exec.WithContext(ctx).Get(func() (result, error) {
		cctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
		uri := s.objectURI(key)
		req, err := http.NewRequestWithContext(cctx, method, s.base.Scheme+"://"+s.base.Host+uri, nil)
		if err != nil {
			cancel()
			return result{}, err
		}
		signHeaders(s.cfg, req, s.base.Host, uri, s.clock().UTC())
		resp, err := s.http.Do(req)
		if err != nil {
			cancel()
			return result{}, fmt.Errorf("%w: %w", errTransient, err)
		}
		switch {
		case resp.StatusCode == http.StatusNotFound:
			_ = resp.Body.Close()
			cancel()
			return result{}, port.ErrObjectNotFound
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			_ = resp.Body.Close()
			cancel()
			return result{}, fmt.Errorf("%w: status %d", errTransient, resp.StatusCode)
		case resp.StatusCode >= 300:
			_ = resp.Body.Close()
			cancel()
			return result{}, fmt.Errorf("s3: %s %s: status %d", method, key, resp.StatusCode)
		}
		resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
		return result{resp: resp}, nil
	})
	return r.resp, err
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// ---------- AWS Signature V4 ----------

const (
	algorithm       = "AWS4-HMAC-SHA256"
	unsignedPayload = "UNSIGNED-PAYLOAD"
	amzDateFormat   = "20060102T150405Z"
)

// presign returns the query string of a presigned request signing host and the given headers.
func presign(cfg Config, method, host, uri string, headers map[string]string, t time.Time, ttl time.Duration) string {
	date := t.Format("20060102")
	scope := date + "/" + cfg.Region + "/s3/aws4_request"
	hdrs := map[string]string{"host": host}
	for k, v := range headers {
		hdrs[strings.ToLower(k)] = v
	}
	names := sortedKeys(hdrs)
	q := map[string]string{
		"X-Amz-Algorithm":     algorithm,
		"X-Amz-Credential":    cfg.AccessKey + "/" + scope,
		"X-Amz-Date":          t.Format(amzDateFormat),
		"X-Amz-Expires":       strconv.Itoa(int(ttl.Seconds())),
		"X-Amz-SignedHeaders": strings.Join(names, ";"),
	}
	canonicalQuery := canonicalQueryString(q)
	creq := method + "\n" + uri + "\n" + canonicalQuery + "\n" + canonicalHeaders(hdrs, names) + "\n" + strings.Join(names, ";") + "\n" + unsignedPayload
	sig := signature(cfg, date, t.Format(amzDateFormat), scope, creq)
	return canonicalQuery + "&X-Amz-Signature=" + sig
}

// signHeaders adds the Authorization header of a request without body.
func signHeaders(cfg Config, req *http.Request, host, uri string, t time.Time) {
	date := t.Format("20060102")
	scope := date + "/" + cfg.Region + "/s3/aws4_request"
	amz := t.Format(amzDateFormat)
	req.Header.Set("x-amz-date", amz)
	req.Header.Set("x-amz-content-sha256", unsignedPayload)
	hdrs := map[string]string{"host": host, "x-amz-date": amz, "x-amz-content-sha256": unsignedPayload}
	names := sortedKeys(hdrs)
	creq := req.Method + "\n" + uri + "\n\n" + canonicalHeaders(hdrs, names) + "\n" + strings.Join(names, ";") + "\n" + unsignedPayload
	sig := signature(cfg, date, amz, scope, creq)
	req.Header.Set("Authorization", algorithm+" Credential="+cfg.AccessKey+"/"+scope+", SignedHeaders="+strings.Join(names, ";")+", Signature="+sig)
}

func signature(cfg Config, date, amzDate, scope, canonicalRequest string) string {
	h := sha256.Sum256([]byte(canonicalRequest))
	sts := algorithm + "\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(h[:])
	k := hmacSHA256([]byte("AWS4"+cfg.SecretKey), date)
	k = hmacSHA256(k, cfg.Region)
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	return hex.EncodeToString(hmacSHA256(k, sts))
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func canonicalHeaders(h map[string]string, names []string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n + ":" + strings.TrimSpace(h[n]) + "\n")
	}
	return b.String()
}

func canonicalQueryString(q map[string]string) string {
	keys := sortedKeys(q)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, uriEncode(k, false)+"="+uriEncode(q[k], false))
	}
	return strings.Join(parts, "&")
}

// uriEncode is the SigV4 URI encoding (RFC 3986 unreserved characters kept; '/' kept in object keys).
func uriEncode(s string, keepSlash bool) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && keepSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
