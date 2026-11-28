package command

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/preview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// maxPreviewItems is LearningPreviewSession.items maxItems.
const maxPreviewItems = 200

// activitySession opens the preview of an activity version of any status (F058-AC-10/11, F027-AC-16/17): the
// ActivityRender a student would get (attemptview.Render: no question source, evaluation or pass threshold) and the
// QuestionRender of the questions fixed by the version, in order and without answer key, option feedback or hints. The
// questions an attempt would draw at start (selection, rules of a question set, the source of a mini-game) are not drawn
// in a preview [Confirmed — owner "chốt hết", F058 Clarifications 2026-11-28]: render only for them. Nothing is written
// but the Valkey session and the idempotency record — no attempt, learning event, XP or game session (ADR-0031).
func (h Preview) activitySession(ctx context.Context, p authz.Principal, in PreviewInput) (out PreviewView, err error) {
	id := ids.New()
	seed := binary.BigEndian.Uint64(id[8:])
	var render dto.ActivityRenderView
	var items []PreviewItem
	var lang string
	if err := h.Session.Read(ctx, func(ctx context.Context) error {
		src, err := h.activitySource(ctx, in.ContentID, in.Version)
		if err != nil {
			return err
		}
		lang = preview.Language(in.DisplayLanguage, src.Version.ContentLanguage)
		render = attemptview.Render(src, h.Activities, lang)
		qids, err := h.fixedQuestions(ctx, src)
		if err != nil {
			return err
		}
		for i, qid := range qids {
			v, err := h.QuestionVersions.VersionByID(ctx, qid)
			if err != nil {
				return err
			}
			item, err := h.render(v, i+1, seed+uint64(i), lang)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	}); err != nil {
		return out, err
	}
	s := preview.Session{ID: id, TenantID: ptrClone(p.TenantID), PrincipalID: p.UserID, ContentType: in.ContentType, ContentID: in.ContentID,
		Version: in.Version, DisplayLanguage: lang, ExpiresAt: now(h.Clock).Add(h.ttl()).Truncate(time.Second)}
	for _, it := range items {
		s.Items = append(s.Items, preview.Item{Position: it.Position, QuestionVersionID: it.QuestionVersionID, Seed: seed + uint64(it.Position-1)})
	}
	if items == nil {
		items = []PreviewItem{}
	}
	out = PreviewView{ID: id, Render: &render, Items: items, ExpiresAt: s.ExpiresAt}
	return out, h.store(ctx, s, out)
}

// activitySource reads version versionNo of an activity in the scope of ctx (another tenant's activity, or an unknown
// version, is 404) with its plugin and parsed config.
func (h Preview) activitySource(ctx context.Context, activityID uuid.UUID, versionNo int) (ActivitySource, error) {
	d, err := h.Docs.Doc(ctx, contentmodel.TypeActivity, activityID)
	if err != nil {
		return ActivitySource{}, err
	}
	v, err := h.Docs.Version(ctx, contentmodel.TypeActivity, activityID, versionNo)
	if err != nil {
		return ActivitySource{}, err
	}
	src := ActivitySource{Doc: d, Version: v, Body: attemptview.ParseBody(v.Body)}
	if src.Plugin, err = ActivityPlugin(h.Activities, src.Body.ActivityType); err != nil {
		return src, err
	}
	if src.Config, err = activity.ParseConfig(src.Body.Config); err != nil {
		return src, problem.New(problem.ActivityConfigInvalid, "the activity config cannot be read")
	}
	return src, nil
}

// fixedQuestions are the question versions an activity version fixes, in the order of an unshuffled attempt: the cues of
// a media activity, the conclusion questions of a simulation, the questions of a "questions" source or the fixed items
// of the question set (the pinned version, else its newest version). At most maxPreviewItems.
func (h Preview) fixedQuestions(ctx context.Context, src ActivitySource) ([]uuid.UUID, error) {
	var out []uuid.UUID
	var err error
	switch pl := src.Plugin.(type) {
	case activity.GameScored:
		return nil, nil
	case activity.Timeline:
		if out, err = pl.CueQuestions(src.DomainVersion()); err != nil {
			return nil, problem.New(problem.ActivityConfigInvalid, "the media config cannot be read")
		}
	case activity.Simulated:
		if out, err = pl.SimulationQuestions(src.DomainVersion()); err != nil {
			return nil, problem.New(problem.ActivityConfigInvalid, "the simulation config cannot be read")
		}
	default:
		qs := src.Config.Content.QuestionSource
		switch qs.Kind {
		case activity.SourceQuestions:
			for _, q := range qs.Questions {
				out = append(out, q.QuestionVersionID)
			}
		case activity.SourceQuestionSet:
			if qs.QuestionSetID != nil {
				if out, err = h.setItems(ctx, *qs.QuestionSetID, qs.QuestionSetVersion); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(out) > maxPreviewItems {
		out = out[:maxPreviewItems]
	}
	return out, nil
}

// setItems are the fixed items of a question set version (pinned number, else the newest version).
func (h Preview) setItems(ctx context.Context, setID uuid.UUID, versionNo *int) ([]uuid.UUID, error) {
	var v dto.DocVersion
	var err error
	if versionNo != nil {
		v, err = h.Docs.Version(ctx, contentmodel.TypeQuestionSet, setID, *versionNo)
	} else {
		v, err = h.Docs.Latest(ctx, contentmodel.TypeQuestionSet, setID)
	}
	if err != nil {
		return nil, err
	}
	var body struct {
		Definition questionset.Definition `json:"definition"`
	}
	_ = json.Unmarshal(v.Body, &body)
	var out []uuid.UUID
	for _, it := range body.Definition.Items {
		out = append(out, it.QuestionVersionID)
	}
	if len(out) > 0 || len(body.Definition.SelectionRules) > 0 {
		return out, nil
	}
	fixed, err := h.Docs.Items(ctx, []uuid.UUID{v.ID})
	if err != nil {
		return nil, err
	}
	for _, it := range fixed[v.ID] {
		out = append(out, it.QuestionVersionID)
	}
	return out, nil
}
