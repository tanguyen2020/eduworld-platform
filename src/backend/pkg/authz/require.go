package authz

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Resource is what a use case is about to touch. TenantID nil means a platform-owned row whose
// visibility (e.g. assigned platform content) is already decided by the repository/RLS.
type Resource struct {
	TenantID  *uuid.UUID
	SchoolID  *uuid.UUID
	SubjectID *uuid.UUID
}

// Hooks are wired by the composition root.
type Hooks struct {
	// Recorder receives authz.denied; nil disables the audit (tests, tools).
	Recorder audit.Recorder
}

var (
	hooksMu sync.RWMutex
	hooks   Hooks
)

// Configure installs hooks (once at startup).
func Configure(h Hooks) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	hooks = h
}

func recorder() audit.Recorder {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return hooks.Recorder
}

// Require authorizes perm on res for p, explicitly at the start of every use case (AGENTS R26):
//   - resource outside the tenants visible to the request → 404 NOT_FOUND (no existence leak);
//   - no grant holds perm, or the grants holding it are limited to another school/subject → 403
//     PERMISSION_DENIED (no separate scope code, RB-113), audited as authz.denied.
func Require(ctx context.Context, p Principal, perm Permission, res Resource) error {
	if !p.Valid() {
		return problem.New(problem.Unauthenticated, "")
	}
	if res.TenantID != nil && !tenantVisible(ctx, p, *res.TenantID) {
		return problem.New(problem.NotFound, "")
	}
	held := false
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) {
			continue
		}
		held = true
		if covers(g.SchoolID, res.SchoolID) && covers(g.SubjectID, res.SubjectID) {
			return nil
		}
	}
	reason := "missing_permission"
	if held {
		reason = "outside_grant_scope"
	}
	deny(ctx, p, perm, reason)
	return problem.New(problem.PermissionDenied, "")
}

// covers: a grant without a limit covers everything; a limited grant covers only the same id.
func covers(limit, target *uuid.UUID) bool {
	return limit == nil || (target != nil && *limit == *target)
}

func tenantVisible(ctx context.Context, p Principal, tenant uuid.UUID) bool {
	if p.TenantID != nil {
		return *p.TenantID == tenant
	}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, ok := s.(tenancy.CrossTenantScope); ok {
			return slices.Contains(cs.TenantIDs(), tenant)
		}
	}
	return false
}

func deny(ctx context.Context, p Principal, perm Permission, why string) {
	rec := recorder()
	if rec == nil {
		return
	}
	// A failed denial audit never turns a denial into an allow; the outbox recorder logs its own errors.
	e := deniedEvent(string(problem.PermissionDenied), map[string]any{"permission": string(perm), "why": why, "user_id": p.UserID.String()})
	e.TargetType, e.TargetID = "permission", string(perm)
	_ = rec.Record(ctx, e)
}

func deniedEvent(reason string, metadata map[string]any) audit.Event {
	return audit.Event{Action: audit.ActionAuthzDenied, Result: audit.ResultDenied, Reason: reason, Metadata: metadata}
}
