// Package lookup is the contract_v1 adapter of port.SisLookupPort used by login sync branch B/C (F017 US6,
// SIS_INTEGRATION §4.5, ADR-0061): one lookupStudent call with the timeout sis_login_sync_timeout_ms, no
// retry, the record validated against the Student DTO (unknown fields dropped and counted), a bulkhead of
// sis_login_sync_concurrency calls and a circuit breaker per connection so a hanging SIS never slows the
// logins of another connection.
package lookup

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/bulkhead"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Defaults of CONFIG_DEFAULTS: sis_login_sync_timeout_ms, sis_login_sync_concurrency, sis_login_sync_breaker.
const (
	DefaultTimeout          = 3000 * time.Millisecond
	DefaultConcurrency      = 20
	DefaultBreakerThreshold = 5
	DefaultBreakerDelay     = 30 * time.Second
)

// ErrThrottled: the bulkhead of the connection is full or its circuit is open (no call made).
var ErrThrottled = errors.New("sis lookup: throttled")

// ConnectionLoader loads the connection in the scope of the caller (nil, nil when absent).
type ConnectionLoader func(ctx context.Context, id uuid.UUID) (*write.ContractConnectionRow, error)

// Adapter implements port.SisLookupPort for contract_v1 connections.
type Adapter struct {
	Load             ConnectionLoader
	Providers        port.SisProviderFactory
	Timeout          time.Duration
	Concurrency      int
	BreakerThreshold int
	BreakerDelay     time.Duration

	mu     sync.Mutex
	guards map[uuid.UUID]failsafe.Executor[map[string]any]
}

var _ port.SisLookupPort = (*Adapter)(nil)

// counted reports whether an error is a SIS failure for the breaker (not found and unsupported are answers).
func counted(_ map[string]any, err error) bool {
	return err != nil && !errors.Is(err, port.ErrSisLookupNotFound) && !errors.Is(err, context.Canceled) && !errors.Is(err, bulkhead.ErrFull)
}

func (a *Adapter) guard(id uuid.UUID) failsafe.Executor[map[string]any] {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.guards == nil {
		a.guards = map[uuid.UUID]failsafe.Executor[map[string]any]{}
	}
	if g, ok := a.guards[id]; ok {
		return g
	}
	n := a.Concurrency
	if n <= 0 {
		n = DefaultConcurrency
	}
	th := a.BreakerThreshold
	if th <= 0 {
		th = DefaultBreakerThreshold
	}
	delay := a.BreakerDelay
	if delay <= 0 {
		delay = DefaultBreakerDelay
	}
	cb := observability.ObserveCircuit(circuitbreaker.NewBuilder[map[string]any](), "identity", observability.IntegrationSIS).HandleIf(counted).WithFailureThreshold(uint(th)).WithDelay(delay).Build()
	bh := bulkhead.NewBuilder[map[string]any](uint(n)).Build() // no wait: a full bulkhead refuses at once
	g := failsafe.With[map[string]any](cb, bh)
	a.guards[id] = g
	return g
}

// Lookup returns the Student record of login_username at the school of the connection.
func (a *Adapter) Lookup(ctx context.Context, connectionID uuid.UUID, school, username string) (map[string]any, error) {
	c, err := a.Load(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if c == nil || c.ProviderKind != sis.ProviderContractV1 || !c.State.Active() || !sis.HasCapability(c.Capabilities, sis.CapStudentLookup) {
		return nil, port.ErrSisLookupUnsupported // F017-AC-23: no network call
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	lctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rec, err := a.guard(connectionID).WithContext(lctx).Get(func() (map[string]any, error) {
		p, err := a.Providers.For(lctx, port.ContractConnection{ID: c.ID, TenantID: c.TenantID, BaseURL: c.Config.BaseURL, AuthType: c.Config.AuthType,
			TokenURL: c.Config.TokenURL, OAuthScope: c.Config.OAuthScope, SecretRef: c.SecretRef})
		if err != nil {
			return nil, err
		}
		rec, err := p.LookupStudent(lctx, school, username)
		switch {
		case errors.Is(err, port.ErrSisNotFound):
			return nil, port.ErrSisLookupNotFound
		case err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(lctx.Err(), context.DeadlineExceeded)):
			return nil, fmt.Errorf("%w: %w", port.ErrSisLookupTimeout, err)
		}
		return rec, err
	})
	switch {
	case errors.Is(err, bulkhead.ErrFull), errors.Is(err, circuitbreaker.ErrOpen):
		return nil, ErrThrottled
	case err == nil && sis.Str(rec, "student_external_id") == "":
		return nil, port.ErrSisBadResponse
	}
	return rec, err
}
