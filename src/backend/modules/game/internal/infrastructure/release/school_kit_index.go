// Package release loads the signed school-kit index of the release CDN (F134 US3, DEPLOYMENT §9): GET
// <base>/school-kit/<channel>.json and <channel>.json.sig (base64 RSA-SHA256 PKCS#1 v1.5 of the index bytes, signed with
// the release key of F133), verify the signature with the configured public key, then the schema; only a verified index
// is cached (Valkey, 5 minutes) and returned. Every call has a timeout, the idempotent GETs are retried on transient
// failures and a circuit breaker protects the CDN (AGENTS R29). The adapter never logs the index content.
package release

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/installer"
)

// CacheTTL of a verified index (plan F134: ew:game:school_kit_index:<channel>, 5 minutes).
const CacheTTL = 5 * time.Minute

const (
	cachePrefix = "ew:game:school_kit_index:"
	maxIndex    = 1 << 20 // 1 MiB: 50 kits are a few kB
	maxSig      = 4096
)

// Cache is the Valkey cache of the verified index (every error = a miss, ADR-0004).
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// Config of the adapter: EW_CLIENT_RELEASES_BASE_URL and the PEM of EW_CLIENT_RELEASE_PUBKEY_FILE (R3).
type Config struct {
	BaseURL      string
	PublicKeyPEM []byte
	Timeout      time.Duration // default 5 s per request
}

// ErrSignature: the signature does not verify (F134-AC-14).
var ErrSignature = errors.New("release: school-kit index signature does not verify")

var errTransient = errors.New("release: transient CDN failure")

// SchoolKitIndex implements port.SchoolKitIndex.
type SchoolKitIndex struct {
	base  *url.URL
	key   *rsa.PublicKey
	http  *http.Client
	cache Cache
	exec  failsafe.Executor[[]byte]
}

var _ port.SchoolKitIndex = (*SchoolKitIndex)(nil)

// New builds the adapter; cache may be nil.
func New(cfg Config, client *http.Client, cache Cache) (*SchoolKitIndex, error) {
	u, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("release: EW_CLIENT_RELEASES_BASE_URL must be an absolute URL")
	}
	key, err := ParsePublicKey(cfg.PublicKeyPEM)
	if err != nil {
		return nil, err
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	transient := func(_ []byte, err error) bool { return errors.Is(err, errTransient) }
	cb := circuitbreaker.NewBuilder[[]byte]().HandleIf(transient).WithFailureThresholdRatio(5, 10).WithDelay(30 * time.Second).Build()
	rp := retrypolicy.NewBuilder[[]byte]().HandleIf(transient).WithMaxRetries(2).
		WithBackoff(100*time.Millisecond, time.Second).ReturnLastFailure().Build()
	return &SchoolKitIndex{base: u, key: key, http: client, cache: cache, exec: failsafe.With[[]byte](rp, cb)}, nil
}

// ParsePublicKey reads an RSA public key in PEM (PKIX "PUBLIC KEY" or PKCS#1 "RSA PUBLIC KEY").
func ParsePublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	b, _ := pem.Decode(pemBytes)
	if b == nil {
		return nil, errors.New("release: EW_CLIENT_RELEASE_PUBKEY_FILE is not a PEM public key")
	}
	if k, err := x509.ParsePKIXPublicKey(b.Bytes); err == nil {
		if rk, ok := k.(*rsa.PublicKey); ok && rk.N.BitLen() >= 2048 {
			return rk, nil
		}
		return nil, errors.New("release: the release public key must be RSA ≥ 2048 bits")
	}
	rk, err := x509.ParsePKCS1PublicKey(b.Bytes)
	if err != nil || rk.N.BitLen() < 2048 {
		return nil, errors.New("release: the release public key must be RSA ≥ 2048 bits")
	}
	return rk, nil
}

// Verify checks the base64 RSA-SHA256 signature of body and the schema of the index (exported for tests/tools).
func Verify(key *rsa.PublicKey, body, sig []byte, channel string) (installer.Index, error) {
	var ix installer.Index
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil {
		return ix, ErrSignature
	}
	sum := sha256.Sum256(body)
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], raw); err != nil {
		return ix, ErrSignature
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ix); err != nil {
		return ix, fmt.Errorf("%w: %w", installer.ErrInvalidIndex, err)
	}
	if err := ix.Validate(channel); err != nil {
		return ix, err
	}
	return ix, nil
}

// Load implements port.SchoolKitIndex.
func (s *SchoolKitIndex) Load(ctx context.Context, channel string) (installer.Index, error) {
	key := cachePrefix + channel
	if s.cache != nil {
		if b, ok, err := s.cache.Get(ctx, key); err == nil && ok {
			var ix installer.Index
			if json.Unmarshal(b, &ix) == nil && ix.Validate(channel) == nil {
				return ix, nil
			}
		}
	}
	body, err := s.get(ctx, "school-kit/"+channel+".json", maxIndex)
	if err != nil {
		return installer.Index{}, err
	}
	sig, err := s.get(ctx, "school-kit/"+channel+".json.sig", maxSig)
	if err != nil {
		return installer.Index{}, err
	}
	ix, err := Verify(s.key, body, sig, channel)
	if err != nil {
		return installer.Index{}, err
	}
	if s.cache != nil {
		if b, err := json.Marshal(ix); err == nil {
			_ = s.cache.Set(ctx, key, b, CacheTTL)
		}
	}
	return ix, nil
}

func (s *SchoolKitIndex) get(ctx context.Context, path string, limit int64) ([]byte, error) {
	u := *s.base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + path
	return s.exec.WithContext(ctx).Get(func() ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errTransient, err)
		}
		defer func() { _ = resp.Body.Close() }()
		switch {
		case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
			return nil, fmt.Errorf("%w: %s %d", errTransient, path, resp.StatusCode)
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("release: %s: status %d", path, resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errTransient, err)
		}
		if int64(len(b)) > limit {
			return nil, fmt.Errorf("release: %s larger than %d bytes", path, limit)
		}
		return b, nil
	})
}
