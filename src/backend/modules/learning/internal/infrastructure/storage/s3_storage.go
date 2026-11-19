// Package storage is the S3-compatible object storage adapter of module learning (media pipeline F027, ADR-0044,
// ADR-0071: floci in every environment through EW_S3_ENDPOINT, path-style URLs) and the CDN URL signer of
// ew-media-private. Requests are signed with AWS Signature V4; every call has a timeout, HEAD/GET (and PUT of a
// seekable body) are retried on transient failures and a circuit breaker protects the storage (AGENTS R29).
// Same pattern as the adapter of module identity (no cross-module import, R13).
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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// Config of the adapter (EW_S3_ENDPOINT, EW_S3_REGION, EW_S3_ACCESS_KEY, EW_S3_SECRET_KEY). Buckets are named per
// call (ew-media-quarantine, ew-media-public, ew-media-private).
type Config struct {
	Endpoint  string // e.g. http://floci:4566
	Region    string
	AccessKey string
	SecretKey string
	Timeout   time.Duration // per request (default 30 s)
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
	if err != nil || u.Host == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3: endpoint and credentials are required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if client == nil {
		client = &http.Client{}
	}
	transient := func(_ result, err error) bool { return errors.Is(err, errTransient) }
	cb := circuitbreaker.NewBuilder[result]().HandleIf(transient).WithFailureThresholdRatio(5, 10).
		WithDelay(30 * time.Second).Build()
	rp := retrypolicy.NewBuilder[result]().HandleIf(transient).WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, time.Second).ReturnLastFailure().Build()
	return &S3{cfg: cfg, base: u, http: client, exec: failsafe.With[result](rp, cb), clock: time.Now}, nil
}

func (s *S3) objectURI(bucket, key string) string {
	return s.base.Path + "/" + uriEncode(bucket, false) + "/" + uriEncode(key, true)
}

// PresignPut implements port.ObjectStorage: the signature covers Content-Length (the size limit) and Content-Type.
func (s *S3) PresignPut(_ context.Context, bucket, key string, size int64, mime string, ttl time.Duration) (string, map[string]string, error) {
	uri := s.objectURI(bucket, key)
	q := presign(s.cfg, http.MethodPut, s.base.Host, uri, map[string]string{"content-length": strconv.FormatInt(size, 10), "content-type": mime},
		s.clock().UTC(), ttl)
	return s.base.Scheme + "://" + s.base.Host + uri + "?" + q, map[string]string{"Content-Type": mime}, nil
}

// Head implements port.ObjectStorage.
func (s *S3) Head(ctx context.Context, bucket, key string) (int64, error) {
	resp, err := s.do(ctx, http.MethodHead, bucket, key)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.ContentLength, nil
}

// Open implements port.ObjectStorage.
func (s *S3) Open(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	resp, err := s.do(ctx, http.MethodGet, bucket, key)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

// Put implements port.ObjectStorage. A body that is an io.ReadSeeker is retried; any other body is sent once.
func (s *S3) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, mime, cacheControl string) error {
	hdr := map[string]string{"content-type": mime, "cache-control": cacheControl}
	seeker, retryable := body.(io.ReadSeeker)
	if !retryable {
		resp, err := s.once(ctx, http.MethodPut, bucket, key, body, size, hdr)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		return nil
	}
	r, err := s.exec.WithContext(ctx).Get(func() (result, error) {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return result{}, err
		}
		resp, err := s.once(ctx, http.MethodPut, bucket, key, seeker, size, hdr) //nolint:bodyclose // closed by Put after the retries
		return result{resp: resp}, err
	})
	if err != nil {
		return err
	}
	_ = r.resp.Body.Close()
	return nil
}

// result wraps the response of one attempt (the body is closed by the caller of do).
type result struct{ resp *http.Response }

func (s *S3) do(ctx context.Context, method, bucket, key string) (*http.Response, error) {
	r, err := s.exec.WithContext(ctx).Get(func() (result, error) {
		resp, err := s.once(ctx, method, bucket, key, nil, 0, nil) //nolint:bodyclose // closed by the caller of do
		return result{resp: resp}, err
	})
	return r.resp, err
}

// once sends one signed request; the response body is closed by the caller.
func (s *S3) once(ctx context.Context, method, bucket, key string, body io.Reader, size int64, hdr map[string]string) (*http.Response, error) {
	cctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	uri := s.objectURI(bucket, key)
	req, err := http.NewRequestWithContext(cctx, method, s.base.Scheme+"://"+s.base.Host+uri, body)
	if err != nil {
		cancel()
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	signHeaders(s.cfg, req, s.base.Host, uri, hdr, s.clock().UTC())
	resp, err := s.http.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%w: %w", errTransient, err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		_ = resp.Body.Close()
		cancel()
		return nil, port.ErrObjectNotFound
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("%w: status %d", errTransient, resp.StatusCode)
	case resp.StatusCode >= 300:
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("s3: %s %s/%s: status %d", method, bucket, key, resp.StatusCode)
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
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

// signHeaders adds the Authorization header (payload unsigned) signing host, the x-amz headers and extra.
func signHeaders(cfg Config, req *http.Request, host, uri string, extra map[string]string, t time.Time) {
	date := t.Format("20060102")
	scope := date + "/" + cfg.Region + "/s3/aws4_request"
	amz := t.Format(amzDateFormat)
	req.Header.Set("x-amz-date", amz)
	req.Header.Set("x-amz-content-sha256", unsignedPayload)
	hdrs := map[string]string{"host": host, "x-amz-date": amz, "x-amz-content-sha256": unsignedPayload}
	for k, v := range extra {
		hdrs[strings.ToLower(k)] = v
	}
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

// EnsureBucket creates a bucket when missing (local setup and integration tests; deploy/compose s3-init creates
// the buckets of the environments).
func (s *S3) EnsureBucket(ctx context.Context, bucket string) error {
	cctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	uri := s.base.Path + "/" + uriEncode(bucket, false)
	req, err := http.NewRequestWithContext(cctx, http.MethodPut, s.base.Scheme+"://"+s.base.Host+uri, nil)
	if err != nil {
		return err
	}
	signHeaders(s.cfg, req, s.base.Host, uri, nil, s.clock().UTC())
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("s3: create bucket %s: status %d", bucket, resp.StatusCode)
	}
	return nil
}

// Stat returns the headers of an object (Content-Type, Cache-Control, ETag): operations and tests check that the
// content-addressed variants are immutable for the CDN.
func (s *S3) Stat(ctx context.Context, bucket, key string) (http.Header, error) {
	resp, err := s.do(ctx, http.MethodHead, bucket, key)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	return resp.Header, nil
}
