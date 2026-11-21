package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// PlatformAdmin serves the F045 reads: the trash of every tenant (System Admin) and of the session tenant,
// the assignments of a platform resource, clone jobs and merges. Data of other modules is read through their
// facade (OwnerServiceGateway), never their database (R13, R29).
type PlatformAdmin struct {
	Model   read.PlatformAdminReadModel
	Owners  port.OwnerServiceGateway
	Cursors *page.Codec
}

// TrashInput is the query of listTrash / listTenantTrash (TenantIDs is ignored for a tenant user, AC-02).
type TrashInput struct {
	ResourceType string
	TenantIDs    []uuid.UUID
	Cursor       string
	Limit        int
}

// TrashItemView is TrashItem of the API.
type TrashItemView struct {
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     *uuid.UUID `json:"tenant_id"`
	Title        string     `json:"title"`
	DeletedAt    time.Time  `json:"deleted_at"`
	DeletedBy    uuid.UUID  `json:"deleted_by"`
	DeleteReason string     `json:"delete_reason,omitempty"`
	Restorable   bool       `json:"restorable"`
}

// TrashPageView is the response of the trash listings.
type TrashPageView struct {
	Data []TrashItemView `json:"data"`
	Page page.Info       `json:"page"`
}

func platformRead(ctx context.Context, perm authz.Permission) (context.Context, authz.Principal, error) {
	p, err := authenticated(ctx)
	if err != nil {
		return ctx, p, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return ctx, p, err
	}
	if !p.IsPlatform() {
		return ctx, p, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return ctx, p, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), p, nil
}

// ListTrash lists the trash of the tenants of tenant_ids (empty = every active tenant and the platform data)
// for System Admin: an explicit CrossTenantScope recorded as cross_tenant.accessed (F045-AC-01, AC-19).
func (q PlatformAdmin) ListTrash(ctx context.Context, in TrashInput) (out TrashPageView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTrash")
	defer func() { done(err) }()
	pctx, p, err := platformRead(ctx, authz.PermPlatformTrashRead)
	if err != nil {
		return out, err
	}
	module, err := platformadmin.Owner(in.ResourceType)
	if err != nil {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "unknown resource type")
	}
	if len(in.TenantIDs) > 100 {
		return out, problem.Invalid("tenant_ids", problem.SubInvalid, "at most 100 tenants")
	}
	tenants := in.TenantIDs
	if len(tenants) == 0 {
		if tenants, err = q.Model.ActiveTenantIDs(pctx); err != nil {
			return out, err
		}
	}
	cctx := pctx
	if len(tenants) > 0 {
		sc, err := authz.CrossTenantScope(p, tenants, tenancy.ReasonSystemAdminCrossTenant)
		if err != nil {
			return out, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		if err := tenancy.RecordCrossTenantScope(tenancy.WithModule(ctx, platformadmin.ModuleIdentity), sc, tenancy.ModeRead); err != nil {
			return out, err
		}
		cctx = tenancy.WithScope(ctx, sc)
	}
	if module == platformadmin.ModuleIdentity {
		return q.accounts(ctx, in, tenants)
	}
	// tenant_ids empty: the owning module lists every tenant plus the platform data (System Admin).
	res, err := q.Owners.ListTrash(cctx, module, port.TrashQuery{ResourceType: in.ResourceType, TenantIDs: in.TenantIDs, Cursor: in.Cursor,
		Limit: page.Limit(in.Limit)})
	if err != nil {
		return out, err
	}
	return trashView(res, page.Limit(in.Limit)), nil
}

// ListTenantTrash lists the trash of the session tenant (tenant.trash.read, F045-AC-22).
func (q PlatformAdmin) ListTenantTrash(ctx context.Context, in TrashInput) (out TrashPageView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantTrash")
	defer func() { done(err) }()
	p, err := authenticated(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermTenantTrashRead, authz.Resource{}); err != nil {
		return out, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	module, err := platformadmin.Owner(in.ResourceType)
	if err != nil {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "unknown resource type")
	}
	tenants := []uuid.UUID{*p.TenantID} // tenant_ids of a tenant user is ignored (AC-02)
	if module == platformadmin.ModuleIdentity {
		return q.accounts(ctx, in, tenants)
	}
	res, err := q.Owners.ListTrash(tenancy.WithScope(ctx, sc), module, port.TrashQuery{ResourceType: in.ResourceType, TenantIDs: tenants,
		Cursor: in.Cursor, Limit: page.Limit(in.Limit)})
	if err != nil {
		return out, err
	}
	return trashView(res, page.Limit(in.Limit)), nil
}

func trashView(res port.TrashPage, limit int) TrashPageView {
	out := TrashPageView{Data: make([]TrashItemView, 0, len(res.Data)), Page: page.Info{Limit: limit, HasMore: res.HasMore, NextCursor: res.NextCursor}}
	for _, e := range res.Data {
		title := e.Title
		if title == "" {
			title = e.ResourceType
		}
		out.Data = append(out.Data, TrashItemView{ResourceType: e.ResourceType, ResourceID: e.ResourceID, TenantID: e.TenantID, Title: title,
			DeletedAt: e.DeletedAt, DeletedBy: e.DeletedBy, DeleteReason: e.DeleteReason, Restorable: e.Restorable})
	}
	return out
}

// accounts lists the identity-owned trash types: deleted accounts read under tenancy.Bypass(trash) with the
// explicit tenant list (audit + eduworld_tenant_filter_bypass_total, F045-AC-06). The configuration types
// school_settings / tenant_settings have no trash rows (OQ F045-4).
func (q PlatformAdmin) accounts(ctx context.Context, in TrashInput, tenants []uuid.UUID) (TrashPageView, error) {
	limit := page.Limit(in.Limit)
	out := TrashPageView{Data: []TrashItemView{}, Page: page.Info{Limit: limit}}
	if (in.ResourceType != platformadmin.ResourceUser && in.ResourceType != platformadmin.ResourceStudent) || len(tenants) == 0 {
		return out, nil
	}
	after, err := q.Cursors.Decode(in.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	var afterAt *time.Time
	var afterID uuid.UUID
	if after != nil {
		t, err := time.Parse(time.RFC3339Nano, after.Key)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		afterAt, afterID = &t, after.ID
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, platformadmin.ModuleIdentity), tenancy.ReasonTrash,
		tenancy.WithTenantIDs(tenants...), tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		return out, err
	}
	rows, err := q.Model.DeletedAccounts(bctx, in.ResourceType, tenants, afterAt, afterID, limit)
	if err != nil {
		return out, err
	}
	res := port.TrashPage{Data: rows}
	if len(rows) > limit {
		res.Data = rows[:limit]
		last := res.Data[limit-1]
		c := q.Cursors.Encode(page.Cursor{Key: last.DeletedAt.UTC().Format(time.RFC3339Nano), ID: last.ResourceID})
		res.HasMore, res.NextCursor = true, &c
	}
	return trashView(res, limit), nil
}

// AssignmentsInput is the query of listTenantAssignments.
type AssignmentsInput struct {
	ResourceType string
	ResourceID   uuid.UUID
	Cursor       string
	Limit        int
}

// TenantAssignmentItem is TenantAssignment of the API.
type TenantAssignmentItem struct {
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	AssignedBy   uuid.UUID `json:"assigned_by"`
	AssignedAt   time.Time `json:"assigned_at"`
}

// AssignmentsPage is the response of listTenantAssignments.
type AssignmentsPage struct {
	Data []TenantAssignmentItem `json:"data"`
	Page page.Info              `json:"page"`
}

// ListTenantAssignments lists the active assignments of a platform resource from its owning module.
func (q PlatformAdmin) ListTenantAssignments(ctx context.Context, in AssignmentsInput) (out AssignmentsPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.ListTenantAssignments")
	defer func() { done(err) }()
	pctx, _, err := platformRead(ctx, authz.PermPlatformTenantDataAssign)
	if err != nil {
		return out, err
	}
	if !platformadmin.Assignable(in.ResourceType) {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a platform resource type that can be assigned")
	}
	module, _ := platformadmin.Owner(in.ResourceType)
	limit := page.Limit(in.Limit)
	res, err := q.Owners.ListAssignments(pctx, module, in.ResourceType, in.ResourceID, in.Cursor, limit)
	if err != nil {
		return out, err
	}
	out = AssignmentsPage{Data: []TenantAssignmentItem{}, Page: page.Info{Limit: limit, HasMore: res.HasMore, NextCursor: res.NextCursor}}
	for _, a := range res.Data {
		if a.Active {
			out.Data = append(out.Data, TenantAssignmentItem{ResourceType: a.ResourceType, ResourceID: a.ResourceID, TenantID: a.TenantID,
				AssignedBy: a.AssignedBy, AssignedAt: a.AssignedAt})
		}
	}
	return out, nil
}

// ClonePairView is one id_map entry of TenantCloneJob.
type ClonePairView = platformadmin.ClonePair

// TenantCloneView is TenantCloneJob of the API.
type TenantCloneView struct {
	JobID        uuid.UUID       `json:"job_id"`
	Status       string          `json:"status"`
	Source       uuid.UUID       `json:"source_tenant_id"`
	Target       uuid.UUID       `json:"target_tenant_id"`
	ResourceType string          `json:"resource_type"`
	ClonedCount  int             `json:"cloned_count"`
	IDMap        []ClonePairView `json:"id_map"`
	Errors       []string        `json:"errors"`
	CreatedAt    time.Time       `json:"created_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
}

// GetTenantClone returns a clone job (F045-AC-17).
func (q PlatformAdmin) GetTenantClone(ctx context.Context, id uuid.UUID) (out TenantCloneView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetTenantClone")
	defer func() { done(err) }()
	pctx, _, err := platformRead(ctx, authz.PermPlatformTenantDataClone)
	if err != nil {
		return out, err
	}
	v, err := q.Model.CloneJob(pctx, id)
	if err != nil {
		return out, err
	}
	if v == nil {
		return out, problem.New(problem.NotFound, "")
	}
	out = TenantCloneView{JobID: v.JobID, Status: v.Status, Source: v.Source, Target: v.Target, ResourceType: v.ResourceType,
		IDMap: []ClonePairView{}, Errors: v.Errors, CreatedAt: v.CreatedAt, FinishedAt: v.FinishedAt}
	if len(v.IDMap) > 0 {
		if err := json.Unmarshal(v.IDMap, &out.IDMap); err != nil {
			return out, err
		}
	}
	out.ClonedCount = len(out.IDMap)
	if out.Errors == nil {
		out.Errors = []string{}
	}
	if v.FinishedAt == nil { // in progress: the step errors seen so far (retries pending)
		for _, s := range v.Steps {
			if s.Error != "" {
				out.Errors = append(out.Errors, platformadmin.StepError(s.Service, s.No, s.Error))
			}
		}
	}
	return out, nil
}

// MergeStepItem is one step of UserMergeJob.
type MergeStepItem struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// UserMergeItem is UserMergeJob of the API.
type UserMergeItem struct {
	MergeID  uuid.UUID       `json:"merge_id"`
	SagaID   uuid.UUID       `json:"saga_id"`
	Status   string          `json:"status"`
	From     uuid.UUID       `json:"from_user_id"`
	Into     uuid.UUID       `json:"into_user_id"`
	MergedAt time.Time       `json:"merged_at"`
	Steps    []MergeStepItem `json:"steps"`
}

// GetUserMerge returns a merge with the status and error of each step (F045-AC-29).
func (q PlatformAdmin) GetUserMerge(ctx context.Context, id uuid.UUID) (out UserMergeItem, err error) {
	ctx, done := observability.UseCase(ctx, "identity.query.GetUserMerge")
	defer func() { done(err) }()
	pctx, _, err := platformRead(ctx, authz.PermPlatformUserMerge)
	if err != nil {
		return out, err
	}
	v, err := q.Model.UserMerge(pctx, id)
	if err != nil {
		return out, err
	}
	if v == nil {
		return out, problem.New(problem.NotFound, "")
	}
	out = UserMergeItem{MergeID: v.MergeID, SagaID: v.SagaID, Status: platformadmin.MergeStatus(v.SagaStatus), From: v.From, Into: v.Into,
		MergedAt: v.MergedAt, Steps: make([]MergeStepItem, 0, len(v.Steps))}
	for _, s := range v.Steps {
		out.Steps = append(out.Steps, MergeStepItem{Service: s.Service, Status: platformadmin.MergeStepStatus(s.Status),
			Error: platformadmin.Bound(s.Error, 1000)})
	}
	return out, nil
}
