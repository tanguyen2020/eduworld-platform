package command

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Events and audit actions of the catalogue administration (F116 US2). Neither carries a term, a variant, a note or an
// allowlist phrase (R14, R15, F116-AC-19).
const (
	EventSensitiveTermCreated = "sensitive_term.created"
	EventSensitiveTermUpdated = "sensitive_term.updated"
	EventSensitiveTermDeleted = "sensitive_term.deleted"
	EventCatalogPublished     = "sensitive_catalog.published"
	EventTenantCategory       = "chat_policy.tenant_category_toggled"

	AuditCategoryCreated  = "chat.sensitive_category.created"
	AuditCategoryUpdated  = "chat.sensitive_category.updated"
	AuditCategoryDeleted  = "chat.sensitive_category.deleted"
	AuditTermCreated      = "chat.sensitive_term.created"
	AuditTermUpdated      = "chat.sensitive_term.updated"
	AuditTermDeleted      = "chat.sensitive_term.deleted"
	AuditAllowCreated     = "chat.sensitive_allowlist.created"
	AuditAllowDeleted     = "chat.sensitive_allowlist.deleted"
	AuditCatalogPublished = "chat.sensitive_catalog.published"
	AuditTenantCategory   = "chat.tenant_category.toggled"
)

// CatalogAdmin are the write use cases of the sensitive-term catalogue (F116 US2, MODERATION §11): platform categories,
// terms and allowlist (System Admin, chat.sensitive_terms.write), the extra terms of a tenant (Safety Team,
// chat.tenant_terms.write), the tenant category toggles (chat.tenant_category.toggle) and the publishes. Every write
// stays a draft until the next publish; each use case authorizes first (authz.Require), then runs in one UnitOfWork
// with its outbox event and audit.
type CatalogAdmin struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.CatalogRepository
	Events      write.EventPublisher
	Audit       write.Auditor
	Regression  port.RegressionSuite
	Notifier    port.CatalogNotifier
	// OnPublished sets eduworld_chat_sensitive_catalog_version{scope} (nil: none).
	OnPublished func(scope string, version int64)
	Now         func() time.Time
}

func (h CatalogAdmin) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// CategoryInput is SensitiveTermCategoryInput.
type CategoryInput struct {
	Code              string
	Name              string
	Description       *string
	Severity          string
	DefaultAction     string
	Enabled           bool
	AllowTenantToggle bool
}

// TermInput is SensitiveTermInput.
type TermInput struct {
	CategoryID       uuid.UUID
	Term             string
	MatchType        string
	Locale           string
	Variants         []string
	SeverityOverride *string
	ActionOverride   *string
	Enabled          bool
	Note             *string
}

// AllowInput is SensitiveAllowlistInput.
type AllowInput struct {
	Phrase string
	Locale string
	Reason *string
}

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

// platformWrite authorizes a write of the platform catalogue and returns its scope: the platform rows (tenant_id NULL)
// are written by chat_platform. A System Admin who selected a tenant (X-Tenant-Id) still writes the platform rows.
func platformWrite(ctx context.Context, perm authz.Permission) (authz.Principal, tenancy.Scope, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, nil, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return p, nil, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return p, nil, problem.New(problem.PermissionDenied, "")
	}
	d := p.PrincipalData
	d.TenantID = nil
	sc, err := authz.PlatformScope(authz.NewPrincipal(d))
	if err != nil {
		return p, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return p, sc, nil
}

// tenantWrite authorizes a write for the target tenant of X-Tenant-Id (AGENTS R20: a platform write on tenant data
// always has one target tenant) and returns the tenant scope.
func tenantWrite(ctx context.Context, perm authz.Permission) (authz.Principal, uuid.UUID, tenancy.Scope, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, uuid.Nil, nil, err
	}
	if p.TenantID == nil {
		return p, uuid.Nil, nil, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant := *p.TenantID
	if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: &tenant}); err != nil {
		return p, uuid.Nil, nil, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return p, uuid.Nil, nil, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return p, uuid.Nil, nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return p, tenant, sc, nil
}

// termProblem maps the domain validation of a term to SENSITIVE_TERM_INVALID (field and rule, never the term).
func termProblem(err error) error {
	var it catalog.ErrInvalidTerm
	if errors.As(err, &it) {
		return problem.New(problem.SensitiveTermInvalid, "").AddField(it.Field, problem.SubInvalid, it.Reason)
	}
	if errors.Is(err, write.ErrDuplicate) {
		return problem.New(problem.Conflict, "the entry already exists")
	}
	if errors.Is(err, write.ErrStale) {
		return versionMismatch()
	}
	return err
}

func versionMismatch() error { return problem.New(problem.VersionMismatch, "") }

func (h CatalogAdmin) record(ctx context.Context, action, targetType string, target uuid.UUID, reason string, meta map[string]any) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: targetType, TargetID: target.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: meta})
}

// ---------- categories (System Admin)

func (in CategoryInput) validate() error {
	if !catalog.ValidSeverity(in.Severity) {
		return problem.Invalid("severity", problem.SubInvalid, "unknown severity")
	}
	if !catalog.ValidAction(in.DefaultAction) {
		return problem.Invalid("default_action", problem.SubInvalid, "unknown action")
	}
	if strings.TrimSpace(in.Name) == "" {
		return problem.Invalid("name", problem.SubInvalid, "required")
	}
	if err := catalog.CheckCategoryChange(in.Code, in.Enabled, in.AllowTenantToggle, false); err != nil {
		return problem.Invalid("enabled", problem.SubInvalid, "the grooming category always runs and cannot be toggled (ADR-0078)")
	}
	return nil
}

// CreateCategory implements createSensitiveCategory.
func (h CatalogAdmin) CreateCategory(ctx context.Context, in CategoryInput) (out catalog.StoredCategory, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateSensitiveCategory")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return out, err
	}
	if err := in.validate(); err != nil {
		return out, err
	}
	out = catalog.StoredCategory{ID: ids.New(), Code: in.Code, Name: in.Name, Description: in.Description, Severity: in.Severity,
		DefaultAction: in.DefaultAction, Enabled: in.Enabled, AllowTenantToggle: in.AllowTenantToggle, Version: 1, UpdatedAt: h.now()}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if err := h.Repo.InsertCategory(ctx, out, p.UserID); err != nil {
			return termProblem(err)
		}
		return h.record(ctx, AuditCategoryCreated, "sensitive_category", out.ID, "", map[string]any{"category_code": out.Code,
			"severity": out.Severity})
	})
	return out, err
}

// UpdateCategory implements updateSensitiveCategory (If-Match): the code is immutable (the engine and the tenants refer
// to it); the grooming category stays enabled and not toggleable.
func (h CatalogAdmin) UpdateCategory(ctx context.Context, id uuid.UUID, expected int64, in CategoryInput) (out catalog.StoredCategory, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateSensitiveCategory")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.Category(ctx, id)
		if err != nil {
			return err
		}
		if cur == nil {
			return notFound()
		}
		if cur.Version != expected {
			return versionMismatch()
		}
		if in.Code != cur.Code {
			return problem.Invalid("code", problem.SubInvalid, "the code of a category cannot change")
		}
		if err := in.validate(); err != nil {
			return err
		}
		out = *cur
		out.Name, out.Description, out.Severity, out.DefaultAction = in.Name, in.Description, in.Severity, in.DefaultAction
		out.Enabled, out.AllowTenantToggle, out.Version, out.UpdatedAt = in.Enabled, in.AllowTenantToggle, cur.Version+1, h.now()
		if err := h.Repo.UpdateCategory(ctx, out, p.UserID); err != nil {
			return termProblem(err)
		}
		return h.record(ctx, AuditCategoryUpdated, "sensitive_category", id, "", map[string]any{"category_code": out.Code,
			"severity": out.Severity, "enabled": out.Enabled, "allow_tenant_toggle": out.AllowTenantToggle})
	})
	return out, err
}

// DeleteCategory implements deleteSensitiveCategory: soft-delete (R21) of a category without live terms; the grooming
// category cannot be removed (ADR-0078).
func (h CatalogAdmin) DeleteCategory(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DeleteSensitiveCategory")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return err
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.Category(ctx, id)
		if err != nil {
			return err
		}
		if cur == nil {
			return notFound()
		}
		if catalog.CheckCategoryChange(cur.Code, true, false, true) != nil {
			return problem.Invalid("category_id", problem.SubInvalid, "the grooming category cannot be removed (ADR-0078)")
		}
		n, err := h.Repo.LiveTerms(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return problem.New(problem.Conflict, "the category still has terms")
		}
		if err := h.Repo.SoftDeleteCategory(ctx, id, p.UserID); err != nil {
			return err
		}
		return h.record(ctx, AuditCategoryDeleted, "sensitive_category", id, "", map[string]any{"category_code": cur.Code})
	})
}

// ---------- terms (platform: System Admin; tenant: Safety Team)

func (in TermInput) term(id uuid.UUID, tenant *uuid.UUID) catalog.StoredTerm {
	return catalog.StoredTerm{ID: id, TenantID: tenant, CategoryID: in.CategoryID, Term: in.Term, MatchType: in.MatchType, Locale: in.Locale,
		Variants: in.Variants, SeverityOverride: in.SeverityOverride, ActionOverride: in.ActionOverride, Enabled: in.Enabled,
		Status: catalog.StatusDraft, Note: in.Note, Version: 1}
}

// checkTerm validates t in the transaction: rules of the domain, a live existing category (a tenant only adds terms to
// the platform categories, MODERATION §11.3), at most MaxTenantPatterns patterns in the scope.
func (h CatalogAdmin) checkTerm(ctx context.Context, t catalog.StoredTerm) (*catalog.StoredCategory, error) {
	if err := catalog.ValidateTerm(t); err != nil {
		return nil, termProblem(err)
	}
	cat, err := h.Repo.Category(ctx, t.CategoryID)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, problem.New(problem.SensitiveTermInvalid, "").AddField("category_id", problem.SubInvalid, "unknown category")
	}
	if t.MatchType == catalog.MatchPattern {
		n, err := h.Repo.Patterns(ctx, t.TenantID, t.ID)
		if err != nil {
			return nil, err
		}
		if n >= moderation.MaxTenantPatterns {
			return nil, problem.New(problem.SensitiveTermInvalid, "").AddField("match_type", problem.SubInvalid, "at most 200 patterns")
		}
	}
	return cat, nil
}

func (h CatalogAdmin) termEvent(ctx context.Context, typ string, t catalog.StoredTerm, cat *catalog.StoredCategory, actor uuid.UUID) error {
	payload := map[string]any{"tenant_id": t.TenantID, "term_id": t.ID, "category_id": t.CategoryID, "category_code": cat.Code,
		"scope": t.Scope(), "match_type": t.MatchType, "locale": t.Locale, "actor_id": actor, "occurred_at": h.now()}
	e := write.Event{Type: typ, SubjectType: "sensitive_term", SubjectID: t.ID.String(), PartitionKey: t.ID.String(), OccurredAt: h.now(),
		Payload: payload}
	if t.TenantID != nil {
		e.TenantID = t.TenantID
	} else {
		e.Platform = true
	}
	if err := h.Events.Publish(ctx, e); err != nil {
		return err
	}
	action := map[string]string{EventSensitiveTermCreated: AuditTermCreated, EventSensitiveTermUpdated: AuditTermUpdated,
		EventSensitiveTermDeleted: AuditTermDeleted}[typ]
	return h.record(ctx, action, "sensitive_term", t.ID, "", map[string]any{"category_code": cat.Code, "scope": t.Scope(),
		"match_type": t.MatchType, "locale": t.Locale})
}

func (h CatalogAdmin) createTerm(ctx context.Context, p authz.Principal, sc tenancy.Scope, tenant *uuid.UUID, in TermInput) (catalog.StoredTerm, error) {
	t := in.term(ids.New(), tenant)
	t.CreatedAt, t.UpdatedAt = h.now(), h.now()
	err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cat, err := h.checkTerm(ctx, t)
		if err != nil {
			return err
		}
		if err := h.Repo.InsertTerm(ctx, t, p.UserID); err != nil {
			return termProblem(err)
		}
		return h.termEvent(ctx, EventSensitiveTermCreated, t, cat, p.UserID)
	})
	return t, err
}

// loadTerm returns the term of the scope; a platform term reached through the tenant routes is SENSITIVE_TERM_INVALID
// (a tenant never edits the platform catalogue, F116-AC-16).
func (h CatalogAdmin) loadTerm(ctx context.Context, tenant *uuid.UUID, id uuid.UUID) (*catalog.StoredTerm, error) {
	cur, err := h.Repo.Term(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, notFound()
	}
	if tenant != nil && cur.TenantID == nil {
		return nil, problem.New(problem.SensitiveTermInvalid, "").AddField("term_id", problem.SubInvalid, "a platform term is not a tenant term")
	}
	return cur, nil
}

func (h CatalogAdmin) updateTerm(ctx context.Context, p authz.Principal, sc tenancy.Scope, tenant *uuid.UUID, id uuid.UUID, expected int64,
	in TermInput) (catalog.StoredTerm, error) {
	var out catalog.StoredTerm
	err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.loadTerm(ctx, tenant, id)
		if err != nil {
			return err
		}
		if cur.Version != expected {
			return versionMismatch()
		}
		out = in.term(id, tenant)
		out.Version, out.CreatedAt, out.UpdatedAt = cur.Version+1, cur.CreatedAt, h.now()
		cat, err := h.checkTerm(ctx, out)
		if err != nil {
			return err
		}
		if err := h.Repo.UpdateTerm(ctx, out, p.UserID); err != nil {
			return termProblem(err)
		}
		return h.termEvent(ctx, EventSensitiveTermUpdated, out, cat, p.UserID)
	})
	return out, err
}

func (h CatalogAdmin) deleteTerm(ctx context.Context, p authz.Principal, sc tenancy.Scope, tenant *uuid.UUID, id uuid.UUID) error {
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.loadTerm(ctx, tenant, id)
		if err != nil {
			return err
		}
		cat, err := h.Repo.Category(ctx, cur.CategoryID)
		if err != nil {
			return err
		}
		if cat == nil {
			cat = &catalog.StoredCategory{ID: cur.CategoryID, Code: "unknown"}
		}
		if err := h.Repo.SoftDeleteTerm(ctx, tenant, id, p.UserID); err != nil {
			return err
		}
		return h.termEvent(ctx, EventSensitiveTermDeleted, *cur, cat, p.UserID)
	})
}

// CreateTerm implements createSensitiveTerm (platform term, draft).
func (h CatalogAdmin) CreateTerm(ctx context.Context, in TermInput) (out catalog.StoredTerm, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateSensitiveTerm")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return out, err
	}
	return h.createTerm(ctx, p, sc, nil, in)
}

// UpdateTerm implements updateSensitiveTerm (If-Match; back to draft until the next publish).
func (h CatalogAdmin) UpdateTerm(ctx context.Context, id uuid.UUID, expected int64, in TermInput) (out catalog.StoredTerm, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateSensitiveTerm")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return out, err
	}
	return h.updateTerm(ctx, p, sc, nil, id, expected, in)
}

// DeleteTerm implements deleteSensitiveTerm (soft-delete, in force from the next version).
func (h CatalogAdmin) DeleteTerm(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DeleteSensitiveTerm")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return err
	}
	return h.deleteTerm(ctx, p, sc, nil, id)
}

// ---------- allowlist (platform only: a tenant only tightens, MODERATION §11.3)

// CreateAllow implements createSensitiveAllowlistEntry.
func (h CatalogAdmin) CreateAllow(ctx context.Context, in AllowInput) (out catalog.StoredAllow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateSensitiveAllowlistEntry")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return out, err
	}
	if strings.TrimSpace(in.Phrase) == "" {
		return out, problem.Invalid("phrase", problem.SubInvalid, "required")
	}
	if in.Locale != catalog.LocaleVI && in.Locale != catalog.LocaleEN {
		return out, problem.Invalid("locale", problem.SubInvalid, "must be vi or en")
	}
	out = catalog.StoredAllow{ID: ids.New(), Phrase: in.Phrase, Locale: in.Locale, Reason: in.Reason, CreatedAt: h.now()}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if err := h.Repo.InsertAllow(ctx, out, p.UserID); err != nil {
			return termProblem(err)
		}
		return h.record(ctx, AuditAllowCreated, "sensitive_allowlist", out.ID, "", map[string]any{"locale": out.Locale})
	})
	return out, err
}

// DeleteAllow implements deleteSensitiveAllowlistEntry (soft-delete).
func (h CatalogAdmin) DeleteAllow(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DeleteSensitiveAllowlistEntry")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return err
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Repo.Allow(ctx, id)
		if err != nil {
			return err
		}
		if cur == nil {
			return notFound()
		}
		if err := h.Repo.SoftDeleteAllow(ctx, id, p.UserID); err != nil {
			return err
		}
		return h.record(ctx, AuditAllowDeleted, "sensitive_allowlist", id, "", map[string]any{"locale": cur.Locale})
	})
}
