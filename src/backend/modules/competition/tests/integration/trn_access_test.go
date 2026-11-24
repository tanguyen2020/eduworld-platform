//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// T033 / F107-AC-25, AC-26 (TC-AUTH-014, TC-TEN-001, TC-TEN-011): a student of the audience lists and reads the
// tournament and its bracket with nicknames only; out of the scope (other school, excluded, grade, other tenant) or a
// draft is 404; the organizer of the school reads it, another school 404; staff with a game token is
// GAME_ACCESS_STUDENT_ONLY; a student on an organizer route PERMISSION_DENIED; the header of another tenant TENANT_MISMATCH.
func TestF107TournamentAccess(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	e.assignTpl(e.tenantB, tplSingleElim)
	admin := e.adminOf(e.tenantA, e.schoolA)
	an, chi := e.kid(e.tenantA, e.schoolA, 3), e.kid(e.tenantA, e.schoolA, 3)
	far := e.kid(e.tenantA, e.schoolA2, 3)
	g4 := e.kid(e.tenantA, e.schoolA, 4)
	dung := e.kid(e.tenantB, e.schoolB, 3)
	ch := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) {
		b["exclude_student_ids"] = []string{chi.id.String()}
	}), 201)
	tid := tournamentOf(t, ch)
	path := "/competition/tournaments/" + tid
	if r := e.call(an, http.MethodGet, path, "", nil); r.status != 404 {
		t.Fatalf("a draft: %d", r.status)
	}
	e.staff(admin, e.tenantA, http.MethodPost, path+":publish", key(), nil)
	if r := e.call(an, http.MethodPost, path+"/entries", key(), nil); r.status != 201 {
		t.Fatalf("register: %d %s", r.status, r.raw)
	}
	list := e.call(an, http.MethodGet, "/competition/tournaments", "", nil)
	if list.status != 200 || len(list.data()) != 1 || list.data()[0]["tournament_id"] != tid {
		t.Fatalf("student list: %d %s", list.status, list.raw)
	}
	mine := e.call(an, http.MethodGet, "/competition/tournaments?scope=mine", "", nil)
	if len(mine.data()) != 1 {
		t.Fatalf("scope mine: %s", mine.raw)
	}
	get := e.call(an, http.MethodGet, path, "", nil)
	if get.status != 200 || get.body["my_entry"] == nil {
		t.Fatalf("get with my entry: %d %s", get.status, get.raw)
	}
	br := e.call(an, http.MethodGet, path+"/bracket", "", nil)
	if br.status != 200 {
		t.Fatalf("bracket: %d %s", br.status, br.raw)
	}
	for _, en := range br.body["entries"].([]any) {
		for _, p := range en.(map[string]any)["players"].([]any) {
			for k := range p.(map[string]any) {
				if k != "student_id" && k != "nickname" && k != "avatar_config" {
					t.Fatalf("a player carries %s (R16): %s", k, br.raw)
				}
			}
		}
	}
	if strings.Contains(br.raw, "seed_value") || strings.Contains(br.raw, "full_name") {
		t.Fatalf("student bracket leaks: %s", br.raw)
	}
	for _, k := range []kid{chi, far, g4} {
		if r := e.call(k, http.MethodGet, path, "", nil); r.status != 404 {
			t.Fatalf("%s out of the scope: %d", k.nickname, r.status)
		}
		if r := e.call(k, http.MethodGet, "/competition/tournaments", "", nil); len(r.data()) != 0 {
			t.Fatalf("%s lists a tournament out of its scope: %s", k.nickname, r.raw)
		}
	}
	if r := e.call(dung, http.MethodGet, path, "", nil); r.status != 404 {
		t.Fatalf("other tenant: %d", r.status)
	}
	if r := e.call(dung, http.MethodGet, path+"/bracket", "", nil); r.status != 404 {
		t.Fatalf("other tenant bracket: %d", r.status)
	}
	if r := e.staff(admin, e.tenantA, http.MethodGet, path, "", nil); r.status != 200 || r.body["my_entry"] != nil {
		t.Fatalf("organizer: %d %s", r.status, r.raw)
	}
	if r := e.staff(admin, e.tenantA, http.MethodGet, path+"/bracket", "", nil); r.status != 200 || !strings.Contains(r.raw, "seed_value") {
		t.Fatalf("organizer bracket: %d %s", r.status, r.raw)
	}
	other := e.adminOf(e.tenantA, e.schoolA2)
	if r := e.staff(other, e.tenantA, http.MethodGet, path, "", nil); r.status != 404 {
		t.Fatalf("another school: %d", r.status)
	}
	if r := e.staff(other, e.tenantA, http.MethodGet, "/competition/tournaments", "", nil); len(r.data()) != 0 {
		t.Fatalf("another school lists: %s", r.raw)
	}
	if r := e.staff(e.adminOf(e.tenantB, e.schoolB), e.tenantB, http.MethodGet, path, "", nil); r.status != 404 {
		t.Fatalf("another tenant: %d", r.status)
	}
	if r := e.staff(e.staffToken(e.tenantA, "academic_manager"), e.tenantA, http.MethodGet, "/competition/tournaments", "", nil); r.status != 403 {
		t.Fatalf("academic manager: %d", r.status)
	}
	sg := e.staffGame(e.tenantA)
	if r := e.staff(sg, e.tenantA, http.MethodPost, path+"/entries", key(), nil); r.status != 403 || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff with a game token: %d %s", r.status, r.raw)
	}
	if r := e.staff(sg, e.tenantA, http.MethodGet, "/competition/challenges", "", nil); r.status != 403 || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff on listMyChallenges: %d %s", r.status, r.raw)
	}
	node := uuid.NewString()
	for _, c := range []struct{ method, path string }{{http.MethodPost, path + ":publish"}, {http.MethodPost, path + ":cancel"},
		{http.MethodPost, path + "/entries:confirm"}, {http.MethodPost, path + ":generate-bracket"},
		{http.MethodPost, path + "/nodes/" + node + ":confirm"}, {http.MethodPost, path + "/nodes/" + node + ":override"}} {
		var body any
		if strings.HasSuffix(c.path, ":cancel") {
			body = map[string]any{"reason": "x"}
		}
		if strings.HasSuffix(c.path, ":override") {
			body = map[string]any{"winner_entry_id": uuid.NewString(), "reason": "x"}
		}
		if r := e.call(an, c.method, c.path, key(), body); r.status != 403 || r.code() != "PERMISSION_DENIED" {
			t.Fatalf("student on %s: %d %s", c.path, r.status, r.raw)
		}
	}
	h := map[string]string{"X-Tenant-Id": e.tenantB.String()}
	if r := e.do(http.MethodGet, path, an.token, h, nil, ""); r.status != 403 || r.code() != "TENANT_MISMATCH" {
		t.Fatalf("header of another tenant: %d %s", r.status, r.raw)
	}
	cancel := e.staff(admin, e.tenantA, http.MethodPost, path+":cancel", key(), map[string]any{"reason": "Hoãn giải"})
	if cancel.status != 200 || cancel.str("status") != "cancelled" {
		t.Fatalf("cancel: %d %s", cancel.status, cancel.raw)
	}
	authztest.Cover(t, "cancelTournament", authztest.CrossTenant, authztest.RoleDenied)
}

// T034 / F107-AC-27: a student of the audience of a timed-quiz challenge lists it and joins in the window (the room is
// returned, the same before and after the start), before the window or after the end 422, outside the audience 404; at
// starts_at the rooms (at most 40 students each) are created as unranked timed_quiz matches and the challenge runs; at
// ends_at it completes.
func TestF107JoinChallenge(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplQuickQuiz)
	admin := e.adminOf(e.tenantA, e.schoolA)
	kids := e.kidsOf(41, 3)
	chi := kids[40]
	far := e.kid(e.tenantA, e.schoolA2, 3)
	ch := e.createCh(admin, e.tenantA, e.challengeBody(tplQuickQuiz, e.schoolA, 3, func(b map[string]any) {
		b["exclude_student_ids"] = []string{chi.id.String()}
	}), 201)
	cid := ch.str("challenge_id")
	an := kids[0]
	if r := e.call(an, http.MethodGet, "/competition/challenges", "", nil); r.status != 200 || len(r.data()) != 1 || r.data()[0]["challenge_id"] != cid {
		t.Fatalf("my challenges: %d %s", r.status, r.raw)
	}
	if r := e.call(chi, http.MethodGet, "/competition/challenges", "", nil); len(r.data()) != 0 {
		t.Fatalf("an excluded student lists it: %s", r.raw)
	}
	join := "/competition/challenges/" + cid + ":join"
	if r := e.call(an, http.MethodPost, join, key(), nil); r.status != 422 || r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("before the window: %d %s", r.status, r.raw)
	}
	e.clock.Add(2*time.Hour - 4*time.Minute) // starts_at − 4 min (window opens 5 min before)
	early := e.call(an, http.MethodPost, join, key(), nil)
	if early.status != 200 || early.str("match_id") == "" || early.str("match_id") != early.str("room_id") {
		t.Fatalf("join in the window: %d %s", early.status, early.raw)
	}
	for _, k := range []kid{chi, far} {
		if r := e.call(k, http.MethodPost, join, key(), nil); r.status != 404 {
			t.Fatalf("%s outside the audience: %d", k.nickname, r.status)
		}
	}
	e.clock.Add(4 * time.Minute)
	e.tick()
	var status, matches string
	_ = e.su.QueryRow(`SELECT status, match_ids::text FROM competition.challenges WHERE challenge_id = $1`, cid).Scan(&status, &matches)
	if status != "running" {
		t.Fatalf("running at starts_at: %s", status)
	}
	if n := e.count(`SELECT count(*) FROM competition.matches WHERE match_id::text = ANY(SELECT jsonb_array_elements_text(match_ids) FROM
		competition.challenges WHERE challenge_id = $1) AND mode = 'timed_quiz' AND ranked = false`, cid); n != 1 {
		t.Fatalf("40 students (41 − excluded) → 1 room, got %d", n)
	}
	if n := e.count(`SELECT count(*) FROM competition.match_participants WHERE match_id = $1`, early.str("match_id")); n != 40 {
		t.Fatalf("room of %d students", n)
	}
	late := e.call(an, http.MethodPost, join, key(), nil)
	if late.status != 200 || late.str("match_id") != early.str("match_id") {
		t.Fatalf("the same room after the start: %s", late.raw)
	}
	if len(e.push.of(an.id, "matchmaking.match.assigned")) != 1 {
		t.Fatal("the room is pushed to the students (matchmaking.match.assigned)")
	}
	e.clock.Add(10 * time.Minute)
	if r := e.call(an, http.MethodPost, join, key(), nil); r.status != 422 {
		t.Fatalf("after the end: %d", r.status)
	}
	e.tick()
	_ = e.su.QueryRow(`SELECT status FROM competition.challenges WHERE challenge_id = $1`, cid).Scan(&status)
	if status != "completed" {
		t.Fatalf("completed at ends_at: %s", status)
	}
	// two rooms when the audience exceeds 40.
	for i := 0; i < 2; i++ {
		e.kid(e.tenantA, e.schoolA, 5)
	}
	big := e.kidsOf(39, 5)
	_ = big
	c2 := e.createCh(admin, e.tenantA, e.challengeBody(tplQuickQuiz, e.schoolA, 5, nil), 201)
	e.clock.Add(2 * time.Hour)
	e.tick()
	if n := e.count(`SELECT jsonb_array_length(match_ids) FROM competition.challenges WHERE challenge_id = $1`, c2.str("challenge_id")); n != 2 {
		t.Fatalf("41 students → 2 rooms, got %d", n)
	}
	authztest.Cover(t, "listMyChallenges", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "joinChallenge", authztest.CrossTenant, authztest.RoleDenied)
}

// T036 / F107-AC-29 (TC-SVC-006): the facade of module competition assigns a published template to tenants for the
// identity saga (idempotent by Idempotency-Key, audit per tenant), lists and revokes (soft-delete); a challenge created
// before the revoke keeps running; afterwards the tenant cannot create from it (403 RESOURCE_NOT_ASSIGNED); a
// non-template resource 422, a tenant user 403. Only module identity may import this facade (x-allowed-callers).
func TestF107Assignments(t *testing.T) {
	e := setupTrn(t)
	sys := authztest.Platform("system_admin")
	store := idempotency.NewStore(e.app, 0, nil)
	f := facade.Facade{Assign: &e.assign, AssignList: &e.alist, Idempotency: store}
	ctxOf := func(p authz.Principal, idem string) context.Context {
		ctx := tenancy.WithModule(authz.WithPrincipal(context.Background(), p), "competition")
		if idem != "" {
			ctx = modcall.WithIdempotencyKey(ctx, idem)
		}
		return ctx
	}
	body := api.ApplyTenantAssignmentsJSONRequestBody{ResourceType: "challenge_template", ResourceId: tplSingleElim, TenantIds: []uuid.UUID{e.tenantA, e.tenantB}}
	k := key()
	res, err := f.ApplyTenantAssignments(ctxOf(sys, k), api.ApplyTenantAssignmentsRequestObject{Body: &body})
	if err != nil {
		t.Fatal(err)
	}
	if l := res.(api.ApplyTenantAssignments200JSONResponse); len(l.Data) != 2 || l.Data[0].Status != "active" {
		t.Fatalf("apply: %+v", l)
	}
	if _, err := f.ApplyTenantAssignments(ctxOf(sys, k), api.ApplyTenantAssignmentsRequestObject{Body: &body}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND deleted_at IS NULL`, tplSingleElim); n != 2 {
		t.Fatalf("idempotent apply: %d rows", n)
	}
	if len(e.audits(command.AuditTenantDataAssigned)) != 2 {
		t.Fatal("audit per tenant")
	}
	lres, err := f.ListResourceAssignments(ctxOf(sys, ""), api.ListResourceAssignmentsRequestObject{Params: api.ListResourceAssignmentsParams{
		ResourceType: "challenge_template", ResourceId: tplSingleElim}})
	if err != nil || len(lres.(api.ListResourceAssignments200JSONResponse).Data) != 2 {
		t.Fatalf("list: %v", err)
	}
	admin := e.adminOf(e.tenantB, e.schoolB)
	e.kid(e.tenantB, e.schoolB, 3)
	before := e.createCh(admin, e.tenantB, e.challengeBody(tplSingleElim, e.schoolB, 3, nil), 201)
	rv := api.RevokeTenantAssignmentsJSONRequestBody{ResourceType: "challenge_template", ResourceId: tplSingleElim, TenantIds: []uuid.UUID{e.tenantB},
		Reason: "Hết mùa thi đấu"}
	rres, err := f.RevokeTenantAssignments(ctxOf(sys, key()), api.RevokeTenantAssignmentsRequestObject{Body: &rv})
	if err != nil || len(rres.(api.RevokeTenantAssignments200JSONResponse).Data) != 1 {
		t.Fatalf("revoke: %v", err)
	}
	if n := e.count(`SELECT count(*) FROM infra.tenant_resource_assignments WHERE resource_id = $1 AND tenant_id = $2 AND deleted_at IS NOT NULL`,
		tplSingleElim, e.tenantB); n != 1 {
		t.Fatal("revoke soft-deletes the assignment")
	}
	if r := e.createCh(admin, e.tenantB, e.challengeBody(tplSingleElim, e.schoolB, 3, nil), 403); r.code() != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("after the revoke: %s", r.raw)
	}
	if r := e.staff(admin, e.tenantB, http.MethodPost, "/competition/tournaments/"+tournamentOf(t, before)+":publish", key(), nil); r.status != 200 {
		t.Fatalf("a tournament created before keeps running: %d %s", r.status, r.raw)
	}
	bad := api.ApplyTenantAssignmentsJSONRequestBody{ResourceType: "challenge_template", ResourceId: uuid.New(), TenantIds: []uuid.UUID{e.tenantA}}
	if _, err := f.ApplyTenantAssignments(ctxOf(sys, key()), api.ApplyTenantAssignmentsRequestObject{Body: &bad}); err == nil ||
		!strings.Contains(err.Error(), "VALIDATION_FAILED") && !strings.Contains(err.Error(), "resource_id") {
		t.Fatalf("not a template: %v", err)
	}
	if _, err := f.ApplyTenantAssignments(ctxOf(authztest.Staff(e.tenantA, "school_admin"), key()), api.ApplyTenantAssignmentsRequestObject{Body: &body}); err == nil {
		t.Fatal("a tenant user assigns")
	}
	if callers := api.AllowedCallers["applyTenantAssignments"]; len(callers) != 1 || callers[0] != "identity" {
		t.Fatalf("x-allowed-callers: %v", callers)
	}
}
