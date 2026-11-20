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
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Progress reads the progress and mastery of a student (F031 US3, LEARNING_ANALYTICS §4, §8): levels are computed with
// the decay of the active model at read time; the stored values are never changed nor returned (F031-AC-11).
type Progress struct {
	Session read.Session
	Model   read.ProgressReadModel
	Clock   func() time.Time
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
		out, err = q.view(ctx, p.UserID, subjectID)
		return err
	})
	return out, err
}

// Student is getStudentProgress (F031-AC-16): analytics.class.read in the school of the student (a grant limited to
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
	if !holds(p, authz.PermAnalyticsClassRead) {
		return out, authz.Require(ctx, p, authz.PermAnalyticsClassRead, authz.Resource{TenantID: p.TenantID})
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
		if !coversSchool(p, authz.PermAnalyticsClassRead, school) {
			_ = authz.Require(ctx, p, authz.PermAnalyticsClassRead, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
			return problem.New(problem.NotFound, "")
		}
		if err := authz.Require(ctx, p, authz.PermAnalyticsClassRead, authz.Resource{TenantID: p.TenantID, SchoolID: &school,
			SubjectID: subjectID}); err != nil {
			return err
		}
		out, err = q.view(ctx, studentID, subjectID)
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

func (q Progress) view(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) (dto.LearningProgressView, error) {
	out := dto.LearningProgressView{StudentID: studentID, Objectives: []dto.ObjectiveProgressView{}}
	cfg, err := q.Model.ActiveModel(ctx)
	if err != nil {
		return out, err
	}
	p := mastery.DefaultParams()
	if len(cfg.Params) > 0 {
		if err := json.Unmarshal(cfg.Params, &p); err != nil {
			return out, err
		}
	}
	m := mastery.WeightedEvidence{ModelVersion: cfg.ModelVersion, P: p}
	ms, err := q.Model.Masteries(ctx, studentID, subjectID)
	if err != nil {
		return out, err
	}
	now := q.now()
	for _, o := range ms {
		_, level := m.Effective(o.State, now)
		v := dto.ObjectiveProgressView{ObjectiveID: o.ObjectiveID, Level: string(level), EvidenceCount: o.State.EvidenceCount}
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
