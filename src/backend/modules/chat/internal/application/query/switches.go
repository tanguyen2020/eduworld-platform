package query

import (
	"bytes"
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// maxPending bounds the pending requests listed per level (ModerationSwitchState.pending_requests maxItems).
const maxPending = 50

// Switches are the read use cases of the moderation switches (F116 US3): System Admin only (chat.policy.toggle,
// internal gateway); the platform level, and the tenants of a cross-tenant read (tenant_ids → CrossTenantScope,
// audited). Read session only (R26).
type Switches struct {
	Read       ReadSession
	Reader     read.SwitchAdminReader
	Moderation read.ModerationReader
	Cursors    *page.Codec
	Now        func() time.Time
}

func (q Switches) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

// SwitchLevel is ModerationSwitchState of one level with its pending requests.
type SwitchLevel struct {
	TenantID   *uuid.UUID
	Configured *read.SwitchRow
	Effective  moderation.Effective
	Pending    []read.SwitchRequestRow
}

// Body renders the level (ModerationSwitchState with pending_requests).
func (l SwitchLevel) Body() map[string]any {
	return service.StateBody(l.TenantID, l.Configured, l.Effective, l.Pending)
}

// SwitchOverview is the response of listModerationSwitches: the platform level and one keyset page of the tenant levels.
type SwitchOverview struct {
	Platform SwitchLevel
	Tenants  []SwitchLevel
	Page     page.Info
}

// SwitchHistoryPage is a page of listModerationSwitchHistory.
type SwitchHistoryPage struct {
	Data []read.SwitchChangeRow
	Page page.Info
}

func switchReader(ctx context.Context) error {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermChatPolicyToggle, authz.Resource{}); err != nil {
		return err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return problem.New(problem.PermissionDenied, "")
	}
	return nil
}

// ListModerationSwitches implements listModerationSwitches: the platform level and one page of the tenants of tenant_ids
// (System Admin CrossTenantScope, audited; none without tenant_ids) with the configured and effective values, the steps
// off and the requests waiting for approval. The tenants are paged in tenant_id order after the cursor and only the
// tenants of the page are read from the database (API_DESIGN §5, owner rule 2026-11-23).
func (q Switches) ListModerationSwitches(ctx context.Context, limit int, cursor string) (out SwitchOverview, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListModerationSwitches")
	defer func() { done(err) }()
	if err := switchReader(ctx); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	cur, err := q.Cursors.Decode(cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	scoped := append([]uuid.UUID(nil), crossTenants(ctx)...)
	slices.SortFunc(scoped, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	scoped = slices.Compact(scoped)
	var tenants []uuid.UUID
	for _, t := range scoped {
		if cur == nil || bytes.Compare(t[:], cur.ID[:]) > 0 {
			tenants = append(tenants, t)
		}
	}
	out.Page = page.Info{Limit: limit}
	if len(tenants) > limit {
		tenants = tenants[:limit]
		next := q.Cursors.Encode(page.Cursor{ID: tenants[limit-1]})
		out.Page.HasMore, out.Page.NextCursor = true, &next
	}
	now := q.now()
	var platform *read.SwitchRow
	rows := map[uuid.UUID]*read.SwitchRow{}
	var platformPending, tenantPending []read.SwitchRequestRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if platform, _, err = q.Moderation.Switches(ctx, uuid.Nil); err != nil {
			return err
		}
		if platformPending, err = q.Reader.PendingRequests(ctx, nil, now, maxPending); err != nil {
			return err
		}
		if len(tenants) == 0 {
			return nil
		}
		if rows, err = q.Reader.TenantSwitches(ctx, tenants); err != nil {
			return err
		}
		tenantPending, err = q.Reader.PendingRequests(ctx, tenants, now, maxPending*len(tenants))
		return err
	})
	if err != nil {
		return out, err
	}
	out.Platform = SwitchLevel{Configured: platform, Effective: service.EffectiveOf(platform, nil, now),
		Pending: append([]read.SwitchRequestRow{}, platformPending...)}
	byTenant := map[uuid.UUID][]read.SwitchRequestRow{}
	for _, r := range tenantPending {
		if r.TenantID != nil && len(byTenant[*r.TenantID]) < maxPending {
			byTenant[*r.TenantID] = append(byTenant[*r.TenantID], r)
		}
	}
	out.Tenants = make([]SwitchLevel, 0, len(tenants))
	for _, t := range tenants {
		t := t
		pending := byTenant[t]
		if pending == nil {
			pending = []read.SwitchRequestRow{}
		}
		out.Tenants = append(out.Tenants, SwitchLevel{TenantID: &t, Configured: rows[t], Effective: service.EffectiveOf(platform, rows[t], now),
			Pending: pending})
	}
	return out, nil
}

// ListModerationSwitchHistory implements listModerationSwitchHistory: the changes of the platform level, or of the
// tenants of tenant_ids; newest first.
func (q Switches) ListModerationSwitchHistory(ctx context.Context, limit int, cursor string) (out SwitchHistoryPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListModerationSwitchHistory")
	defer func() { done(err) }()
	if err := switchReader(ctx); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	var after *read.Cursor
	if cursor != "" && q.Cursors != nil {
		cur, err := q.Cursors.Decode(cursor)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		if after, err = timeCursor(cur); err != nil {
			return out, err
		}
	}
	var rows []read.SwitchChangeRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Reader.History(ctx, crossTenants(ctx), after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit && q.Cursors != nil {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		s := q.Cursors.Encode(page.Cursor{Key: last.ChangedAt.UTC().Format(time.RFC3339Nano), ID: last.ID})
		out.Page.NextCursor, out.Page.HasMore = &s, true
	}
	if rows == nil {
		rows = []read.SwitchChangeRow{}
	}
	out.Data = rows
	return out, nil
}
