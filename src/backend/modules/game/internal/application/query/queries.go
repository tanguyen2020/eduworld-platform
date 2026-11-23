// Package query holds the read use cases of module game (ADR-0087): they read read models in a READ ONLY session,
// never write, never publish events.
package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
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
	// Students reads the school and grade of the student (F077-AC-07: the event entries of the tenant manifest are kept
	// for the audience of the student only); nil = no event filter.
	Students port.StudentDirectory
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
		if h.Students == nil {
			return nil
		}
		st, err := h.Students.Student(ctx, tenant, p.UserID)
		if err != nil {
			return err
		}
		school, grade := uuid.Nil, 0
		if st != nil {
			school, grade = st.SchoolID, st.Grade
		}
		out.Entries, out.ETag = filterEvents(m.Entries, m.ETag, school, grade)
		return nil
	})
	return out, err
}

// filterEvents drops the event entries whose audience (schools, grades) does not hold the student (F077-AC-07); the
// ETag of a filtered view names what was dropped (a student of another audience never reuses a cached body).
func filterEvents(entries json.RawMessage, etag string, school uuid.UUID, grade int) (json.RawMessage, string) {
	doc := map[string]json.RawMessage{}
	if len(entries) == 0 || json.Unmarshal(entries, &doc) != nil {
		return entries, etag
	}
	var items []json.RawMessage
	if json.Unmarshal(doc["items"], &items) != nil {
		return entries, etag
	}
	kept := make([]json.RawMessage, 0, len(items))
	var dropped []string
	for _, it := range items {
		var head struct {
			ItemType string `json:"item_type"`
			ItemID   string `json:"item_id"`
			Audience *struct {
				SchoolIDs   []uuid.UUID `json:"school_ids"`
				GradeLevels []int       `json:"grade_levels"`
			} `json:"audience"`
		}
		if json.Unmarshal(it, &head) == nil && head.ItemType == "event" && head.Audience != nil &&
			((len(head.Audience.SchoolIDs) > 0 && !slices.Contains(head.Audience.SchoolIDs, school)) || !slices.Contains(head.Audience.GradeLevels, grade)) {
			dropped = append(dropped, head.ItemID)
			continue
		}
		kept = append(kept, it)
	}
	if len(dropped) == 0 {
		return entries, etag
	}
	b, err := json.Marshal(kept)
	if err != nil {
		return entries, etag
	}
	doc["items"] = b
	out, err := json.Marshal(doc)
	if err != nil {
		return entries, etag
	}
	sum := sha256.Sum256([]byte(strings.Join(dropped, ",")))
	return out, etag + "-" + hex.EncodeToString(sum[:4])
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
		out, err = h.list(ctx, []uuid.UUID{tenant}, false, in)
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
	if !ok || !p.IsSystemAdmin {
		return out, problem.New(problem.PermissionDenied, "")
	}
	var tenants []uuid.UUID
	all := false
	switch s := sc.(type) {
	case tenancy.CrossTenantScope:
		tenants = s.TenantIDs()
	case tenancy.PlatformScope:
		// tenant_ids absent = every tenant (contract TenantIds, MULTITENANCY §4.1 rule 6): an audited bypass with the
		// reason of the closed list (cross_tenant.accessed, ADR-0089), like the other "all tenants" reads.
		if ctx, err = tenancy.Bypass(tenancy.WithModule(ctx, "game"), tenancy.ReasonSystemAdminCrossTenant,
			tenancy.WithMode(tenancy.ModeRead)); err != nil {
			return out, err
		}
		all = true
	default:
		return out, problem.New(problem.PermissionDenied, "")
	}
	err = h.Read.Read(ctx, func(ctx context.Context) error {
		out, err = h.list(ctx, tenants, all, in)
		return err
	})
	return out, err
}

func (h LoginEvents) list(ctx context.Context, tenants []uuid.UUID, all bool, in LoginEventsInput) (LoginEventsPage, error) {
	limit := page.Limit(in.Limit)
	f := read.LoginFilter{TenantIDs: tenants, AllTenants: all, StudentID: in.StudentID, From: in.From, To: in.To, Limit: limit}
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
