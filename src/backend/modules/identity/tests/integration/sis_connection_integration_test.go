//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F011-AC-26: csv_import connection, one active connection per school (409), schools of the tenant only,
// platform permission only.
func TestSisConnections(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-conn-a")
	b := seedTenant(ctx, t, e.platform, "pgd-conn-b")
	codec, _ := page.NewCodec([]byte(strings.Repeat("k", 32)))
	h := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit, Cursors: codec}
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	ta := uuid.MustParse(a.tenant)

	if _, err := h.Create(sa, ta, "contract_v1", ""); code(err) != problem.ValidationFailed {
		t.Fatalf("contract_v1 in F011: %v", err)
	}
	c1, err := h.Create(sa, ta, "csv_import", "pilot")
	if err != nil || c1.Status != "active" || c1.TenantID != ta {
		t.Fatalf("create = %+v %v", c1, err)
	}
	c2, err := h.Create(sa, ta, "csv_import", "second")
	if err != nil {
		t.Fatal(err)
	}
	s1, s2 := uuid.MustParse(a.school), uuid.MustParse(a.otherSchool)
	got, err := h.SetSchools(sa, c1.ID, []uuid.UUID{s1, s2}, "assign")
	if err != nil || len(got) != 2 {
		t.Fatalf("set schools = %+v %v", got, err)
	}
	if _, err := h.SetSchools(sa, c2.ID, []uuid.UUID{s1}, "steal"); code(err) != problem.SisSchoolAlreadyConnected {
		t.Fatalf("school of another connection: %v", err)
	}
	if _, err := h.SetSchools(sa, c1.ID, []uuid.UUID{uuid.MustParse(b.school)}, "foreign"); code(err) != problem.ValidationFailed {
		t.Fatalf("school of another tenant: %v", err)
	}
	// Narrow the set: s2 becomes inactive and can move to c2.
	if _, err := h.SetSchools(sa, c1.ID, []uuid.UUID{s1}, "narrow"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetSchools(sa, c2.ID, []uuid.UUID{s2}, "move"); err != nil {
		t.Fatalf("move freed school: %v", err)
	}
	// API_DESIGN §5: cursor pages — limit 1 returns one row, has_more and a cursor to the second (last) row.
	first, err := h.Schools(sa, c1.ID, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Data) != 1 || !first.Page.HasMore || first.Page.NextCursor == nil {
		t.Fatalf("first page: %+v", first)
	}
	second, err := h.Schools(sa, c1.ID, 1, *first.Page.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Data) != 1 || second.Page.HasMore || second.Data[0].SchoolID == first.Data[0].SchoolID {
		t.Fatalf("second page: %+v", second)
	}
	list := append(first.Data, second.Data...)
	active := 0
	for _, s := range list {
		if s.Active {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active schools of c1 = %d (%+v)", active, list)
	}
	if _, err := h.Schools(sa, uuid.New(), 0, ""); code(err) != problem.NotFound {
		t.Fatalf("unknown connection: %v", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := h.Create(authztest.Ctx(ctx, authztest.Staff(ta, "tenant_admin")), ta, "csv_import", "")
		return err
	}())
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' IN ('sis_connection.created', 'sis_connection.schools_set')`); n < 5 {
		t.Fatalf("audit events = %d", n)
	}
}
