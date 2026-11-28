package outbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// errPlatformToTenant: a platform event (tenant_id = null) reached a consumer that only handles tenant events.
var errPlatformToTenant = errors.New("platform event delivered to a tenant consumer")

// eventScope is the scope of the consumer transaction for an event of tenant: EventScope of the tenant, or the platform
// scope of the consumer when tenant is nil and the consumer handles platform events (o.Platform).
func eventScope(o ConsumerOptions, tenant *uuid.UUID) (tenancy.Scope, error) {
	if tenant == nil {
		if !o.Platform {
			return nil, errPlatformToTenant
		}
		return authz.SystemPlatformScope("consumer-" + strings.ReplaceAll(strings.ReplaceAll(o.Name, ".", "-"), "_", "-"))
	}
	return authz.EventScope(*tenant)
}

// Replay applies envelopes that stand for the current state of a source row (`worker projection rebuild`, F003-AC-39,
// F003 Clarifications 2026-11-28) through the handler of a declared consumer: one transaction of the consumer's UnitOfWork
// under the scope the consumer uses for an event of tenant (EventScope; platform scope for nil when o.Platform), the
// envelopes in order. Every envelope must belong to tenant. Nothing is written to infra.processed_events (the envelopes
// were never delivered) and nothing is dead-lettered: the first handler error rolls the whole transaction back and is
// returned, so the projection keeps what it had.
func Replay(ctx context.Context, o ConsumerOptions, tenant *uuid.UUID, envs []Envelope) (err error) {
	if o.UoW == nil || o.Handler == nil || o.Module == "" || len(o.Name) <= len(o.Module) {
		return errors.New("outbox: replay needs the module, name, unit of work and handler of a consumer")
	}
	for _, e := range envs {
		if !sameTenant(e.TenantID, tenant) {
			return fmt.Errorf("outbox: replay of %s %s outside the tenant of the transaction", e.EventType, e.EventID)
		}
		if e.EventID == uuid.Nil || e.EventType == "" || e.OccurredAt.IsZero() {
			return fmt.Errorf("outbox: replay of an invalid envelope %q", e.EventType)
		}
	}
	scope, err := eventScope(o, tenant)
	if err != nil {
		return err
	}
	ctx = tenancy.WithModule(ctx, o.Module)
	ctx, done := observability.UseCase(ctx, o.Module+".replay."+o.Name[len(o.Module)+1:])
	defer func() { done(err) }()
	c := &Consumer{o: o}
	return c.inTx(ctx, scope, func(ctx context.Context) error {
		for _, e := range envs {
			if err := o.Handler(ctx, e); err != nil {
				return fmt.Errorf("outbox: replay of %s %s by %s: %w", e.EventType, e.Subject.ID, o.Name, err)
			}
		}
		return nil
	})
}

func sameTenant(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
