package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/breakglass"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/oncall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ShiftPage is a page of listModerationOncallShifts.
type ShiftPage struct {
	Data []read.OncallShiftRow
	Page page.Info
}

// GrantPage is a page of listBreakGlassGrants.
type GrantPage struct {
	Data []read.GrantRow
	Page page.Info
}

// BreakGlassMessagePage is a page of listBreakGlassMessages with the grant it was read under (the presentation records
// the view through command.Safety.RecordBreakGlassView before answering).
type BreakGlassMessagePage struct {
	Data  []read.BreakGlassMessage
	Page  page.Info
	Grant read.GrantRow
}

// SafetyOps are the read use cases of the Safety Team operations (F124): the on-call shifts (platform data) and the
// break-glass grants and messages of the System Admin. The break-glass reads lift the tenant filter ONLY here, through
// tenancy.Bypass(ctx, "break_glass") — audited (cross_tenant.accessed) and counted
// (eduworld_tenant_filter_bypass_total{reason="break_glass"}) — with an explicit tenant condition in SQL (R27).
type SafetyOps struct {
	Read    ReadSession
	Oncall  read.OncallReader
	Grants  read.BreakGlassReader
	Cursors *page.Codec
	Now     func() time.Time
}

func (q SafetyOps) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

// keyset decodes a cursor whose key is an RFC 3339 instant and whose id breaks ties.
func (q SafetyOps) keyset(cursor string) (*time.Time, *uuid.UUID, error) {
	if cursor == "" || q.Cursors == nil {
		return nil, nil, nil
	}
	cur, err := q.Cursors.Decode(cursor)
	if err != nil || cur == nil {
		return nil, nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, cur.Key)
	if err != nil {
		return nil, nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	id := cur.ID
	return &at, &id, nil
}

func (q SafetyOps) next(at time.Time, id uuid.UUID) *string {
	if q.Cursors == nil {
		return nil
	}
	s := q.Cursors.Encode(page.Cursor{Key: at.UTC().Format(time.RFC3339Nano), ID: id})
	return &s
}

// ListOncallShifts implements listModerationOncallShifts (F124-AC-16): moderation.case.read, platform staff; shifts
// meeting [from, to), one role; keyset (starts_at, shift_id), LIMIT n+1 in SQL.
func (q SafetyOps) ListOncallShifts(ctx context.Context, from, to *time.Time, role string, limit int, cursor string) (out ShiftPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListOncallShifts")
	defer func() { done(err) }()
	if _, err := moderator(ctx, authz.PermModerationCaseRead); err != nil {
		return out, err
	}
	if role != "" && !slices.Contains(oncall.Roles, role) {
		return out, problem.Invalid("oncall_role", problem.SubInvalid, "primary or secondary")
	}
	if from != nil && to != nil && !to.After(*from) {
		return out, problem.Invalid("to", problem.SubInvalid, "must be after from")
	}
	limit = page.Limit(limit)
	at, id, err := q.keyset(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.OncallShiftRow
	if err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Oncall.ListShifts(ctx, read.ShiftFilter{From: from, To: to, Role: role, AfterStart: at, AfterID: id, Limit: limit})
		return err
	}); err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.Start, last.ID), true
	}
	out.Data = rows
	return out, nil
}

// glass lifts the tenant filter for one break-glass read (reason break_glass, audited, counted).
func glass(ctx context.Context, tenants ...uuid.UUID) (context.Context, error) {
	return tenancy.Bypass(tenancy.WithModule(ctx, "chat"), tenancy.ReasonBreakGlass, tenancy.WithTenantIDs(tenants...))
}

// glassReader authorizes a break-glass read: platform staff holding one of perms (safety_moderator and tenant roles → 403
// PERMISSION_DENIED).
func glassReader(ctx context.Context, perms ...authz.Permission) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, problem.New(problem.Unauthenticated, "")
	}
	allowed := false
	for _, perm := range perms {
		allowed = allowed || p.Has(perm)
	}
	if !allowed || p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(perms[0]), "why": "break_glass"})
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// ListBreakGlassGrants implements listBreakGlassGrants (F124-AC-20): platform.break_glass.approve sees every request
// (requester, reason, scope, messages viewed) for the audit after the fact; platform.break_glass.request alone sees its own.
func (q SafetyOps) ListBreakGlassGrants(ctx context.Context, tenants []uuid.UUID, status string, limit int, cursor string) (out GrantPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListBreakGlassGrants")
	defer func() { done(err) }()
	p, err := glassReader(ctx, authz.PermPlatformBreakGlassApprove, authz.PermPlatformBreakGlassRequest)
	if err != nil {
		return out, err
	}
	if status != "" && !slices.Contains(breakglass.Statuses, status) {
		return out, problem.Invalid("status", problem.SubInvalid, "unknown status")
	}
	if len(tenants) > maxTenantFilter {
		return out, problem.Invalid("tenant_ids", problem.SubInvalid, "too many tenant ids")
	}
	limit = page.Limit(limit)
	at, id, err := q.keyset(cursor)
	if err != nil {
		return out, err
	}
	f := read.GrantFilter{TenantIDs: tenants, Status: status, BeforeAt: at, BeforeID: id, Limit: limit}
	if !p.Has(authz.PermPlatformBreakGlassApprove) {
		me := p.UserID
		f.RequestedBy = &me
	}
	bctx, err := glass(ctx, tenants...)
	if err != nil {
		return out, err
	}
	var rows []read.GrantRow
	if err = q.Read.Read(bctx, func(ctx context.Context) error {
		rows, err = q.Grants.ListGrants(ctx, f)
		return err
	}); err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.RequestedAt, last.ID), true
	}
	out.Data = rows
	return out, nil
}

// ListBreakGlassMessages implements listBreakGlassMessages (F124-AC-19, AC-22): only the requester of an approved,
// unexpired grant reads the messages of its scope (channel, or messages sent by the user) in the target tenant and
// [window_from, window_to); anyone else, an expired/revoked/pending grant → 403 BREAK_GLASS_NOT_ACTIVE (audited denial).
// Purged gameplay messages are gone and are never restored from evidence. Keyset (created_at, message_id), LIMIT n+1.
func (q SafetyOps) ListBreakGlassMessages(ctx context.Context, id uuid.UUID, limit int, cursor string) (out BreakGlassMessagePage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListBreakGlassMessages")
	defer func() { done(err) }()
	p, err := glassReader(ctx, authz.PermPlatformBreakGlassRequest)
	if err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	at, after, err := q.keyset(cursor)
	if err != nil {
		return out, err
	}
	bctx, err := glass(ctx)
	if err != nil {
		return out, err
	}
	now := q.now()
	var rows []read.BreakGlassMessage
	err = q.Read.Read(bctx, func(ctx context.Context) error {
		g, err := q.Grants.Grant(ctx, id)
		if err != nil {
			return err
		}
		if g == nil {
			return problem.New(problem.NotFound, "")
		}
		dg := breakglass.Grant{Status: g.Status, RequestedBy: g.RequestedBy, ExpiresAt: g.ExpiresAt}
		if !dg.ReadableBy(p.UserID, now) {
			authz.RecordDenied(ctx, string(problem.BreakGlassNotActive), map[string]any{"grant_id": id.String(), "status": g.Status})
			return problem.New(problem.BreakGlassNotActive, "")
		}
		out.Grant = *g
		rows, err = q.Grants.Messages(ctx, read.MessageFilter{TenantID: g.TenantID, ScopeType: g.ScopeType, ScopeID: g.ScopeID,
			From: g.WindowFrom, To: g.WindowTo, AfterAt: at, AfterID: after, Limit: limit})
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.CreatedAt, last.MessageID), true
	}
	out.Data = rows
	return out, nil
}
