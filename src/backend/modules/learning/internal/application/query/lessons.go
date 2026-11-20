package query

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Lessons reads a lesson for its student (getLesson, F031-AC-25, AC-29): the version pinned by the student's session, or
// the released version before any session; activities[].completed comes from the session. Never an answer.
type Lessons struct {
	Session read.Session
	Model   read.LessonReadModel
}

// Get is getLesson.
func (q Lessons) Get(ctx context.Context, lessonID uuid.UUID) (out dto.LessonView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetLesson")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		s, err := q.Model.Session(ctx, p.UserID, lessonID)
		if err != nil {
			return err
		}
		var pinned *uuid.UUID
		if s != nil {
			pinned = &s.LessonVersionID
		}
		v, err := q.Model.Version(ctx, lessonID, pinned)
		if err != nil {
			return err
		}
		if v == nil || (pinned == nil && v.Status != "published") {
			return problem.New(problem.NotFound, "")
		}
		body, err := lesson.ParseBody(v.Body)
		if err != nil {
			return problem.New(problem.Internal, "the lesson body cannot be read")
		}
		acts := lesson.Activities(v.Body)
		ids := make([]uuid.UUID, len(acts))
		for i, a := range acts {
			ids[i] = a.ActivityID
		}
		info, err := q.Model.Activities(ctx, ids)
		if err != nil {
			return err
		}
		out = dto.LessonView{LessonID: lessonID, Version: v.VersionNo, Title: v.Title, SubjectID: v.SubjectID, ObjectiveIDs: v.ObjectiveIDs,
			EstimatedMinutes: body.EstimatedMinutes, CompletionRule: body.CompletionRule, Activities: []dto.LessonActivityView{}}
		if len(v.Summary) > 0 && string(v.Summary) != "null" {
			out.Summary = v.Summary
		}
		if out.ObjectiveIDs == nil {
			out.ObjectiveIDs = []uuid.UUID{}
		}
		if len(v.GradeLevels) > 0 {
			g := v.GradeLevels[0]
			out.GradeLevel = &g
		}
		for _, a := range acts {
			av := dto.LessonActivityView{ActivityID: a.ActivityID, ActivityType: info[a.ActivityID].ActivityType, Required: a.Required}
			if t := info[a.ActivityID].Title; len(t) > 0 {
				av.Title = t
			}
			if s != nil {
				av.Completed = slices.Contains(s.CompletedActivityIDs, a.ActivityID)
			}
			out.Activities = append(out.Activities, av)
		}
		if s != nil && s.Status == lesson.SessionCompleted {
			out.CompletedAt = s.EndedAt
		}
		return nil
	})
	return out, err
}
