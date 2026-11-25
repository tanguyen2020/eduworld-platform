package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LearningBossFacade is createQuestionPlan, scorePlanAnswer and getLearningMaterialRenders of the learning facade
// (x-allowed-callers: game since F103).
type LearningBossFacade interface {
	CreateQuestionPlan(ctx context.Context, req learningapi.CreateQuestionPlanRequestObject) (learningapi.CreateQuestionPlanResponseObject, error)
	ScorePlanAnswer(ctx context.Context, req learningapi.ScorePlanAnswerRequestObject) (learningapi.ScorePlanAnswerResponseObject, error)
	GetLearningMaterialRenders(ctx context.Context, req learningapi.GetLearningMaterialRendersRequestObject) (learningapi.GetLearningMaterialRendersResponseObject, error)
}

// LearningBoss implements port.BossLearning and the learning side of port.ClueRenders on the learning facade (F103
// research R1, R3) through pkg/modcall: span, deadline, TenantScope and Principal of the member, Idempotency-Key = the
// plan key (createQuestionPlan) or "<plan>:<index>:<student>" (scorePlanAnswer). A deadline, a 5xx or no facade is
// port.ErrBossLearningUnavailable; a 4xx is returned as is.
type LearningBoss struct{ F LearningBossFacade }

var _ port.BossLearning = LearningBoss{}

func bossOp(name string, kind modcall.Kind) modcall.Op {
	return modcall.Op{Caller: "game", Callee: "learning", Operation: name, Kind: kind, AllowedCallers: learningapi.AllowedCallers[name],
		ContextRequired: learningapi.ContextRequired[name]}
}

func unavailable(err error) error {
	var pe *problem.Error
	if errors.As(err, &pe) && problem.Status(pe.Code) < 500 {
		return err
	}
	return errors.Join(port.ErrBossLearningUnavailable, err)
}

// CreateBossPlan implements port.BossLearning: mode boss, context boss_encounter of the encounter, attempt mode challenge,
// the activity of the requirement for the grade of the member, one participant.
func (l LearningBoss) CreateBossPlan(ctx context.Context, in port.BossQuestionPlan) (port.BossPlan, error) {
	var out port.BossPlan
	if l.F == nil {
		return out, port.ErrBossLearningUnavailable
	}
	ctxType, mode := learningapi.QuestionPlanRequestContextTypeBossEncounter, learningapi.QuestionPlanRequestAttemptModeChallenge
	encounter := in.EncounterID
	tps := in.TimePerQuestion
	if tps < 5 {
		tps = 20
	}
	body := learningapi.CreateQuestionPlanJSONRequestBody{MatchId: in.PlanKey, Mode: learningapi.Boss,
		GradeLevel: in.GradeLevel, QuestionCount: max(1, min(in.QuestionCount, 30)), Participants: []uuid.UUID{in.StudentID},
		ContextType: &ctxType, ContextId: &encounter, AttemptMode: &mode, TimePerQuestionSeconds: &tps}
	body.ActivityRef = &struct {
		ContentId uuid.UUID `json:"content_id"`
		Version   *int      `json:"version,omitempty"`
	}{ContentId: in.ActivityID, Version: in.ActivityVersion}
	ctx = modcall.WithIdempotencyKey(ctx, in.PlanKey.String())
	resp, err := modcall.Call(ctx, bossOp("createQuestionPlan", modcall.Write),
		func(ctx context.Context) (learningapi.CreateQuestionPlanResponseObject, error) {
			return l.F.CreateQuestionPlan(ctx, learningapi.CreateQuestionPlanRequestObject{Body: &body})
		})
	if err != nil {
		return out, unavailable(err)
	}
	v, ok := resp.(learningapi.CreateQuestionPlan201JSONResponse)
	if !ok {
		return out, fmt.Errorf("%w: response %T", port.ErrBossLearningUnavailable, resp)
	}
	out.PlanID, out.ExpiresAt = v.PlanId, v.ExpiresAt.UTC()
	for _, it := range v.Items {
		out.Items = append(out.Items, port.BossPlanItem{Index: it.Index, QuestionVersionID: it.QuestionVersionId, QuestionType: string(it.QuestionType),
			TimeLimitSeconds: it.TimeLimitSeconds, Render: it.RenderPayload})
	}
	return out, nil
}

// ScoreBossAnswer implements port.BossLearning: the raw response with the server time; module learning grades it with
// the answer key and records it for the student (ADR-0016).
func (l LearningBoss) ScoreBossAnswer(ctx context.Context, planID uuid.UUID, index int, student uuid.UUID, response []byte,
	receivedAt time.Time) (port.BossScore, error) {
	var out port.BossScore
	if l.F == nil {
		return out, port.ErrBossLearningUnavailable
	}
	var r learningapi.QuestionResponse
	if err := r.UnmarshalJSON(response); err != nil || len(response) == 0 {
		return out, problem.Invalid("response", problem.SubInvalid, "not an answer")
	}
	body := learningapi.ScorePlanAnswerJSONRequestBody{Index: index, StudentId: student, Response: r, ReceivedAt: receivedAt.UTC()}
	ctx = modcall.WithIdempotencyKey(ctx, fmt.Sprintf("%s:%d:%s", planID, index, student))
	resp, err := modcall.Call(ctx, bossOp("scorePlanAnswer", modcall.Write),
		func(ctx context.Context) (learningapi.ScorePlanAnswerResponseObject, error) {
			return l.F.ScorePlanAnswer(ctx, learningapi.ScorePlanAnswerRequestObject{PlanId: planID, Body: &body})
		})
	if err != nil {
		return out, unavailable(err)
	}
	v, ok := resp.(learningapi.ScorePlanAnswer200JSONResponse)
	if !ok {
		return out, fmt.Errorf("%w: response %T", port.ErrBossLearningUnavailable, resp)
	}
	out.IsCorrect = v.IsCorrect
	if v.Late != nil {
		out.Late = *v.Late
	}
	if v.RapidGuess != nil {
		out.RapidGuess = *v.RapidGuess
	}
	return out, nil
}

// MaterialRenders returns the renders of learning materials by id (getLearningMaterialRenders, at most 4 per call).
func (l LearningBoss) MaterialRenders(ctx context.Context, refs []quest.ClueRef) (map[string]port.ClueRender, error) {
	out := map[string]port.ClueRender{}
	if l.F == nil {
		return nil, port.ErrBossLearningUnavailable
	}
	if len(refs) == 0 {
		return out, nil
	}
	body := learningapi.GetLearningMaterialRendersJSONRequestBody{}
	for _, r := range refs {
		id, err := uuid.Parse(r.ID)
		if err != nil {
			return nil, problem.New(problem.ItemRefInvalid, "clue_content_ref")
		}
		body.Refs = append(body.Refs, struct {
			Id      uuid.UUID `json:"id"`
			Version *int      `json:"version,omitempty"`
		}{Id: id, Version: r.Version})
	}
	resp, err := modcall.Call(ctx, bossOp("getLearningMaterialRenders", modcall.Read),
		func(ctx context.Context) (learningapi.GetLearningMaterialRendersResponseObject, error) {
			return l.F.GetLearningMaterialRenders(ctx, learningapi.GetLearningMaterialRendersRequestObject{Body: &body})
		})
	if err != nil {
		return nil, unavailable(err)
	}
	v, ok := resp.(learningapi.GetLearningMaterialRenders200JSONResponse)
	if !ok {
		return nil, fmt.Errorf("%w: response %T", port.ErrBossLearningUnavailable, resp)
	}
	raw, err := json.Marshal(v.Data)
	if err != nil {
		return nil, err
	}
	var data []struct {
		ID     uuid.UUID         `json:"id"`
		Title  map[string]string `json:"title"`
		Blocks []map[string]any  `json:"blocks"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	for _, d := range data {
		out[d.ID.String()] = port.ClueRender{Title: d.Title, Blocks: d.Blocks}
	}
	return out, nil
}

// LateBossLearning is LearningBoss while module learning is wired after Register (Module.Wire): until then every call
// is port.ErrBossLearningUnavailable.
type LateBossLearning struct{ L *LateLearning }

var _ port.BossLearning = LateBossLearning{}

func (l LateBossLearning) get() LearningBoss {
	if l.L == nil {
		return LearningBoss{}
	}
	l.L.mu.RLock()
	defer l.L.mu.RUnlock()
	f, _ := l.L.f.(LearningBossFacade)
	return LearningBoss{F: f}
}

// CreateBossPlan implements port.BossLearning.
func (l LateBossLearning) CreateBossPlan(ctx context.Context, in port.BossQuestionPlan) (port.BossPlan, error) {
	return l.get().CreateBossPlan(ctx, in)
}

// ScoreBossAnswer implements port.BossLearning.
func (l LateBossLearning) ScoreBossAnswer(ctx context.Context, planID uuid.UUID, index int, student uuid.UUID, response []byte,
	receivedAt time.Time) (port.BossScore, error) {
	return l.get().ScoreBossAnswer(ctx, planID, index, student, response, receivedAt)
}

// MaterialRenders returns the renders of learning materials (see LearningBoss.MaterialRenders).
func (l LateBossLearning) MaterialRenders(ctx context.Context, refs []quest.ClueRef) (map[string]port.ClueRender, error) {
	return l.get().MaterialRenders(ctx, refs)
}
