package command

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/platformadmin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// MaxAssignTenants is the limit of tenant_ids per assignment request (F045-AC-26).
const MaxAssignTenants = 100

// StudentRestorer restores a student cleaned up by the inactive-student cleanup (F016 saga student_restore,
// F016-AC-19). Implemented by F016 (command.StudentRestore); it assumes an authorized caller and the tenant
// scope on ctx. Nil = not wired: restoring a student from the trash answers 503 UNAVAILABLE.
type StudentRestorer interface {
	RestoreStudentByAdmin(ctx context.Context, tenantID, studentID uuid.UUID) error
}

// PlatformAdminDeps are the ports of the F045 commands (Platform Administration).
type PlatformAdminDeps struct {
	UoW    write.UnitOfWork
	Repo   write.PlatformAdminRepository
	Owners port.OwnerServiceGateway
	Events write.EventPublisher
	Audit  write.Auditor
	Sagas  saga.Engine
	// Students restores cleaned-up students (F016); nil until wired.
	Students StudentRestorer
	Clock    func() time.Time
}

func (d PlatformAdminDeps) now() time.Time {
	if d.Clock != nil {
		return d.Clock().UTC()
	}
	return time.Now().UTC()
}

// systemAdmin authorizes perm for System Admin (or platform staff holding it); a tenant principal is refused
// unless it is a System Admin carrying a target tenant in X-Tenant-Id (R20).
func systemAdmin(ctx context.Context, perm authz.Permission) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.TenantID != nil && !p.IsSystemAdmin {
		return p, problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return p, err
	}
	if p.TenantID == nil && !p.IsPlatform() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// asPlatform returns ctx with the platform form of the System Admin (no session tenant) and its platform scope:
// the platform-ops facades serve platform principals (the original Principal, on-behalf, ADR-0074).
func asPlatform(ctx context.Context, p authz.Principal) (context.Context, error) {
	if p.TenantID != nil {
		p.TenantID, p.TenantType = nil, ""
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return ctx, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(authz.WithPrincipal(ctx, p), sc), nil
}

// onBehalf rebuilds, for an asynchronous saga step or an event, the platform principal of the System Admin who
// requested the operation, limited to the permission the step needs [Proposed, OQ F045-5]: the original request
// is gone, the step still acts for that user (audit actor) and never with more rights.
func onBehalf(ctx context.Context, userID uuid.UUID, perm authz.Permission) (context.Context, error) {
	p := authz.NewPrincipal(authz.PrincipalData{UserID: userID, UserType: authz.UserTypeStaff, IsSystemAdmin: true,
		Channel: authz.ChannelPortal, MFA: true,
		Grants: []authz.Grant{{Role: "system_admin", ScopeKind: authz.RoleScopePlatform, Permissions: []authz.Permission{perm}}}})
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(authz.WithPrincipal(ctx, p), sc), nil
}

// derivedKey is a stable UUIDv7 Idempotency-Key for a call made on behalf of an event: timestamp at, random
// bits from the hash of parts (a redelivered event replays the callee's stored result).
func derivedKey(at time.Time, parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	var u uuid.UUID
	binary.BigEndian.PutUint64(u[0:8], uint64(at.UnixMilli())<<16)
	copy(u[6:], sum[:10])
	u[6] = (u[6] & 0x0f) | 0x70
	u[8] = (u[8] & 0x3f) | 0x80
	return u.String()
}

// auditIn records e in the audit of tenant (envelope tenant_id), inside the transaction of ctx.
func auditIn(ctx context.Context, a write.Auditor, tenant uuid.UUID, e audit.Event) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return a.Record(tenancy.WithScope(ctx, sc), e)
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id != uuid.Nil && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// ---------- assign / unassign (US2) ----------

// AssignInput is the body of assignToTenants.
type AssignInput struct {
	ResourceType string
	ResourceID   uuid.UUID
	TenantIDs    []uuid.UUID
	ApplyToAll   bool
	ApplyToNew   bool
}

// TenantAssignmentView is one TenantAssignment of the API.
type TenantAssignmentView struct {
	ResourceType string     `json:"resource_type"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	AssignedBy   *uuid.UUID `json:"assigned_by,omitempty"`
	AssignedAt   time.Time  `json:"assigned_at"`
}

// AssignmentsResult is the response of assignToTenants.
type AssignmentsResult struct {
	Data []TenantAssignmentView `json:"data"`
}

// AssignmentPayload is the payload of tenant_data.assigned / tenant_data.unassigned.
type AssignmentPayload struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	TenantID     string `json:"tenant_id"`
	AssignmentID string `json:"assignment_id,omitempty"`
	OwnerModule  string `json:"owner_module"`
	Via          string `json:"via,omitempty"`
}

// AssignToTenants implements assignToTenants (F045-AC-07, AC-08, AC-11, AC-12).
type AssignToTenants struct{ PlatformAdminDeps }

func (h PlatformAdminDeps) resolveTenants(ctx context.Context, ids []uuid.UUID, all bool) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if all {
			var err error
			out, err = h.Repo.ActiveTenantIDs(ctx)
			return err
		}
		out = ids
		bad, err := h.Repo.InactiveTenants(ctx, ids)
		if err != nil {
			return err
		}
		if len(bad) > 0 { // never half an assignment (edge case of the spec)
			return problem.Invalid("tenant_ids", problem.SubInvalid, "every tenant must exist and be active")
		}
		return nil
	})
	return out, err
}

// Handle runs the command.
func (h AssignToTenants) Handle(ctx context.Context, in AssignInput) (out AssignmentsResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.AssignToTenants")
	defer func() { done(err) }()
	p, err := systemAdmin(ctx, authz.PermPlatformTenantDataAssign)
	if err != nil {
		return out, err
	}
	if !platformadmin.Assignable(in.ResourceType) {
		return out, problem.Invalid("resource_type", problem.SubInvalid, "is not a platform resource type that can be assigned")
	}
	module, _ := platformadmin.Owner(in.ResourceType)
	tenants := dedupe(in.TenantIDs)
	switch {
	case len(tenants) > MaxAssignTenants:
		return out, problem.Invalid("tenant_ids", problem.SubInvalid, "at most 100 tenants")
	case len(tenants) == 0 && !in.ApplyToAll && !in.ApplyToNew:
		return out, problem.Invalid("tenant_ids", problem.SubRequired, "tenant_ids is required unless apply_to_all_tenants")
	}
	pctx, err := asPlatform(ctx, p)
	if err != nil {
		return out, err
	}
	if tenants, err = h.resolveTenants(pctx, tenants, in.ApplyToAll); err != nil {
		return out, err
	}
	var assigned []port.ResourceAssignment
	if len(tenants) > 0 {
		if assigned, err = h.Owners.ApplyAssignments(pctx, module, in.ResourceType, in.ResourceID, tenants); err != nil {
			return out, err
		}
	}
	out.Data = []TenantAssignmentView{}
	err = h.UoW.Do(pctx, func(ctx context.Context) error {
		for _, a := range assigned {
			if !slices.Contains(tenants, a.TenantID) || !a.Active {
				continue
			}
			if err := h.assigned(ctx, module, a, "request"); err != nil {
				return err
			}
			by := a.AssignedBy
			out.Data = append(out.Data, TenantAssignmentView{ResourceType: in.ResourceType, ResourceID: in.ResourceID, TenantID: a.TenantID,
				AssignedBy: &by, AssignedAt: a.AssignedAt})
		}
		if in.ApplyToNew {
			if err := h.Repo.AddDefaultAssignment(ctx, write.DefaultAssignment{ID: uuid.Must(uuid.NewV7()), ResourceType: in.ResourceType,
				ResourceID: in.ResourceID, OwnerModule: module, CreatedBy: p.UserID}); err != nil {
				return err
			}
			if err := h.Audit.Record(ctx, audit.Event{Action: "tenant_data.default_assignment_added", TargetType: in.ResourceType,
				TargetID: in.ResourceID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"owner_module": module}}); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// assigned publishes tenant_data.assigned and records its audit in the tenant (F045-AC-07).
func (h PlatformAdminDeps) assigned(ctx context.Context, module string, a port.ResourceAssignment, via string) error {
	t := a.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: "tenant_data.assigned", SubjectType: "tenant_assignment", SubjectID: a.AssignmentID.String(),
		TenantID: &t, Payload: AssignmentPayload{ResourceType: a.ResourceType, ResourceID: a.ResourceID.String(), TenantID: t.String(),
			AssignmentID: a.AssignmentID.String(), OwnerModule: module, Via: via}}); err != nil {
		return err
	}
	return auditIn(ctx, h.Audit, t, audit.Event{Action: "tenant_data.assigned", TargetType: a.ResourceType, TargetID: a.ResourceID.String(),
		Result: audit.ResultSuccess, Metadata: map[string]any{"assignment_id": a.AssignmentID.String(), "owner_module": module, "via": via}})
}

// UnassignInput is the body of unassignFromTenants.
type UnassignInput struct {
	ResourceType string
	ResourceID   uuid.UUID
	TenantIDs    []uuid.UUID
	Reason       string
}

// UnassignFromTenants implements unassignFromTenants (F045-AC-10, AC-12).
type UnassignFromTenants struct{ PlatformAdminDeps }

// defaultUnassignReason is sent to the owning module when System Admin gives no reason (the callee needs 3–1000
// characters) [Proposed].
const defaultUnassignReason = "unassigned by System Admin"

// Handle runs the command.
func (h UnassignFromTenants) Handle(ctx context.Context, in UnassignInput) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UnassignFromTenants")
	defer func() { done(err) }()
	p, err := systemAdmin(ctx, authz.PermPlatformTenantDataAssign)
	if err != nil {
		return err
	}
	if !platformadmin.Assignable(in.ResourceType) {
		return problem.Invalid("resource_type", problem.SubInvalid, "is not a platform resource type that can be assigned")
	}
	module, _ := platformadmin.Owner(in.ResourceType)
	tenants := dedupe(in.TenantIDs)
	switch {
	case len(tenants) == 0:
		return problem.Invalid("tenant_ids", problem.SubRequired, "")
	case len(tenants) > MaxAssignTenants:
		return problem.Invalid("tenant_ids", problem.SubInvalid, "at most 100 tenants")
	}
	reason := in.Reason
	if !platformadmin.ValidReason(reason) {
		reason = defaultUnassignReason
	}
	pctx, err := asPlatform(ctx, p)
	if err != nil {
		return err
	}
	revoked, err := h.Owners.RevokeAssignments(pctx, module, in.ResourceType, in.ResourceID, tenants, reason)
	if err != nil {
		return err
	}
	return h.UoW.Do(pctx, func(ctx context.Context) error {
		for _, a := range revoked {
			if !slices.Contains(tenants, a.TenantID) {
				continue
			}
			t := a.TenantID
			if err := h.Events.Publish(ctx, write.Event{Type: "tenant_data.unassigned", SubjectType: "tenant_assignment",
				SubjectID: a.AssignmentID.String(), TenantID: &t, Payload: AssignmentPayload{ResourceType: in.ResourceType,
					ResourceID: in.ResourceID.String(), TenantID: t.String(), AssignmentID: a.AssignmentID.String(), OwnerModule: module}}); err != nil {
				return err
			}
			if err := auditIn(ctx, h.Audit, t, audit.Event{Action: "tenant_data.unassigned", TargetType: in.ResourceType,
				TargetID: in.ResourceID.String(), Result: audit.ResultSuccess, Reason: in.Reason,
				Metadata: map[string]any{"assignment_id": a.AssignmentID.String(), "owner_module": module}}); err != nil {
				return err
			}
		}
		return idempotency.SetResult(ctx, nil)
	})
}

// ApplyDefaultAssignments assigns the default platform resources to a new tenant (consumer of tenant.created,
// F045-AC-11). Each call acts on behalf of the System Admin who configured the default.
type ApplyDefaultAssignments struct{ PlatformAdminDeps }

// Handle runs the command for the tenant of event eventID (occurred at).
func (h ApplyDefaultAssignments) Handle(ctx context.Context, tenantID, eventID uuid.UUID, at time.Time) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ApplyDefaultAssignments")
	defer func() { done(err) }()
	sc, err := authz.SystemPlatformScope("tenant-defaults")
	if err != nil {
		return 0, err
	}
	sctx := tenancy.WithScope(ctx, sc)
	var defaults []write.DefaultAssignment
	if err := h.UoW.Do(sctx, func(ctx context.Context) error {
		bad, err := h.Repo.InactiveTenants(ctx, []uuid.UUID{tenantID})
		if err != nil || len(bad) > 0 {
			return err // a tenant created disabled gets nothing
		}
		defaults, err = h.Repo.DefaultAssignments(ctx)
		return err
	}); err != nil {
		return 0, err
	}
	for _, d := range defaults {
		actx, err := onBehalf(ctx, d.CreatedBy, authz.PermPlatformTenantDataAssign)
		if err != nil {
			return n, err
		}
		actx = modcall.WithIdempotencyKey(actx, derivedKey(at, eventID.String(), d.ID.String()))
		res, err := h.Owners.ApplyAssignments(actx, d.OwnerModule, d.ResourceType, d.ResourceID, []uuid.UUID{tenantID})
		var pe *problem.Error
		if errors.As(err, &pe) && problem.Status(pe.Code) >= 400 && problem.Status(pe.Code) < 500 {
			// The resource was deleted or is no longer platform data: skip it, keep the others.
			slog.WarnContext(ctx, "default assignment skipped", "resource_type", d.ResourceType, "code", string(pe.Code))
			continue
		}
		if err != nil {
			return n, err // transient: the event is redelivered, done calls replay
		}
		if err := h.UoW.Do(actx, func(ctx context.Context) error {
			for _, a := range res {
				if a.TenantID == tenantID && a.Active {
					if err := h.assigned(ctx, d.OwnerModule, a, "default_for_new_tenant"); err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ---------- trash (US4) ----------

// RestoreInput is a restore of a trash item.
type RestoreInput struct {
	ResourceType string
	ID           uuid.UUID
	Reason       string
}

// RestoreTrashItem implements restoreTrashItem (System Admin) and restoreTenantTrashItem (Tenant Admin)
// (F045-AC-20, AC-21, AC-22).
type RestoreTrashItem struct{ PlatformAdminDeps }

// Admin restores for System Admin: X-Tenant-Id = the tenant of the record, absent for platform data.
func (h RestoreTrashItem) Admin(ctx context.Context, in RestoreInput) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RestoreTrashItem")
	defer func() { done(err) }()
	p, err := systemAdmin(ctx, authz.PermPlatformTrashRestore)
	if err != nil {
		return err
	}
	return h.restore(ctx, p, in)
}

// Tenant restores in the tenant of the session (tenant.trash.restore).
func (h RestoreTrashItem) Tenant(ctx context.Context, in RestoreInput) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RestoreTenantTrashItem")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	if p.TenantID == nil {
		return problem.New(problem.PermissionDenied, "")
	}
	if err := authz.Require(ctx, p, authz.PermTenantTrashRestore, authz.Resource{}); err != nil {
		return err
	}
	return h.restore(ctx, p, in)
}

func (h RestoreTrashItem) restore(ctx context.Context, p authz.Principal, in RestoreInput) error {
	if !platformadmin.ValidReason(in.Reason) {
		return problem.Invalid("reason", problem.SubInvalid, "must be 3–1000 characters")
	}
	module, err := platformadmin.Owner(in.ResourceType)
	if err != nil {
		return problem.Invalid("resource_type", problem.SubInvalid, "unknown resource type")
	}
	if !platformadmin.RestorableThroughTrash(in.ResourceType) {
		return problem.New(problem.Conflict, "a merged account cannot be restored from the trash")
	}
	var sc tenancy.Scope
	if p.TenantID != nil {
		ts, err := authz.ScopeFor(p)
		if err != nil {
			return problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		sc = ts
	} else {
		ps, err := authz.PlatformScope(p)
		if err != nil {
			return problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		sc = ps
	}
	sctx := tenancy.WithScope(ctx, sc)
	switch {
	case in.ResourceType == platformadmin.ResourceStudent:
		if p.TenantID == nil {
			return problem.Invalid("X-Tenant-Id", problem.SubRequired, "a student belongs to a tenant")
		}
		if h.Students == nil {
			return problem.New(problem.Unavailable, "").Retry().Wrap(errors.New("student restore (F016) is not wired"))
		}
		if err := h.Students.RestoreStudentByAdmin(sctx, *p.TenantID, in.ID); err != nil {
			return err
		}
	case module == platformadmin.ModuleIdentity:
		return problem.New(problem.NotFound, "") // class_template, *_settings have no trash rows (OQ F045-4)
	default:
		if err := h.Owners.RestoreTrashItem(sctx, module, in.ResourceType, in.ID, in.Reason); err != nil {
			return err
		}
	}
	return h.UoW.Do(sctx, func(ctx context.Context) error {
		if err := h.Audit.Record(ctx, audit.Event{Action: in.ResourceType + ".restored", TargetType: in.ResourceType, TargetID: in.ID.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"owner_module": module, "via": "trash"}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, nil)
	})
}

// ---------- clone (US3) ----------

// CloneInput is the body of createTenantClone.
type CloneInput struct {
	Source, Target uuid.UUID
	ResourceType   string
	ResourceIDs    []uuid.UUID
	IdempotencyKey string
}

// CloneJobView is TenantCloneJob of the API.
type CloneJobView struct {
	JobID        uuid.UUID                 `json:"job_id"`
	Status       string                    `json:"status"`
	Source       uuid.UUID                 `json:"source_tenant_id"`
	Target       uuid.UUID                 `json:"target_tenant_id"`
	ResourceType string                    `json:"resource_type"`
	ClonedCount  int                       `json:"cloned_count"`
	IDMap        []platformadmin.ClonePair `json:"id_map"`
	Errors       []string                  `json:"errors"`
	CreatedAt    time.Time                 `json:"created_at"`
	FinishedAt   *time.Time                `json:"finished_at,omitempty"`
}

// cloneSubject is the subject of a tenant_clone saga (the resource ids stay in the job row: ≤ 16 KiB here).
type cloneSubject struct {
	JobID uuid.UUID                 `json:"job_id"`
	Plan  []platformadmin.CloneStep `json:"plan"`
}

// CreateTenantClone implements createTenantClone (F045-AC-13, AC-15, AC-16): job + saga in one transaction,
// executed by the job platform-saga.
type CreateTenantClone struct{ PlatformAdminDeps }

func cloneInvalid(err error) error {
	switch {
	case errors.Is(err, platformadmin.ErrCloneSameTenant):
		return problem.Invalid("target_tenant_id", problem.SubInvalid, "the target tenant must differ from the source")
	case errors.Is(err, platformadmin.ErrCloneNotAllowed):
		return problem.Invalid("resource_type", problem.SubInvalid, "this resource type cannot be cloned")
	default:
		return problem.Invalid("resource_ids", problem.SubInvalid, err.Error())
	}
}

// Handle runs the command.
func (h CreateTenantClone) Handle(ctx context.Context, in CloneInput) (out CloneJobView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateTenantClone")
	defer func() { done(err) }()
	p, err := systemAdmin(ctx, authz.PermPlatformTenantDataClone)
	if err != nil {
		return out, err
	}
	plan, err := platformadmin.PlanClone(in.Source, in.Target, in.ResourceType, in.ResourceIDs)
	if err != nil {
		return out, cloneInvalid(err)
	}
	if in.IdempotencyKey == "" {
		return out, problem.New(problem.IdempotencyKeyRequired, "")
	}
	pctx, err := asPlatform(ctx, p)
	if err != nil {
		return out, err
	}
	now := h.now()
	jobID := uuid.Must(uuid.NewV7())
	err = h.UoW.Do(pctx, func(ctx context.Context) error {
		bad, err := h.Repo.InactiveTenants(ctx, []uuid.UUID{in.Source, in.Target})
		if err != nil {
			return err
		}
		if len(bad) > 0 {
			field := "source_tenant_id"
			if bad[0] == in.Target {
				field = "target_tenant_id"
			}
			return problem.Invalid(field, problem.SubInvalid, "the tenant does not exist or is not active")
		}
		target, by := in.Target, p.UserID
		def := CloneDefinition(h.PlatformAdminDeps, len(plan))
		for i, st := range plan {
			def.Steps[i].Service = st.Module // saga_steps.service names the module of the step
		}
		run, created, err := h.Sagas.Enqueue(ctx, def, &target,
			cloneSubject{JobID: jobID, Plan: plan}, in.IdempotencyKey, &by)
		if err != nil {
			return err
		}
		if !created { // same key replayed after the HTTP record expired: the existing job
			j, err := h.Repo.CloneJobBySaga(ctx, run.ID)
			if err != nil || j == nil {
				return errors.Join(err, problem.New(problem.Conflict, "the Idempotency-Key was used for another operation"))
			}
			out = CloneJobView{JobID: j.ID, Status: j.Status, Source: j.Source, Target: j.Target, ResourceType: j.ResourceType,
				IDMap: []platformadmin.ClonePair{}, Errors: []string{}, CreatedAt: j.CreatedAt}
			return idempotency.SetResult(ctx, out)
		}
		if err := h.Repo.CreateCloneJob(ctx, write.CloneJob{ID: jobID, Source: in.Source, Target: in.Target, ResourceType: in.ResourceType,
			ResourceIDs: in.ResourceIDs, SagaID: run.ID, RequestedBy: p.UserID, IdempotencyKey: in.IdempotencyKey, CreatedAt: now}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "tenant_data.clone_requested", TargetType: "tenant_clone_job", TargetID: jobID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"source_tenant_id": in.Source.String(), "target_tenant_id": in.Target.String(),
				"resource_type": in.ResourceType, "count": len(in.ResourceIDs)}}); err != nil {
			return err
		}
		out = CloneJobView{JobID: jobID, Status: platformadmin.CloneQueued, Source: in.Source, Target: in.Target, ResourceType: in.ResourceType,
			IDMap: []platformadmin.ClonePair{}, Errors: []string{}, CreatedAt: now}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// ---------- merge (US6) ----------

// MergeInput is the body of mergeUsers.
type MergeInput struct {
	From, Into, IssueID uuid.UUID
	Reason              string
	IdempotencyKey      string
}

// MergeStepView is one step of UserMergeJob.
type MergeStepView struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// UserMergeView is UserMergeJob of the API.
type UserMergeView struct {
	MergeID  uuid.UUID       `json:"merge_id"`
	SagaID   uuid.UUID       `json:"saga_id"`
	Status   string          `json:"status"`
	From     uuid.UUID       `json:"from_user_id"`
	Into     uuid.UUID       `json:"into_user_id"`
	MergedAt time.Time       `json:"merged_at"`
	Steps    []MergeStepView `json:"steps"`
}

// mergeSubject is the subject of a user_merge saga.
type mergeSubject struct {
	MergeID uuid.UUID `json:"merge_id"`
}

// MergeUsers implements mergeUsers (F045-AC-27, AC-30, AC-31, AC-32): validates, writes identity.user_merges and
// the saga user_merge in one transaction; the job platform-saga runs it. A merge of the same pair whose saga is
// blocked/failed is reopened (System Admin re-runs it after fixing the cause, F045-AC-29).
type MergeUsers struct{ PlatformAdminDeps }

func mergeSteps(run write.SagaRun) []MergeStepView {
	out := make([]MergeStepView, 0, len(run.Steps))
	for _, s := range run.Steps {
		out = append(out, MergeStepView{Service: s.Service, Status: platformadmin.MergeStepStatus(s.Status), Error: s.Error})
	}
	return out
}

// Handle runs the command.
func (h MergeUsers) Handle(ctx context.Context, in MergeInput) (out UserMergeView, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.MergeUsers")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformUserMerge, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.IsSystemAdmin { // School Admin proposes through the reconciliation issue only (F011)
		return out, problem.New(problem.PermissionDenied, "")
	}
	if p.TenantID == nil {
		return out, problem.Invalid("X-Tenant-Id", problem.SubRequired, "the target tenant is required")
	}
	if !platformadmin.ValidReason(in.Reason) {
		return out, problem.Invalid("reason", problem.SubInvalid, "must be 3–1000 characters")
	}
	if in.IdempotencyKey == "" {
		return out, problem.New(problem.IdempotencyKeyRequired, "")
	}
	tenant := *p.TenantID
	pctx, err := asPlatform(ctx, p)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(pctx, func(ctx context.Context) error {
		if in.From == in.Into {
			return problem.New(problem.Conflict, platformadmin.ErrMergeSameUser.Error())
		}
		from, fromMember, err := h.Repo.MergeAccount(ctx, in.From, tenant)
		if err != nil {
			return err
		}
		prev, err := h.Repo.MergeByFrom(ctx, in.From)
		if err != nil {
			return err
		}
		if prev != nil { // the source was merged before: re-run a blocked/failed merge of the same pair, refuse anything else
			if prev.Into != in.Into || prev.TenantID != tenant {
				return problem.New(problem.Conflict, platformadmin.ErrMergeAlreadyMerged.Error())
			}
			run, err := h.Sagas.Reopen(ctx, prev.SagaID)
			if err != nil || run == nil {
				return errors.Join(err, problem.New(problem.NotFound, ""))
			}
			if run.Status != saga.StatusRunning { // succeeded (merged) — only a blocked/failed merge is re-run
				return problem.New(problem.Conflict, platformadmin.ErrMergeAlreadyMerged.Error())
			}
			out = UserMergeView{MergeID: prev.ID, SagaID: prev.SagaID, Status: platformadmin.MergeStatus(run.Status), From: prev.From,
				Into: prev.Into, MergedAt: prev.MergedAt, Steps: mergeSteps(*run)}
			return idempotency.SetResult(ctx, out)
		}
		into, intoMember, err := h.Repo.MergeAccount(ctx, in.Into, tenant)
		if err != nil {
			return err
		}
		if from == nil || into == nil || !fromMember || !intoMember {
			return problem.New(problem.NotFound, "")
		}
		issue, school, err := h.Repo.MergeIssue(ctx, in.IssueID, tenant)
		if err != nil {
			return err
		}
		if issue == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := platformadmin.CheckMerge(*from, *into, *issue); err != nil {
			return problem.New(problem.Conflict, err.Error())
		}
		mergeID := uuid.Must(uuid.NewV7())
		by := p.UserID
		run, _, err := h.Sagas.Enqueue(ctx, MergeDefinition(h.PlatformAdminDeps), &tenant, mergeSubject{MergeID: mergeID}, in.IdempotencyKey, &by)
		if err != nil {
			return err
		}
		issueID := in.IssueID
		m := write.UserMerge{ID: mergeID, TenantID: tenant, From: in.From, Into: in.Into, SchoolID: school, Reason: in.Reason, MergedBy: by,
			MergedAt: now, SagaID: run.ID, IssueID: &issueID}
		if err := h.Repo.CreateMerge(ctx, m); err != nil {
			return err
		}
		if err := auditIn(ctx, h.Audit, tenant, audit.Event{Action: "user.merge_requested", TargetType: "user", TargetID: in.From.String(),
			Result: audit.ResultSuccess, Reason: in.Reason, Metadata: map[string]any{"merge_id": mergeID.String(),
				"from_user_id": in.From.String(), "into_user_id": in.Into.String()}}); err != nil {
			return err
		}
		out = UserMergeView{MergeID: mergeID, SagaID: run.ID, Status: platformadmin.MergeStatus(run.Status), From: in.From, Into: in.Into,
			MergedAt: now, Steps: mergeSteps(run)}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// errNoMerge is a saga run without its merge row (never expected: both are written in one transaction).
var errNoMerge = fmt.Errorf("platformadmin: saga without merge")
