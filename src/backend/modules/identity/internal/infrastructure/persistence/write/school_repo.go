package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entprovince "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/province"
	entsucc "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/provincesuccession"
	entschool "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SchoolRepo implements write.SchoolRepository (org.schools).
type SchoolRepo struct{}

var _ write.SchoolRepository = SchoolRepo{}

func provinceToDomain(p *ent.Province) catalog.Province {
	return catalog.Province{ID: p.ID, Code: p.Code, CountryCode: p.CountryCode, Status: p.Status, Name: p.NameI18n}
}

func schoolToDomain(r *ent.School, prov *catalog.Province) *school.School {
	return &school.School{ID: r.ID, TenantID: r.TenantID, Code: r.Code, Name: school.Name{Vi: r.NameI18n["vi"], En: r.NameI18n["en"]},
		CountryCode: r.CountryCode, Province: prov, MoetCode: r.MoetCode, Level: school.Level(r.SchoolLevel), GradeMin: int(r.GradeMin),
		GradeMax: int(r.GradeMax), Address: r.Address, Status: school.Status(r.Status), IsVirtual: r.IsVirtual, Version: r.Version}
}

func nameI18n(n school.Name) map[string]string {
	m := map[string]string{"vi": n.Vi}
	if n.En != "" {
		m["en"] = n.En
	}
	return m
}

func provinceID(p *catalog.Province) *uuid.UUID {
	if p == nil {
		return nil
	}
	id := p.ID
	return &id
}

// Create inserts the school (tenant_id explicit: required by a cross-tenant scope).
func (SchoolRepo) Create(ctx context.Context, s *school.School) error {
	row, err := client(ctx).School.Create().SetID(s.ID).SetTenantID(s.TenantID).SetCode(s.Code).SetName(s.Name.Vi).
		SetNameI18n(nameI18n(s.Name)).SetCountryCode(s.CountryCode).SetNillableProvinceID(provinceID(s.Province)).
		SetNillableMoetCode(s.MoetCode).SetSchoolLevel(string(s.Level)).SetGradeMin(int16(s.GradeMin)).SetGradeMax(int16(s.GradeMax)).
		SetNillableAddress(s.Address).SetStatus(string(s.Status)).SetIsVirtual(s.IsVirtual).SetSettings(map[string]any{}).Save(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "a school with this code or MOET code exists").Wrap(err)
	}
	if err != nil {
		return err
	}
	s.Version = row.Version
	return nil
}

// Get loads a school of the scope and its province.
func (SchoolRepo) Get(ctx context.Context, id uuid.UUID) (*school.School, error) {
	c := client(ctx)
	row, err := c.School.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	var prov *catalog.Province
	if row.ProvinceID != nil {
		p, err := c.Province.Get(ctx, *row.ProvinceID)
		if err != nil {
			return nil, err
		}
		v := provinceToDomain(p)
		prov = &v
	}
	return schoolToDomain(row, prov), nil
}

// Save updates the mutable fields with optimistic concurrency.
func (SchoolRepo) Save(ctx context.Context, s *school.School) error {
	u := client(ctx).School.Update().Where(entschool.ID(s.ID), entschool.Version(s.Version)).
		SetName(s.Name.Vi).SetNameI18n(nameI18n(s.Name)).SetCountryCode(s.CountryCode).
		SetSchoolLevel(string(s.Level)).SetGradeMin(int16(s.GradeMin)).SetGradeMax(int16(s.GradeMax)).
		SetStatus(string(s.Status)).AddVersion(1)
	if s.Province != nil {
		u = u.SetProvinceID(s.Province.ID)
	} else {
		u = u.ClearProvinceID()
	}
	if s.MoetCode != nil {
		u = u.SetMoetCode(*s.MoetCode)
	} else {
		u = u.ClearMoetCode()
	}
	if s.Address != nil {
		u = u.SetAddress(*s.Address)
	} else {
		u = u.ClearAddress()
	}
	n, err := u.Save(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "a school with this MOET code exists").Wrap(err)
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.Conflict, "school was modified")
	}
	s.Version++
	return nil
}

// CountInTenant counts the live schools of a tenant visible in the scope.
func (SchoolRepo) CountInTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	return client(ctx).School.Query().Where(entschool.TenantID(tenantID)).Count(ctx)
}

// CatalogRepo implements write.CatalogRepository.
type CatalogRepo struct{}

var _ write.CatalogRepository = CatalogRepo{}

// ProvinceByCode returns the province with the code.
func (CatalogRepo) ProvinceByCode(ctx context.Context, code string) (catalog.Province, error) {
	p, err := client(ctx).Province.Query().Where(entprovince.Code(code)).Only(ctx)
	if ent.IsNotFound(err) {
		return catalog.Province{}, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return catalog.Province{}, err
	}
	return provinceToDomain(p), nil
}

// Successor returns the successor of a former province.
func (CatalogRepo) Successor(ctx context.Context, provinceID uuid.UUID) (*catalog.Province, error) {
	c := client(ctx)
	s, err := c.ProvinceSuccession.Query().Where(entsucc.ID(provinceID)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := c.Province.Get(ctx, s.NewProvinceID)
	if err != nil {
		return nil, err
	}
	v := provinceToDomain(p)
	return &v, nil
}
