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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/room"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// downRegistry is a TEST DOUBLE of the room registry with Valkey down (R17): every call fails.
type downRegistry struct{}

var errValkeyDown = errors.New("valkey down")

func (downRegistry) Announce(context.Context, roomhost.Room) error { return errValkeyDown }
func (downRegistry) Acquire(context.Context, string, string, time.Duration) (bool, error) {
	return false, errValkeyDown
}
func (downRegistry) Renew(context.Context, string, string, time.Duration, time.Time) (bool, error) {
	return false, errValkeyDown
}
func (downRegistry) Release(context.Context, string, string) error        { return errValkeyDown }
func (downRegistry) Room(context.Context, string) (*roomhost.Room, error) { return nil, errValkeyDown }
func (downRegistry) Forget(context.Context, string) error                 { return errValkeyDown }
func (downRegistry) Orphans(context.Context, time.Time, int) ([]string, error) {
	return nil, errValkeyDown
}

func (e *env) attemptStatus(ctx context.Context, t *testing.T, attempt string) (string, string) {
	t.Helper()
	var status string
	var reason *string
	if err := e.platform.QueryRowContext(ctx, `SELECT status, end_reason FROM game.challenge_attempts WHERE challenge_attempt_id = $1`,
		attempt).Scan(&status, &reason); err != nil {
		t.Fatal(err)
	}
	if reason == nil {
		return status, ""
	}
	return status, *reason
}

// F076-AC-13, AC-19 (TC-CHL-001): the attempt ends on the server clock whatever the client clock: nextChallengeActivity
// after deadline_at is 422; the job challenge-deadline ends it from PostgreSQL (Valkey down: the room cannot even be
// announced, start still works) once deadline_grace passed, exactly once; a result graded after the end changes nothing.
func TestChallengeDeadlineValkeyDown(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	e.challengeRoom = room.Challenges{Registry: downRegistry{}, Type: "challenge_solo", Expirer: e.challengesUC()}
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	st := e.startChallenge(t, an, chlTime3, "hard", "")
	if st.status != http.StatusCreated {
		t.Fatalf("start with Valkey down: %d %s", st.status, st.raw)
	}
	attempt := st.str("challenge_attempt_id")
	la := learningAttemptOf(t, e.nextActivity(t, an, attempt, ""))
	e.clock.Add(451 * time.Second) // the client clock is never read: only the server clock moves
	if r := e.nextActivity(t, an, attempt, ""); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("next after the deadline: %d %s", r.status, r.raw)
	}
	if n, err := e.challengesUC().ExpireDue(ctx, 100); err != nil || n != 0 {
		t.Fatalf("within deadline_grace: %d %v", n, err)
	}
	e.clock.Add(2 * time.Second)
	if n, err := e.challengesUC().ExpireDue(ctx, 100); err != nil || n != 1 {
		t.Fatalf("job: %d %v", n, err)
	}
	if n, err := e.challengesUC().ExpireDue(ctx, 100); err != nil || n != 0 {
		t.Fatalf("job again: %d %v", n, err)
	}
	if s, why := e.attemptStatus(ctx, t, attempt); s != "failed" || why != "deadline_passed" {
		t.Fatalf("expired: %s %s", s, why)
	}
	if ev := e.completedEvents(ctx, t, attempt); len(ev) != 1 || ev[0]["status"] != "failed" {
		t.Fatalf("one challenge.completed: %+v", ev)
	}
	// Edge case: the result of the open activity arrives after the end — the learning record stays, the challenge not.
	e.challengeGraded(ctx, t, an, la, 3, 3, e.clock.Now(), uuid.New())
	if r := e.challengeAttempt(t, an, attempt); r.str("status") != "failed" || r.body["score"] != float64(0) {
		t.Fatalf("late result: %s", r.raw)
	}
}

// F076-AC-14, AC-15 and the edge cases: abandon keeps the learning attempts and writes challenge.completed
// (abandoned); abandoning again is 422; another device (takeover) reads the checkpoint and continues; a challenge
// without a time limit is abandoned after attempt_resume_window; a student cleaned up loses the attempt in progress.
func TestChallengeAbandonResume(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	sv := e.startChallenge(t, an, chlSurvival, "easy", "").str("challenge_attempt_id")
	la := learningAttemptOf(t, e.nextActivity(t, an, sv, ""))
	ab := e.abandonChallenge(t, an, sv, "")
	if ab.status != http.StatusOK || ab.str("status") != "abandoned" || ab.str("ended_at") == "" {
		t.Fatalf("abandon: %d %s", ab.status, ab.raw)
	}
	if ids := e.learningIDs(ctx, t, sv); len(ids) != 1 || ids[0] != la {
		t.Fatalf("learning attempts kept: %v", ids)
	}
	if ev := e.completedEvents(ctx, t, sv); len(ev) != 1 || ev[0]["status"] != "abandoned" || ev[0]["end_reason"] != "player" {
		t.Fatalf("challenge.completed (abandoned): %+v", ev)
	}
	if r := e.abandonChallenge(t, an, sv, ""); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("abandon twice: %d %s", r.status, r.raw)
	}
	// AC-15: weekly without time limit; a second device reads the checkpoint and continues.
	dung := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA2, 7)
	w := e.startChallenge(t, dung, chlWeekly7, "easy", "").str("challenge_attempt_id")
	e.challengeGraded(ctx, t, dung, learningAttemptOf(t, e.nextActivity(t, dung, w, "")), 2, 3, e.clock.Now(), uuid.New())
	other := e.gameToken(dung, uuid.NewString()) // another device, another Keycloak session
	r := e.do(t, http.MethodGet, "/game/challenge-attempts/"+w, other, th(dung), nil)
	if r.status != http.StatusOK || r.str("status") != "in_progress" || r.body["score"] != float64(2) || r.body["activities_completed"] != float64(1) ||
		r.body["deadline_at"] != nil {
		t.Fatalf("checkpoint on another device: %s", r.raw)
	}
	e.clock.Add(23 * time.Hour)
	if n := e.do(t, http.MethodPost, "/game/challenge-attempts/"+w+"/activities:next", other, merge(th(dung), key()), nil); n.status != http.StatusOK {
		t.Fatalf("continue within the resume window: %d %s", n.status, n.raw)
	}
	e.clock.Add(time.Hour + 3*time.Second)
	if n, err := e.challengesUC().ExpireDue(ctx, 100); err != nil || n != 1 {
		t.Fatalf("resume window job: %d %v", n, err)
	}
	if s, why := e.attemptStatus(ctx, t, w); s != "abandoned" || why != "resume_expired" {
		t.Fatalf("resume window: %s %s", s, why)
	}
	// Edge case: student.cleaned_up ends the attempt in progress (student_unavailable).
	binh := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	b := e.startChallenge(t, binh, chlTime3, "easy", "").str("challenge_attempt_id")
	payload, _ := json.Marshal(map[string]any{"student_id": binh.id, "tenant_id": binh.tenant})
	tt := binh.tenant
	env := outbox.Envelope{EventID: uuid.New(), EventType: "student.cleaned_up", SchemaVersion: 1, OccurredAt: e.clock.Now(), Producer: "identity",
		TenantID: &tt, Payload: payload}
	c := consumer.SessionEndOnRevoke{Sessions: e.sessions, Challenges: e.challengesUC()}
	if err := e.uow.InTenantTx(ctx, scopeOf(t, binh.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatal(err)
	}
	if s, why := e.attemptStatus(ctx, t, b); s != "abandoned" || why != "student_unavailable" {
		t.Fatalf("cleaned up: %s %s", s, why)
	}
}
