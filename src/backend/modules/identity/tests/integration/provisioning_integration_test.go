//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeKCUsers is a TEST DOUBLE (R17) of the Keycloak user endpoints; the HTTP adapter is covered by
// internal/infrastructure/keycloak tests.
type fakeKCUsers struct {
	port.KeycloakAdmin
	mu        sync.Mutex
	users     map[string]port.KCNewUser // username → user
	ids       map[string]string         // username → id
	failRoles int
	roles     map[string][]string
}

func newFakeKCUsers() *fakeKCUsers {
	return &fakeKCUsers{users: map[string]port.KCNewUser{}, ids: map[string]string{}, roles: map[string][]string{}}
}

func (f *fakeKCUsers) CreateUser(_ context.Context, u port.KCNewUser) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.users[u.Username]; ok {
		return "", port.ErrKCConflict
	}
	f.users[u.Username], f.ids[u.Username] = u, "kc-"+u.Username
	return f.ids[u.Username], nil
}

func (f *fakeKCUsers) FindUserByAttribute(_ context.Context, name, value string) (port.KCUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for n, u := range f.users {
		for _, v := range u.Attributes[name] {
			if v == value {
				return port.KCUser{ID: f.ids[n], Username: n, Enabled: u.Enabled}, nil
			}
		}
	}
	return port.KCUser{}, port.ErrKCNotFound
}

func (f *fakeKCUsers) AddRealmRoles(_ context.Context, id string, roles ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRoles > 0 {
		f.failRoles--
		return errors.New("keycloak unavailable")
	}
	f.roles[id] = append(f.roles[id], roles...)
	return nil
}

// F011-AC-08, AC-09, AC-13 with the real repositories and outbox.
func TestProvisionStudentAccount(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-prov")
	ta := uuid.MustParse(a.tenant)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	run := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))
	importStudents := func(school uuid.UUID, csv string) uuid.UUID {
		t.Helper()
		c, err := conns.Create(sa, ta, "csv_import", "pilot")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conns.SetSchools(sa, c.ID, []uuid.UUID{school}, "pilot"); err != nil {
			t.Fatal(err)
		}
		res, err := run.Handle(admin, command.RunImportInput{ConnectionID: c.ID, SchoolID: school, Files: sisFiles(t, map[string]string{"students.csv": csv})})
		if err != nil || res.Status != "succeeded" {
			t.Fatalf("import = %+v %v", res, err)
		}
		return c.ID
	}
	const header = "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n"
	c1 := importStudents(uuid.MustParse(a.school), header+"HS-1,TH-A,001,An,3,active,hs.p.an\nHS-2,TH-A,002,Binh,3,active,hs.p.binh\n")
	// AC-13: another SIS sends the same student_external_id for another child → another record and link.
	c2 := importStudents(uuid.MustParse(a.otherSchool), header+"HS-1,TH-B,001,Khac,3,active,hs.p.khac\n")
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.sis_identity_links WHERE student_external_id = 'HS-1'
		AND sis_connection_id IN ($1, $2) AND status = 'active' AND tenant_id = $3`, c1, c2, a.tenant); n != 2 {
		t.Fatalf("links of HS-1 = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'student.sis_link_created'`); n != 3 {
		t.Fatalf("sis_link_created = %d", n)
	}
	userOf := func(username string) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := e.platform.QueryRowContext(ctx, `SELECT user_id FROM identity.users WHERE username = $1`, username).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	kc := newFakeKCUsers()
	h := command.ProvisionStudentAccount{UoW: e.uow, KC: kc, Accounts: write.SisSyncRepo{}, Sync: write.SisSyncRepo{}, Events: e.deps.Events,
		Audit: e.deps.Audit, LocalIssuer: "http://kc/realms/eduworld"}
	scope, _ := authz.EventScope(ta)
	inA := tenancy.WithScope(ctx, scope)

	// AC-08: Keycloak user without email, attributes, role; identity linked; user.provisioned; membership event.
	an := userOf("hs.p.an")
	if err := h.Handle(inA, an); err != nil {
		t.Fatal(err)
	}
	u := kc.users["hs.p.an"]
	if u.Attributes["eduworld_user_id"][0] != an.String() || u.Attributes["ew_sis_connection"][0] != c1.String() ||
		u.Attributes["ew_external_id"][0] != "HS-1" || len(kc.roles["kc-hs.p.an"]) != 1 {
		t.Fatalf("keycloak user = %+v roles = %v", u, kc.roles)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.external_identities e JOIN identity.users u ON u.user_id = e.user_id
		WHERE e.user_id = $1 AND e.subject = 'kc-hs.p.an' AND u.provisioned_at IS NOT NULL AND u.status = 'pending'`, an); n != 1 {
		t.Fatalf("identity = %d", n)
	}
	// Redelivery is a no-op.
	if err := h.Handle(inA, an); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'user.provisioned' AND envelope->'payload'->>'source' = 'sis'`); n != 1 {
		t.Fatalf("user.provisioned = %d", n)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'tenant_membership.changed' AND envelope->'payload'->>'user_id' = $1`, an.String()); n != 1 {
		t.Fatalf("membership events = %d", n)
	}

	// AC-09: a failure after the Keycloak user was created; the redelivery finds it by eduworld_user_id.
	binh := userOf("hs.p.binh")
	kc.failRoles = 1
	if err := h.Handle(inA, binh); err == nil {
		t.Fatal("role failure must be retried")
	}
	if err := h.Handle(inA, binh); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if len(kc.users) != 2 || count(ctx, t, e.platform, `SELECT count(*) FROM identity.external_identities WHERE user_id = $1`, binh) != 1 {
		t.Fatal("redelivery must link the existing Keycloak user")
	}

	// AC-09: the username belongs to another Keycloak account → duplicate_identifier, no identity.
	khac := userOf("hs.p.khac")
	kc.users["hs.p.khac"] = port.KCNewUser{Username: "hs.p.khac", Attributes: map[string][]string{"eduworld_user_id": {uuid.NewString()}}}
	kc.ids["hs.p.khac"] = "kc-someone-else"
	if err := h.Handle(inA, khac); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM sis.reconciliation_issues WHERE issue_type = 'duplicate_identifier' AND sync_run_id IS NULL
		AND entity_refs->>'user_id' = $1 AND tenant_id = $2`, khac.String(), a.tenant); n != 1 {
		t.Fatalf("keycloak conflict issues = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.external_identities WHERE user_id = $1`, khac); n != 0 {
		t.Fatalf("identity linked to another account: %d", n)
	}
}
