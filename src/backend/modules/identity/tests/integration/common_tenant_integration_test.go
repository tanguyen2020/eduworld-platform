//go:build integration

package integration

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// kcUsers is a fake Keycloak for provisioning: users, passwords (temporary flag), roles, members.
type kcUsers struct {
	port.KeycloakAdmin
	mu        sync.Mutex
	users     map[string]string // username → id
	temporary map[string]bool   // id → temporary
	roles     map[string][]string
	members   map[string]bool
	disabled  map[string]bool
}

func newKCUsers() *kcUsers {
	return &kcUsers{users: map[string]string{}, temporary: map[string]bool{}, roles: map[string][]string{}, members: map[string]bool{}, disabled: map[string]bool{}}
}

func (k *kcUsers) CreateUser(_ context.Context, u port.KCNewUser) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.users[u.Username]; ok {
		return "", port.ErrKCConflict
	}
	id := uuid.NewString()
	k.users[u.Username] = id
	return id, nil
}
func (k *kcUsers) SetPassword(_ context.Context, id, _ string, temporary bool) error {
	k.temporary[id] = temporary
	return nil
}
func (k *kcUsers) AddRealmRoles(_ context.Context, id string, roles ...string) error {
	k.roles[id] = append(k.roles[id], roles...)
	return nil
}
func (k *kcUsers) AddOrganizationMember(_ context.Context, _, id string) error {
	k.members[id] = true
	return nil
}
func (k *kcUsers) DisableUser(_ context.Context, id string) error { k.disabled[id] = true; return nil }

// F010-AC-24, AC-25, AC-26 (no game path), AC-28, AC-29.
func TestCreateCommonStudents(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	if _, err := e.platform.ExecContext(ctx, `UPDATE platform.tenants SET keycloak_org_id = $1 WHERE tenant_id = $2 AND deleted_at IS NULL`,
		uuid.New(), tenant.CommonTenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, username, display_name, email, provisioned_at, deleted_at)
		VALUES ($1, now(), now(), 'staff', 'taken.user', 'x', 'taken@x.vn', now(), now())`, uuid.New()); err != nil {
		t.Fatal(err)
	}
	kc := newKCUsers()
	h := command.CreateCommonStudents{UoW: e.uow, Tenants: write.TenantRepo{}, Accounts: write.AccountRepo{}, KC: kc,
		Events: e.deps.Events, Audit: e.deps.Audit, LocalIssuer: "http://localhost:8180/realms/eduworld"}
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	in := command.CreateCommonStudentsInput{TenantID: tenant.CommonTenantID, Students: []command.CommonStudentInput{
		{Username: " HS.TuDo ", DisplayName: "Tự Do", GradeLevel: 4},
		{Username: "hs.lop6", DisplayName: "Lớp Sáu", GradeLevel: 6, ExternalRef: "CRM-001"},
		{Username: "Taken.User", DisplayName: "Trùng", GradeLevel: 5},
	}}
	out, err := h.Handle(sa, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 3 || out.CreatedCount != 2 || out.FailedCount != 1 || out.PasswordRedacted {
		t.Fatalf("counts = %+v", out)
	}
	if r := out.Results[2]; r.Status != "failed" || r.ErrorCode != "USERNAME_TAKEN" || r.Username != "taken.user" || r.RowNo != 3 {
		t.Fatalf("duplicate row = %+v (soft-deleted users keep their username)", r)
	}
	tudo, lop6 := out.Results[0], out.Results[1]
	if tudo.Username != "hs.tudo" || tudo.InitialPassword == "" || lop6.InitialPassword == "" {
		t.Fatalf("created rows = %+v %+v", tudo, lop6)
	}
	// AC-24: user, student in the virtual school, enrollment in common-g4, Keycloak role + membership.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users u JOIN org.students s ON s.student_id = u.user_id
		JOIN org.enrollments en ON en.student_id = u.user_id AND en.status = 'active'
		WHERE u.user_id = $1 AND u.user_type = 'student' AND u.primary_tenant_id = $2 AND s.school_id = $3
		AND en.class_id = '0193a000-0000-7000-8000-000000000014' AND en.source = 'platform_common'`,
		tudo.UserID, tenant.CommonTenantID, tenant.CommonSchoolID); n != 1 {
		t.Fatal("common student rows missing")
	}
	idTudo, id6 := kc.users["hs.tudo"], kc.users["hs.lop6"]
	// AC-29: grades 6–9 temporary password, grades 1–5 not.
	if kc.temporary[idTudo] || !kc.temporary[id6] {
		t.Fatalf("temporary flags: g4=%v g6=%v", kc.temporary[idTudo], kc.temporary[id6])
	}
	if kc.roles[idTudo][0] != "student" || !kc.members[idTudo] {
		t.Fatal("Keycloak role/membership missing")
	}
	// AC-28: external_ref stored as identity platform_common; a second use conflicts.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.external_identities WHERE issuer = 'platform_common' AND subject = 'CRM-001' AND user_id = $1 AND deleted_at IS NULL`, lop6.UserID); n != 1 {
		t.Fatal("external_ref identity missing")
	}
	again, err := h.Handle(sa, command.CreateCommonStudentsInput{TenantID: tenant.CommonTenantID, Students: []command.CommonStudentInput{
		{Username: "hs.khac", DisplayName: "Khác", GradeLevel: 2, ExternalRef: "CRM-001"}}})
	if err != nil || again.Results[0].ErrorCode != "IDENTITY_CONFLICT" {
		t.Fatalf("duplicate external_ref = %+v %v", again, err)
	}
	// AC-29: the password never reaches the outbox (events, audit).
	var envelopes string
	if err := e.relay.QueryRowContext(ctx, `SELECT coalesce(string_agg(envelope::text, ' '), '') FROM infra.outbox_events`).Scan(&envelopes); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(envelopes, tudo.InitialPassword) || strings.Contains(envelopes, lop6.InitialPassword) {
		t.Fatal("initial password found in the outbox")
	}
	if !strings.Contains(envelopes, `"source": "platform_common"`) && !strings.Contains(envelopes, `"source":"platform_common"`) {
		t.Fatal("user.provisioned with source platform_common missing")
	}
	// Redacted copy stored for replay.
	red := out.Redacted()
	if !red.PasswordRedacted || red.Results[0].InitialPassword != "" || out.Results[0].InitialPassword == "" {
		t.Fatalf("redaction = %+v", red)
	}
	// AC-25: missing/out-of-range grade → 422 for the whole request; not the common tenant → 422.
	for _, bad := range []command.CreateCommonStudentsInput{
		{TenantID: tenant.CommonTenantID, Students: []command.CommonStudentInput{{Username: "hs.ok1", DisplayName: "a", GradeLevel: 0}}},
		{TenantID: tenant.CommonTenantID, Students: []command.CommonStudentInput{{Username: "hs@mail", DisplayName: "a", GradeLevel: 3}}},
		{TenantID: uuid.New(), Students: []command.CommonStudentInput{{Username: "hs.ok2", DisplayName: "a", GradeLevel: 3}}},
	} {
		if _, err := h.Handle(sa, bad); code(err) != problem.ValidationFailed {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	authztest.ExpectDenied(t, func() error {
		_, err := h.Handle(authztest.Ctx(ctx, authztest.Staff(authztest.TenantA, "tenant_admin")), in)
		return err
	}())
}
