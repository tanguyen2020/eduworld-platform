package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SchoolDeps are the ports of the school commands.
type SchoolDeps struct {
	UoW     write.UnitOfWork
	Tenants write.TenantRepository
	Schools write.SchoolRepository
	Catalog write.CatalogRepository
	Events  write.EventPublisher
	Audit   write.Auditor
}

// SchoolPayload is the payload of school.updated (specs/events/payloads/identity/school.updated.v1).
type SchoolPayload struct {
	SchoolID       string            `json:"school_id"`
	TenantID       string            `json:"tenant_id"`
	Status         string            `json:"status"`
	Name           string            `json:"name"`
	NameI18n       dto.LocalizedName `json:"name_i18n"`
	ProvinceCode   *string           `json:"province_code"`
	SchoolLevel    string            `json:"school_level"`
	IsVirtual      bool              `json:"is_virtual"`
	SettingsSubset map[string]any    `json:"settings_subset"`
	ChangedFields  []string          `json:"changed_fields"`
	SourceVersion  int64             `json:"source_version"`
}

func schoolPayload(s *school.School, changed []string) SchoolPayload {
	if changed == nil {
		changed = []string{}
	}
	p := SchoolPayload{SchoolID: s.ID.String(), TenantID: s.TenantID.String(), Status: string(s.Status), Name: s.Name.Vi,
		NameI18n: dto.LocalizedName{Vi: s.Name.Vi, En: s.Name.En}, SchoolLevel: string(s.Level), IsVirtual: s.IsVirtual,
		SettingsSubset: map[string]any{}, ChangedFields: changed, SourceVersion: s.Version}
	if s.Province != nil {
		c := s.Province.Code
		p.ProvinceCode = &c
	}
	return p
}

// SchoolView maps the aggregate to the School schema.
func SchoolView(s *school.School) dto.School {
	v := dto.School{SchoolID: s.ID, TenantID: s.TenantID, Code: s.Code, Name: dto.LocalizedName{Vi: s.Name.Vi, En: s.Name.En},
		MoetCode: s.MoetCode, SchoolLevel: string(s.Level), GradeMin: s.GradeMin, GradeMax: s.GradeMax, Address: s.Address,
		Status: string(s.Status), IsVirtual: s.IsVirtual, Version: s.Version}
	if s.CountryCode != "" {
		c := s.CountryCode
		v.CountryCode = &c
	}
	if s.Province != nil {
		v.Province = &dto.ProvinceRef{Code: s.Province.Code, Name: s.Province.Name}
	}
	return v
}

// adminTenantWrite authorizes a System Admin write into one target tenant: platform.tenant.manage, then
// CrossTenantScope of exactly that tenant (ADR-0089 §3, audited as cross_tenant.accessed mode=write).
func adminTenantWrite(ctx context.Context, target uuid.UUID) (context.Context, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantManage, authz.Resource{}); err != nil {
		return ctx, err
	}
	sc, err := appscope.TargetTenant(p, target)
	if err != nil {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "target_tenant_scope"})
		return ctx, err
	}
	return tenancy.WithScope(ctx, sc), nil
}

// tenantSchoolWrite authorizes tenant.school.write on one school of the session tenant (a school-limited
// grant only covers its school: F010-AC-18).
func tenantSchoolWrite(ctx context.Context, schoolID uuid.UUID) (context.Context, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, uuid.Nil, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, uuid.Nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	if err := authz.Require(ctx, p, authz.PermTenantSchoolWrite, authz.Resource{TenantID: p.TenantID, SchoolID: &schoolID}); err != nil {
		return ctx, uuid.Nil, err
	}
	return tenancy.WithScope(ctx, sc), *p.TenantID, nil
}

// province resolves a province code chosen by a person: unknown or not in force → 422 (F010-AC-07).
func (d SchoolDeps) province(ctx context.Context, code string) (catalog.Province, error) {
	p, err := d.Catalog.ProvinceByCode(ctx, code)
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.NotFound {
		return p, problem.Invalid("province_code", problem.SubInvalid, "unknown province")
	}
	if err != nil {
		return p, err
	}
	if !p.Active() {
		return p, problem.Invalid("province_code", problem.SubInvalid, "must be a province in force")
	}
	return p, nil
}

func (d SchoolDeps) emit(ctx context.Context, s *school.School, action, reason string, changed []string) error {
	tid := s.TenantID
	if err := d.Events.Publish(ctx, write.Event{Type: "school.updated", SubjectType: "school", SubjectID: s.ID.String(),
		TenantID: &tid, Payload: schoolPayload(s, changed)}); err != nil {
		return err
	}
	return d.Audit.Record(ctx, audit.Event{Action: action, TargetType: "school", TargetID: s.ID.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: map[string]any{"changed_fields": changed, "status": string(s.Status)}})
}

// CreateAdminTenantSchoolHandler creates a school in a target tenant (createAdminTenantSchool).
type CreateAdminTenantSchoolHandler interface {
	Handle(ctx context.Context, in dto.CreateSchool) (dto.School, error)
}

// CreateAdminTenantSchool implements CreateAdminTenantSchoolHandler.
type CreateAdminTenantSchool struct{ SchoolDeps }

// Handle runs the command.
func (h CreateAdminTenantSchool) Handle(ctx context.Context, in dto.CreateSchool) (out dto.School, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateAdminTenantSchool")
	defer func() { done(err) }()
	if ctx, err = adminTenantWrite(ctx, in.TenantID); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Tenants.Get(ctx, in.TenantID)
		if err != nil {
			return err
		}
		n, err := h.Schools.CountInTenant(ctx, t.ID)
		if err != nil {
			return err
		}
		if err := school.CanAdd(t.Type, n); err != nil {
			return invalid(err)
		}
		prov, err := h.province(ctx, in.ProvinceCode)
		if err != nil {
			return err
		}
		s, err := school.New(ids.New(), t.ID, in.Code, school.Name{Vi: in.Name.Vi, En: in.Name.En}, prov, school.Level(in.SchoolLevel),
			in.GradeMin, in.GradeMax, in.MoetCode, in.Address)
		if err != nil {
			return invalid(err)
		}
		if err := h.Schools.Create(ctx, s); err != nil {
			return err
		}
		if err := h.emit(ctx, s, "school.created", in.Reason, nil); err != nil {
			return err
		}
		out = SchoolView(s)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// UpdateSchoolHandler updates a school: UpdateTenantSchool (tenant.school.write, display fields) or
// UpdateAdminTenantSchool (System Admin, also status).
type UpdateSchoolHandler interface {
	Handle(ctx context.Context, in dto.UpdateSchool) (dto.School, error)
}

// UpdateTenantSchool implements UpdateSchoolHandler for the session tenant (updateTenantSchool).
type UpdateTenantSchool struct{ SchoolDeps }

// Handle runs the command.
func (h UpdateTenantSchool) Handle(ctx context.Context, in dto.UpdateSchool) (out dto.School, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateTenantSchool")
	defer func() { done(err) }()
	if in.Status != nil {
		return out, problem.Invalid("status", problem.SubInvalid, "only System Admin changes the status")
	}
	if ctx, _, err = tenantSchoolWrite(ctx, in.SchoolID); err != nil {
		return out, err
	}
	return h.update(ctx, in)
}

// UpdateAdminTenantSchool implements UpdateSchoolHandler for System Admin (updateAdminTenantSchool).
type UpdateAdminTenantSchool struct{ SchoolDeps }

// Handle runs the command.
func (h UpdateAdminTenantSchool) Handle(ctx context.Context, in dto.UpdateSchool) (out dto.School, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateAdminTenantSchool")
	defer func() { done(err) }()
	if ctx, err = adminTenantWrite(ctx, in.TenantID); err != nil {
		return out, err
	}
	return h.update(ctx, in)
}

func (d SchoolDeps) update(ctx context.Context, in dto.UpdateSchool) (out dto.School, err error) {
	err = d.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := d.Schools.Get(ctx, in.SchoolID)
		if err != nil {
			return err
		}
		if in.TenantID != uuid.Nil && s.TenantID != in.TenantID {
			return problem.New(problem.NotFound, "")
		}
		if in.IfMatch != s.Version {
			return problem.New(problem.VersionMismatch, "")
		}
		patch, changed, err := d.patch(ctx, in)
		if err != nil {
			return err
		}
		err = s.Apply(patch)
		if errors.Is(err, school.ErrNoChange) {
			out = SchoolView(s)
			return nil
		}
		if errors.Is(err, school.ErrVirtual) {
			return problem.Invalid("school_id", problem.SubInvalid, "the virtual school of the common tenant cannot be changed")
		}
		if err != nil {
			return invalid(err)
		}
		if err := d.Schools.Save(ctx, s); err != nil {
			return err
		}
		if err := d.emit(ctx, s, "school.updated", in.Reason, changed); err != nil {
			return err
		}
		out = SchoolView(s)
		return nil
	})
	return out, err
}

func (d SchoolDeps) patch(ctx context.Context, in dto.UpdateSchool) (school.Patch, []string, error) {
	var p school.Patch
	var changed []string
	if in.Name != nil {
		p.Name = &school.Name{Vi: in.Name.Vi, En: in.Name.En}
		changed = append(changed, "name")
	}
	if in.ProvinceCode != nil {
		prov, err := d.province(ctx, *in.ProvinceCode)
		if err != nil {
			return p, nil, err
		}
		p.Province = &prov
		changed = append(changed, "province_code")
	}
	if in.MoetCode.Set {
		v := in.MoetCode.Value
		p.MoetCode = &v
		changed = append(changed, "moet_code")
	}
	if in.SchoolLevel != nil {
		l := school.Level(*in.SchoolLevel)
		p.Level = &l
		changed = append(changed, "school_level")
	}
	if in.GradeMin != nil || in.GradeMax != nil {
		p.GradeMin, p.GradeMax = in.GradeMin, in.GradeMax
		changed = append(changed, "grades")
	}
	if in.Address.Set {
		v := in.Address.Value
		p.Address = &v
		changed = append(changed, "address")
	}
	if in.Status != nil {
		st := school.Status(*in.Status)
		p.Status = &st
		changed = append(changed, "status")
	}
	return p, changed, nil
}

// ApproveTenantSchoolHandler approves a pending_review school (approveTenantSchool, F010-AC-09).
type ApproveTenantSchoolHandler interface {
	Handle(ctx context.Context, schoolID uuid.UUID, ifMatch int64) (dto.School, error)
}

// ApproveTenantSchool implements ApproveTenantSchoolHandler.
type ApproveTenantSchool struct{ SchoolDeps }

// Handle runs the command.
func (h ApproveTenantSchool) Handle(ctx context.Context, schoolID uuid.UUID, ifMatch int64) (out dto.School, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ApproveTenantSchool")
	defer func() { done(err) }()
	if ctx, _, err = tenantSchoolWrite(ctx, schoolID); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Schools.Get(ctx, schoolID)
		if err != nil {
			return err
		}
		if ifMatch != s.Version {
			return problem.New(problem.VersionMismatch, "")
		}
		if err := s.Approve(); errors.Is(err, school.ErrNotPending) {
			return problem.New(problem.InvalidStateTransition, "the school is not pending_review")
		} else if err != nil {
			return invalid(err)
		}
		if err := h.Schools.Save(ctx, s); err != nil {
			return err
		}
		if err := h.emit(ctx, s, "school.approved", "", []string{"status"}); err != nil {
			return err
		}
		out = SchoolView(s)
		return nil
	})
	return out, err
}
