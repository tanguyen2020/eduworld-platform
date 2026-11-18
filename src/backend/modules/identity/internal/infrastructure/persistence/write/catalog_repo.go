package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entcountry "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/country"
	entprovince "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/province"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CatalogWriter implements write.CatalogWriter (catalog.*).
type CatalogWriter struct{}

var _ write.CatalogWriter = CatalogWriter{}

func conflictOr(err error, msg string) error {
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, msg).Wrap(err)
	}
	return err
}

// CreateCountry inserts a country.
func (CatalogWriter) CreateCountry(ctx context.Context, c *catalog.Country) error {
	row, err := client(ctx).Country.Create().SetID(c.Code).SetNameI18n(c.Name).SetDefaultLocale(c.DefaultLocale).SetStatus(c.Status).Save(ctx)
	if err != nil {
		return conflictOr(err, "country exists")
	}
	c.Version = row.Version
	return nil
}

// GetCountry loads a live country.
func (CatalogWriter) GetCountry(ctx context.Context, code string) (*catalog.Country, error) {
	r, err := client(ctx).Country.Get(ctx, code)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	return &catalog.Country{Code: r.ID, Name: r.NameI18n, DefaultLocale: r.DefaultLocale, Status: r.Status, Version: r.Version}, nil
}

// SaveCountry updates where version matches.
func (CatalogWriter) SaveCountry(ctx context.Context, c *catalog.Country) error {
	n, err := client(ctx).Country.Update().Where(entcountry.ID(c.Code), entcountry.Version(c.Version)).
		SetNameI18n(c.Name).SetDefaultLocale(c.DefaultLocale).SetStatus(c.Status).AddVersion(1).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.Conflict, "country was modified")
	}
	c.Version++
	return nil
}

func provinceRecord(r *ent.Province) *catalog.ProvinceRecord {
	return &catalog.ProvinceRecord{Province: provinceToDomain(r), OfficialCode: r.OfficialCode, ShortName: r.ShortNameI18n,
		ProvinceType: r.ProvinceType, EffectiveFrom: r.EffectiveFrom, EffectiveTo: r.EffectiveTo, Version: r.Version}
}

// CreateProvince inserts a province.
func (CatalogWriter) CreateProvince(ctx context.Context, p *catalog.ProvinceRecord) error {
	row, err := client(ctx).Province.Create().SetID(p.ID).SetCountryCode(p.CountryCode).SetCode(p.Code).SetNillableOfficialCode(p.OfficialCode).
		SetNameI18n(p.Name).SetShortNameI18n(p.ShortName).SetProvinceType(p.ProvinceType).SetNillableEffectiveFrom(p.EffectiveFrom).
		SetNillableEffectiveTo(p.EffectiveTo).SetStatus(p.Status).Save(ctx)
	if err != nil {
		return conflictOr(err, "province code exists")
	}
	p.Version = row.Version
	return nil
}

// GetProvince loads a live province.
func (CatalogWriter) GetProvince(ctx context.Context, id uuid.UUID) (*catalog.ProvinceRecord, error) {
	r, err := client(ctx).Province.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	return provinceRecord(r), nil
}

// SaveProvince updates where version matches.
func (CatalogWriter) SaveProvince(ctx context.Context, p *catalog.ProvinceRecord) error {
	u := client(ctx).Province.Update().Where(entprovince.ID(p.ID), entprovince.Version(p.Version)).SetCountryCode(p.CountryCode).
		SetNameI18n(p.Name).SetShortNameI18n(p.ShortName).SetProvinceType(p.ProvinceType).SetStatus(p.Status).AddVersion(1)
	if p.OfficialCode != nil {
		u = u.SetOfficialCode(*p.OfficialCode)
	} else {
		u = u.ClearOfficialCode()
	}
	if p.EffectiveFrom != nil {
		u = u.SetEffectiveFrom(*p.EffectiveFrom)
	} else {
		u = u.ClearEffectiveFrom()
	}
	if p.EffectiveTo != nil {
		u = u.SetEffectiveTo(*p.EffectiveTo)
	} else {
		u = u.ClearEffectiveTo()
	}
	n, err := u.Save(ctx)
	if err != nil {
		return conflictOr(err, "province official code exists")
	}
	if n == 0 {
		return problem.New(problem.Conflict, "province was modified")
	}
	p.Version++
	return nil
}

// CreateSuccession records the successor of a former unit (PK old_province_id → 409 when one exists).
func (CatalogWriter) CreateSuccession(ctx context.Context, s catalog.Succession) error {
	_, err := client(ctx).ProvinceSuccession.Create().SetID(s.OldID).SetNewProvinceID(s.NewID).SetEffectiveDate(s.EffectiveDate).
		SetKind(s.Kind).Save(ctx)
	return conflictOr(err, "the former unit already has a successor")
}
