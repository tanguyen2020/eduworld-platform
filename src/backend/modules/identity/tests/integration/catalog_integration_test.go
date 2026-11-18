//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Catalog writes (ADMIN_PORTAL §3.16, ADR-0060): province lifecycle and succession with province.updated.
func TestCatalogWrites(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h := command.Catalog{CatalogDeps: command.CatalogDeps{UoW: e.uow, Catalog: write.CatalogWriter{}, Events: e.deps.Events, Audit: e.deps.Audit}}
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	name := map[string]string{"vi": "Tỉnh Thử", "en": "Test Province"}
	from := "2026-01-01"
	p, err := h.CreateProvince(sa, command.ProvinceInput{Code: "vn.thu", CountryCode: "VN", Name: name, ShortName: name,
		ProvinceType: "province", EffectiveFrom: &from, Status: "active", Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.CreateProvince(sa, command.ProvinceInput{Code: "vn.thu", CountryCode: "VN", Name: name, ShortName: name,
		ProvinceType: "province", Status: "active"}); code(err) != problem.Conflict {
		t.Fatalf("duplicate code: %v", err)
	}
	// Deactivation needs effective_to (inactive ⇔ effective_to).
	if _, err := h.UpdateProvince(sa, command.ProvinceInput{ID: p.ID, IfMatch: p.Version, Status: "inactive"}); code(err) != problem.ValidationFailed {
		t.Fatalf("inactive without effective_to: %v", err)
	}
	to := "2026-07-01"
	off, err := h.UpdateProvince(sa, command.ProvinceInput{ID: p.ID, IfMatch: p.Version, Status: "inactive", EffectiveTo: &to})
	if err != nil || off.Status != "inactive" || off.Version != p.Version+1 {
		t.Fatalf("deactivate = %+v %v", off, err)
	}
	var hanoi string
	if err := e.platform.QueryRowContext(ctx, `SELECT province_id FROM catalog.provinces WHERE code = 'vn.ha-noi'`).Scan(&hanoi); err != nil {
		t.Fatal(err)
	}
	s, err := h.CreateProvinceSuccession(sa, command.SuccessionInput{OldProvinceID: p.ID, NewProvinceID: mustUUID(t, hanoi),
		EffectiveDate: "2026-07-01", Kind: "merged"})
	if err != nil || s.NewID.String() != hanoi {
		t.Fatalf("succession = %+v %v", s, err)
	}
	if _, err := h.CreateProvinceSuccession(sa, command.SuccessionInput{OldProvinceID: p.ID, NewProvinceID: mustUUID(t, hanoi),
		EffectiveDate: "2026-07-01", Kind: "merged"}); code(err) != problem.Conflict {
		t.Fatalf("second successor: %v", err)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'province.updated' AND tenant_id IS NULL
		AND envelope->'payload'->>'province_id' = $1`, p.ID.String()); n != 3 {
		t.Fatalf("province.updated events = %d, want created, deactivated, succession_recorded", n)
	}
	// Countries.
	c, err := h.CreateCountry(sa, command.CountryInput{Code: "SG", Name: map[string]string{"vi": "Singapore", "en": "Singapore"}, DefaultLocale: "en-US"})
	if err != nil || c.Status != "inactive" {
		t.Fatalf("country = %+v %v", c, err)
	}
	if _, err := h.UpdateCountry(sa, "SG", command.CountryInput{IfMatch: c.Version, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := h.CreateCountry(authztest.Ctx(ctx, authztest.Staff(authztest.TenantA, "tenant_admin")), command.CountryInput{Code: "MY",
			Name: map[string]string{"vi": "Mã Lai", "en": "Malaysia"}})
		return err
	}())
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
