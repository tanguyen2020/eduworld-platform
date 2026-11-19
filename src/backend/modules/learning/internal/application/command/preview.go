package command

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mathx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/preview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Preview serves the preview sessions of learning content (F027 US4, FR-CNT-010, FR-CMS-006, ADMIN_PORTAL §5): any
// version visible in the scope (drafts included) rendered without answers, and a "try it" graded on the server by
// the question bank. Nothing is written to PostgreSQL and no event is published: no attempt, learning event, XP,
// game session or outbox (F027-AC-17, ADR-0031); the session lives in Valkey for preview_token_ttl.
type Preview struct {
	// UoW only stores the idempotency record of createLearningPreviewSession (replay of the same Idempotency-Key);
	// no learning table is written.
	UoW       write.UnitOfWork
	Session   read.Session
	Questions read.QuestionReadModel
	Registry  *question.Registry
	Schema    port.QuestionSchema
	Store     port.PreviewStore
	TTL       time.Duration // preview_token_ttl (0 = PreviewTokenTTL)
	Clock     func() time.Time
}

// PreviewInput is LearningPreviewRequest.
type PreviewInput struct {
	ContentType     string
	ContentID       uuid.UUID
	Version         int
	DisplayLanguage string
}

// PreviewItem is one QuestionRender of a preview.
type PreviewItem struct {
	Position          int            `json:"position"`
	QuestionVersionID uuid.UUID      `json:"question_version_id"`
	Type              question.Type  `json:"type"`
	Stem              any            `json:"stem"`
	RenderPayload     map[string]any `json:"render_payload"`
	MaxScore          float64        `json:"max_score"`
}

// PreviewView is LearningPreviewSession.
type PreviewView struct {
	ID        uuid.UUID     `json:"preview_session_id"`
	Items     []PreviewItem `json:"items"`
	ExpiresAt time.Time     `json:"expires_at"`
}

func (h Preview) ttl() time.Duration {
	if h.TTL > 0 {
		return h.TTL
	}
	return PreviewTokenTTL
}

func previewRead(ctx context.Context) (authz.Principal, error) {
	return authorize(ctx, authz.PermLearningContentRead)
}

// CreateSession opens a preview of one version (F027-AC-16, AC-19): another tenant's content is 404, a missing
// learning_content.read 403. Question previews are supported; other content types come with their features.
func (h Preview) CreateSession(ctx context.Context, in PreviewInput) (out PreviewView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateLearningPreviewSession")
	defer func() { done(err) }()
	p, err := previewRead(ctx)
	if err != nil {
		return out, err
	}
	if in.ContentType != string(contentmodel.TypeQuestion) {
		return out, problem.Invalid("content_type", problem.SubInvalid, "preview of "+in.ContentType+" is not available yet")
	}
	var v dto.QuestionVersion
	if err := h.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		v, err = h.Questions.Version(ctx, in.ContentID, in.Version)
		return err
	}); err != nil {
		return out, err
	}
	id := ids.New()
	seed := binary.BigEndian.Uint64(id[8:])
	lang := preview.Language(in.DisplayLanguage, v.ContentLanguage)
	item, err := h.render(v, 1, seed, lang)
	if err != nil {
		return out, err
	}
	s := preview.Session{ID: id, TenantID: ptrClone(p.TenantID), PrincipalID: p.UserID, ContentType: in.ContentType, ContentID: in.ContentID,
		Version: in.Version, DisplayLanguage: lang, Items: []preview.Item{{Position: 1, QuestionVersionID: v.ID, Seed: seed}},
		ExpiresAt: now(h.Clock).Add(h.ttl()).Truncate(time.Second)}
	out = PreviewView{ID: id, Items: []PreviewItem{item}, ExpiresAt: s.ExpiresAt}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.Put(ctx, s, h.ttl()); err != nil {
			return problem.New(problem.Unavailable, "preview store unavailable").Retry().Wrap(err)
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// content builds the scorer/renderer input of a version of any status (CMS read model: answer key included).
func content(v dto.QuestionVersion) (question.Content, error) {
	var rule question.ScoringRule
	if !isNull(v.ScoringRule) {
		if err := json.Unmarshal(v.ScoringRule, &rule); err != nil {
			return question.Content{}, err
		}
	}
	var assets []question.MathAsset
	if !isNull(v.MathAssets) {
		if err := json.Unmarshal(v.MathAssets, &assets); err != nil {
			return question.Content{}, err
		}
	}
	return question.Content{VersionID: v.ID, Type: question.Type(v.Type), Stem: v.Stem, Render: v.RenderPayload, AnswerKey: v.AnswerKey,
		Scoring: rule, MaxScore: v.MaxScore, Language: v.ContentLanguage, MathAssets: assets}, nil
}

func (h Preview) render(v dto.QuestionVersion, position int, seed uint64, lang string) (PreviewItem, error) {
	c, err := content(v)
	if err != nil {
		return PreviewItem{}, err
	}
	r, _, err := h.Registry.Render(c, seed)
	if errors.Is(err, question.ErrTypeNotEnabled) {
		return PreviewItem{}, problem.Invalid("content_id", question.SubTypeNotEnabled, v.Type+" is not enabled")
	}
	if err != nil {
		return PreviewItem{}, err
	}
	payload, _ := preview.Localize(r.Payload, lang).(map[string]any)
	return PreviewItem{Position: position, QuestionVersionID: v.ID, Type: r.Type, Stem: preview.Localize(r.Stem, lang), RenderPayload: payload,
		MaxScore: r.MaxScore}, nil
}

// PreviewResponseInput is PreviewResponseRequest.
type PreviewResponseInput struct {
	SessionID uuid.UUID
	Position  int
	Response  json.RawMessage
}

// OptionFeedbackView is the feedback of a chosen option.
type OptionFeedbackView struct {
	OptionID string `json:"option_id"`
	Feedback any    `json:"feedback,omitempty"`
}

// HintLevel is one hint of a preview result.
type HintLevel struct {
	Level int `json:"level"`
	Hint  any `json:"hint"`
}

// PreviewResult is PreviewResponseResult.
type PreviewResult struct {
	Position          int                  `json:"position"`
	QuestionVersionID uuid.UUID            `json:"question_version_id"`
	IsCorrect         bool                 `json:"is_correct"`
	Score             float64              `json:"score"`
	MaxScore          float64              `json:"max_score"`
	Parts             []question.Part      `json:"parts,omitempty"`
	Explanation       any                  `json:"explanation,omitempty"`
	Feedback          []OptionFeedbackView `json:"feedback,omitempty"`
	HintLevels        []HintLevel          `json:"hint_levels,omitempty"`
}

// SubmitResponse grades a try in a live preview session (F027-AC-18): the answer key of the previewed version (a
// draft included) on the server, explanation, feedback of the chosen options and the hints by level; an expired or
// foreign session is 404. Nothing is stored.
func (h Preview) SubmitResponse(ctx context.Context, in PreviewResponseInput) (out PreviewResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SubmitPreviewResponse")
	defer func() { done(err) }()
	p, err := previewRead(ctx)
	if err != nil {
		return out, err
	}
	s, err := h.Store.Get(ctx, in.SessionID)
	if errors.Is(err, port.ErrPreviewNotFound) || (err == nil && (s.Expired(now(h.Clock)) || !s.OwnedBy(p.UserID, p.TenantID))) {
		return out, problem.New(problem.NotFound, "the preview session expired or does not exist")
	}
	if err != nil {
		return out, problem.New(problem.Unavailable, "preview store unavailable").Retry().Wrap(err)
	}
	it, ok := s.Item(in.Position)
	if !ok {
		return out, problem.Invalid("position", problem.SubInvalid, "is not a question of the preview")
	}
	var v dto.QuestionVersion
	if err := h.Session.Read(ctx, func(ctx context.Context) error {
		var err error
		v, err = h.Questions.Version(ctx, s.ContentID, s.Version)
		return err
	}); err != nil {
		return out, err
	}
	if v.ID != it.QuestionVersionID {
		return out, problem.New(problem.NotFound, "the previewed version changed")
	}
	c, err := content(v)
	if err != nil {
		return out, err
	}
	if h.Schema != nil && h.Schema.ValidateResponse(v.Type, in.Response) != nil {
		return out, problem.New(problem.InvalidResponse, "the response does not match the question type")
	}
	g, err := h.Registry.GradeIn(c, question.Presentation{QuestionVersionID: v.ID, Seed: it.Seed}, in.Response)
	if errors.Is(err, question.ErrInvalidResponse) {
		return out, problem.New(problem.InvalidResponse, "the response does not match the question type")
	}
	if err != nil {
		return out, err
	}
	rendered := make([]mathx.Rendered, len(c.MathAssets))
	for i, a := range c.MathAssets {
		rendered[i] = mathx.Rendered{Latex: a.Latex, SVGAsset: a.SVGAsset, SpeechText: a.SpeechText}
	}
	rich := func(raw json.RawMessage) any {
		if isNull(raw) {
			return nil
		}
		var x any
		if json.Unmarshal(raw, &x) != nil {
			return nil
		}
		return preview.Localize(mathx.Transform(x, rendered, false), s.DisplayLanguage)
	}
	out = PreviewResult{Position: in.Position, QuestionVersionID: v.ID, IsCorrect: g.IsCorrect, Score: g.Score, MaxScore: g.MaxScore, Parts: g.Parts,
		Explanation: rich(v.Explanation)}
	var fb map[string]question.OptionFeedback
	if !isNull(v.OptionFeedback) {
		_ = json.Unmarshal(v.OptionFeedback, &fb)
	}
	for _, id := range g.SelectedOptionIDs {
		if f, ok := fb[id]; ok && !isNull(f.Feedback) {
			out.Feedback = append(out.Feedback, OptionFeedbackView{OptionID: id, Feedback: rich(f.Feedback)})
		}
	}
	for i, hint := range hintsOf(v.Hints) {
		out.HintLevels = append(out.HintLevels, HintLevel{Level: i + 1, Hint: rich(hint)})
	}
	return out, nil
}
