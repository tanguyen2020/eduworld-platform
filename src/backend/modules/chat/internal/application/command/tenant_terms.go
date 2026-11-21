package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func eventOf(typ, subjectType, subject string, tenant *uuid.UUID, at time.Time, payload map[string]any) write.Event {
	e := write.Event{Type: typ, SubjectType: subjectType, SubjectID: subject, PartitionKey: subject, OccurredAt: at, Payload: payload}
	if tenant != nil {
		e.TenantID = tenant
	} else {
		e.Platform = true
	}
	return e
}

// CreateTenantTerm implements createTenantSensitiveTerm (Safety Team, chat.tenant_terms.write; X-Tenant-Id = target
// tenant): an extra term of an existing platform category, in force only in that tenant after its next publish
// (F116-AC-16).
func (h CatalogAdmin) CreateTenantTerm(ctx context.Context, in TermInput) (out catalog.StoredTerm, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateTenantSensitiveTerm")
	defer func() { done(err) }()
	p, tenant, sc, err := tenantWrite(ctx, authz.PermChatTenantTermsWrite)
	if err != nil {
		return out, err
	}
	return h.createTerm(ctx, p, sc, &tenant, in)
}

// UpdateTenantTerm implements updateTenantSensitiveTerm (If-Match); a platform term → SENSITIVE_TERM_INVALID.
func (h CatalogAdmin) UpdateTenantTerm(ctx context.Context, id uuid.UUID, expected int64, in TermInput) (out catalog.StoredTerm, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateTenantSensitiveTerm")
	defer func() { done(err) }()
	p, tenant, sc, err := tenantWrite(ctx, authz.PermChatTenantTermsWrite)
	if err != nil {
		return out, err
	}
	return h.updateTerm(ctx, p, sc, &tenant, id, expected, in)
}

// DeleteTenantTerm implements deleteTenantSensitiveTerm (soft-delete); a platform term → SENSITIVE_TERM_INVALID.
func (h CatalogAdmin) DeleteTenantTerm(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DeleteTenantSensitiveTerm")
	defer func() { done(err) }()
	p, tenant, sc, err := tenantWrite(ctx, authz.PermChatTenantTermsWrite)
	if err != nil {
		return err
	}
	return h.deleteTerm(ctx, p, sc, &tenant, id)
}

// TenantCategory is TenantSensitiveCategory: a platform category as seen in a tenant (never a term).
type TenantCategory struct {
	Category        catalog.StoredCategory
	EnabledInTenant bool
	TenantTermCount int
}

// ToggleTenantCategory implements toggleTenantSensitiveCategory (Safety Team/System Admin, chat.tenant_category.toggle,
// X-Tenant-Id = target tenant; F116-AC-17): only a category with allow_tenant_toggle (otherwise 403
// TENANT_TOGGLE_NOT_ALLOWED), a reason of at least 10 characters, audit and chat_policy.tenant_category_toggled in the
// transaction; after the commit the instances reload the catalogue view of the tenant (ew:chat:switches:changed).
func (h CatalogAdmin) ToggleTenantCategory(ctx context.Context, code string, enabled bool, reason string) (out TenantCategory, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ToggleTenantSensitiveCategory")
	defer func() { done(err) }()
	p, tenant, sc, err := tenantWrite(ctx, authz.PermChatTenantCategoryToggle)
	if err != nil {
		return out, err
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) < moderation.ToggleReasonMin {
		return out, problem.Invalid("reason", problem.SubInvalid, "at least 10 characters")
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cat, err := h.Repo.CategoryByCode(ctx, code)
		if err != nil {
			return err
		}
		if cat == nil {
			return notFound()
		}
		if !cat.AllowTenantToggle || catalog.CheckCategoryChange(cat.Code, true, true, false) != nil {
			return problem.New(problem.TenantToggleNotAllowed, "")
		}
		if err := h.Repo.SetTenantCategory(ctx, tenant, cat.ID, enabled, reason, p.UserID); err != nil {
			return err
		}
		n, err := h.Repo.TenantTerms(ctx, tenant, cat.ID)
		if err != nil {
			return err
		}
		out = TenantCategory{Category: *cat, EnabledInTenant: enabled, TenantTermCount: n}
		now := h.now()
		t := tenant
		if err := h.Events.Publish(ctx, eventOf(EventTenantCategory, "sensitive_category", cat.ID.String(), &t, now,
			map[string]any{"tenant_id": tenant, "category_id": cat.ID, "category_code": cat.Code, "enabled": enabled, "reason": reason,
				"actor_id": p.UserID, "occurred_at": now})); err != nil {
			return err
		}
		if err := h.record(ctx, AuditTenantCategory, "sensitive_category", cat.ID, reason, map[string]any{"category_code": cat.Code,
			"enabled": enabled}); err != nil {
			return err
		}
		if h.AfterCommit == nil || h.Notifier == nil {
			return nil
		}
		return h.AfterCommit(ctx, func(ctx context.Context) {
			if err := h.Notifier.SwitchesChanged(context.WithoutCancel(ctx), &t); err != nil {
				slog.WarnContext(ctx, "chat: category change not announced (instances reconcile)", "error", err)
			}
		})
	})
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.TenantToggleNotAllowed { // audited outside the rolled-back transaction
		authz.RecordDenied(ctx, string(problem.TenantToggleNotAllowed), map[string]any{"category_code": code})
	}
	return out, err
}
