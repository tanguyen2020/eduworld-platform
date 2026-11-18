package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CatalogDeps are the ports of the catalog commands (platform.catalog.write, platform scope).
type CatalogDeps struct {
	UoW     write.UnitOfWork
	Catalog write.CatalogWriter
	Events  write.EventPublisher
	Audit   write.Auditor
}

// CountryInput is CountryInput of the contract (code only on create).
type CountryInput struct {
	Code          string            `json:"country_code,omitempty"`
	Name          map[string]string `json:"name,omitempty"`
	DefaultLocale string            `json:"default_locale,omitempty"`
	Status        string            `json:"status,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	IfMatch       int64             `json:"-"`
}

// ProvinceInput is ProvinceInput of the contract (code only on create; dates YYYY-MM-DD).
type ProvinceInput struct {
	ID            uuid.UUID         `json:"-"`
	IfMatch       int64             `json:"-"`
	Code          string            `json:"code,omitempty"`
	CountryCode   string            `json:"country_code,omitempty"`
	OfficialCode  *string           `json:"official_code,omitempty"`
	Name          map[string]string `json:"name,omitempty"`
	ShortName     map[string]string `json:"short_name,omitempty"`
	ProvinceType  string            `json:"province_type,omitempty"`
	EffectiveFrom *string           `json:"effective_from,omitempty"`
	EffectiveTo   *string           `json:"effective_to,omitempty"`
	Status        string            `json:"status,omitempty"`
	Reason        string            `json:"reason,omitempty"`
}

// SuccessionInput is ProvinceSuccessionInput.
type SuccessionInput struct {
	OldProvinceID uuid.UUID `json:"old_province_id"`
	NewProvinceID uuid.UUID `json:"new_province_id"`
	EffectiveDate string    `json:"effective_date"`
	Kind          string    `json:"kind"`
	Reason        string    `json:"reason,omitempty"`
}

// Catalog implements the catalog write operations (createCountry, updateCountry, createProvince,
// updateProvince, createProvinceSuccession — ADMIN_PORTAL §3.16, ADR-0060).
type Catalog struct{ CatalogDeps }

func catalogInvalid(err error) error {
	var ce catalog.ErrInvalid
	if errors.As(err, &ce) {
		return problem.Invalid(ce.Field, problem.SubInvalid, ce.Reason)
	}
	return err
}

func parseDate(field string, s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		return nil, problem.Invalid(field, problem.SubInvalid, "must be YYYY-MM-DD")
	}
	return &t, nil
}

func (h Catalog) audit(ctx context.Context, action, target, id, reason string) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: target, TargetID: id, Result: audit.ResultSuccess, Reason: reason})
}

// CreateCountry adds a country.
func (h Catalog) CreateCountry(ctx context.Context, in CountryInput) (out catalog.Country, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateCountry")
	defer func() { done(err) }()
	if ctx, _, err = platformWrite(ctx, authz.PermPlatformCatalogWrite); err != nil {
		return out, err
	}
	c := catalog.Country{Code: in.Code, Name: in.Name, DefaultLocale: in.DefaultLocale, Status: in.Status}
	if c.Status == "" {
		c.Status = catalog.StatusInactive
	}
	if err = c.Validate(); err != nil {
		return out, catalogInvalid(err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Catalog.CreateCountry(ctx, &c); err != nil {
			return err
		}
		return h.audit(ctx, "country.created", "country", c.Code, in.Reason)
	})
	return c, err
}

// UpdateCountry changes name, locale or status of a country (If-Match).
func (h Catalog) UpdateCountry(ctx context.Context, code string, in CountryInput) (out catalog.Country, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateCountry")
	defer func() { done(err) }()
	if ctx, _, err = platformWrite(ctx, authz.PermPlatformCatalogWrite); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Catalog.GetCountry(ctx, code)
		if err != nil {
			return err
		}
		if in.IfMatch != c.Version {
			return problem.New(problem.VersionMismatch, "")
		}
		if in.Name != nil {
			c.Name = in.Name
		}
		if in.DefaultLocale != "" {
			c.DefaultLocale = in.DefaultLocale
		}
		if in.Status != "" {
			c.Status = in.Status
		}
		if err := c.Validate(); err != nil {
			return catalogInvalid(err)
		}
		if err := h.Catalog.SaveCountry(ctx, c); err != nil {
			return err
		}
		out = *c
		return h.audit(ctx, "country.updated", "country", c.Code, in.Reason)
	})
	return out, err
}

func (h Catalog) provinceEvent(ctx context.Context, p *catalog.ProvinceRecord, op string, successor *string) error {
	payload := map[string]any{"province_id": p.ID.String(), "code": p.Code, "country_code": p.CountryCode, "official_code": p.OfficialCode,
		"name": map[string]string{"vi": p.Name["vi"], "en": p.Name["en"]}, "status": p.Status, "op": op, "successor_code": successor}
	if p.EffectiveTo != nil {
		payload["effective_to"] = p.EffectiveTo.Format(time.DateOnly)
	} else {
		payload["effective_to"] = nil
	}
	return h.Events.Publish(ctx, write.Event{Type: "province.updated", SubjectType: "province", SubjectID: p.ID.String(), Platform: true, Payload: payload})
}

func applyProvince(p *catalog.ProvinceRecord, in ProvinceInput) error {
	if in.CountryCode != "" {
		p.CountryCode = in.CountryCode
	}
	if in.OfficialCode != nil {
		p.OfficialCode = in.OfficialCode
	}
	if in.Name != nil {
		p.Name = in.Name
	}
	if in.ShortName != nil {
		p.ShortName = in.ShortName
	}
	if in.ProvinceType != "" {
		p.ProvinceType = in.ProvinceType
	}
	if in.Status != "" {
		p.Status = in.Status
	}
	from, err := parseDate("effective_from", in.EffectiveFrom)
	if err != nil {
		return err
	}
	if in.EffectiveFrom != nil {
		p.EffectiveFrom = from
	}
	to, err := parseDate("effective_to", in.EffectiveTo)
	if err != nil {
		return err
	}
	if in.EffectiveTo != nil {
		p.EffectiveTo = to
	}
	return catalogInvalid(p.Validate())
}

// CreateProvince adds a province (province.updated op=created).
func (h Catalog) CreateProvince(ctx context.Context, in ProvinceInput) (out catalog.ProvinceRecord, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateProvince")
	defer func() { done(err) }()
	if ctx, _, err = platformWrite(ctx, authz.PermPlatformCatalogWrite); err != nil {
		return out, err
	}
	if in.Code == "" {
		return out, problem.Invalid("code", problem.SubRequired, "required")
	}
	p := catalog.ProvinceRecord{Province: catalog.Province{ID: ids.New(), Code: in.Code, Status: catalog.StatusActive}, Version: 1}
	if err = applyProvince(&p, in); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Catalog.CreateProvince(ctx, &p); err != nil {
			return err
		}
		if err := h.provinceEvent(ctx, &p, "created", nil); err != nil {
			return err
		}
		return h.audit(ctx, "province.created", "province", p.ID.String(), in.Reason)
	})
	return p, err
}

// UpdateProvince changes a province (If-Match); deactivation sets effective_to (province.updated).
func (h Catalog) UpdateProvince(ctx context.Context, in ProvinceInput) (out catalog.ProvinceRecord, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateProvince")
	defer func() { done(err) }()
	if ctx, _, err = platformWrite(ctx, authz.PermPlatformCatalogWrite); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		p, err := h.Catalog.GetProvince(ctx, in.ID)
		if err != nil {
			return err
		}
		if in.IfMatch != p.Version {
			return problem.New(problem.VersionMismatch, "")
		}
		wasActive := p.Active()
		if err := applyProvince(p, in); err != nil {
			return err
		}
		if err := h.Catalog.SaveProvince(ctx, p); err != nil {
			return err
		}
		op := "updated"
		if wasActive && !p.Active() {
			op = "deactivated"
		}
		if err := h.provinceEvent(ctx, p, op, nil); err != nil {
			return err
		}
		out = *p
		return h.audit(ctx, "province.updated", "province", p.ID.String(), in.Reason)
	})
	return out, err
}

// CreateProvinceSuccession records the successor of a former unit (province.updated op=succession_recorded).
func (h Catalog) CreateProvinceSuccession(ctx context.Context, in SuccessionInput) (out catalog.Succession, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateProvinceSuccession")
	defer func() { done(err) }()
	if ctx, _, err = platformWrite(ctx, authz.PermPlatformCatalogWrite); err != nil {
		return out, err
	}
	on, err := time.Parse(time.DateOnly, in.EffectiveDate)
	if err != nil {
		return out, problem.Invalid("effective_date", problem.SubInvalid, "must be YYYY-MM-DD")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		old, err := h.Catalog.GetProvince(ctx, in.OldProvinceID)
		if err != nil {
			return problem.Invalid("old_province_id", problem.SubInvalid, "unknown province")
		}
		next, err := h.Catalog.GetProvince(ctx, in.NewProvinceID)
		if err != nil {
			return problem.Invalid("new_province_id", problem.SubInvalid, "unknown province")
		}
		s, err := catalog.NewSuccession(old.Province, next.Province, on, in.Kind)
		if err != nil {
			return catalogInvalid(err)
		}
		if err := h.Catalog.CreateSuccession(ctx, s); err != nil {
			return err
		}
		code := next.Code
		if err := h.provinceEvent(ctx, old, "succession_recorded", &code); err != nil {
			return err
		}
		out = s
		return h.audit(ctx, "province.succession_recorded", "province", old.ID.String(), in.Reason)
	})
	return out, err
}
