package command

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
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
type UserLifecycle struct {
	UoW    write.UnitOfWork
	Repo   write.LifecycleRepository
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
	return h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
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
		if target.Status == t.to {
			return nil // already there: idempotent, no event
		}
		allowed := false
		for _, f := range t.from {
			allowed = allowed || f == target.Status
		}
		if !allowed {
			return problem.New(problem.Conflict, "the account cannot move from "+target.Status+" to "+t.to)
		}
		if err := h.Repo.SetStatus(ctx, userID, target.Status, t.to); err != nil {
			return err
		}
		by := p.UserID
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
			Metadata: map[string]any{"from": target.Status, "to": t.to, "route": route}}
		if target.StudentSchoolID != nil {
			ev.SchoolID = target.StudentSchoolID.String()
		}
		return h.Audit.Record(ctx, ev)
	})
}
