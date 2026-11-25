package contract

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/modules"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// internalSchema returns a component schema of specs/openapi/modules/learning.internal.v1.yaml (the callee contract).
func internalSchema(t *testing.T, name string) *openapi3.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..")
	l := openapi3.NewLoader()
	l.IsExternalRefsAllowed = true
	doc, err := l.LoadFromFile(filepath.Join(root, "specs", "openapi", "modules", "learning.internal.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := doc.Components.Schemas[name]
	if s == nil || s.Value == nil {
		t.Fatalf("no schema %s", name)
	}
	return s.Value
}

// studentCtx is a player request of a student with its TenantScope and the pending Idempotency-Key of the httpx chain.
func studentCtx(t *testing.T, student, tenant uuid.UUID, key uuid.UUID) context.Context {
	t.Helper()
	tt := tenant
	p := authz.NewPrincipal(authz.PrincipalData{UserID: student, UserType: authz.UserTypeStudent, TenantID: &tt, TenantType: authz.TenantTypeSchool})
	sc, err := authz.ScopeFor(p)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(authz.WithPrincipal(context.Background(), p), sc)
	pending := idempotency.NewStore(nil, 0, nil).NewPending(idempotency.Key{UserID: student, Method: "POST", Route: "/game/quests/{quest_id}/steps/{step_id}/activities:start",
		Key: key, IssuedOn: time.Now().UTC()}, [32]byte{}, 201)
	return idempotency.WithPending(ctx, pending)
}

// T027 (F072-AC-14, AC-18, facade test of the caller, AGENTS R29): the adapter of module game calls createAttempt of the
// generated learning Fake with a request valid against AttemptCreateRequest of the internal spec (mode quest, activity
// pinned in the quest version, context quest_step), forwards the Idempotency-Key of the player request, maps the
// answer AttemptCreated, maps unknown/unpublished content to ITEM_REF_INVALID and errors or deadlines of module learning
// to port.ErrLearningUnavailable (503 + Retry-After upstream).
func TestLearningCreateAttemptFacade(t *testing.T) {
	student, tenant, progress, version := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	content, pinned, session := uuid.New(), uuid.New(), uuid.New()
	key := uuid.Must(uuid.NewV7())
	reqSchema := internalSchema(t, "AttemptCreateRequest")
	respSchema := internalSchema(t, "AttemptCreated")
	var got learningapi.AttemptCreateRequest
	var gotKey, gotCaller string
	attempt := uuid.New()
	fake := &learningapi.Fake{CreateAttemptFunc: func(ctx context.Context, req learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		got = *req.Body
		gotKey, _ = modcall.IdempotencyKeyFrom(ctx)
		gotCaller = modcall.CallerFrom(ctx)
		if p, ok := authz.PrincipalFrom(ctx); !ok || p.UserID != student {
			t.Error("the principal of the student is not passed on")
		}
		return learningapi.CreateAttempt201JSONResponse{AttemptId: attempt, Status: "in_progress", QuestionCount: 3, StartedAt: time.Now().UTC(),
			ContentVersionId: &pinned}, nil
	}}
	a := modules.LearningAttempts{F: fake}
	out, err := a.CreateAttempt(studentCtx(t, student, tenant, key), port.LearningAttempt{StudentID: student, ContentID: content, ContentVersionID: &pinned,
		QuestProgressID: progress, QuestVersionID: version, StepID: "solve", GameSessionID: &session})
	if err != nil || out.AttemptID != attempt || out.ContentVersionID == nil || *out.ContentVersionID != pinned || out.QuestionCount != 3 {
		t.Fatalf("create: %+v %v", out, err)
	}
	if gotKey != key.String() || gotCaller != "game" {
		t.Fatalf("Idempotency-Key %q, caller %q", gotKey, gotCaller)
	}
	b, _ := json.Marshal(got)
	var doc any
	_ = json.Unmarshal(b, &doc)
	if err := reqSchema.VisitJSON(doc); err != nil {
		t.Fatalf("AttemptCreateRequest of the internal spec: %v\n%s", err, b)
	}
	if got.Mode != learningapi.AttemptModeQuest || got.Activity.ContentType != learningapi.ActivityRefContentTypeActivity || got.Activity.ContentId != content ||
		got.Context.ContextType != learningapi.AttemptCreateRequestContextContextTypeQuestStep || got.Context.ContextId != progress || *got.Context.StepId != "solve" ||
		*got.Context.QuestVersionId != version || *got.Context.GameSessionId != session {
		t.Fatalf("request: %s", b)
	}
	rb, _ := json.Marshal(learningapi.CreateAttempt201JSONResponse{AttemptId: attempt, Status: "in_progress", QuestionCount: 3, StartedAt: time.Now().UTC()})
	var rdoc any
	_ = json.Unmarshal(rb, &rdoc)
	if err := respSchema.VisitJSON(rdoc); err != nil {
		t.Fatalf("AttemptCreated of the internal spec: %v", err)
	}
	for name, c := range map[string]struct {
		fail    error
		delay   time.Duration
		unavail bool
		code    problem.Code
	}{
		"not found":     {fail: problem.New(problem.NotFound, ""), code: problem.ItemRefInvalid},
		"not published": {fail: problem.New(problem.ContentNotPublished, ""), code: problem.ItemRefInvalid},
		"invalid":       {fail: problem.Invalid("/activity/content_type", problem.SubInvalid, "x"), code: problem.ValidationFailed},
		"internal":      {fail: errors.New("boom"), unavail: true},
		"deadline":      {delay: time.Second, unavail: true},
	} {
		fake.CreateAttemptFunc = func(ctx context.Context, _ learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
			if c.delay > 0 {
				select {
				case <-time.After(c.delay):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, c.fail
		}
		ctx, cancel := context.WithTimeout(studentCtx(t, student, tenant, key), 100*time.Millisecond)
		_, err := a.CreateAttempt(ctx, port.LearningAttempt{StudentID: student, ContentID: content, QuestProgressID: progress, QuestVersionID: version, StepID: "solve"})
		cancel()
		var pe *problem.Error
		switch {
		case c.unavail && !errors.Is(err, port.ErrLearningUnavailable):
			t.Errorf("%s: %v, want unavailable", name, err)
		case !c.unavail && (!errors.As(err, &pe) || pe.Code != c.code):
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
	if _, err := (modules.LearningAttempts{}).CreateAttempt(context.Background(), port.LearningAttempt{}); !errors.Is(err, port.ErrLearningUnavailable) {
		t.Fatalf("not wired: %v", err)
	}
}

// F076 T005 (F076-AC-08, AC-12, facade test of the caller, AGENTS R29): the challenge adapter calls createAttempt with
// a request valid against AttemptCreateRequest of the internal spec — mode challenge, context challenge_attempt, the
// difficulty band, the grade of the student, the objectives, one item and its time for a survival round — forwards
// the Idempotency-Key, maps an unpublished activity to ITEM_REF_INVALID and a failure of module learning to
// port.ErrLearningUnavailable.
func TestLearningCreateChallengeAttemptFacade(t *testing.T) {
	student, tenant, challengeAttempt, activity, objective := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	key := uuid.Must(uuid.NewV7())
	reqSchema := internalSchema(t, "AttemptCreateRequest")
	var got learningapi.AttemptCreateRequest
	var gotKey string
	attempt := uuid.New()
	fake := &learningapi.Fake{CreateAttemptFunc: func(ctx context.Context, req learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		got = *req.Body
		gotKey, _ = modcall.IdempotencyKeyFrom(ctx)
		return learningapi.CreateAttempt201JSONResponse{AttemptId: attempt, Status: "in_progress", QuestionCount: 1, StartedAt: time.Now().UTC()}, nil
	}}
	a := modules.LearningAttempts{F: fake}
	one, secs := 1, 37
	out, err := a.CreateChallengeAttempt(studentCtx(t, student, tenant, key), port.ChallengeLearningAttempt{StudentID: student, ActivityID: activity,
		ChallengeAttemptID: challengeAttempt, DifficultyMin: 3, DifficultyMax: 4, GradeLevel: 3, ObjectiveIDs: []uuid.UUID{objective},
		QuestionCount: &one, TimeLimitSeconds: &secs})
	if err != nil || out.AttemptID != attempt || gotKey != key.String() {
		t.Fatalf("create: %+v %v key %q", out, err, gotKey)
	}
	b, _ := json.Marshal(got)
	var doc any
	_ = json.Unmarshal(b, &doc)
	if err := reqSchema.VisitJSON(doc); err != nil {
		t.Fatalf("AttemptCreateRequest of the internal spec: %v\n%s", err, b)
	}
	if got.Mode != learningapi.AttemptModeChallenge || got.Context.ContextType != learningapi.AttemptCreateRequestContextContextTypeChallengeAttempt || got.Context.ContextId != challengeAttempt ||
		*got.Activity.DifficultyMin != 3 || *got.Activity.DifficultyMax != 4 || *got.Activity.GradeLevel != 3 || *got.Activity.QuestionCount != 1 ||
		*got.Activity.TimeLimitSeconds != 37 || (*got.Activity.ObjectiveIds)[0] != objective || got.Activity.ContentId != activity {
		t.Fatalf("request: %s", b)
	}
	fake.CreateAttemptFunc = func(context.Context, learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		return nil, problem.New(problem.ContentNotPublished, "")
	}
	if _, err := a.CreateChallengeAttempt(studentCtx(t, student, tenant, key), port.ChallengeLearningAttempt{StudentID: student, ActivityID: activity,
		ChallengeAttemptID: challengeAttempt, DifficultyMin: 1, DifficultyMax: 2, GradeLevel: 3}); !errors.Is(err, problem.New(problem.ItemRefInvalid, "")) {
		var pe *problem.Error
		if !errors.As(err, &pe) || pe.Code != problem.ItemRefInvalid {
			t.Fatalf("unpublished: %v", err)
		}
	}
	fake.CreateAttemptFunc = func(context.Context, learningapi.CreateAttemptRequestObject) (learningapi.CreateAttemptResponseObject, error) {
		return nil, errors.New("boom")
	}
	if _, err := a.CreateChallengeAttempt(studentCtx(t, student, tenant, key), port.ChallengeLearningAttempt{StudentID: student, ActivityID: activity,
		ChallengeAttemptID: challengeAttempt, DifficultyMin: 1, DifficultyMax: 2, GradeLevel: 3}); !errors.Is(err, port.ErrLearningUnavailable) {
		t.Fatalf("learning down: %v", err)
	}
}
