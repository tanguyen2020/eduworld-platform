package query

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// ShownRecommendations is the number of recommendations shown at once (LEARNING_ANALYTICS §6).
const ShownRecommendations = 3

// SelfStudy reads the review list and the recommendations of the student of the session (F032 US4, US6): the
// student's own rows only, in the tenant of the session; no answer, explanation or hint.
type SelfStudy struct {
	Session read.Session
	Model   read.SelfStudyReadModel
	Reasons read.ReasonCatalog
	Cursors *page.Codec
	Clock   func() time.Time
}

func (q SelfStudy) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

// WrongQuestions is listMyWrongQuestions (F032-AC-23).
func (q SelfStudy) WrongQuestions(ctx context.Context, subjectID *uuid.UUID, in PageInput) (out Page[dto.WrongQuestionView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListMyWrongQuestions")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		rs, err := q.Model.WrongQuestions(ctx, p.UserID, subjectID)
		if err != nil {
			return err
		}
		views := make([]dto.WrongQuestionView, len(rs))
		for i, r := range rs {
			views[i] = dto.WrongQuestionView{EntryID: r.EntryID, QuestionVersionID: r.QuestionVersionID, SubjectID: r.SubjectID,
				ObjectiveIDs: r.ObjectiveIDs, WrongCount: r.WrongCount, LastWrongAt: r.LastWrongAt, NextReviewAt: r.NextDueAt}
		}
		out, err = paginate(views, func(v dto.WrongQuestionView) uuid.UUID { return v.EntryID }, in, q.Cursors)
		return err
	})
	return out, err
}

// Recommendations is getMyRecommendations (F032-AC-29): at most three active recommendations by priority with the
// reason in the language asked (vi/en).
func (q SelfStudy) Recommendations(ctx context.Context, subjectID *uuid.UUID, lang string) (out dto.RecommendationListView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetMyRecommendations")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	now := q.now()
	out = dto.RecommendationListView{Items: []dto.RecommendationView{}, GeneratedAt: now}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		rs, err := q.Model.Recommendations(ctx, p.UserID, subjectID, now)
		if err != nil {
			return err
		}
		var latest time.Time
		for _, r := range rs {
			if len(out.Items) == ShownRecommendations {
				break
			}
			params := map[string]string{}
			_ = json.Unmarshal(r.ReasonParams, &params)
			v := dto.RecommendationView{RecommendationID: r.ID, RuleCode: r.RuleCode, Kind: r.Kind, TargetType: r.TargetType, TargetID: r.TargetID,
				ObjectiveIDs: r.ObjectiveIDs, SubjectID: r.SubjectID, GradeLevel: r.Grade, Reason: r.Reason, ReasonCode: r.ReasonCode,
				ReasonText: q.Reasons.Text(r.ReasonCode, lang, params), Priority: r.Priority}
			if len(r.ObjectiveIDs) > 0 {
				o := r.ObjectiveIDs[0]
				v.ObjectiveID = &o
			}
			if v.ObjectiveIDs == nil {
				v.ObjectiveIDs = []uuid.UUID{}
			}
			out.Items = append(out.Items, v)
			if r.GeneratedAt.After(latest) {
				latest = r.GeneratedAt
			}
		}
		if !latest.IsZero() {
			out.GeneratedAt = latest
		}
		return nil
	})
	return out, err
}

// MyMastery is getMyMastery (F032-AC-34): the level after decay and the icon of the grade band of each objective; the
// numeric value only for grades 6–9; the student's own data only.
func (q Progress) MyMastery(ctx context.Context, subjectID *uuid.UUID, grade *int) (out dto.MyMasteryView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetMyMastery")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	out.Items = []dto.MyMasteryItemView{}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		m, err := q.model(ctx)
		if err != nil {
			return err
		}
		band, err := q.band(ctx, p.UserID)
		if err != nil {
			return err
		}
		out.GradeBand = band
		ds, err := q.Details.MasteryDetails(ctx, p.UserID, subjectID, grade)
		if err != nil {
			return err
		}
		if q.Arms != nil && p.TenantID != nil {
			// F035-AC-24: a student of a live arm sees the mastery of the arm model; school reports keep the active model.
			arm, err := q.Arms.LiveArm(ctx, *p.TenantID, p.UserID)
			if err != nil {
				return err
			}
			if arm != nil {
				params := mastery.DefaultParams()
				if len(arm.Params) > 0 {
					_ = json.Unmarshal(arm.Params, &params)
				}
				m = mastery.WeightedEvidence{ModelVersion: arm.ModelVersion, P: params}
				for i := range ds {
					if st, ok := arm.States[ds[i].ObjectiveID]; ok {
						ds[i].State = st
					}
				}
			}
		}
		now := q.now()
		for _, d := range ds {
			v, level := m.Effective(d.State, now)
			it := dto.MyMasteryItemView{ObjectiveID: d.ObjectiveID, ObjectiveCode: d.Code, ObjectiveName: truncateRunes(d.Name, 255),
				SubjectID: d.SubjectID, TopicID: d.TopicID, Level: string(level), IconCode: mastery.IconCode(band, level),
				EvidenceCount: d.State.EvidenceCount}
			if mastery.ShowsValue(band) {
				r := math.Round(v*1000) / 1000
				it.Value = &r
			}
			if d.State.LastEvidenceAt != nil {
				t := d.State.LastEvidenceAt.UTC()
				it.LastEvidenceAt = &t
			}
			out.Items = append(out.Items, it)
		}
		return nil
	})
	return out, err
}

// band is the grade band of a student of the session tenant (g1_3 when unknown).
func (q Progress) band(ctx context.Context, studentID uuid.UUID) (string, error) {
	s, err := q.Model.Student(ctx, studentID)
	if err != nil || s == nil {
		return mastery.GradeBand(nil), err
	}
	g := s.GradeLevel
	return mastery.GradeBand(&g), nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
