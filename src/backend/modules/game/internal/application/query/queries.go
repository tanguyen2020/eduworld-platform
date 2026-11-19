// Package query holds the read use cases of module game (ADR-0087): they read read models in a READ ONLY session,
// never write, never publish events.
package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

// ---------- client-config (F014 US4) ----------

// ClientConfigSource is the public configuration of the game client (FR-GAME-008; CONFIG_DEFAULTS
// min_client_version, supported_locales, game_remember_username). Never a client id, secret or OIDC setting.
type ClientConfigSource struct {
	MinClientVersion        string
	LatestClientVersion     string
	LoginURL                string // full URL of POST /auth/game/login ("" = omitted)
	RememberUsernameDefault bool
	SupportedLocales        []string
	Maintenance             func() Maintenance
}

// Maintenance is the maintenance flag of client-config.
type Maintenance struct {
	Active  bool
	Message string
	Until   *time.Time
}

// ClientConfig is the response of getClientConfig.
type ClientConfig struct {
	MinClientVersion        string
	LatestClientVersion     string
	UpdateRequired          bool
	Maintenance             Maintenance
	LoginURL                string
	RememberUsernameDefault bool
	SupportedLocales        []string
}

// GetClientConfig implements getClientConfig (F014-AC-19, AC-21): public, no token and no X-Tenant-Id.
func (s ClientConfigSource) GetClientConfig(ctx context.Context, platform, version string) (out ClientConfig, err error) {
	_, done := observability.UseCase(ctx, "game.query.GetClientConfig")
	defer func() { done(err) }()
	if !gamesession.ValidPlatform(platform) {
		return out, problem.Invalid("platform", problem.SubInvalid, "unknown platform")
	}
	latest := s.LatestClientVersion
	if latest == "" {
		latest = s.MinClientVersion
	}
	out = ClientConfig{MinClientVersion: s.MinClientVersion, LatestClientVersion: latest,
		UpdateRequired: !gamesession.Supported(version, s.MinClientVersion), LoginURL: s.LoginURL,
		RememberUsernameDefault: s.RememberUsernameDefault, SupportedLocales: s.SupportedLocales}
	if s.Maintenance != nil {
		out.Maintenance = s.Maintenance()
	}
	return out, nil
}

// ---------- content manifest (F014 US5) ----------

// ContentManifest is the response of getContentManifest.
type ContentManifest struct {
	TenantID         uuid.UUID
	ManifestVersion  int64
	ETag             string
	GeneratedAt      *time.Time
	MinClientVersion string
	Entries          json.RawMessage // items, bundles, areas of the stored manifest; nil = empty manifest
}

// GetContentManifest reads the latest content manifest of the session tenant (F014-AC-22, AC-23).
type GetContentManifest struct {
	Read             read.Session
	Manifests        read.ManifestReadModel
	MinClientVersion string
}

// EmptyETag is the ETag of the empty manifest of a tenant (manifest_version 0): stable per tenant.
func EmptyETag(tenant uuid.UUID) string {
	sum := sha256.Sum256([]byte("empty-manifest:" + tenant.String()))
	return "m0-" + hex.EncodeToString(sum[:8])
}

// Handle returns the manifest of the tenant of the Principal; a tenant without published content gets an empty
// manifest with manifest_version 0. Another tenant's manifest is never readable (TenantScope + RLS).
func (h GetContentManifest) Handle(ctx context.Context) (out ContentManifest, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetContentManifest")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	tenant := *p.TenantID
	out = ContentManifest{TenantID: tenant, ETag: EmptyETag(tenant), MinClientVersion: h.MinClientVersion}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		m, err := h.Manifests.Latest(ctx, tenant)
		if err != nil || m == nil {
			return err
		}
		at := m.GeneratedAt
		out.ManifestVersion, out.ETag, out.GeneratedAt, out.Entries = m.Version, m.ETag, &at, m.Entries
		return nil
	})
	return out, err
}

// ---------- login history (F014 US2) ----------

// LoginEvents lists the game login history (listTenantStudentLoginEvents, listLoginEventsCrossTenant).
type LoginEvents struct {
	Read     read.Session
	Events   read.LoginEventReadModel
	Students read.StudentReadModel
	Cursors  *page.Codec
}

// LoginEventsInput are the filters of both operations.
type LoginEventsInput struct {
	StudentID *uuid.UUID
	From, To  *time.Time
	Cursor    string
	Limit     int
}

// LoginEventsPage is one page of history.
type LoginEventsPage struct {
	Data []read.LoginEvent
	Page page.Info
}

// ForStudent implements listTenantStudentLoginEvents (F014-AC-12): lms.student_activity.read in the tenant of the
// session; a student of another tenant, or of a school outside the caller's grants, is 404 NOT_FOUND.
func (h LoginEvents) ForStudent(ctx context.Context, student uuid.UUID, in LoginEventsInput) (out LoginEventsPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListTenantStudentLoginEvents")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	tenant := *p.TenantID
	if !p.Has(authz.PermLmsStudentActivityRead) {
		// no grant holds the permission: 403 PERMISSION_DENIED, audited by Require
		return out, authz.Require(ctx, p, authz.PermLmsStudentActivityRead, authz.Resource{TenantID: &tenant})
	}
	in.StudentID = &student
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		st, err := h.Students.Student(ctx, tenant, student)
		if err != nil {
			return err
		}
		if st == nil || !schoolCovered(p, st.SchoolID) {
			return problem.New(problem.NotFound, "")
		}
		out, err = h.list(ctx, []uuid.UUID{tenant}, in)
		return err
	})
	return out, err
}

// schoolCovered: a grant holding the permission without a school limit, or limited to the student's school.
func schoolCovered(p authz.Principal, school uuid.UUID) bool {
	for _, g := range p.Grants {
		for _, perm := range g.Permissions {
			if perm == authz.PermLmsStudentActivityRead && (g.SchoolID == nil || *g.SchoolID == school) {
				return true
			}
		}
	}
	return false
}

// CrossTenant implements listLoginEventsCrossTenant (F014-AC-13): System Admin only, under the CrossTenantScope
// of tenant_ids (audited as cross_tenant.accessed by the read session); every row carries its tenant_id.
func (h LoginEvents) CrossTenant(ctx context.Context, in LoginEventsInput) (out LoginEventsPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListLoginEventsCrossTenant")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformCrossTenantRead, authz.Resource{}); err != nil {
		return out, err
	}
	sc, ok := tenancy.ScopeFrom(ctx)
	cs, cross := sc.(tenancy.CrossTenantScope)
	if !ok || !cross || !p.IsSystemAdmin {
		return out, problem.New(problem.PermissionDenied, "")
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		out, err = h.list(ctx, cs.TenantIDs(), in)
		return err
	})
	return out, err
}

func (h LoginEvents) list(ctx context.Context, tenants []uuid.UUID, in LoginEventsInput) (LoginEventsPage, error) {
	limit := page.Limit(in.Limit)
	f := read.LoginFilter{TenantIDs: tenants, StudentID: in.StudentID, From: in.From, To: in.To, Limit: limit}
	cur, err := h.Cursors.Decode(in.Cursor)
	if err != nil {
		return LoginEventsPage{}, problem.New(problem.BadRequest, "invalid cursor")
	}
	if cur != nil {
		at, err := time.Parse(time.RFC3339Nano, cur.Key)
		if err != nil {
			return LoginEventsPage{}, problem.New(problem.BadRequest, "invalid cursor")
		}
		f.AfterAt, f.AfterID = &at, cur.ID
	}
	rows, err := h.Events.List(ctx, f)
	if err != nil {
		return LoginEventsPage{}, err
	}
	out := LoginEventsPage{Data: rows, Page: page.Info{Limit: limit}}
	if len(rows) > limit {
		out.Data = rows[:limit]
		last := out.Data[limit-1]
		next := h.Cursors.Encode(page.Cursor{Key: last.LoginAt.UTC().Format(time.RFC3339Nano), ID: last.LoginEventID})
		out.Page.HasMore, out.Page.NextCursor = true, &next
	}
	if out.Data == nil {
		out.Data = []read.LoginEvent{}
	}
	return out, nil
}
