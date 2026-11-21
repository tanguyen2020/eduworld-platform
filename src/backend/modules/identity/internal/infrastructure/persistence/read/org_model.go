package read

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entcountry "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/country"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/predicate"
	entprovince "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/province"
	entschool "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// OrgModel implements read.OrgReadModel in a read-only session (ent global filters + RLS).
type OrgModel struct{ Session *entx.ReadSession }

var _ read.OrgReadModel = OrgModel{}

func cl(ctx context.Context) *ent.Client { return ent.NewClient(ent.Driver(entx.Driver(ctx))) }

func date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

// Countries returns every live country.
func (m OrgModel) Countries(ctx context.Context) (out []dto.Country, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := cl(ctx).Country.Query().Order(ent.Asc(entcountry.FieldID)).All(ctx)
		for _, r := range rows {
			out = append(out, dto.Country{CountryCode: r.ID, Name: r.NameI18n, DefaultLocale: r.DefaultLocale, Status: r.Status,
				Version: r.Version})
		}
		return err
	})
	return out, err
}

// Provinces returns the provinces of a country (optionally by status) with the successor code of former units.
func (m OrgModel) Provinces(ctx context.Context, country, status string) (out []dto.Province, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		c := cl(ctx)
		q := c.Province.Query()
		if country != "" {
			q = q.Where(entprovince.CountryCode(country))
		}
		if status != "" {
			q = q.Where(entprovince.Status(status))
		}
		rows, err := q.Order(ent.Asc(entprovince.FieldCode)).All(ctx)
		if err != nil {
			return err
		}
		succ, err := c.ProvinceSuccession.Query().All(ctx)
		if err != nil {
			return err
		}
		all, err := c.Province.Query().All(ctx)
		if err != nil {
			return err
		}
		codes := map[uuid.UUID]string{}
		for _, p := range all {
			codes[p.ID] = p.Code
		}
		next := map[uuid.UUID]string{}
		for _, s := range succ {
			next[s.ID] = codes[s.NewProvinceID]
		}
		for _, r := range rows {
			p := dto.Province{ProvinceID: r.ID, Code: r.Code, CountryCode: r.CountryCode, OfficialCode: r.OfficialCode, Name: r.NameI18n,
				ShortName: r.ShortNameI18n, ProvinceType: r.ProvinceType, EffectiveFrom: date(r.EffectiveFrom), EffectiveTo: date(r.EffectiveTo),
				Status: r.Status, Version: r.Version}
			if s, ok := next[r.ID]; ok {
				p.SuccessorCode = &s
			}
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

func schoolDTO(r *ent.School, prov map[uuid.UUID]*ent.Province) dto.School {
	s := dto.School{SchoolID: r.ID, TenantID: r.TenantID, Code: r.Code, Name: dto.LocalizedName{Vi: r.NameI18n["vi"], En: r.NameI18n["en"]},
		MoetCode: r.MoetCode, SchoolLevel: r.SchoolLevel, GradeMin: int(r.GradeMin), GradeMax: int(r.GradeMax), Address: r.Address,
		Status: r.Status, IsVirtual: r.IsVirtual, Version: r.Version}
	if s.Name.Vi == "" {
		s.Name.Vi = r.Name
	}
	cc := r.CountryCode
	s.CountryCode = &cc
	if r.ProvinceID != nil {
		if p := prov[*r.ProvinceID]; p != nil {
			s.Province = &dto.ProvinceRef{Code: p.Code, Name: p.NameI18n}
		}
	}
	return s
}

func provincesOf(ctx context.Context, rows []*ent.School) (map[uuid.UUID]*ent.Province, error) {
	ids := []uuid.UUID{}
	for _, r := range rows {
		if r.ProvinceID != nil {
			ids = append(ids, *r.ProvinceID)
		}
	}
	out := map[uuid.UUID]*ent.Province{}
	if len(ids) == 0 {
		return out, nil
	}
	ps, err := cl(ctx).Province.Query().Where(entprovince.IDIn(ids...)).All(ctx)
	for _, p := range ps {
		out[p.ID] = p
	}
	return out, err
}

// after is the keyset predicate (key, id) > cursor on the given columns.
func after(keyCol, idCol string, c *page.Cursor) func(*sql.Selector) {
	return func(s *sql.Selector) {
		s.Where(sql.Or(sql.GT(s.C(keyCol), c.Key), sql.And(sql.EQ(s.C(keyCol), c.Key), sql.GT(s.C(idCol), c.ID))))
	}
}

// Schools returns schools of the scope ordered by (code, school_id).
func (m OrgModel) Schools(ctx context.Context, f read.SchoolFilter) (out []dto.School, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		c := cl(ctx)
		q := c.School.Query()
		if f.TenantID != nil {
			q = q.Where(entschool.TenantID(*f.TenantID))
		}
		if f.SchoolIDs != nil {
			q = q.Where(entschool.IDIn(f.SchoolIDs...))
		}
		if f.Status != "" {
			q = q.Where(entschool.Status(f.Status))
		}
		if f.Level != "" {
			q = q.Where(entschool.SchoolLevel(f.Level))
		}
		if f.Q != "" {
			q = q.Where(entschool.Or(entschool.CodeContainsFold(f.Q), entschool.NameContainsFold(f.Q)))
		}
		if f.ProvinceCode != "" {
			p, err := c.Province.Query().Where(entprovince.Code(f.ProvinceCode)).Only(ctx)
			if ent.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			q = q.Where(entschool.ProvinceID(p.ID))
		}
		if f.After != nil {
			q = q.Where(predicate.School(after(entschool.FieldCode, entschool.FieldID, f.After)))
		}
		rows, err := q.Order(ent.Asc(entschool.FieldCode), ent.Asc(entschool.FieldID)).Limit(f.Limit + 1).All(ctx)
		if err != nil {
			return err
		}
		prov, err := provincesOf(ctx, rows)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, schoolDTO(r, prov))
		}
		return nil
	})
	return out, err
}

// School returns one school of the scope (404 outside it).
func (m OrgModel) School(ctx context.Context, id uuid.UUID) (out dto.School, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		r, err := cl(ctx).School.Get(ctx, id)
		if ent.IsNotFound(err) {
			return problem.New(problem.NotFound, "")
		}
		if err != nil {
			return err
		}
		prov, err := provincesOf(ctx, []*ent.School{r})
		out = schoolDTO(r, prov)
		return err
	})
	return out, err
}

// StudentSummary returns the student with its school and grade (404 outside the scope; no class — ADR-0095).
func (m OrgModel) StudentSummary(ctx context.Context, studentID uuid.UUID) (out dto.StudentSummary, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		c := cl(ctx)
		st, err := c.Student.Get(ctx, studentID)
		if ent.IsNotFound(err) {
			return problem.New(problem.NotFound, "")
		}
		if err != nil {
			return err
		}
		out = dto.StudentSummary{StudentID: st.ID, SchoolID: st.SchoolID, GradeLevel: int(st.GradeLevel), DisplayName: st.FullName}
		if u, err := c.User.Get(ctx, studentID); err == nil && u.DisplayName != nil {
			out.DisplayName = *u.DisplayName
		}
		if sc, err := c.School.Get(ctx, st.SchoolID); err == nil {
			out.SchoolName = sc.Name
		}
		return nil
	})
	return out, err
}
