//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/devseed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// realmImport is a TEST DOUBLE (R17) of the Keycloak Admin port holding the users of
// deploy/keycloak/realm-eduworld.dev.json (id "kc-<username>"); the HTTP adapter is covered by
// internal/infrastructure/keycloak tests. Any call the seed must not make panics (embedded nil port).
type realmImport struct {
	port.KeycloakAdmin
	mu    sync.Mutex
	users map[string]bool
	roles map[string][]string
}

func newRealmImport(t *testing.T) *realmImport {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "deploy", "keycloak", "realm-eduworld.dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Users []struct {
			Username string `json:"username"`
		} `json:"users"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	f := &realmImport{users: map[string]bool{}, roles: map[string][]string{}}
	for _, u := range r.Users {
		f.users[u.Username] = true
	}
	return f
}

func (f *realmImport) FindUserByUsername(_ context.Context, username string) (port.KCUser, error) {
	if !f.users[username] {
		return port.KCUser{}, port.ErrKCNotFound
	}
	return port.KCUser{ID: "kc-" + username, Username: username, Enabled: true}, nil
}

func (f *realmImport) AddRealmRoles(_ context.Context, id string, roles ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.roles[id] = append(f.roles[id], roles...)
	return nil
}

// The dev seed (LOCAL_DEVELOPMENT §5) creates the demo data through the use cases on PostgreSQL with every migration,
// and a second run changes nothing: no row, no event, no audit.
func TestDevSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	plan, err := devseed.Load(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "identity", "seed"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := outbox.NewWriter("identity", "core", nil)
	if err != nil {
		t.Fatal(err)
	}
	const issuer = "http://keycloak:8180/realms/eduworld"
	kc := newRealmImport(t)
	s, err := identity.NewDevSeed(identity.DevSeedDeps{UoW: e.uow, Read: e.read, Writer: w, Recorder: outbox.NewAuditRecorder(w, e.app),
		KC: kc, Issuers: []string{issuer}})
	if err != nil {
		t.Fatal(err)
	}

	first, err := s.Run(ctx, plan)
	if err != nil {
		t.Fatalf("first run: %v (report %+v)", err, first)
	}
	want := devseed.Report{Tenants: 2, Schools: 3, Connections: 2, Imports: 3, Students: 19, PlatformStaff: 9, TenantStaff: 6, Locked: 1}
	got := first
	got.Grants = 0
	if got != want || first.Grants != 16 {
		t.Fatalf("first run = %+v, want %+v and 16 grants", first, want)
	}
	n := func(q string, args ...any) int { return count(ctx, t, e.platform, q, args...) }
	// Game login path (F013): every SIS student is linked to its realm-import subject and placed in its school and grade
	// (ADR-0095: org.students effective, no enrollment).
	if c := n(`SELECT count(*) FROM identity.users u JOIN identity.external_identities x ON x.user_id = u.user_id AND x.issuer = $1
		AND x.subject = 'kc-' || u.username AND x.deleted_at IS NULL
		JOIN org.students st ON st.student_id = u.user_id AND st.status = 'active' AND st.deleted_at IS NULL
		WHERE u.user_type = 'student' AND u.deleted_at IS NULL`, issuer); c != 19 {
		t.Fatalf("linked and placed students = %d, want 19 (every grade 1–9 has a demo student)", c)
	}
	if len(kc.roles["kc-hs.an.3a"]) == 0 {
		t.Fatal("the provisioning did not give the realm role student")
	}
	if c := n(`SELECT count(*) FROM identity.users WHERE username = 'hs.locked' AND status = 'locked'`); c != 1 {
		t.Fatal("hs.locked is not locked")
	}
	if c := n(`SELECT count(*) FROM identity.users WHERE username = 'hs.nosis'`); c != 0 {
		t.Fatal("hs.nosis must stay without account (ACCOUNT_NOT_PROVISIONED)")
	}
	// Portal path (F012): sa.admin is platform staff with system_admin; ad.truong.a is School Admin of its school only.
	if c := n(`SELECT count(*) FROM identity.users u JOIN identity.user_roles ur ON ur.user_id = u.user_id AND ur.tenant_id IS NULL
		AND ur.deleted_at IS NULL JOIN identity.roles r ON r.role_id = ur.role_id AND r.code = 'system_admin'
		WHERE u.username IN ('sa.admin', 'sa.admin2') AND u.primary_tenant_id IS NULL AND u.user_type = 'staff'`); c != 2 {
		t.Fatalf("system admins = %d", c)
	}
	if c := n(`SELECT count(*) FROM identity.users u JOIN identity.user_roles ur ON ur.user_id = u.user_id AND ur.deleted_at IS NULL
		JOIN identity.roles r ON r.role_id = ur.role_id AND r.code = 'school_admin'
		JOIN org.schools s ON s.school_id = ur.scope_id AND s.code = 'DEMOA1' AND s.tenant_id = ur.tenant_id
		WHERE u.username = 'ad.truong.a' AND ur.scope_type = 'school'`); c != 1 {
		t.Fatal("ad.truong.a is not School Admin of DEMOA1")
	}
	if c := n(`SELECT count(DISTINCT ur.tenant_id) FROM identity.users u JOIN identity.user_roles ur ON ur.user_id = u.user_id
		AND ur.deleted_at IS NULL WHERE u.username = 'qlcm.truong.a'`); c != 2 {
		t.Fatalf("qlcm.truong.a holds roles in %d tenants, want 2", c)
	}

	events := func() int { return count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`) }
	rows := func() int {
		return n(`SELECT (SELECT count(*) FROM platform.tenants) + (SELECT count(*) FROM org.schools)
			+ (SELECT count(*) FROM identity.users) + (SELECT count(*) FROM identity.external_identities) + (SELECT count(*) FROM identity.user_roles)
			+ (SELECT count(*) FROM sis.sync_runs) + (SELECT count(*) FROM sis.sis_connections)`)
	}
	beforeEvents, beforeRows := events(), rows()
	second, err := s.Run(ctx, plan)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.Changed() {
		t.Fatalf("second run changed %+v", second)
	}
	if ev, r := events(), rows(); ev != beforeEvents || r != beforeRows {
		t.Fatalf("second run wrote: outbox %d → %d, rows %d → %d", beforeEvents, ev, beforeRows, r)
	}
}
