//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F076-AC-04, AC-05 (TC-CHL-001, TC-CHL-002): start pins the current version, sets deadline_at on the server clock
// (grade 3: 300 s × 1.5) and the lives of the level (survival hard: 3), writes challenge.started and answers 201; a
// level not allowed for the grade (expert for grade 3) or a party_id is 403 CHALLENGE_NOT_ELIGIBLE and writes nothing.
// A new version published during the play keeps the attempt on its pinned version (ADR-0043).
func TestChallengeStart(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	now := e.clock.Now()
	r := e.startChallenge(t, an, chlTime3, "hard", "")
	if r.status != http.StatusCreated || r.str("status") != "in_progress" || r.str("difficulty") != "hard" || r.body["challenge_version"] != float64(1) {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	deadline, err := time.Parse(time.RFC3339Nano, r.str("deadline_at"))
	if err != nil || !deadline.Equal(now.Add(450*time.Second)) || r.body["lives_left"] != nil {
		t.Fatalf("deadline on the server clock: %v %s", err, r.raw)
	}
	attempt := r.str("challenge_attempt_id")
	var version, period string
	if err := e.platform.QueryRowContext(ctx, `SELECT a.challenge_version_id::text, a.period_key FROM game.challenge_attempts a
		WHERE a.challenge_attempt_id = $1`, attempt).Scan(&version, &period); err != nil {
		t.Fatal(err)
	}
	if period != "2026-11-19" {
		t.Fatalf("period of the day (ICT, reset 04:00): %s", period)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'challenge.started'
		AND envelope->'payload'->>'challenge_attempt_id' = $1 AND envelope->'payload'->>'difficulty' = 'hard'
		AND envelope->>'tenant_id' = $2`, attempt, an.tenant.String()); n != 1 {
		t.Fatalf("challenge.started: %d", n)
	}
	// A new version while playing: the attempt stays pinned to version 1.
	e.publishChallenges(ctx, t, []command.ContentItem{challengeItem(chlTime3, "chl.math.g3_time", "time", []int{3}, func(m map[string]any) {
		m["goal"] = map[string]any{"metric": "correct_answers", "target": 8}
	})}, e.tenantA)
	if r := e.challengeAttempt(t, an, attempt); r.body["challenge_version"] != float64(1) {
		t.Fatalf("pinned version: %s", r.raw)
	}
	var pinned string
	_ = e.platform.QueryRowContext(ctx, `SELECT challenge_version_id::text FROM game.challenge_attempts WHERE challenge_attempt_id = $1`,
		attempt).Scan(&pinned)
	if pinned != version {
		t.Fatal("the attempt changed version")
	}
	// Survival hard for grade 7: 3 lives, 900 s.
	dung := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA2, 7)
	s := e.startChallenge(t, dung, chlSurvival, "hard", "")
	if s.status != http.StatusCreated || s.body["lives_left"] != float64(3) {
		t.Fatalf("survival: %d %s", s.status, s.raw)
	}
	// AC-05: expert for grade 3, a party_id.
	binh := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	if r := e.startChallenge(t, binh, chlSurvival, "expert", ""); r.status != http.StatusForbidden || r.code() != string(problem.ChallengeNotEligible) {
		t.Fatalf("expert for grade 3: %d %s", r.status, r.raw)
	}
	party := e.do(t, http.MethodPost, "/game/challenges/"+chlSurvival+":start", e.gtoken(binh), merge(th(binh), key()),
		map[string]any{"difficulty": "easy", "party_id": uuid.NewString()})
	if party.status != http.StatusForbidden || party.code() != string(problem.ChallengeNotEligible) {
		t.Fatalf("party_id: %d %s", party.status, party.raw)
	}
	if r := e.startChallenge(t, binh, chlSurvival, "impossible", ""); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown level: %d", r.status)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.challenge_attempts WHERE student_id = $1`, binh.id); n != 0 {
		t.Fatalf("attempts written: %d", n)
	}
}

// F076-AC-06, AC-07 (TC-CHL-006, TC-CHL-008): the same Idempotency-Key answers the same attempt; no key is 428; another
// attempt in progress is 409 CONFLICT; challenge_attempts_per_day attempts used (abandoned ones count) is 409
// CHALLENGE_ATTEMPT_LIMIT with retry_after (the next reset, 04:00 ICT); a failure at hard waits challenge_fail_cooldown.
func TestChallengeStartLimits(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	k := newKey()
	first := e.startChallenge(t, an, chlTime3, "easy", k)
	again := e.startChallenge(t, an, chlTime3, "easy", k)
	if first.status != http.StatusCreated || again.status != http.StatusCreated || again.str("challenge_attempt_id") != first.str("challenge_attempt_id") {
		t.Fatalf("same key: %s / %s", first.raw, again.raw)
	}
	noKey := e.do(t, http.MethodPost, "/game/challenges/"+chlTime3+":start", e.gtoken(an), th(an), map[string]any{"difficulty": "easy"})
	if noKey.status != http.StatusPreconditionRequired {
		t.Fatalf("no key: %d %s", noKey.status, noKey.raw)
	}
	if r := e.startChallenge(t, an, chlSurvival, "easy", ""); r.status != http.StatusConflict || r.code() != string(problem.Conflict) {
		t.Fatalf("another in progress: %d %s", r.status, r.raw)
	}
	// Four more attempts of the day (abandoned count), then the sixth is refused until 04:00 ICT the next day.
	if r := e.abandonChallenge(t, an, first.str("challenge_attempt_id"), ""); r.status != http.StatusOK {
		t.Fatalf("abandon: %s", r.raw)
	}
	for i := 0; i < 4; i++ {
		r := e.startChallenge(t, an, chlTime3, "easy", "")
		if r.status != http.StatusCreated {
			t.Fatalf("attempt %d: %d %s", i+2, r.status, r.raw)
		}
		if a := e.abandonChallenge(t, an, r.str("challenge_attempt_id"), ""); a.status != http.StatusOK {
			t.Fatalf("abandon %d: %s", i+2, a.raw)
		}
	}
	limit := e.startChallenge(t, an, chlTime3, "easy", "")
	if limit.status != http.StatusConflict || limit.code() != string(problem.ChallengeAttemptLimit) {
		t.Fatalf("sixth attempt: %d %s", limit.status, limit.raw)
	}
	if limit.str("retry_after") != "2026-11-19T21:00:00Z" {
		t.Fatalf("retry_after: %s", limit.raw)
	}
	if d := e.do(t, http.MethodGet, "/game/challenges/"+chlTime3, e.gtoken(an), th(an), nil); d.body["attempts_left"] != float64(0) {
		t.Fatalf("attempts left: %s", d.raw)
	}
	// The next day the attempts come back.
	e.clock.Add(24 * time.Hour)
	next := e.startChallenge(t, an, chlTime3, "hard", "")
	if next.status != http.StatusCreated {
		t.Fatalf("next day: %d %s", next.status, next.raw)
	}
	// A failure at hard (deadline passed) waits challenge_fail_cooldown (60 s) before the next start.
	e.clock.Add(451*time.Second + 2*time.Second)
	if _, err := e.challengesUC().Expire(ctx, an.tenant, uuid.MustParse(next.str("challenge_attempt_id"))); err != nil {
		t.Fatal(err)
	}
	cool := e.startChallenge(t, an, chlTime3, "easy", "")
	if cool.status != http.StatusConflict || cool.code() != string(problem.ChallengeAttemptLimit) || cool.str("retry_after") == "" {
		t.Fatalf("cooldown: %d %s", cool.status, cool.raw)
	}
	e.clock.Add(61 * time.Second)
	if r := e.startChallenge(t, an, chlTime3, "easy", ""); r.status != http.StatusCreated {
		t.Fatalf("after the cooldown: %d %s", r.status, r.raw)
	}
}
