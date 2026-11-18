package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
)

func countryView(c catalog.Country) dto.Country {
	return dto.Country{CountryCode: c.Code, Name: c.Name, DefaultLocale: c.DefaultLocale, Status: c.Status}
}

func dateStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func provinceView(p catalog.ProvinceRecord) dto.Province {
	return dto.Province{ProvinceID: p.ID, Code: p.Code, CountryCode: p.CountryCode, OfficialCode: p.OfficialCode, Name: p.Name,
		ShortName: p.ShortName, ProvinceType: p.ProvinceType, EffectiveFrom: dateStr(p.EffectiveFrom), EffectiveTo: dateStr(p.EffectiveTo),
		Status: p.Status}
}

func (h Handlers) createCountry(w http.ResponseWriter, r *http.Request) {
	var in command.CountryInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Catalog.CreateCountry(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusCreated, countryView(out))
}

func (h Handlers) updateCountry(w http.ResponseWriter, r *http.Request) {
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.CountryInput{IfMatch: v}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Catalog.UpdateCountry(r.Context(), chi.URLParam(r, "code"), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, countryView(out))
}

func (h Handlers) createProvince(w http.ResponseWriter, r *http.Request) {
	var in command.ProvinceInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Catalog.CreateProvince(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusCreated, provinceView(out))
}

func (h Handlers) updateProvince(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "province_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in := command.ProvinceInput{ID: id, IfMatch: v}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Catalog.UpdateProvince(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.Version)
	writeJSON(w, http.StatusOK, provinceView(out))
}

func (h Handlers) createProvinceSuccession(w http.ResponseWriter, r *http.Request) {
	var in command.SuccessionInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Catalog.CreateProvinceSuccession(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"old_province_id": out.OldID, "new_province_id": out.NewID,
		"effective_date": out.EffectiveDate.Format(time.DateOnly), "kind": out.Kind})
}
