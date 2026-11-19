package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// MaxFacadeLimit is the maximum of the facade parameter limit (internal-common Limit).
const MaxFacadeLimit = 200

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

func platformOp(ctx context.Context, perm authz.Permission) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return p, err
	}
	if !p.IsPlatform() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

func limitOf(n int) int {
	switch {
	case n <= 0:
		return 50
	case n > MaxFacadeLimit:
		return MaxFacadeLimit
	}
	return n
}

// Assignment is one active assignment.
type Assignment struct {
	AssignmentID uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	TenantID     uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
}

// AssignmentPage is a page of assignments.
type AssignmentPage struct {
	Data       []Assignment
	NextCursor *string
	HasMore    bool
}

// ListResourceAssignmentsInput is the query of facade listResourceAssignments.
type ListResourceAssignmentsInput struct {
	ResourceType string
	ResourceID   uuid.UUID
	Cursor       string
	Limit        int
}

// ListResourceAssignmentsHandler is the use case behind facade listResourceAssignments.
type ListResourceAssignmentsHandler interface {
	Handle(ctx context.Context, in ListResourceAssignmentsInput) (AssignmentPage, error)
}

// ListResourceAssignments implements ListResourceAssignmentsHandler: System Admin reads the assignments of one
// resource across tenants through tenancy.Bypass(system_admin_cross_tenant) (audit cross_tenant.accessed +
// eduworld_tenant_filter_bypass_total).
type ListResourceAssignments struct {
	Session     read.Session
	Assignments read.AssignmentReadModel
	Cursors     *page.Codec
}

var _ ListResourceAssignmentsHandler = ListResourceAssignments{}

// Handle runs the query.
func (h ListResourceAssignments) Handle(ctx context.Context, in ListResourceAssignmentsInput) (out AssignmentPage, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListResourceAssignments")
	defer func() { done(err) }()
	if _, err := platformOp(ctx, authz.PermPlatformTenantDataAssign); err != nil {
		return out, err
	}
	rt, err := contentmodel.ParseResourceType(in.ResourceType)
	if err != nil {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a resource type of module learning")
	}
	if in.ResourceID == uuid.Nil {
		return out, problem.Invalid("resource_id", problem.SubRequired, "is required")
	}
	after, err := h.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	afterID := uuid.Nil
	if after != nil {
		afterID = after.ID
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonSystemAdminCrossTenant, tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		return out, err
	}
	limit := limitOf(in.Limit)
	err = h.Session.Read(bctx, func(ctx context.Context) error {
		rows, err := h.Assignments.ByResource(ctx, rt, in.ResourceID, afterID, limit)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			c := h.Cursors.Encode(page.Cursor{ID: rows[len(rows)-1].ID})
			out.NextCursor, out.HasMore = &c, true
		}
		for _, a := range rows {
			out.Data = append(out.Data, Assignment{AssignmentID: a.ID, ResourceType: string(a.ResourceType), ResourceID: a.ResourceID,
				TenantID: a.TenantID, AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt})
		}
		return nil
	})
	return out, err
}

// CheckReferenceHandler decides whether the session tenant may reference a learning resource (a question set
// using a question, a lesson using a material…): visible → ok; a platform resource not assigned to the tenant
// → 403 RESOURCE_NOT_ASSIGNED; anything else → 404 (F020-AC-18, API_DESIGN §4.1).
type CheckReferenceHandler interface {
	Handle(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID) error
}

// CheckReference implements CheckReferenceHandler. The platform existence check runs in its own read session
// under SystemPlatformScope("learning-reference-check") (tenant_id IS NULL rows only); it answers a code, never
// row data [Proposed, F020 clarification 2026-11-19].
type CheckReference struct {
	Session   read.Session
	Resources *port.Registry
}

var _ CheckReferenceHandler = CheckReference{}

// Handle runs the check with the scope of ctx (the tenant session).
func (h CheckReference) Handle(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.CheckReference")
	defer func() { done(err) }()
	if _, ok := tenancy.ScopeFrom(ctx); !ok {
		return problem.New(problem.Unauthenticated, "")
	}
	handler, ok := h.Resources.Get(rt)
	if !ok {
		return problem.Invalid("resource_type", problem.SubInvalid, "resource type cannot be referenced yet")
	}
	visible := false
	if err := h.Session.Read(ctx, func(ctx context.Context) error {
		owners, err := handler.Owners(ctx, []uuid.UUID{id})
		_, visible = owners[id]
		return err
	}); err != nil {
		return err
	}
	if visible {
		return nil
	}
	ps, err := authz.SystemPlatformScope("learning-reference-check")
	if err != nil {
		return err
	}
	platform := false
	if err := h.Session.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
		owners, err := handler.Owners(ctx, []uuid.UUID{id})
		_, platform = owners[id]
		return err
	}); err != nil {
		return err
	}
	if platform {
		return problem.New(problem.ResourceNotAssigned, "")
	}
	return problem.New(problem.NotFound, "")
}

// module labels data access and bypass metrics (eduworld_tenant_filter_bypass_total{module}).
const module = "learning"
