//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/challengerules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/challengeschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/challengesql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// Test challenges (F076): learning activities, subject and objective of the test content (module learning is the
// generated TEST DOUBLE of the facade — R17).
const (
	chlActivity1 = "0193a5f4-6c1e-7b2a-9d1f-0000000c7601"
	chlActivity2 = "0193a5f4-6c1e-7b2a-9d1f-0000000c7602"
	chlSubject   = "0193a5f4-6c1e-7b2a-9d1f-0000000720f1"
	chlObjective = "0193a5f4-6c1e-7b2a-9d1f-0000000a0001"
	chlTime3     = "0193a5f4-6c1e-7b2a-9d1f-000000076101" // time, grade 3, 300 s, goal 5 correct answers
	chlSurvival  = "0193a5f4-6c1e-7b2a-9d1f-000000076102" // survival, grades 3–7, goal 10 rounds
	chlWeekly7   = "0193a5f4-6c1e-7b2a-9d1f-000000076103" // weekly, grade 7, no time limit
)

// challengeCfg is the configuration of the tests (CONFIG_DEFAULTS values).
func challengeCfg() command.ChallengeConfig {
	return command.ChallengeConfig{Reset: quest.Reset{Hour: 4, Location: ict}, LearningRetry: 5 * time.Second, JobBatch: 100,
		Rules: challenge.Rules{DefaultLives: map[string]int{"easy": 5, "normal": 4, "hard": 3, "expert": 3, "legendary": 2},
			DefaultSeconds: map[string]int{"easy": 40, "normal": 30, "hard": 25, "expert": 20, "legendary": 20}, AttemptsPerDay: 5,
			FailCooldown: time.Minute, CooldownFrom: "hard", G13TimeFactor: 1.5, G13MaxLevel: "hard", ResumeWindow: 24 * time.Hour,
			DeadlineGrace: 2 * time.Second, RegenStreak: 3}}
}

// challengeRules are the publish checks of the challenges.
func challengeRules() *command.ChallengePublishRules {
	return &command.ChallengePublishRules{Schema: challengeschema.Validator{}, Rules: challengeCfg().Rules, Content: challengesql.Content{}}
}

// challengePush records challenge.attempt.updated pushes (TEST DOUBLE of port.ChallengeRealtime, R17).
type challengePush struct {
	mu   sync.Mutex
	msgs []map[string]any
}

func (p *challengePush) ChallengeAttemptUpdated(_ context.Context, student uuid.UUID, payload map[string]any, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := map[string]any{"student": student.String()}
	for k, v := range payload {
		m[k] = v
	}
	p.msgs = append(p.msgs, m)
	return nil
}

func (p *challengePush) all() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]map[string]any(nil), p.msgs...)
}

// challengesUC returns the challenge use cases on the test databases with the doubles of env.
func (e *env) challengesUC() command.Challenges {
	return command.Challenges{UoW: e.uow, AfterCommit: entx.AfterCommit, Content: challengesql.Content{}, World: worldsql.Content{},
		Quests: questsql.Content{}, Store: challengesql.Store{}, Events: writerepo.Publisher{W: e.writer},
		Learning: modules.LearningAttempts{F: e.learning.f}, Realtime: e.challengePush, Room: e.challengeRoom,
		Metrics: observability.ChallengeMetrics{}, Config: challengeCfg(), Now: e.clock.Now}
}

func (e *env) challengeQueries() query.Challenges {
	codec, _ := page.NewCodec([]byte(strings.Repeat("c", 32)))
	c := challengeCfg()
	return query.Challenges{Read: e.read, Content: challengesql.Content{}, World: worldsql.Content{}, Quests: questsql.Content{}, Cursors: codec,
		Rules: c.Rules, Reset: c.Reset, Now: e.clock.Now}
}

type challengeProxy struct{ e *env }

func (p challengeProxy) Start(ctx context.Context, id uuid.UUID, d string, party *uuid.UUID) (challengerules.AttemptView, error) {
	return p.e.challengesUC().Start(ctx, id, d, party)
}
func (p challengeProxy) Next(ctx context.Context, id uuid.UUID) (command.NextView, error) {
	return p.e.challengesUC().Next(ctx, id)
}
func (p challengeProxy) Abandon(ctx context.Context, id uuid.UUID) (challengerules.AttemptView, error) {
	return p.e.challengesUC().Abandon(ctx, id)
}

type challengeQueryProxy struct{ e *env }

func (p challengeQueryProxy) ListChallenges(ctx context.Context, in query.ChallengeListInput) (query.ChallengePage, error) {
	return p.e.challengeQueries().ListChallenges(ctx, in)
}
func (p challengeQueryProxy) GetChallenge(ctx context.Context, id uuid.UUID) (query.ChallengeDetail, error) {
	return p.e.challengeQueries().GetChallenge(ctx, id)
}
func (p challengeQueryProxy) GetChallengeAttempt(ctx context.Context, id uuid.UUID) (challengerules.AttemptView, error) {
	return p.e.challengeQueries().GetChallengeAttempt(ctx, id)
}

// profiles of the five levels of CHALLENGE_BOSS_SYSTEM §2.2 (expert from grade 4, legendary grades 6–7).
func fiveLevels(grades []int) []any {
	var expert, legend []int
	for _, g := range grades {
		if g >= 4 {
			expert = append(expert, g)
		}
		if g >= 6 {
			legend = append(legend, g)
		}
	}
	out := []any{
		map[string]any{"level": "easy", "question_difficulty_min": 1, "question_difficulty_max": 2, "reward_multiplier": 1},
		map[string]any{"level": "normal", "question_difficulty_min": 2, "question_difficulty_max": 3, "reward_multiplier": 1.3},
		map[string]any{"level": "hard", "question_difficulty_min": 3, "question_difficulty_max": 4, "reward_multiplier": 1.6},
	}
	if len(expert) > 0 {
		out = append(out, map[string]any{"level": "expert", "question_difficulty_min": 4, "question_difficulty_max": 5, "reward_multiplier": 2,
			"grade_levels": expert})
	}
	if len(legend) > 0 {
		out = append(out, map[string]any{"level": "legendary", "question_difficulty_min": 5, "question_difficulty_max": 5, "reward_multiplier": 2.5,
			"grade_levels": legend})
	}
	return out
}

// challengeItem is an authored challenge (challenge.schema.json) of the test content, modified by edit.
func challengeItem(id, code, typ string, grades []int, edit func(m map[string]any)) command.ContentItem {
	m := map[string]any{"schema_version": 1, "challenge_id": id, "code": code, "owner_scope": "platform", "tenant_id": nil, "challenge_type": typ,
		"name": map[string]any{"vi": "Thử thách " + typ, "en": "Challenge " + typ}, "grade_levels": grades, "subject_id": chlSubject,
		"objective_ids": []string{chlObjective}, "mode": "solo", "difficulties": fiveLevels(grades),
		"activity_source": map[string]any{"kind": "activity_ref_list", "activity_refs": []any{
			map[string]any{"activity_type": "quiz", "activity_id": chlActivity1}, map[string]any{"activity_type": "quiz", "activity_id": chlActivity2}}},
		"goal": map[string]any{"metric": "correct_answers", "target": 5}, "time_limit_seconds": 300, "rewards": map[string]any{}}
	if edit != nil {
		edit(m)
	}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemChallenge, Payload: b}
}

// challengeSet is the test content: a time challenge of grade 3, a survival challenge of grades 3–7 (hard: 3 lives),
// a weekly challenge of grade 7 without time limit.
func challengeSet() []command.ContentItem {
	return []command.ContentItem{
		challengeItem(chlTime3, "chl.math.g3_time", "time", []int{3}, nil),
		challengeItem(chlSurvival, "chl.math.survival", "survival", []int{3, 4, 5, 6, 7}, func(m map[string]any) {
			m["goal"], m["time_limit_seconds"] = map[string]any{"metric": "rounds_survived", "target": 10}, 900
		}),
		challengeItem(chlWeekly7, "chl.math.g7_weekly", "weekly", []int{7}, func(m map[string]any) { delete(m, "time_limit_seconds") }),
	}
}

// publishChallenges publishes the world seed and items, then assigns every challenge of items to tenants.
func (e *env) publishChallenges(ctx context.Context, t *testing.T, items []command.ContentItem, tenants ...uuid.UUID) {
	t.Helper()
	if _, err := e.publish(ctx, t, items, "challenges"); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		var m struct {
			ID uuid.UUID `json:"challenge_id"`
		}
		_ = json.Unmarshal(it.Payload, &m)
		for _, tenant := range tenants {
			if _, err := e.migrator.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments (assignment_id, tenant_id, created_at,
					updated_at, resource_type, resource_id, assigned_by, assigned_at)
				VALUES ($1, $2, now(), now(), 'challenge', $3, $4, now()) ON CONFLICT DO NOTHING`, uuid.New(), tenant, m.ID, uuid.Nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// setupChallenges is the environment with the world seed and the challenge set assigned to tenant A only.
func setupChallenges(ctx context.Context, t *testing.T) *env {
	t.Helper()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	e.publishChallenges(ctx, t, challengeSet(), e.tenantA)
	return e
}

// studentOfGrade projects an active student of grade into ref.students and registers its principal.
func (e *env) studentOfGrade(ctx context.Context, t *testing.T, tenant, school uuid.UUID, grade int) student {
	t.Helper()
	s := e.newStudent(ctx, t, tenant, school, "active")
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.students SET grade_level = $2 WHERE student_id = $1`, s.id, grade); err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *env) startChallenge(t *testing.T, s student, id, difficulty, k string) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/challenges/"+id+":start", e.gtoken(s), merge(th(s), map[string]string{"Idempotency-Key": k}),
		map[string]any{"difficulty": difficulty})
}

func (e *env) nextActivity(t *testing.T, s student, attempt, k string) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/challenge-attempts/"+attempt+"/activities:next", e.gtoken(s),
		merge(th(s), map[string]string{"Idempotency-Key": k}), nil)
}

func (e *env) abandonChallenge(t *testing.T, s student, attempt, k string) resp {
	t.Helper()
	if k == "" {
		k = newKey()
	}
	return e.do(t, http.MethodPost, "/game/challenge-attempts/"+attempt+":abandon", e.gtoken(s),
		merge(th(s), map[string]string{"Idempotency-Key": k}), nil)
}

func (e *env) challengeAttempt(t *testing.T, s student, attempt string) resp {
	t.Helper()
	return e.do(t, http.MethodGet, "/game/challenge-attempts/"+attempt, e.gtoken(s), th(s), nil)
}

// learningAttemptOf returns the learning attempt id of a nextChallengeActivity answer.
func learningAttemptOf(t *testing.T, r resp) string {
	t.Helper()
	a, _ := r.body["attempt"].(map[string]any)
	id, _ := a["attempt_id"].(string)
	if id == "" {
		t.Fatalf("no learning attempt: %s", r.raw)
	}
	return id
}

// challengeGraded delivers activity.graded (mode challenge) to the consumer game.challenge_progress.
func (e *env) challengeGraded(ctx context.Context, t *testing.T, s student, attempt string, correct, items int, at time.Time, eventID uuid.UUID) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"attempt_id": attempt, "student_id": s.id, "mode": "challenge", "correct_count": correct,
		"item_count": items, "passed": correct == items})
	tt := s.tenant
	env := outbox.Envelope{EventID: eventID, EventType: "activity.graded", SchemaVersion: 1, OccurredAt: at, Producer: "learning", TenantID: &tt,
		Payload: b}
	c := consumer.ChallengeProgress{Challenges: e.challengesUC()}
	if err := e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatal(err)
	}
}

// completedEvents returns the challenge.completed payloads of an attempt in the outbox.
func (e *env) completedEvents(ctx context.Context, t *testing.T, attempt string) []map[string]any {
	t.Helper()
	rows, err := e.relay.QueryContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'challenge.completed'
		AND envelope->'payload'->>'challenge_attempt_id' = $1`, attempt)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		out = append(out, m)
	}
	return out
}

// str returns a string field of the body.
func (r resp) str(k string) string {
	s, _ := r.body[k].(string)
	return s
}

// items returns an array of objects of the body.
func (r resp) items(k string) []map[string]any {
	arr, _ := r.body[k].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, it := range arr {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
