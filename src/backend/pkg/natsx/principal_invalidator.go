package natsx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// PrincipalCache is implemented by authz.CachedResolver.
type PrincipalCache interface {
	Invalidate(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) error
	InvalidateTenant(ctx context.Context, tenantID uuid.UUID) error
	InvalidateAll(ctx context.Context) error
}

// InvalidationSubjects are the identity events that change a Principal (API_DESIGN §12.2, F003-AC-23).
var InvalidationSubjects = []string{
	"ew.evt.identity.user_role.changed", "ew.evt.identity.role.updated", "ew.evt.identity.tenant_membership.changed",
	"ew.evt.identity.user.access_revoked", "ew.evt.identity.user.status_changed", "ew.evt.identity.student.placement_changed", "ew.evt.identity.tenant.updated",
}

// PrincipalInvalidator clears the Principal cache of this process on identity events. It uses an
// ordered (ephemeral, deliver-new) consumer so every replica sees every event and clears its own L1;
// Valkey deletes are idempotent, so several replicas doing them is harmless.
type PrincipalInvalidator struct {
	JS     jetstream.JetStream
	Cache  PrincipalCache
	Logger *slog.Logger
}

type invalidation struct {
	EventType string     `json:"event_type"`
	TenantID  *uuid.UUID `json:"tenant_id"`
	Payload   struct {
		UserID    *uuid.UUID `json:"user_id"`
		StudentID *uuid.UUID `json:"student_id"`
		TenantID  *uuid.UUID `json:"tenant_id"`
	} `json:"payload"`
}

// Apply invalidates for one envelope (exported for tests).
func (p *PrincipalInvalidator) Apply(ctx context.Context, data []byte) error {
	var e invalidation
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	tenant := e.Payload.TenantID
	if tenant == nil {
		tenant = e.TenantID
	}
	switch e.EventType {
	case "role.updated":
		return p.Cache.InvalidateAll(ctx)
	case "tenant.updated":
		if tenant == nil {
			return errors.New("natsx: tenant.updated without tenant_id")
		}
		return p.Cache.InvalidateTenant(ctx, *tenant)
	case "user.access_revoked", "user.status_changed": // F012-AC-14: status is part of the Principal
		if e.Payload.UserID == nil {
			return errors.New("natsx: " + e.EventType + " without user_id")
		}
		return p.Cache.Invalidate(ctx, *e.Payload.UserID, nil)
	case "user_role.changed", "tenant_membership.changed":
		if e.Payload.UserID == nil {
			return errors.New("natsx: " + e.EventType + " without user_id")
		}
		return p.Cache.Invalidate(ctx, *e.Payload.UserID, tenant)
	case "student.placement_changed": // school/grade of the Principal (ADR-0095); a move may change the tenant: every tenant
		if e.Payload.StudentID == nil {
			return errors.New("natsx: student.placement_changed without student_id")
		}
		return p.Cache.Invalidate(ctx, *e.Payload.StudentID, nil)
	}
	return nil
}

// Run consumes EVT_IDENTITY invalidation subjects until ctx ends.
func (p *PrincipalInvalidator) Run(ctx context.Context) error {
	log := p.Logger
	if log == nil {
		log = slog.Default()
	}
	cons, err := p.JS.OrderedConsumer(ctx, EventStream("identity"), jetstream.OrderedConsumerConfig{
		FilterSubjects: InvalidationSubjects, DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return err
	}
	cc, err := cons.Consume(func(m jetstream.Msg) {
		if err := p.Apply(ctx, m.Data()); err != nil {
			log.WarnContext(ctx, "principal invalidation failed", "subject", m.Subject(), "error", err)
		}
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	cc.Stop()
	return nil
}
