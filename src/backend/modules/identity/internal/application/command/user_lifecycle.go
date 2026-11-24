package command

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Lifecycle actions.
const (
	LifecycleLock       = "lock"
	LifecycleUnlock     = "unlock"
	LifecycleDeactivate = "deactivate"
)

// Routes of the lifecycle use cases.
const (
	LifecycleRouteTenant   = "tenant"   // lockTenantUser/unlockTenantUser (tenant.user.lock, students only)
	LifecycleRoutePlatform = "platform" // lockUser/unlockUser/deactivateUser (platform.user.manage)
)

// UserLifecycle implements lockTenantUser, unlockTenantUser, lockUser, unlockUser and deactivateUser (F016
// US1, ACCOUNT_PROVISIONING §6): status change, history (reason visible to staff only), audit and
// user.status_changed + user.access_revoked / user.access_restored in the same transaction. The consumers
// of those events disable Keycloak, revoke game sessions (F013) and end game sessions (F014).
// Locks have a source (F124, identity.account_locks): the tenant route locks/unlocks as school_admin, the platform
// route as platform_admin; an unlock releases the caller's source only and the account stays locked while another lock
// remains — 409 ACCOUNT_LOCK_HELD_BY_SAFETY when a lock of the Safety Team remains (F124-AC-05), 409 CONFLICT otherwise.
type UserLifecycle struct {
	UoW    write.UnitOfWork
	Repo   write.LifecycleRepository
	Locks  write.AccountLockRepository
	Events write.EventPublisher
	Audit  write.Auditor
}

type transition struct {
	from                             []string
	to, revoke, restore, auditAction string
}

var transitions = map[string]transition{
	LifecycleLock:       {from: []string{"active", "pending"}, to: "locked", revoke: "locked", auditAction: "user.locked"},
	LifecycleUnlock:     {from: []string{"locked"}, to: "active", restore: "unlocked", auditAction: "user.unlocked"},
	LifecycleDeactivate: {from: []string{"active", "pending", "locked"}, to: "deactivated", revoke: "deactivated", auditAction: "user.deactivated"},
}

// Change applies action to userID through route.
func (h UserLifecycle) Change(ctx context.Context, route, action string, userID uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UserLifecycle."+action)
	defer func() { done(err) }()
	t, ok := transitions[action]
	if !ok || (route == LifecycleRouteTenant && action == LifecycleDeactivate) {
		return problem.New(problem.Internal, "unknown lifecycle action")
	}
	if n := len(strings.TrimSpace(reason)); n < 3 || n > 500 {
		return problem.Invalid("reason", problem.SubInvalid, "3..500 characters")
	}
	if h.Locks == nil {
		return problem.New(problem.Internal, "account locks are not wired")
	}
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	var sc tenancy.Scope
	switch route {
	case LifecycleRouteTenant:
		if p.TenantID == nil {
			return problem.New(problem.PermissionDenied, "")
		}
		if sc, err = authz.ScopeFor(p); err != nil {
			return problem.New(problem.PermissionDenied, "").Wrap(err)
		}
	default:
		if !p.IsSystemAdmin {
			if err := authz.Require(ctx, p, authz.PermPlatformUserManage, authz.Resource{}); err != nil {
				return err
			}
		}
		if sc, err = authz.SystemPlatformScope("user-lifecycle"); err != nil {
			return err
		}
	}
	var held error // still locked by another source: committed, then answered 409
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		target, err := h.Repo.Target(ctx, userID)
		if err != nil {
			return err
		}
		if target == nil {
			return problem.New(problem.NotFound, "")
		}
		tenant := target.StudentTenantID
		if tenant == nil {
			tenant = target.PrimaryTenantID
		}
		if route == LifecycleRouteTenant {
			// Students of the session tenant only (RLS hides the others → 404); staff → 403 (F016-AC-05).
			if target.UserType != string(authz.UserTypeStudent) {
				if target.StudentTenantID == nil && (target.PrimaryTenantID == nil || *target.PrimaryTenantID != *p.TenantID) {
					return problem.New(problem.NotFound, "")
				}
				return problem.New(problem.PermissionDenied, "")
			}
			if target.StudentTenantID == nil || *target.StudentTenantID != *p.TenantID {
				return problem.New(problem.NotFound, "")
			}
			if err := authz.Require(ctx, p, authz.PermTenantUserLock, authz.Resource{TenantID: p.TenantID, SchoolID: target.StudentSchoolID}); err != nil {
				return err
			}
		}
		by := p.UserID
		source := user.SourceOfRoute(route)
		now := time.Now().UTC()
		switch action {
		case LifecycleLock:
			if target.Status != t.to && !slices.Contains(t.from, target.Status) {
				return problem.New(problem.Conflict, "the account cannot move from "+target.Status+" to "+t.to)
			}
			why := reason
			added, err := h.Locks.Add(ctx, write.AccountLock{TenantID: tenant, UserID: userID, Source: source, Reason: &why, CreatedBy: &by,
				CreatedAt: now})
			if err != nil {
				return err
			}
			if target.Status == t.to { // already locked by another source: one more lock, no status event
				if !added {
					return nil
				}
				return h.Audit.Record(ctx, audit.Event{Action: t.auditAction, TargetType: "user", TargetID: userID.String(), Result: audit.ResultSuccess,
					Reason: reason, Metadata: map[string]any{"from": target.Status, "to": t.to, "route": route, "lock_source": source}})
			}
		case LifecycleUnlock:
			if _, err := h.Locks.ReleaseSource(ctx, userID, source, &by, reason, nil, now); err != nil {
				return err
			}
			if target.Status != "locked" {
				return nil // already active: idempotent, no event
			}
			remaining, err := h.Locks.ActiveSources(ctx, userID)
			if err != nil {
				return err
			}
			if out := user.AfterRelease(remaining); !out.Active {
				held = problem.New(problem.Conflict, "the account is still locked by another source")
				if out.HeldBySafety {
					held = problem.New(problem.AccountLockHeldBySafety, "")
				}
				return h.Audit.Record(ctx, audit.Event{Action: "user.lock_released", TargetType: "user", TargetID: userID.String(),
					Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"route": route, "lock_source": source, "status": "locked"}})
			}
		}
		if target.Status == t.to {
			return nil // already there: idempotent, no event
		}
		if !slices.Contains(t.from, target.Status) {
			return problem.New(problem.Conflict, "the account cannot move from "+target.Status+" to "+t.to)
		}
		if err := h.Repo.SetStatus(ctx, userID, target.Status, t.to); err != nil {
			return err
		}
		if err := h.Repo.RecordStatusChange(ctx, write.StatusChange{UserID: userID, TenantID: tenant, From: target.Status, To: t.to,
			Reason: reason, Source: "staff", By: &by}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "user.status_changed", SubjectType: "user", SubjectID: userID.String(),
			TenantID: tenant, Platform: tenant == nil, Payload: map[string]any{"user_id": userID, "from": target.Status, "to": t.to,
				"reason": "staff_" + action}}); err != nil {
			return err
		}
		switch {
		case t.revoke != "":
			if err := h.Events.Publish(ctx, write.Event{Type: "user.access_revoked", SubjectType: "user", SubjectID: userID.String(),
				TenantID: tenant, Platform: tenant == nil, Payload: map[string]any{"user_id": userID, "reason": t.revoke, "tenant_id": tenant}}); err != nil {
				return err
			}
		case t.restore != "":
			if err := h.Events.Publish(ctx, write.Event{Type: "user.access_restored", SubjectType: "user", SubjectID: userID.String(),
				TenantID: tenant, Platform: tenant == nil, Payload: map[string]any{"user_id": userID, "reason": t.restore, "tenant_id": tenant}}); err != nil {
				return err
			}
		}
		ev := audit.Event{Action: t.auditAction, TargetType: "user", TargetID: userID.String(), Result: audit.ResultSuccess, Reason: reason,
			Metadata: map[string]any{"from": target.Status, "to": t.to, "route": route, "lock_source": source}}
		if target.StudentSchoolID != nil {
			ev.SchoolID = target.StudentSchoolID.String()
		}
		return h.Audit.Record(ctx, ev)
	})
	if err != nil {
		return err
	}
	return held
}
