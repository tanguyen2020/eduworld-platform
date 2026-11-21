package query

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Progress reads the progress and mastery of a student (F031 US3, LEARNING_ANALYTICS §4, §8): levels are computed with
// the decay of the active model at read time; the stored values are never changed nor returned (F031-AC-11).
type Progress struct {
	Session read.Session
	Model   read.ProgressReadModel
	// Details reads the masteries with their curriculum metadata (getMyMastery, F032 US7).
	Details read.SelfStudyReadModel
	Clock   func() time.Time
	// Arms reads the live arm of a student (F035-AC-24); nil = no experiment.
	Arms read.ArmReader
}

func (q Progress) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

// Mine is getMyProgress (F031-AC-15): the student of the session only.
func (q Progress) Mine(ctx context.Context, subjectID *uuid.UUID) (out dto.LearningProgressView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetMyProgress")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.view(ctx, p.UserID, subjectID, &p)
		return err
	})
	return out, err
}

// liveArm is the live mastery_model arm of the student of the session (F035-AC-24): its model and the states computed
// with it; m and no state otherwise (no experiment, control arm, shadow). Only the student experience reads it; school
// reports keep the active model.
func (q Progress) liveArm(ctx context.Context, p authz.Principal, m mastery.WeightedEvidence) (mastery.WeightedEvidence,
	map[uuid.UUID]mastery.State, error) {
	if q.Arms == nil || p.TenantID == nil {
		return m, nil, nil
	}
	arm, err := q.Arms.LiveArm(ctx, *p.TenantID, p.UserID)
	if err != nil || arm == nil {
		return m, nil, err
	}
	params := mastery.DefaultParams()
	if len(arm.Params) > 0 {
		if err := json.Unmarshal(arm.Params, &params); err != nil {
			return m, nil, fmt.Errorf("arm model %s: %w", arm.ModelVersion, err)
		}
	}
	return mastery.WeightedEvidence{ModelVersion: arm.ModelVersion, P: params}, arm.States, nil
}

// Student is getStudentProgress (F031-AC-16): analytics.school.read in the school of the student (a grant limited to
// another school answers 404, no existence leak) and in the subject when the grant is subject-limited (403
// PERMISSION_DENIED; a subject-limited grant must name its subject) [Proposed]. The caller records the audit
// learning.profile_viewed through command.ProfileViews after a successful read.
func (q Progress) Student(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) (out dto.LearningProgressView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetStudentProgress")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil || p.UserType == authz.UserTypeStudent {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if !holds(p, authz.PermAnalyticsSchoolRead) {
		return out, authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, authz.Resource{TenantID: p.TenantID})
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		s, err := q.Model.Student(ctx, studentID)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		school := s.SchoolID
		if !coversSchool(p, authz.PermAnalyticsSchoolRead, school) {
			_ = authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
			return problem.New(problem.NotFound, "")
		}
		if err := authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, authz.Resource{TenantID: p.TenantID, SchoolID: &school,
			SubjectID: subjectID}); err != nil {
			return err
		}
		out, err = q.view(ctx, studentID, subjectID, nil)
		out.SchoolID = school
		return err
	})
	return out, err
}

func holds(p authz.Principal, perm authz.Permission) bool {
	for _, g := range p.Grants {
		for _, x := range g.Permissions {
			if x == perm {
				return true
			}
		}
	}
	return false
}

// coversSchool reports whether a grant holding perm covers the school (subject aside).
func coversSchool(p authz.Principal, perm authz.Permission, school uuid.UUID) bool {
	for _, g := range p.Grants {
		for _, x := range g.Permissions {
			if x == perm && (g.SchoolID == nil || *g.SchoolID == school) {
				return true
			}
		}
	}
	return false
}

// model is the active mastery model (its decay gives the levels at read time).
func (q Progress) model(ctx context.Context) (mastery.WeightedEvidence, error) {
	cfg, err := q.Model.ActiveModel(ctx)
	if err != nil {
		return mastery.WeightedEvidence{}, err
	}
	p := mastery.DefaultParams()
	if len(cfg.Params) > 0 {
		if err := json.Unmarshal(cfg.Params, &p); err != nil {
			return mastery.WeightedEvidence{}, err
		}
	}
	return mastery.WeightedEvidence{ModelVersion: cfg.ModelVersion, P: p}, nil
}

// view builds the progress of a student; self is the student of the session for getMyProgress (a live experiment arm
// applies, F035-AC-24), nil for getStudentProgress (always the active model).
func (q Progress) view(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID, self *authz.Principal) (dto.LearningProgressView, error) {
	out := dto.LearningProgressView{StudentID: studentID, Objectives: []dto.ObjectiveProgressView{}}
	m, err := q.model(ctx)
	if err != nil {
		return out, err
	}
	var states map[uuid.UUID]mastery.State
	if self != nil {
		if m, states, err = q.liveArm(ctx, *self, m); err != nil {
			return out, err
		}
	}
	band, err := q.band(ctx, studentID)
	if err != nil {
		return out, err
	}
	ms, err := q.Model.Masteries(ctx, studentID, subjectID)
	if err != nil {
		return out, err
	}
	now := q.now()
	for _, o := range ms {
		if st, ok := states[o.ObjectiveID]; ok {
			o.State = st
		}
		_, level := m.Effective(o.State, now)
		v := dto.ObjectiveProgressView{ObjectiveID: o.ObjectiveID, Level: string(level), IconCode: mastery.IconCode(band, level),
			EvidenceCount: o.State.EvidenceCount}
		if o.State.LastEvidenceAt != nil {
			t := o.State.LastEvidenceAt.UTC()
			v.LastEvidenceAt = &t
		}
		out.Objectives = append(out.Objectives, v)
	}
	t, err := q.Model.Totals(ctx, studentID, subjectID)
	if err != nil {
		return out, err
	}
	out.Totals = dto.ProgressTotalsView{LessonsCompleted: t.LessonsCompleted, ItemsAnswered: t.ItemsAnswered,
		ActiveMinutes: int(t.ActiveSeconds / 60)}
	if t.ItemsAnswered > 0 {
		a := math.Round(float64(t.ItemsCorrect)/float64(t.ItemsAnswered)*1000) / 1000
		out.Totals.Accuracy = &a
	}
	return out, nil
}
