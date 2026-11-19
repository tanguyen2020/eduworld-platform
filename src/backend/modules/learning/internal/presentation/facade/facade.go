// Package facade implements the in-process facade of module learning (learningapi.StrictServerInterface,
// generated from specs/openapi/modules/learning.internal.v1.yaml): module identity calls its platform-ops
// operations through pkg/modcall, never over HTTP (ADR-0074, AGENTS R29) — no route of this package is ever
// mounted (F020-AC-19). F020 implements the platform-ops operations (assignment, trash, clone step).
package facade

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Facade serves the platform-ops operations of module learning (F020).
type Facade struct {
	Apply       command.ApplyTenantAssignmentsHandler
	Revoke      command.RevokeTenantAssignmentsHandler
	Assignments query.ListResourceAssignmentsHandler
	Trash       query.ListTrashHandler
	Restore     command.RestoreResourceHandler
	Clone       command.ExecuteCloneStepHandler
	// Idempotency stores the results of the writing operations (infra.idempotency_records, idempotency_record_ttl).
	Idempotency *idempotency.Store
}

// facadeMethod is the method column of idempotency records written by the facade.
const facadeMethod = "FACADE"

// idempotent replays the stored result of (Principal, op, Idempotency-Key of ctx) or runs fn with a pending
// record that the command writes in its transaction (idempotency.SetResult). A key reused with another request
// is 422 IDEMPOTENCY_KEY_REUSED.
func idempotent[T any](ctx context.Context, store *idempotency.Store, op string, req any, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	raw, ok := modcall.IdempotencyKeyFrom(ctx)
	if !ok {
		return zero, problem.New(problem.IdempotencyKeyRequired, "")
	}
	p, ok := authz.PrincipalFrom(ctx)
	if !ok {
		return zero, problem.New(problem.Unauthenticated, "")
	}
	key, issued, err := idempotency.ParseKey(raw, store.Clock().Now(), store.TTL())
	if err != nil {
		return zero, err
	}
	return replayOr(ctx, store, idempotency.Key{UserID: p.UserID, Method: facadeMethod, Route: "learning." + op, Key: key, IssuedOn: issued}, req, fn)
}

func replayOr[T any](ctx context.Context, store *idempotency.Store, k idempotency.Key, req any, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	body, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	hash := idempotency.Hash(body)
	replay := func() (T, bool, error) {
		rec, err := store.Lookup(ctx, k)
		if err != nil || rec == nil {
			return zero, false, err
		}
		if rec.Hash != hash {
			return zero, false, problem.New(problem.IdempotencyKeyReused, "")
		}
		var v T
		if err := json.Unmarshal(rec.Body, &v); err != nil {
			return zero, false, err
		}
		return v, true, nil
	}
	if v, found, err := replay(); err != nil || found {
		return v, err
	}
	v, err := fn(idempotency.WithPending(ctx, store.NewPending(k, hash, 200)))
	if errors.Is(err, idempotency.ErrDuplicate) { // a concurrent call with the same key committed first
		if v, found, rerr := replay(); rerr == nil && found {
			return v, nil
		}
	}
	return v, err
}

// stepKey is the idempotency key of a clone step: (job_id, resource_type) whatever the caller's
// Idempotency-Key, dated by the job id when it is a UUIDv7 (F020-AC-27).
func stepKey(job uuid.UUID, resourceType string) idempotency.Key {
	day := time.Unix(0, 0).UTC()
	if job.Version() == 7 {
		sec, nsec := job.Time().UnixTime()
		t := time.Unix(sec, nsec).UTC()
		day = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	return idempotency.Key{UserID: job, Method: facadeMethod, Route: "learning.executeTenantCloneStep:" + resourceType, Key: job, IssuedOn: day}
}
