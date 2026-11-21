//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// handle runs one consumer handler in the transaction the outbox consumer opens (event TenantScope).
func (e *env) handle(t *testing.T, h func(context.Context, outbox.Envelope) error, typ string, tenant uuid.UUID, payload any) error {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	tt := tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: typ, OccurredAt: time.Now(), TenantID: &tt, Payload: b}
	return e.uow.InTenantTx(context.Background(), scope, func(ctx context.Context) error { return h(ctx, env) })
}

// T007 (TC-SVC-003): the consumers chat.projection_identity and chat.projection_game keep ref.tenants, ref.students and
// ref.player_profiles (newer source_version only, never another tenant); cross_school_social_enabled projected from
// tenant.updated takes effect; a cleaned-up or deactivated student gets the pending requests cancelled (edge case); an
// invalid payload goes to the DLQ at once.
func TestProjections(t *testing.T) {
	e := setup(t, envOpts{})
	p := command.Projection{Refs: persistence.Refs{}, Social: persistence.Social{}, Now: e.clock.Now}
	id := consumer.IdentityProjection{P: p}
	game := consumer.GameProjection{P: p}
	tenant, school, school2 := uuid.New(), uuid.New(), uuid.New()
	if err := e.handle(t, id.Handle, "tenant.created", tenant, map[string]any{"tenant_id": tenant, "tenant_type": "unit", "status": "active",
		"changed_fields": []string{}, "source_version": 1}); err != nil {
		t.Fatal(err)
	}
	students := map[string]uuid.UUID{"a": uuid.New(), "b": uuid.New(), "c": uuid.New()}
	for k, s := range students {
		sc := school
		if k == "b" {
			sc = school2
		}
		if err := e.handle(t, id.Handle, "student.synced", tenant, map[string]any{"student_id": s, "school_id": sc, "action": "create",
			"grade_level": 4, "source_version": 1}); err != nil {
			t.Fatal(err)
		}
		if err := e.handle(t, game.Handle, "game_profile.updated", tenant, map[string]any{"student_id": s, "tenant_id": tenant,
			"nickname": "Nick" + k, "avatar_config_hash": nil, "avatar_config": map[string]any{"body": "body_01"}, "level": 2,
			"source_version": 3}); err != nil {
			t.Fatal(err)
		}
	}
	// An older profile is ignored.
	if err := e.handle(t, game.Handle, "game_profile.updated", tenant, map[string]any{"student_id": students["a"], "tenant_id": tenant,
		"nickname": "Old", "avatar_config_hash": nil, "level": 1, "source_version": 2}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM ref.player_profiles WHERE nickname = 'Nicka' AND level = 2`); n != 1 {
		t.Fatal("older profile applied")
	}
	// A student of tenant B claimed by an event of tenant A → DLQ (permanent).
	other := uuid.New()
	e.tenant(other, "school", nil)
	err := e.handle(t, id.Handle, "student.synced", other, map[string]any{"student_id": students["a"], "school_id": school, "action": "update",
		"source_version": 9})
	if !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("tenant mismatch: %v", err)
	}
	if err := e.handle(t, game.Handle, "game_profile.updated", tenant, map[string]any{"student_id": "x"}); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("invalid payload: %v", err)
	}

	// The API now works on the projected data: cross-school refused until tenant.updated enables it.
	a := student{id: students["a"], tenant: tenant, school: school, sub: "kc-a"}
	b := student{id: students["b"], tenant: tenant, school: school2, sub: "kc-b"}
	c := student{id: students["c"], tenant: tenant, school: school, sub: "kc-c"}
	for _, s := range []*student{&a, &b, &c} {
		tt := tenant
		e.res.Set(s.sub, authz.PrincipalData{UserID: s.id, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: authz.TenantTypeSchool})
		s.token = e.idp.TokenSID(s.sub, "eduworld-game-backend", uuid.NewString(), time.Now())
	}
	e.send(a, b, http.StatusForbidden)
	if err := e.handle(t, id.Handle, "tenant.updated", tenant, map[string]any{"tenant_id": tenant, "tenant_type": "unit", "status": "active",
		"settings_subset": map[string]any{"cross_school_social_enabled": true}, "changed_fields": []string{"settings.cross_school_social_enabled"},
		"source_version": 2}); err != nil {
		t.Fatal(err)
	}
	ab := requestID(e.send(a, b, http.StatusCreated))
	// F015 T044: the settings_subset identity publishes from updateTenantSettings (example of the payload schema:
	// both social keys, source_version) replaces the projection and turns cross-school off; an older event cannot
	// turn it back on; a newer one does.
	setting := tenantSettingsExample(t)
	setting["tenant_id"] = tenant
	if err := e.handle(t, id.Handle, "tenant.updated", tenant, setting); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM ref.tenants WHERE tenant_id = $1
		AND settings_subset = '{"cross_school_social_enabled": false, "common_social_enabled": false}'::jsonb`, tenant); n != 1 {
		t.Fatal("ref.tenants.settings_subset not replaced by the identity payload")
	}
	e.send(c, b, http.StatusForbidden)
	stale := map[string]any{"tenant_id": tenant, "tenant_type": "unit", "status": "active", "changed_fields": []string{"settings"},
		"settings_subset": map[string]any{"cross_school_social_enabled": true, "common_social_enabled": false}, "source_version": 3}
	if err := e.handle(t, id.Handle, "tenant.updated", tenant, stale); err != nil {
		t.Fatal(err)
	}
	e.send(c, b, http.StatusForbidden)
	stale["source_version"] = 5
	if err := e.handle(t, id.Handle, "tenant.updated", tenant, stale); err != nil {
		t.Fatal(err)
	}
	if r := e.call(b, http.MethodGet, "/social/friend-requests", "", nil); len(r.data()) != 1 {
		t.Fatalf("incoming of b: %s", r.raw)
	}
	if req, _ := e.call(b, http.MethodGet, "/social/friend-requests", "", nil).data()[0]["requester"].(map[string]any); req["nickname"] != "Nicka" {
		t.Fatalf("projected nickname: %v", req)
	}
	ca := requestID(e.send(c, a, http.StatusCreated))

	// student.cleaned_up of a cancels its pending requests (both directions).
	if err := e.handle(t, id.Handle, "student.cleaned_up", tenant, map[string]any{"student_id": a.id, "tenant_id": tenant,
		"cleaned_at": time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests WHERE request_id IN ($1, $2) AND status = 'cancelled'`, ab, ca); n != 2 {
		t.Fatalf("cancelled after cleanup: %d", n)
	}
	// student.synced deactivate of c cancels its requests too.
	cb := requestID(e.send(c, b, http.StatusCreated))
	if err := e.handle(t, id.Handle, "student.synced", tenant, map[string]any{"student_id": c.id, "school_id": school, "action": "deactivate",
		"source_version": 5}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM social.friend_requests WHERE request_id = $1 AND status = 'cancelled'`, cb); n != 1 {
		t.Fatal("request of a deactivated student still pending")
	}
}

// tenantSettingsExample returns the example of specs/events/payloads/identity/tenant.updated.v1.schema.json that
// carries settings_subset (the shape updateTenantSettings of module identity publishes, F015 T044).
func tenantSettingsExample(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "identity", "tenant.updated.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Examples []map[string]any `json:"examples"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	for _, x := range s.Examples {
		if _, ok := x["settings_subset"]; ok {
			return x
		}
	}
	t.Fatal("no tenant.updated example with settings_subset")
	return nil
}
