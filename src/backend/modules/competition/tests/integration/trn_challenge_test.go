//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

func fieldOf(r presp) string {
	errs, _ := r.body["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	m, _ := errs[0].(map[string]any)
	f, _ := m["field"].(string)
	return f
}

// T009, T010 / F107-AC-01, AC-02, AC-04, AC-05 (TC-CMS-AUTHZ-07, TC-CMS-ASG-01, TC-TEN-024): the school admin creates a
// grade challenge from an assigned template — the tournament (draft, scope school, grade 3) pins the template version,
// the audience is grade 3 ∪ listed − excluded; the same Idempotency-Key replays the same challenge with one audit; a
// parameter outside allowed_params, an unassigned template, a student of another school, the limits and the school hours
// are refused; the name goes through the chat screening (flagged 422, screening down 503 and nothing stored); a
// timed-quiz template creates a scheduled timed quiz without tournament.
func TestF107CreateChallenge(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	e.assignTpl(e.tenantA, tplQuickQuiz)
	admin := e.adminOf(e.tenantA, e.schoolA)
	an, binh, chi := e.kid(e.tenantA, e.schoolA, 3), e.kid(e.tenantA, e.schoolA, 3), e.kid(e.tenantA, e.schoolA, 3)
	em := e.kid(e.tenantA, e.schoolA, 4)
	other := e.kid(e.tenantA, e.schoolA2, 3)
	_ = an
	_ = binh
	body := e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) {
		b["targets"] = []map[string]any{{"target_type": "grade", "grade_level": 3}, {"target_type": "students", "student_ids": []string{em.id.String()}}}
		b["exclude_student_ids"] = []string{chi.id.String()}
	})
	idem := key()
	r := e.staff(admin, e.tenantA, http.MethodPost, "/lms/challenges", idem, body)
	if r.status != 201 || r.str("status") != "scheduled" || r.str("kind") != "grade_challenge" || r.str("format") != "tournament" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	if n, _ := r.body["audience_count"].(float64); n != 3 {
		t.Fatalf("audience = grade 3 (an, binh) + em − chi = 3: %s", r.raw)
	}
	tid := tournamentOf(t, r)
	var status, scope string
	var version uuid.UUID
	var grades string
	if err := e.su.QueryRow(`SELECT status, owner_scope, rule_version_id, grade_levels::text FROM competition.tournaments WHERE tournament_id = $1`,
		tid).Scan(&status, &scope, &version, &grades); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || scope != "school" || version != uuid.MustParse("01936a00-0000-7000-8000-000000001711") || grades != "[3, 4]" {
		t.Fatalf("tournament %s %s %s %s", status, scope, version, grades)
	}
	again := e.staff(admin, e.tenantA, http.MethodPost, "/lms/challenges", idem, body)
	if again.status != 201 || again.str("challenge_id") != r.str("challenge_id") || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if n := len(e.audits("challenge.created")); n != 1 {
		t.Fatalf("%d challenge.created audits", n)
	}
	if a := e.audits("challenge.created")[0]; !strings.Contains(e.rawJSON(a), "01936a00-0000-7000-8000-000000001711") {
		t.Fatalf("audit carries the template version: %v", a)
	}
	// F107-AC-02: a parameter outside allowed_params; an unassigned template.
	bad := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) {
		b["params"] = map[string]any{"subject_id": e.subject.String(), "reward": "x"}
	}), 422)
	if fieldOf(bad) != "params.reward" {
		t.Fatalf("reward parameter: %s", bad.raw)
	}
	if r := e.createCh(admin, e.tenantA, e.challengeBody(tplRoundRobin, e.schoolA, 4, nil), 403); r.code() != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("unassigned template: %s", r.raw)
	}
	// F107-AC-05: limits, another school, schedule, grade.
	cases := []func(b map[string]any){
		func(b map[string]any) { b["name"] = strings.Repeat("a", 81) },
		func(b map[string]any) { b["ends_at"] = b["starts_at"] },
		func(b map[string]any) { b["starts_at"] = e.clock.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339) },
		func(b map[string]any) { b["targets"] = []map[string]any{} },
		func(b map[string]any) {
			b["targets"] = []map[string]any{{"target_type": "students", "student_ids": []string{other.id.String()}}}
		},
		func(b map[string]any) { b["targets"] = []map[string]any{{"target_type": "grade", "grade_level": 10}} },
		func(b map[string]any) {
			ex := make([]string, 501)
			for i := range ex {
				ex[i] = uuid.NewString()
			}
			b["exclude_student_ids"] = ex
		},
		func(b map[string]any) { b["participation"] = "study_group" },
	}
	for i, edit := range cases {
		if r := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, edit), 0); r.status != 422 {
			t.Errorf("case %d: %d %s", i, r.status, r.raw)
		}
	}
	// the name screened by module chat (R15): flagged → 422 on name; down → 503 and nothing stored.
	if r := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) { b["name"] = "Giải " + blockedTerm }),
		422); fieldOf(r) != "name" || strings.Contains(r.raw, blockedTerm) {
		t.Fatalf("flagged name: %s", r.raw)
	}
	before := e.count(`SELECT count(*) FROM competition.challenges`)
	e.screen.down = true
	e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, nil), 503)
	e.screen.down = false
	if e.count(`SELECT count(*) FROM competition.challenges`) != before {
		t.Fatal("a challenge stored without screening")
	}
	// the access hours of the school must cover the match (OQ-17): 07:00–08:00 only.
	e.exec(`UPDATE ref.schools SET settings_subset = '{"allowed_hours":{"timezone":"Asia/Ho_Chi_Minh","windows":[{"days":["mon","tue","wed","thu","fri","sat","sun"],"from":"07:00","to":"08:00"}]}}'
		WHERE school_id = $1`, e.schoolA)
	if r := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, nil), 422); fieldOf(r) != "starts_at" {
		t.Fatalf("outside the school hours: %s", r.raw)
	}
	e.exec(`UPDATE ref.schools SET settings_subset = '{}' WHERE school_id = $1`, e.schoolA)
	// a timed quiz: scheduled, no tournament.
	q := e.createCh(admin, e.tenantA, e.challengeBody(tplQuickQuiz, e.schoolA, 3, nil), 201)
	if q.str("format") != "timed_quiz" || q.body["tournament_id"] != nil || q.str("status") != "scheduled" {
		t.Fatalf("timed quiz: %s", q.raw)
	}
	// a class template stored by mistake is refused (ADR-0095).
	classTpl := uuid.New()
	e.exec(`INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, latest_version_no, owner_scope)
		VALUES ($1, now(), now(), 1, 'challenge-templates', 'legacy_class', 1, 'platform')`, classTpl)
	ver := uuid.New()
	e.exec(`INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
		published_at) VALUES ($1, now(), now(), 1, $2, 1, 'published', 'legacy', '{"kind":"class_challenge","format":"tournament"}', now())`, ver, classTpl)
	e.exec(`UPDATE competition.game_configs SET current_version_id = $1 WHERE config_id = $2`, ver, classTpl)
	e.assignTpl(e.tenantA, classTpl)
	if r := e.createCh(admin, e.tenantA, e.challengeBody(classTpl, e.schoolA, 3, nil), 422); fieldOf(r) != "template_id" {
		t.Fatalf("class template: %s", r.raw)
	}
	authztest.Cover(t, "createChallenge", authztest.CrossTenant, authztest.RoleDenied)
}

// T009 / F107-AC-03 (TC-TEN-003, TC-CMS-TEN-03): only the school admin of the school and the System Admin of the target
// tenant create: academic manager and tenant admin 403, another school of the tenant or a school of another tenant 404,
// a System Admin without tenant_id 422 (with it 201), a student 403, the header of another tenant 403 TENANT_MISMATCH.
func TestF107ChallengeAuthz(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	body := e.challengeBody(tplSingleElim, e.schoolA, 3, nil)
	for _, role := range []string{"academic_manager", "tenant_admin"} {
		if r := e.createCh(e.staffToken(e.tenantA, role), e.tenantA, body, 403); r.code() != "PERMISSION_DENIED" {
			t.Fatalf("%s: %s", role, r.raw)
		}
	}
	admin := e.adminOf(e.tenantA, e.schoolA)
	e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA2, 3, nil), 404)
	e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolB, 3, nil), 404)
	if r := e.createCh(e.sysAdmin(e.tenantA), e.tenantA, body, 422); fieldOf(r) != "tenant_id" {
		t.Fatalf("system admin without tenant: %s", r.raw)
	}
	withTenant := e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) { b["tenant_id"] = e.tenantA })
	e.createCh(e.sysAdmin(e.tenantA), e.tenantA, withTenant, 201)
	other := e.challengeBody(tplSingleElim, e.schoolA, 3, func(b map[string]any) { b["tenant_id"] = e.tenantB })
	if r := e.createCh(admin, e.tenantA, other, 403); r.code() != "TENANT_MISMATCH" {
		t.Fatalf("body tenant of another tenant: %s", r.raw)
	}
	if r := e.createCh(admin, e.tenantB, body, 403); r.code() != "TENANT_MISMATCH" {
		t.Fatalf("header of another tenant: %s", r.raw)
	}
	k := e.kid(e.tenantA, e.schoolA, 3)
	if r := e.call(k, http.MethodPost, "/lms/challenges", key(), body); r.status != 403 {
		t.Fatalf("student: %d %s", r.status, r.raw)
	}
}

// T010 / F107-AC-06: list (grade, status, keyset pages), get (another school 404), cancel (soft: cancelled with its reason
// and its tournament, the row stays), cancel again 422; the challenge publishes no learning event (results stay out of
// the learning reports).
func TestF107ChallengeLifecycle(t *testing.T) {
	e := setupTrn(t)
	e.assignTpl(e.tenantA, tplSingleElim)
	e.assignTpl(e.tenantA, tplQuickQuiz)
	admin := e.adminOf(e.tenantA, e.schoolA)
	e.kidsOf(3, 3)
	c1 := e.createCh(admin, e.tenantA, e.challengeBody(tplSingleElim, e.schoolA, 3, nil), 201)
	e.clock.Add(time.Minute)
	c2 := e.createCh(admin, e.tenantA, e.challengeBody(tplQuickQuiz, e.schoolA, 4, nil), 201)
	p1 := e.staff(admin, e.tenantA, http.MethodGet, "/lms/challenges?limit=1", "", nil)
	if p1.status != 200 || len(p1.data()) != 1 {
		t.Fatalf("page 1: %d %s", p1.status, p1.raw)
	}
	page, _ := p1.body["page"].(map[string]any)
	cursor, _ := page["next_cursor"].(string)
	p2 := e.staff(admin, e.tenantA, http.MethodGet, "/lms/challenges?limit=1&cursor="+cursor, "", nil)
	if len(p2.data()) != 1 || p2.data()[0]["challenge_id"] == p1.data()[0]["challenge_id"] {
		t.Fatalf("page 2: %s", p2.raw)
	}
	if r := e.staff(admin, e.tenantA, http.MethodGet, "/lms/challenges?grade_level=4", "", nil); len(r.data()) != 1 ||
		r.data()[0]["challenge_id"] != c2.str("challenge_id") {
		t.Fatalf("grade filter: %s", r.raw)
	}
	if r := e.staff(admin, e.tenantA, http.MethodGet, "/lms/challenges/"+c1.str("challenge_id"), "", nil); r.status != 200 {
		t.Fatalf("get: %d %s", r.status, r.raw)
	}
	if r := e.staff(e.adminOf(e.tenantA, e.schoolA2), e.tenantA, http.MethodGet, "/lms/challenges/"+c1.str("challenge_id"), "", nil); r.status != 404 {
		t.Fatalf("another school: %d", r.status)
	}
	if r := e.staff(e.adminOf(e.tenantB, e.schoolB), e.tenantB, http.MethodGet, "/lms/challenges/"+c1.str("challenge_id"), "", nil); r.status != 404 {
		t.Fatalf("another tenant: %d", r.status)
	}
	cancel := e.staff(admin, e.tenantA, http.MethodPost, "/lms/challenges/"+c1.str("challenge_id")+":cancel", key(), map[string]any{"reason": "Đổi lịch thi"})
	if cancel.status != 200 || cancel.str("status") != "cancelled" || cancel.str("cancel_reason") != "Đổi lịch thi" {
		t.Fatalf("cancel: %d %s", cancel.status, cancel.raw)
	}
	if e.tstatus(tournamentOf(t, c1)) != "cancelled" {
		t.Fatal("the tournament of the challenge is cancelled with it")
	}
	if r := e.staff(admin, e.tenantA, http.MethodPost, "/lms/challenges/"+c1.str("challenge_id")+":cancel", key(), map[string]any{"reason": "x"}); r.status != 422 ||
		r.code() != "INVALID_STATE_TRANSITION" {
		t.Fatalf("cancel twice: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.challenges WHERE challenge_id = $1 AND deleted_at IS NULL`, c1.str("challenge_id")); n != 1 {
		t.Fatal("a cancelled challenge stays (soft)")
	}
	if len(e.audits("challenge.cancelled")) != 1 || len(e.audits("tournament.cancelled")) != 1 {
		t.Fatal("cancel audits")
	}
	if n := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type NOT IN ('audit.recorded')`); n != 0 {
		t.Fatalf("%d domain events from challenges (no learning report)", n)
	}
	authztest.Cover(t, "listChallenges", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getChallenge", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "cancelChallenge", authztest.CrossTenant, authztest.RoleDenied)
}
