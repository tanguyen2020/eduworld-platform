package command

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// EventLessonCompleted is the learning event and domain event of a completed lesson (payload lesson.completed.v1).
const EventLessonCompleted = "lesson.completed"

// lessonRetries bounds the optimistic retries on a session row inside one transaction.
const lessonRetries = 5

// LessonMetrics counts completions (eduworld_lesson_completions_total{trigger}); nil = not counted.
type LessonMetrics interface {
	LessonCompleted(trigger string)
}

// Lessons are the write use cases of lesson study (F031 US6): the session pinned to the version open at its first attempt
// (R31), its completion by the server against the completion rule, lesson.completed once per student × version.
type Lessons struct {
	Store    write.LessonStore
	Docs     write.DocStore
	Learning write.LearningEventWriter
	Events   write.EventPublisher
	UoW      write.UnitOfWork
	Clock    func() time.Time
	Metrics  LessonMetrics
}

func (h Lessons) now() time.Time { return now(h.Clock).UTC() }

// PinnedLesson is a lesson version with its parsed body.
type PinnedLesson struct {
	Doc        dto.Doc
	Version    dto.DocVersion
	Body       lesson.Body
	Activities []lesson.ActivityRef
}

// LoadLessonVersion reads a lesson visible to the scope of ctx: the version id given, or the released one (404 when the
// lesson is invisible or not released, F031-AC-29).
func LoadLessonVersion(ctx context.Context, docs write.DocStore, lessonID uuid.UUID, versionID *uuid.UUID) (PinnedLesson, error) {
	var out PinnedLesson
	d, err := docs.Doc(ctx, contentmodel.TypeLesson, lessonID)
	if err != nil {
		return out, notFoundOf(err)
	}
	if versionID == nil {
		if d.CurrentVersionID == nil {
			return out, problem.New(problem.NotFound, "")
		}
		versionID = d.CurrentVersionID
	}
	vs, err := docs.DocVersions(ctx, contentmodel.TypeLesson, lessonID)
	if err != nil {
		return out, err
	}
	i := slices.IndexFunc(vs, func(v dto.DocVersion) bool { return v.ID == *versionID })
	if i < 0 {
		return out, problem.New(problem.NotFound, "")
	}
	out.Doc, out.Version = d, vs[i]
	if out.Body, err = lesson.ParseBody(out.Version.Body); err != nil {
		return out, problem.New(problem.Internal, "the lesson body cannot be read")
	}
	out.Activities = lesson.Activities(out.Version.Body)
	return out, nil
}

func notFoundOf(err error) error {
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.NotFound {
		return problem.New(problem.NotFound, "")
	}
	return err
}

// PinInput is the start of an attempt in the context of a lesson.
type PinInput struct {
	StudentID        uuid.UUID
	TenantID         uuid.UUID
	SchoolID         uuid.UUID
	LessonID         uuid.UUID
	ActivityID       uuid.UUID
	Mode             string
	RecommendationID *uuid.UUID
}

// Pin runs in the transaction of the attempt start (F031-AC-25): the open session of the lesson, or a new one pinned to the
// released version; the activity must belong to the pinned version (422 VALIDATION_FAILED). A version the student already
// completed opens no new session (F031-AC-27).
func (h Lessons) Pin(ctx context.Context, in PinInput) error {
	s, err := h.Store.OpenSession(ctx, in.StudentID, in.LessonID)
	if err != nil {
		return err
	}
	var versionID *uuid.UUID
	if s != nil {
		versionID = &s.LessonVersionID
	}
	l, err := LoadLessonVersion(ctx, h.Docs, in.LessonID, versionID)
	if err != nil {
		return err
	}
	if !lesson.Contains(l.Activities, in.ActivityID) {
		return problem.Invalid("/context_id", problem.SubInvalid, "the activity is not part of the lesson version")
	}
	if s != nil {
		return nil
	}
	done, err := h.Store.CompletedVersion(ctx, in.StudentID, l.Version.ID)
	if err != nil || done {
		return err
	}
	mode := in.Mode
	if mode != "assignment" {
		mode = "practice"
	}
	_, err = h.Store.InsertSession(ctx, write.LessonSession{ID: ids.New(), TenantID: in.TenantID, SchoolID: in.SchoolID, StudentID: in.StudentID,
		LessonID: in.LessonID, LessonVersionID: l.Version.ID, LessonVersion: l.Version.VersionNo, ContextMode: mode,
		RecommendationID: in.RecommendationID, StartedAt: h.now()})
	return err
}

// OnGraded is the consumer learning.lesson_completion (F031-AC-26, AC-27): a graded attempt started in the context of a
// lesson marks its activity completed in the open session and completes the session when the rule is satisfied.
func (h Lessons) OnGraded(ctx context.Context, attemptID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.LessonOnGraded")
	defer func() { done(err) }()
	a, err := h.Store.Attempt(ctx, attemptID)
	if err != nil || a == nil || a.ContextType != "lesson" || a.ActivityID == nil {
		return err
	}
	for range lessonRetries {
		s, err := h.Store.OpenSession(ctx, a.StudentID, a.ContextID)
		if err != nil || s == nil {
			return err
		}
		l, err := LoadLessonVersion(ctx, h.Docs, s.LessonID, &s.LessonVersionID)
		if err != nil {
			return err
		}
		if slices.Contains(s.CompletedAttemptIDs, a.AttemptID) || !lesson.Contains(l.Activities, *a.ActivityID) {
			return nil
		}
		next := *s
		if !slices.Contains(next.CompletedActivityIDs, *a.ActivityID) && len(next.CompletedActivityIDs) < 50 {
			next.CompletedActivityIDs = append(slices.Clone(next.CompletedActivityIDs), *a.ActivityID)
		}
		if len(next.CompletedAttemptIDs) < 50 {
			next.CompletedAttemptIDs = append(slices.Clone(next.CompletedAttemptIDs), a.AttemptID)
		}
		next.ActiveMS += max(a.DurationMS, 0)
		ok, err := h.saveOrComplete(ctx, l, &next, "activity_graded")
		if err != nil || ok {
			return err
		}
	}
	return errors.New("lesson: concurrent updates of the session")
}

// saveOrComplete stores the session, completing it when its rule is satisfied; false on a row-version conflict.
func (h Lessons) saveOrComplete(ctx context.Context, l PinnedLesson, s *write.LessonSession, trigger string) (bool, error) {
	ok, _ := lesson.Satisfied(l.Body.CompletionRule, l.Activities, s.CompletedActivityIDs)
	at := h.now()
	if ok {
		s.Status, s.EndedAt = lesson.SessionCompleted, &at
	}
	saved, err := h.Store.UpdateSession(ctx, *s)
	if err != nil || !saved {
		return false, err
	}
	if ok {
		return true, h.completed(ctx, l, *s, trigger)
	}
	return true, nil
}

// lessonKeys namespaces the idempotency key of lesson.completed (UUIDv5 of student × lesson version, F031-AC-27).
var lessonKeys = uuid.MustParse("0193a1f0-31f0-7031-8031-000000000033")

func (h Lessons) completed(ctx context.Context, l PinnedLesson, s write.LessonSession, trigger string) error {
	v := l.Version
	required := 0
	for _, a := range l.Activities {
		if a.Required {
			required++
		}
	}
	progress := 1.0
	if len(l.Activities) > 0 {
		progress = min(1, float64(len(s.CompletedActivityIDs))/float64(len(l.Activities)))
	}
	cv, content := v.ID, s.LessonID
	if err := h.Learning.Write(ctx, write.LearningEvent{ID: ids.New(), OccurredAt: *s.EndedAt, Type: EventLessonCompleted, StudentID: s.StudentID,
		TenantID: s.TenantID, SchoolID: s.SchoolID, ContentID: &content, ContentVersionID: &cv, ObjectiveIDs: nonNilUUIDs(v.ObjectiveIDs),
		Context: s.ContextMode, ActorType: "student", IdempotencyKey: "lsn:" + uuid.NewSHA1(lessonKeys, []byte(s.StudentID.String()+":"+v.ID.String())).String(),
		Payload: map[string]any{"duration_ms": s.ActiveMS, "progress": progress, "verified_by_server": true}}); err != nil {
		return err
	}
	gradeLevels := v.GradeLevels
	if gradeLevels == nil {
		gradeLevels = []int{}
	}
	t := s.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: EventLessonCompleted, SubjectType: "lesson", SubjectID: s.LessonID.String(), TenantID: &t,
		Payload: map[string]any{"lesson_session_id": s.ID, "lesson_id": s.LessonID, "lesson_version_id": v.ID, "lesson_version": v.VersionNo,
			"student_id": s.StudentID, "subject_id": nullableID(v.SubjectID), "grade_levels": gradeLevels,
			"completion_rule": l.Body.CompletionRule, "mode": s.ContextMode, "assignment_id": nullableID(s.AssignmentID),
			"learning_path_id": nullableID(s.LearningPathID), "recommendation_id": nullableID(s.RecommendationID),
			"required_activity_count": required, "completed_activity_count": len(s.CompletedActivityIDs),
			"attempt_ids": nonNilUUIDs(s.CompletedAttemptIDs), "objective_ids": nonNilUUIDs(uniqueUUIDs(v.ObjectiveIDs)),
			"started_at": s.StartedAt, "completed_at": *s.EndedAt, "duration_ms": s.ActiveMS}}); err != nil {
		return err
	}
	if h.Metrics != nil {
		h.Metrics.LessonCompleted(trigger)
	}
	return nil
}

// Complete is completeLesson (F031-AC-28): 200 with the first completed_at when the session is completed or now satisfies
// its rule (completing it); 409 CONFLICT with the missing required activities otherwise, or without session. Nothing the
// client sends about results or durations is read (ADR-0016).
func (h Lessons) Complete(ctx context.Context, lessonID uuid.UUID) (out dto.LessonCompletionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CompleteLesson")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		for range lessonRetries {
			s, err := h.Store.LatestSession(ctx, p.UserID, lessonID)
			if err != nil {
				return err
			}
			if s == nil {
				if _, err := LoadLessonVersion(ctx, h.Docs, lessonID, nil); err != nil {
					return err
				}
				return problem.New(problem.Conflict, "the lesson has no session: start one of its activities first")
			}
			if s.Status == lesson.SessionCompleted {
				out = dto.LessonCompletionView{LessonID: lessonID, CompletedAt: *s.EndedAt}
				return idempotency.SetResult(ctx, out)
			}
			l, err := LoadLessonVersion(ctx, h.Docs, lessonID, &s.LessonVersionID)
			if err != nil {
				return err
			}
			if ok, missing := lesson.Satisfied(l.Body.CompletionRule, l.Activities, s.CompletedActivityIDs); !ok {
				pe := problem.New(problem.Conflict, "the completion rule of the lesson is not satisfied")
				for _, id := range missing {
					pe = pe.AddField("/activities/"+id.String(), "NOT_COMPLETED", "required activity not completed")
				}
				return pe
			}
			saved, err := h.saveOrComplete(ctx, l, s, "complete_lesson")
			if err != nil {
				return err
			}
			if saved {
				out = dto.LessonCompletionView{LessonID: lessonID, CompletedAt: *s.EndedAt}
				return idempotency.SetResult(ctx, out)
			}
		}
		return problem.New(problem.Conflict, "the lesson session changed; retry").Retry()
	})
	return out, err
}
