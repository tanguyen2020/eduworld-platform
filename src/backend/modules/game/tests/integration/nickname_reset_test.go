//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// sanctionEvent is a sanction.applied v1 envelope of module chat (schema owner F116).
func sanctionEvent(t *testing.T, tenant, user, sanction uuid.UUID, typ string, issuer *uuid.UUID) outbox.Envelope {
	t.Helper()
	at := time.Now().UTC()
	b, err := json.Marshal(map[string]any{"tenant_id": tenant, "sanction_id": sanction, "user_id": user, "case_id": uuid.New(),
		"sanction_type": typ, "scope": "all", "channel_id": nil, "reason_code": "moderation", "issued_by_kind": "moderator",
		"issued_by": issuer, "starts_at": at, "until": at, "source_version": at.UnixMicro()})
	if err != nil {
		t.Fatal(err)
	}
	tt := tenant
	return outbox.Envelope{EventID: uuid.New(), EventType: "sanction.applied", SchemaVersion: 1, OccurredAt: at, Producer: "chat",
		TenantID: &tt, Payload: b}
}

func (e *env) deliverSanction(ctx context.Context, t *testing.T, env outbox.Envelope) error {
	t.Helper()
	c := consumer.SanctionNicknameReset{Reset: e.profiles}
	return e.uow.InTenantTx(ctx, scopeOf(t, *env.TenantID), func(ctx context.Context) error { return c.Handle(ctx, env) })
}

// T032 — F136-AC-20 (TC-GAM-014, TC-SVC-003): sanction.applied of type nickname_reset resets the nickname once per
// sanction_id (delivered twice — even after the student chose a new one — it is applied once): nickname and
// nickname_key NULL, setup_required, history "reset" by the moderator, game_profile.updated with nickname null; other
// sanction types are ignored; a payload of another tenant is permanent. The next nickname is not counted by the limit.
func TestNicknameResetConsumer(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.ready(t, an, "Cú Mèo Chăm Chỉ 27")
	if r := e.typeNickname(t, an, "Tên Gây Khó Chịu"); r.status != http.StatusOK { // the one player change of the month
		t.Fatalf("change: %d %s", r.status, r.raw)
	}
	moderator, sanction := uuid.New(), uuid.New()
	if err := e.deliverSanction(ctx, t, sanctionEvent(t, an.tenant, an.id, uuid.New(), "mute", &moderator)); err != nil {
		t.Fatalf("mute: %v", err)
	}
	if p := e.profile(t, an); p.body["nickname"] != "Tên Gây Khó Chịu" {
		t.Fatalf("another sanction type changed the nickname: %s", p.raw)
	}
	ev := sanctionEvent(t, an.tenant, an.id, sanction, "nickname_reset", &moderator)
	if err := e.deliverSanction(ctx, t, ev); err != nil {
		t.Fatalf("reset: %v", err)
	}
	p := e.profile(t, an)
	if p.body["nickname"] != nil || p.body["setup_required"] != true || p.body["avatar_config"] == nil {
		t.Fatalf("after reset: %s", p.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND nickname IS NULL
		AND nickname_key IS NULL`, an.id); n != 1 {
		t.Fatalf("stored reset: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'reset'
		AND old_nickname = 'Tên Gây Khó Chịu' AND new_nickname = '' AND changed_by = $2 AND source_ref_id = $3`, an.id, moderator, sanction); n != 1 {
		t.Fatalf("reset history: %d", n)
	}
	ups := e.payloads(ctx, t, "game_profile.updated")
	if last := ups[len(ups)-1]; last["nickname"] != nil {
		t.Fatalf("game_profile.updated %v", last)
	}
	if e.audits(ctx, t, "game_profile.nickname_reset") != 1 {
		t.Fatal("reset audit")
	}
	// The next nickname is "initial": not counted although the month's change is used.
	if r := e.typeNickname(t, an, "Siêu Nhân Sao Băng"); r.status != http.StatusOK {
		t.Fatalf("choose again: %d %s", r.status, r.raw)
	}
	// The same sanction delivered again (new envelope id: no processed_events guard) is applied once.
	again := ev
	again.EventID = uuid.New()
	if err := e.deliverSanction(ctx, t, again); err != nil {
		t.Fatalf("again: %v", err)
	}
	if p := e.profile(t, an); p.body["nickname"] != "Siêu Nhân Sao Băng" {
		t.Fatalf("redelivery reset again: %s", p.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'reset'`, an.id); n != 1 {
		t.Fatalf("reset rows: %d", n)
	}
	bad := sanctionEvent(t, an.tenant, an.id, uuid.New(), "nickname_reset", &moderator)
	other := e.tenantB
	bad.TenantID = &other
	if err := e.deliverSanction(ctx, t, bad); !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("tenant mismatch: %v", err)
	}
}

// T032 — F136-AC-21 (TC-GAM-014, TC-TEN-001): resetStudentNickname — school_admin of the student's school and
// tenant_admin 200 with audit (actor, reason, old nickname); a student of another school or tenant 404; a student, the
// safety_moderator (platform role without the tenant permission) and a role without lms.student.nickname_reset 403;
// Idempotency-Key required (428) and replayed; reason required (422).
func TestResetStudentNicknameAuthz(t *testing.T) {
	authztest.Cover(t, "resetStudentNickname", authztest.CrossTenant, authztest.RoleDenied)
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.ready(t, an, "Cú Mèo Chăm Chỉ 27")
	e.ready(t, other, "Sóc Nâu Ham Học 42")
	e.ready(t, dung, "Cú Mèo Chăm Chỉ 27")
	adA := e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA))
	adB := e.staff(e.tenantB, "school_admin", authztest.AtSchool(e.schoolB))
	reset := func(token, tenant string, s student, key string, body any) resp {
		h := map[string]string{"X-Tenant-Id": tenant}
		if key != "" {
			h["Idempotency-Key"] = key
		}
		return e.do(t, http.MethodPost, "/tenant/students/"+s.id.String()+"/game-profile/nickname:reset", token, h, body)
	}
	tok := func(sub string) string { return e.portalToken(sub) }
	reason := map[string]any{"reason": "Tên không phù hợp với lứa tuổi"}
	if r := reset(tok(adA), e.tenantA.String(), an, "", reason); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no Idempotency-Key: %d %s", r.status, r.raw)
	}
	if r := reset(tok(adA), e.tenantA.String(), an, newKey(), map[string]any{}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("no reason: %d %s", r.status, r.raw)
	}
	key := newKey()
	r := reset(tok(adA), e.tenantA.String(), an, key, reason)
	if r.status != http.StatusOK || r.body["student_id"] != an.id.String() || r.body["nickname"] != nil || r.body["reset_at"] == nil {
		t.Fatalf("school admin: %d %s", r.status, r.raw)
	}
	if p := e.profile(t, an); p.body["nickname"] != nil || p.body["setup_required"] != true {
		t.Fatalf("after reset: %s", p.raw)
	}
	replay := reset(tok(adA), e.tenantA.String(), an, key, reason)
	if replay.status != http.StatusOK || replay.body["reset_at"] != r.body["reset_at"] {
		t.Fatalf("replay: %d %s", replay.status, replay.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'reset'
		AND old_nickname = 'Cú Mèo Chăm Chỉ 27'`, an.id); n != 1 {
		t.Fatalf("reset history: %d", n)
	}
	if e.audits(ctx, t, "game_profile.nickname_reset") != 1 {
		t.Fatal("audit game_profile.nickname_reset")
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' = 'game_profile.nickname_reset'
		AND envelope::text LIKE '%Tên không phù hợp với lứa tuổi%' AND envelope::text LIKE '%Cú Mèo Chăm Chỉ 27%'`); n != 1 {
		t.Fatal("audit carries the reason and the old nickname")
	}
	if r := reset(tok(adA), e.tenantA.String(), other, newKey(), reason); r.status != http.StatusNotFound {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	if r := reset(tok(adB), e.tenantB.String(), an, newKey(), reason); r.status != http.StatusNotFound {
		t.Fatalf("tenant B → student of tenant A: %d %s", r.status, r.raw)
	}
	if r := reset(tok(adA), e.tenantA.String(), dung, newKey(), reason); r.status != http.StatusNotFound {
		t.Fatalf("tenant A → student of tenant B: %d %s", r.status, r.raw)
	}
	if r := reset(e.gtoken(an), e.tenantA.String(), an, newKey(), reason); r.status != http.StatusForbidden {
		t.Fatalf("student: %d %s", r.status, r.raw)
	}
	if r := reset(tok(e.staff(e.tenantA, "tenant_content_editor")), e.tenantA.String(), other, newKey(), reason); r.status != http.StatusForbidden ||
		r.code() != string(problem.PermissionDenied) {
		t.Fatalf("role without permission: %d %s", r.status, r.raw)
	}
	if r := reset(tok(e.platformStaff("safety_moderator")), e.tenantA.String(), other, newKey(), reason); r.status != http.StatusForbidden {
		t.Fatalf("safety_moderator on the tenant route: %d %s", r.status, r.raw)
	}
	ta := e.staff(e.tenantA, "tenant_admin")
	if r := reset(tok(ta), e.tenantA.String(), other, newKey(), reason); r.status != http.StatusOK {
		t.Fatalf("tenant admin: %d %s", r.status, r.raw)
	}
	// Choosing again after the reset is not counted by the monthly limit.
	if r := e.typeNickname(t, other, "Rồng Xanh Bay Cao"); r.status != http.StatusOK {
		t.Fatalf("choose again: %d %s", r.status, r.raw)
	}
	if r := e.typeNickname(t, other, "Rồng Đỏ Bay Cao"); r.status != http.StatusOK {
		t.Fatalf("one player change still left: %d %s", r.status, r.raw)
	}
}
