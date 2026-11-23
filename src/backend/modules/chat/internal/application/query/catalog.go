package query

import (
	"context"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// AuditTermsTested is the audit of testSensitiveTerms: viewing the catalogue content (plan §Bảo mật).
const AuditTermsTested = "chat.sensitive_terms.tested"

// ScopedReadSession is the read session with an explicit scope (testSensitiveTerms on the catalogue of one tenant).
type ScopedReadSession interface {
	ReadSession
	InTenantRead(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// Catalog are the read use cases of the sensitive-term catalogue (F116 US2): read session only, no write, no event
// (R26). The catalogue content (terms, allowlist) is returned only to platform staff holding chat.sensitive_terms.read
// on the internal gateway; a tenant role sees the categories and their state, never a term (F116-AC-18).
type Catalog struct {
	Read       ScopedReadSession
	Reader     read.CatalogAdminReader
	Moderation read.ModerationReader
	Engine     *moderation.Engine
	Audit      audit.Recorder
	Cursors    *page.Codec
}

// TermPage is a page of terms.
type TermPage struct {
	Data []catalog.StoredTerm
	Page page.Info
}

// AllowPage is a page of allowlist entries.
type AllowPage struct {
	Data []catalog.StoredAllow
	Page page.Info
}

// VersionPage is a page of catalogue versions.
type VersionPage struct {
	Data []catalog.StoredVersion
	Page page.Info
}

func readPrincipal(ctx context.Context, tenant *uuid.UUID) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, problem.New(problem.Unauthenticated, "")
	}
	return p, authz.Require(ctx, p, authz.PermChatSensitiveTermsRead, authz.Resource{TenantID: tenant})
}

// platformStaff authorizes the platform catalogue reads (internal gateway: the platform guard already ran).
func platformStaff(ctx context.Context) (authz.Principal, error) {
	p, err := readPrincipal(ctx, nil)
	if err != nil {
		return p, err
	}
	if !p.HasPlatformRole() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

func (q Catalog) decode(cursor string) (*page.Cursor, error) {
	if q.Cursors == nil {
		return nil, nil
	}
	cur, err := q.Cursors.Decode(cursor)
	if err != nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	return cur, nil
}

func timeCursor(cur *page.Cursor) (*read.Cursor, error) {
	if cur == nil {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, cur.Key)
	if err != nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	return &read.Cursor{At: at, ID: cur.ID}, nil
}

func (q Catalog) next(at time.Time, id uuid.UUID) *string {
	if q.Cursors == nil {
		return nil
	}
	s := q.Cursors.Encode(page.Cursor{Key: at.UTC().Format(time.RFC3339Nano), ID: id})
	return &s
}

// CategoryPage is a page of listSensitiveCategories.
type CategoryPage struct {
	Data []read.CategoryRow
	Page page.Info
}

// categoryCursor decodes the cursor of a category page ((code, category_id), owner rule 2026-11-23: paged in SQL).
func (q Catalog) categoryCursor(cursor string) (*read.CategoryCursor, error) {
	cur, err := q.decode(cursor)
	if err != nil || cur == nil {
		return nil, err
	}
	return &read.CategoryCursor{Code: cur.Key, ID: cur.ID}, nil
}

func (q Catalog) nextCategory(code string, id uuid.UUID) *string {
	if q.Cursors == nil {
		return nil
	}
	s := q.Cursors.Encode(page.Cursor{Key: code, ID: id})
	return &s
}

// ListCategories implements listSensitiveCategories: one keyset page (code, category_id) read in SQL.
func (q Catalog) ListCategories(ctx context.Context, limit int, cursor string) (out CategoryPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSensitiveCategories")
	defer func() { done(err) }()
	if _, err := platformStaff(ctx); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	after, err := q.categoryCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.CategoryRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Reader.Categories(ctx, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.nextCategory(last.Code, last.ID), true
	}
	if rows == nil {
		rows = []read.CategoryRow{}
	}
	out.Data = rows
	return out, nil
}

// crossTenants returns the tenants of a System Admin cross-tenant read (tenant_ids → CrossTenantScope); nil = platform.
func crossTenants(ctx context.Context) []uuid.UUID {
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if cs, ok := s.(tenancy.CrossTenantScope); ok {
			return cs.TenantIDs()
		}
	}
	return nil
}

// TermFilter is the filter of listSensitiveTerms / listTenantSensitiveTerms, applied in SQL (F122 T063).
type TermFilter struct {
	Category  *uuid.UUID
	Text      string
	Locale    string
	MatchType string
	Status    string
}

func (q Catalog) terms(ctx context.Context, tenants []uuid.UUID, f TermFilter, limit int, cursor string) (TermPage, error) {
	var out TermPage
	limit = page.Limit(limit)
	cur, err := q.decode(cursor)
	if err != nil {
		return out, err
	}
	after, err := timeCursor(cur)
	if err != nil {
		return out, err
	}
	var rows []catalog.StoredTerm
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Reader.Terms(ctx, read.TermQuery{Tenants: tenants, CategoryID: f.Category, Text: f.Text, Locale: f.Locale,
			MatchType: f.MatchType, Status: f.Status, After: after, Limit: limit + 1})
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.CreatedAt, last.ID), true
	}
	out.Data = rows
	return out, nil
}

// ListTerms implements listSensitiveTerms: the platform terms, or the extra terms of the tenants of a System Admin
// cross-tenant read (tenant_ids); category, text, locale, match_type and status filter in SQL, keyset kept (F122 T063).
func (q Catalog) ListTerms(ctx context.Context, f TermFilter, limit int, cursor string) (out TermPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSensitiveTerms")
	defer func() { done(err) }()
	if _, err := platformStaff(ctx); err != nil {
		return out, err
	}
	return q.terms(ctx, crossTenants(ctx), f, limit, cursor)
}

// ListTenantTerms implements listTenantSensitiveTerms: the extra terms of the target tenant (X-Tenant-Id).
func (q Catalog) ListTenantTerms(ctx context.Context, category *uuid.UUID, limit int, cursor string) (out TermPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListTenantSensitiveTerms")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || p.TenantID == nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant := *p.TenantID
	if _, err := readPrincipal(ctx, &tenant); err != nil {
		return out, err
	}
	if !p.HasPlatformRole() {
		return out, problem.New(problem.PermissionDenied, "")
	}
	return q.terms(ctx, []uuid.UUID{tenant}, TermFilter{Category: category}, limit, cursor)
}

// ListAllowlist implements listSensitiveAllowlist (platform entries).
func (q Catalog) ListAllowlist(ctx context.Context, limit int, cursor string) (out AllowPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSensitiveAllowlist")
	defer func() { done(err) }()
	if _, err := platformStaff(ctx); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	cur, err := q.decode(cursor)
	if err != nil {
		return out, err
	}
	after, err := timeCursor(cur)
	if err != nil {
		return out, err
	}
	var rows []catalog.StoredAllow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Reader.Allowlist(ctx, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.CreatedAt, last.ID), true
	}
	out.Data = rows
	return out, nil
}

// ListVersions implements listSensitiveCatalogVersions: platform versions, or the versions of the tenants of a System
// Admin cross-tenant read; newest first.
func (q Catalog) ListVersions(ctx context.Context, limit int, cursor string) (out VersionPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListSensitiveCatalogVersions")
	defer func() { done(err) }()
	if _, err := platformStaff(ctx); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	cur, err := q.decode(cursor)
	if err != nil {
		return out, err
	}
	var before int64
	if cur != nil {
		if before, err = strconv.ParseInt(cur.Key, 10, 64); err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
	}
	var rows []catalog.StoredVersion
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err = q.Reader.Versions(ctx, crossTenants(ctx), before, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit && q.Cursors != nil {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		s := q.Cursors.Encode(page.Cursor{Key: strconv.FormatInt(last.Version, 10), ID: last.ID})
		out.Page.NextCursor, out.Page.HasMore = &s, true
	}
	out.Data = rows
	return out, nil
}

// TestMatch is one catalogue match of testSensitiveTerms (rune offsets in the NFC text).
type TestMatch struct {
	CategoryCode string
	TermID       uuid.UUID
	Severity     string
	Action       string
	Start, End   int
}

// TestResult is SensitiveTermTestResult.
type TestResult struct {
	Decision   string
	MaskedText *string
	Matches    []TestMatch
}

// TestInput is SensitiveTermTestRequest.
type TestInput struct {
	Text          string
	TenantID      *uuid.UUID
	IncludeDrafts bool
}

// TestTerms implements testSensitiveTerms (F116-AC-13): the platform catalogue (and the extra terms and category
// settings of a tenant when given), drafts included or the published versions, compiled in memory and run with the real
// engine on the text, every switchable step on. Nothing is written about the text: no message, no decision log; the
// call is audited as a view of the catalogue content (metadata only, never the text or a term).
func (q Catalog) TestTerms(ctx context.Context, in TestInput) (out TestResult, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.TestSensitiveTerms")
	defer func() { done(err) }()
	p, err := platformStaff(ctx)
	if err != nil {
		return out, err
	}
	if n := utf8.RuneCountInString(in.Text); n == 0 || n > 500 {
		return out, problem.Invalid("text", problem.SubInvalid, "must have 1 to 500 characters")
	}
	run := func(ctx context.Context, fn func(ctx context.Context) error) error { return q.Read.Read(ctx, fn) }
	if in.TenantID != nil {
		t := *in.TenantID
		var sc tenancy.Scope
		switch {
		case p.TenantID != nil && *p.TenantID != t:
			return out, problem.New(problem.NotFound, "")
		case p.TenantID != nil:
			if sc, err = authz.ScopeFor(p); err != nil {
				return out, problem.New(problem.PermissionDenied, "").Wrap(err)
			}
		default:
			if sc, err = authz.CrossTenantScope(p, []uuid.UUID{t}, tenancy.ReasonSystemAdminCrossTenant); err != nil {
				return out, problem.New(problem.PermissionDenied, "").Wrap(err)
			}
		}
		run = func(ctx context.Context, fn func(ctx context.Context) error) error {
			return q.Read.InTenantRead(ctx, sc, fn)
		}
	}
	var platform catalog.Snapshot
	var tenantSnap *catalog.Snapshot
	var settings map[uuid.UUID]bool
	err = run(ctx, func(ctx context.Context) error {
		load := func(tenant *uuid.UUID) (*catalog.Snapshot, error) {
			if in.IncludeDrafts {
				s, _, err := q.Reader.LiveSnapshot(ctx, tenant)
				return &s, err
			}
			row, err := q.Moderation.LatestCatalog(ctx, tenant, 0)
			if err != nil || row == nil {
				return nil, err
			}
			s, err := catalog.ParseSnapshot(row.Snapshot)
			return &s, err
		}
		ps, err := load(nil)
		if err != nil {
			return err
		}
		if ps == nil {
			return problem.New(problem.Unavailable, "no platform catalogue").Retry()
		}
		platform = *ps
		if in.TenantID != nil {
			if tenantSnap, err = load(in.TenantID); err != nil {
				return err
			}
			settings, err = q.Moderation.TenantCategorySettings(ctx, *in.TenantID)
		}
		return err
	})
	if err != nil {
		return out, err
	}
	n := q.Engine.Normalizer()
	parts := []*catalog.Compiled{}
	pc, _ := catalog.Compile(n, platform)
	parts = append(parts, pc)
	if tenantSnap != nil {
		tc, _ := catalog.Compile(n, *tenantSnap)
		parts = append(parts, tc)
	}
	view := q.Engine.View(in.Text)
	o := q.Engine.Evaluate(moderation.Input{View: view, Strictness: moderation.StrictnessStandard,
		Switches: moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}, Categories: catalog.NewLookup(platform.Categories, settings),
		Catalog: parts})
	out.Decision = o.Decision
	if out.Decision == moderation.DecisionUnchecked {
		out.Decision = moderation.DecisionAllow
	}
	if o.Decision == moderation.DecisionMask {
		d := o.Display
		out.MaskedText = &d
	}
	out.Matches = make([]TestMatch, 0, len(o.Hits))
	for _, h := range o.Hits {
		out.Matches = append(out.Matches, TestMatch{CategoryCode: h.CategoryCode, TermID: h.TermID, Severity: h.Severity, Action: h.Action,
			Start: utf8.RuneCountInString(view.Text[:h.Start]), End: utf8.RuneCountInString(view.Text[:h.End])})
	}
	target := "platform"
	if in.TenantID != nil {
		target = in.TenantID.String()
	}
	if q.Audit != nil {
		if err := q.Audit.Record(ctx, audit.Event{Action: AuditTermsTested, TargetType: "sensitive_catalog", TargetID: target,
			Result: audit.ResultSuccess, Metadata: map[string]any{"include_drafts": in.IncludeDrafts, "match_count": len(out.Matches),
				"decision": out.Decision}}); err != nil {
			return TestResult{}, err
		}
	}
	return out, nil
}

// TenantCategories is the response of listTenantSensitiveCategories.
type TenantCategories struct {
	Data     []TenantCategoryView
	Page     page.Info
	Switches SwitchState
}

// TenantCategoryView is TenantSensitiveCategory: never a term.
type TenantCategoryView struct {
	Code              string
	Name              string
	Description       *string
	Severity          string
	EnabledPlatform   bool
	EnabledInTenant   bool
	AllowTenantToggle bool
	TenantTermCount   int
}

// SwitchState is ModerationSwitchState of a tenant (read only; the switch commands are US3).
type SwitchState struct {
	TenantID   uuid.UUID
	Configured *read.SwitchRow
	Effective  moderation.Effective
}

// ListTenantCategories implements listTenantSensitiveCategories (/tenant/chat/categories; tenant roles holding
// chat.sensitive_terms.read, F116-AC-18): the platform categories with their state in the tenant and the effective
// moderation switches (banner); never a term, an allowlist entry or a compiled catalogue.
func (q Catalog) ListTenantCategories(ctx context.Context, limit int, cursor string) (out TenantCategories, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListTenantSensitiveCategories")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || p.TenantID == nil {
		return out, problem.New(problem.TenantHeaderRequired, "")
	}
	tenant := *p.TenantID
	if _, err := readPrincipal(ctx, &tenant); err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	after, err := q.categoryCursor(cursor)
	if err != nil {
		return out, err
	}
	var rows []read.TenantCategoryRow
	var platformSw, tenantSw *read.SwitchRow
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if rows, err = q.Reader.TenantCategories(ctx, tenant, after, limit+1); err != nil {
			return err
		}
		platformSw, tenantSw, err = q.Moderation.Switches(ctx, tenant)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		out.Page.NextCursor, out.Page.HasMore = q.nextCategory(last.Code, last.ID), true
	}
	out.Data = make([]TenantCategoryView, 0, len(rows))
	for _, r := range rows {
		in := r.Enabled
		if r.InTenant != nil && r.AllowTenantToggle {
			in = *r.InTenant
		}
		out.Data = append(out.Data, TenantCategoryView{Code: r.Code, Name: r.Name, Description: r.Description, Severity: r.Severity,
			EnabledPlatform: r.Enabled, EnabledInTenant: in, AllowTenantToggle: r.AllowTenantToggle, TenantTermCount: r.TenantTermCount})
	}
	e := moderation.Effective{Platform: service.ToSwitch(platformSw), At: time.Now()}
	if tenantSw != nil {
		t := service.ToSwitch(tenantSw)
		e.Tenant = &t
	}
	out.Switches = SwitchState{TenantID: tenant, Configured: tenantSw, Effective: e}
	return out, nil
}
