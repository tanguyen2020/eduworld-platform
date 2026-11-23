//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func challengeIDs(r resp) []string {
	var out []string
	for _, it := range r.items("data") {
		out = append(out, it["challenge_id"].(string))
	}
	return out
}

func levelsOf(r resp, id string) []string {
	for _, it := range r.items("data") {
		if it["challenge_id"] == id {
			var out []string
			for _, l := range it["available_difficulties"].([]any) {
				out = append(out, l.(string))
			}
			return out
		}
	}
	return nil
}

// F076-AC-01, AC-02, AC-21: the list holds only the challenges published, assigned to the tenant, of the grade of the
// student (never a class), unlocked, open now and playable solo, with the levels of the grade (grades 1–3 up to hard)
// and the attempts left; keyset pages in the database; another grade, a locked or closed challenge is 404 on read and
// 403 CHALLENGE_NOT_ELIGIBLE on start; query parameters outside their maxLength or enum are 422.
func TestChallengeList(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	future := e.clock.Now().Add(48 * time.Hour)
	locked := "0193a5f4-6c1e-7b2a-9d1f-000000076104"
	later := "0193a5f4-6c1e-7b2a-9d1f-000000076105"
	boss := "0193a5f4-6c1e-7b2a-9d1f-000000076106"
	e.publishChallenges(ctx, t, append(challengeSet()[:0:0],
		challengeItem(locked, "chl.math.g3_locked", "time", []int{3}, func(m map[string]any) { m["unlock_rule"] = map[string]any{"min_level": 5} }),
		challengeItem(later, "chl.math.g3_later", "time", []int{3}, func(m map[string]any) {
			m["availability"] = map[string]any{"start_at": future.Format(time.RFC3339)}
		}),
		challengeItem(boss, "chl.math.g3_party", "time", []int{3}, func(m map[string]any) { m["mode"] = "party_required" }),
	), e.tenantA)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	dung := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA2, 7)
	bao := e.studentOfGrade(ctx, t, e.tenantB, e.schoolB, 3)
	list := func(s student, q string) resp {
		return e.do(t, http.MethodGet, "/game/challenges"+q, e.gtoken(s), th(s), nil)
	}
	r := list(an, "")
	if r.status != http.StatusOK || !slices.Equal(challengeIDs(r), []string{chlTime3, chlSurvival}) {
		t.Fatalf("grade 3: %d %s", r.status, r.raw)
	}
	if got := levelsOf(r, chlSurvival); !slices.Equal(got, []string{"easy", "normal", "hard"}) {
		t.Fatalf("grade 3 up to hard: %v", got)
	}
	if r.items("data")[0]["attempts_left"] != float64(5) || r.items("data")[0]["party_required"] != false {
		t.Fatalf("attempts left: %s", r.raw)
	}
	r = list(dung, "")
	if !slices.Equal(challengeIDs(r), []string{chlSurvival, chlWeekly7}) || len(levelsOf(r, chlSurvival)) != 5 {
		t.Fatalf("grade 7: %s", r.raw)
	}
	if r := list(bao, ""); r.status != http.StatusOK || len(challengeIDs(r)) != 0 {
		t.Fatalf("not assigned to tenant B: %s", r.raw)
	}
	// Keyset pages: limit 1 → the next cursor gives the second challenge, then the end.
	p1 := list(dung, "?limit=1")
	page := p1.body["page"].(map[string]any)
	if !slices.Equal(challengeIDs(p1), []string{chlSurvival}) || page["has_more"] != true {
		t.Fatalf("page 1: %s", p1.raw)
	}
	p2 := list(dung, "?limit=1&cursor="+url.QueryEscape(page["next_cursor"].(string)))
	if !slices.Equal(challengeIDs(p2), []string{chlWeekly7}) || p2.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("page 2: %s", p2.raw)
	}
	if r := list(dung, "?type=weekly"); !slices.Equal(challengeIDs(r), []string{chlWeekly7}) {
		t.Fatalf("type filter: %s", r.raw)
	}
	if r := list(an, "?difficulty=expert"); len(challengeIDs(r)) != 0 {
		t.Fatalf("difficulty filter: %s", r.raw)
	}
	// F076-AC-21 (TC-TEN-024).
	for _, q := range []string{"?area_id=" + strings.Repeat("a", 65), "?type=speedrun", "?difficulty=insane", "?cursor=" + strings.Repeat("x", 513)} {
		if r := list(an, q); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) {
			t.Fatalf("%s: %d %s", q[:12], r.status, r.raw)
		}
	}
	// F076-AC-02: detail with attempts left and the server time limit (grade 3: 300 s × 1.5).
	get := func(s student, id string) resp {
		return e.do(t, http.MethodGet, "/game/challenges/"+id, e.gtoken(s), th(s), nil)
	}
	d := get(an, chlTime3)
	if d.status != http.StatusOK || d.body["attempts_left"] != float64(5) || d.body["time_limit_seconds"] != float64(450) || d.body["version"] != float64(1) {
		t.Fatalf("detail: %s", d.raw)
	}
	for _, id := range []string{chlWeekly7, locked, later, boss, uuid.NewString()} {
		if r := get(an, id); r.status != http.StatusNotFound {
			t.Fatalf("%s for grade 3: %d %s", id, r.status, r.raw)
		}
	}
	if r := get(bao, chlTime3); r.status != http.StatusNotFound {
		t.Fatalf("not assigned: %d", r.status)
	}
	// F076-AC-05 on the same cases: start is 403 CHALLENGE_NOT_ELIGIBLE (unknown or unassigned stays 404).
	for _, id := range []string{chlWeekly7, locked, later} {
		if r := e.startChallenge(t, an, id, "easy", ""); r.status != http.StatusForbidden || r.code() != string(problem.ChallengeNotEligible) {
			t.Fatalf("start %s: %d %s", id, r.status, r.raw)
		}
	}
	for _, id := range []string{boss, uuid.NewString()} {
		if r := e.startChallenge(t, an, id, "easy", ""); r.status != http.StatusNotFound {
			t.Fatalf("start %s: %d %s", id, r.status, r.raw)
		}
	}
	if r := e.startChallenge(t, bao, chlTime3, "easy", ""); r.status != http.StatusNotFound {
		t.Fatalf("start not assigned: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.challenge_attempts`); n != 0 {
		t.Fatalf("attempts written: %d", n)
	}
}

// F076-AC-03 (TC-TEN-001): the challenge routes are player routes (staff → 403 GAME_ACCESS_STUDENT_ONLY in
// TestPlayerRoutesStaffDenied); an X-Tenant-Id of another tenant is 403 TENANT_MISMATCH; the attempt of another student
// is 404 on every attempt route.
func TestChallengeAuthz(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	binh := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	r := e.startChallenge(t, an, chlTime3, "hard", "")
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	attempt := r.str("challenge_attempt_id")
	wrong := map[string]string{"X-Tenant-Id": e.tenantB.String(), "Idempotency-Key": newKey()}
	for _, c := range []struct{ method, path string }{{http.MethodGet, "/game/challenges"}, {http.MethodGet, "/game/challenges/" + chlTime3},
		{http.MethodPost, "/game/challenges/" + chlTime3 + ":start"}, {http.MethodGet, "/game/challenge-attempts/" + attempt},
		{http.MethodPost, "/game/challenge-attempts/" + attempt + "/activities:next"}, {http.MethodPost, "/game/challenge-attempts/" + attempt + ":abandon"}} {
		var body any
		if strings.HasSuffix(c.path, ":start") {
			body = map[string]any{"difficulty": "easy"}
		}
		if r := e.do(t, c.method, c.path, e.gtoken(an), wrong, body); r.status != http.StatusForbidden || r.code() != string(problem.TenantMismatch) {
			t.Fatalf("%s %s: %d %s", c.method, c.path, r.status, r.raw)
		}
	}
	if r := e.challengeAttempt(t, binh, attempt); r.status != http.StatusNotFound {
		t.Fatalf("read another student's attempt: %d", r.status)
	}
	if r := e.nextActivity(t, binh, attempt, ""); r.status != http.StatusNotFound {
		t.Fatalf("next on another student's attempt: %d", r.status)
	}
	if r := e.abandonChallenge(t, binh, attempt, ""); r.status != http.StatusNotFound {
		t.Fatalf("abandon another student's attempt: %d", r.status)
	}
	if r := e.challengeAttempt(t, an, attempt); r.status != http.StatusOK || r.str("status") != "in_progress" {
		t.Fatalf("own attempt untouched: %s", r.raw)
	}
	authztest.Cover(t, "listGameChallenges", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getGameChallenge", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "startGameChallenge", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getGameChallengeAttempt", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "nextChallengeActivity", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "abandonGameChallengeAttempt", authztest.RoleDenied, authztest.CrossTenant)
}
