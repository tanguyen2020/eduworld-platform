package command

import (
	"context"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/assignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SubmissionViewOf is the AssignmentSubmission of a stored row.
func SubmissionViewOf(s write.SubmissionRow) dto.AssignmentSubmissionView {
	return dto.AssignmentSubmissionView{AssignmentID: s.AssignmentID, StudentID: s.StudentID, Status: s.Status,
		AttemptsUsed: s.AttemptsUsed, BestAttemptID: s.BestAttemptID, Score: s.Score, MaxScore: s.MaxScore, RapidGuessCount: s.RapidGuessCount,
		DueAtOverride: s.DueAtOverride, SubmittedAt: s.SubmittedAt}
}

// window is the window of a student (personal due date when extended, late window of the assignment).
func window(a write.AssignmentRow, s *write.SubmissionRow) assignment.Window {
	w := assignment.Window{OpenAt: a.OpenAt, DueAt: a.DueAt}
	if a.AllowLate {
		w.Late = time.Duration(a.LateHours) * time.Hour
	}
	if s != nil && s.DueAtOverride != nil {
		w.DueAt = *s.DueAtOverride
	}
	return w
}

// studentOfAssignment reads the submission of a targeted student (404 for a student outside the targets, F041-AC-26).
func (h Assignments) studentOfAssignment(ctx context.Context, a write.AssignmentRow, studentID uuid.UUID) (*write.SubmissionRow, error) {
	s, err := h.Store.Submission(ctx, a.ID, studentID, true)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	return s, nil
}

func validExceptionReason(reason string) error {
	if n := utf8.RuneCountInString(reason); n < 3 || n > assignment.MaxReason {
		return problem.Invalid("/reason", problem.SubInvalid, "must have 3 to 500 characters")
	}
	return nil
}

// studentChange loads an assignment of a published status and a targeted student, applies change and saves the
// submission, with audit and outbox assignment.submission_updated (Idempotency-Key).
func (h Assignments) studentChange(ctx context.Context, name, action string, id, studentID uuid.UUID, reason string,
	change func(ctx context.Context, a write.AssignmentRow, s *write.SubmissionRow, p authz.Principal) error) (out dto.AssignmentSubmissionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command."+name)
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.assignment(ctx, p, authz.PermLmsAssignmentWrite, id)
		if err != nil {
			return err
		}
		switch assignment.Status(a.Status) {
		case assignment.StatusScheduled, assignment.StatusOpen, assignment.StatusClosed:
		default:
			return problem.New(problem.AssignmentNotEditable, "the assignment is not published or already released")
		}
		s, err := h.studentOfAssignment(ctx, *a, studentID)
		if err != nil {
			return err
		}
		if err := change(ctx, *a, s, p); err != nil {
			return err
		}
		if err := h.Store.UpsertSubmission(ctx, *s); err != nil {
			return err
		}
		if err := h.submissionUpdated(ctx, *a, *s); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, auditEvent(action, *a, studentID, reason)); err != nil {
			return err
		}
		out = SubmissionViewOf(*s)
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

// ExtendAssignmentForStudent sets the personal due date of one student (F041-AC-22): later than the current one, a
// reason, audit; a missed student can work again (the scheduler keeps the assignment open for them).
func (h Assignments) ExtendAssignmentForStudent(ctx context.Context, id, studentID uuid.UUID, due time.Time, reason string) (dto.AssignmentSubmissionView, error) {
	if err := validExceptionReason(reason); err != nil {
		return dto.AssignmentSubmissionView{}, err
	}
	return h.studentChange(ctx, "ExtendAssignmentForStudent", "assignment.student_extended", id, studentID, reason,
		func(ctx context.Context, a write.AssignmentRow, s *write.SubmissionRow, p authz.Principal) error {
			if !due.After(window(a, s).DueAt) {
				return problem.Invalid("/due_at", problem.SubInvalid, "must be later than the current due date of the student")
			}
			if s.Status == "excused" {
				return problem.New(problem.Conflict, "the student is excused")
			}
			d := due.UTC()
			s.DueAtOverride = &d
			if s.Status == "missed" {
				s.Status = "not_started"
				if s.AttemptsUsed > 0 {
					s.Status = "in_progress"
				}
			}
			return h.Store.SetException(ctx, a.TenantID, a.SchoolID, a.ID, s.StudentID, "extend", &d, reason, p.UserID)
		})
}

// ExcuseAssignmentForStudent excuses one student (F041-AC-23): excused, out of the completion rate and of missed, a
// reason and audit.
func (h Assignments) ExcuseAssignmentForStudent(ctx context.Context, id, studentID uuid.UUID, reason string) (dto.AssignmentSubmissionView, error) {
	if err := validReason(reason); err != nil {
		return dto.AssignmentSubmissionView{}, err
	}
	return h.studentChange(ctx, "ExcuseAssignmentForStudent", "assignment.student_excused", id, studentID, reason,
		func(ctx context.Context, a write.AssignmentRow, s *write.SubmissionRow, p authz.Principal) error {
			s.Status = "excused"
			return h.Store.SetException(ctx, a.TenantID, a.SchoolID, a.ID, s.StudentID, "excuse", nil, reason, p.UserID)
		})
}

// RemindAssignment queues an in-game reminder to the students of an open assignment who did not submit and are not
// excused (F041-AC-24): at most assignment_remind_limit per day (ICT), otherwise 429 RATE_LIMITED with Retry-After until
// the next day; outbox assignment.reminded and audit.
func (h Assignments) RemindAssignment(ctx context.Context, id uuid.UUID) (out dto.RemindView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RemindAssignment")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	cfg := h.config()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.assignment(ctx, p, authz.PermLmsAssignmentWrite, id)
		if err != nil {
			return err
		}
		if a.Status != string(assignment.StatusOpen) {
			return problem.New(problem.Conflict, "only an open assignment is reminded")
		}
		now := h.now().In(cfg.Location)
		day := now.Format(time.DateOnly)
		n, err := h.Store.RemindersOn(ctx, a.ID, day)
		if err != nil {
			return err
		}
		if n >= cfg.RemindLimit {
			if h.Metrics != nil {
				h.Metrics.Reminder("limited")
			}
			y, m, d := now.Date()
			next := time.Date(y, m, d+1, 0, 0, 0, 0, cfg.Location)
			return &RetryAfter{After: next.Sub(now), Err: problem.New(problem.RateLimited, "assignment_remind_limit reached").Retry()}
		}
		pending, err := h.Store.PendingStudents(ctx, a.ID)
		if err != nil {
			return err
		}
		if err := h.Store.InsertReminder(ctx, a.TenantID, a.SchoolID, a.ID, day, len(pending), p.UserID); err != nil {
			return err
		}
		if len(pending) > 0 {
			if err := h.publish(ctx, EventAssignmentReminded, *a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID,
				"student_ids": pending, "reminder_no": n + 1, "due_at": a.DueAt, "reminded_by": p.UserID}); err != nil {
				return err
			}
		}
		if err := h.audit(ctx, "assignment.reminded", *a, "", map[string]any{"recipients": len(pending), "reminder_no": n + 1}); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.Reminder("sent")
		}
		out = dto.RemindView{Recipients: len(pending)}
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

// ---------- scheduler

// SchedulerReport counts what one run of the job assignment-scheduler did.
type SchedulerReport struct {
	Opened, Closed, Released int
}

// RunScheduler is the job assignment-scheduler (F041-AC-18, AC-20; every minute): it lists the assignments to open or
// close of every tenant under tenancy.Bypass(platform_job), then handles each one in the scope of its tenant and its own
// transaction, the row locked (a second worker sees the new status and skips it).
func (h Assignments) RunScheduler(ctx context.Context, limit int) (rep SchedulerReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.AssignmentScheduler")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = 200
	}
	now := h.now()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return rep, err
	}
	var open, closing []write.DueAssignment
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		if open, err = h.Store.DueToOpen(ctx, now, limit); err != nil {
			return err
		}
		closing, err = h.Store.DueToClose(ctx, now, limit)
		return err
	}); err != nil {
		return rep, err
	}
	for _, d := range open {
		if err := h.inTenant(ctx, d.TenantID, func(ctx context.Context) error { return h.open(ctx, d.ID, now, &rep) }); err != nil {
			return rep, err
		}
	}
	for _, d := range closing {
		if err := h.inTenant(ctx, d.TenantID, func(ctx context.Context) error { return h.close(ctx, d.ID, now, &rep) }); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

func (h Assignments) inTenant(ctx context.Context, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

func (h Assignments) open(ctx context.Context, id uuid.UUID, now time.Time, rep *SchedulerReport) error {
	a, err := h.Store.Assignment(ctx, id, true)
	if err != nil || a == nil || a.Status != string(assignment.StatusScheduled) || a.OpenAt.After(now) {
		return err
	}
	prev := a.Version
	a.Status = string(assignment.StatusOpen)
	if _, err := h.Store.UpdateAssignment(ctx, *a, prev, false); err != nil {
		return err
	}
	rep.Opened++
	h.count(a.Status)
	return nil
}

func (h Assignments) close(ctx context.Context, id uuid.UUID, now time.Time, rep *SchedulerReport) error {
	a, err := h.Store.Assignment(ctx, id, true)
	if err != nil || a == nil || a.Status != string(assignment.StatusOpen) {
		return err
	}
	late := time.Duration(0)
	if a.AllowLate {
		late = time.Duration(a.LateHours) * time.Hour
	}
	if !a.DueAt.Add(late).Before(now) {
		return nil
	}
	missed, err := h.Store.MarkMissed(ctx, a.ID, now, late)
	if err != nil {
		return err
	}
	prev := a.Version
	a.Status, a.ClosedAt = string(assignment.AfterClose(a.ResultRelease)), &now
	if a.Status == string(assignment.StatusResultsReleased) {
		a.ReleasedAt = &now
	}
	if _, err := h.Store.UpdateAssignment(ctx, *a, prev, false); err != nil {
		return err
	}
	if err := h.publish(ctx, EventAssignmentClosed, *a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID, "closed_at": now,
		"missed_count": missed, "status": a.Status}); err != nil {
		return err
	}
	if a.Status == string(assignment.StatusResultsReleased) {
		if err := h.publish(ctx, EventAssignmentResultsReleased, *a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID,
			"released_at": now, "released_by": nil, "trigger": "after_close"}); err != nil {
			return err
		}
		rep.Released++
	}
	rep.Closed++
	h.count(a.Status)
	return nil
}

// ---------- submissions from the attempt events

// AttemptEvent is an attempt event of an assignment (activity.started/submitted/graded, attempt.re_evaluated,
// exam.graded): the attempt and its student.
type AttemptEvent struct {
	Type         string
	AttemptID    uuid.UUID
	StudentID    uuid.UUID
	AssignmentID *uuid.UUID // nil: read from the attempt (attempt.re_evaluated)
}

// ApplyAttemptEvent updates the submission of the student from the attempts of the assignment (F041-AC-15), in the
// transaction and event scope of the consumer (processed_events: once per event): status, attempts used, the attempt
// kept by the score policy with its effective evaluation, late, rapid guesses; assignment.completed the first time
// the student completes, assignment.submission_updated on every change. Attempts outside an assignment are ignored.
func (h Assignments) ApplyAttemptEvent(ctx context.Context, e AttemptEvent) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ApplyAttemptEvent")
	defer func() { done(err) }()
	id := e.AssignmentID
	if id == nil {
		typ, ctxID, student, err := h.Store.AttemptContext(ctx, e.AttemptID)
		if err != nil {
			return err
		}
		if typ != "assignment" {
			return nil
		}
		id, e.StudentID = &ctxID, student
	}
	a, err := h.Store.Assignment(ctx, *id, false)
	if err != nil || a == nil {
		return err
	}
	s, err := h.Store.Submission(ctx, a.ID, e.StudentID, true)
	if err != nil || s == nil {
		return err // not targeted: nothing to track
	}
	prev := *s
	if err := h.recompute(ctx, *a, s); err != nil {
		return err
	}
	if sameSubmission(prev, *s) {
		return nil
	}
	if err := h.Store.UpsertSubmission(ctx, *s); err != nil {
		return err
	}
	if prev.CompletedAt == nil && s.CompletedAt != nil && s.BestAttemptID != nil {
		if err := h.publish(ctx, EventAssignmentCompleted, *a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID,
			"student_id": s.StudentID, "attempt_id": *s.BestAttemptID, "late": s.Status == "late"}); err != nil {
			return err
		}
	}
	return h.submissionUpdated(ctx, *a, *s)
}

func sameSubmission(a, b write.SubmissionRow) bool {
	eq := func(x, y *float64) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqID := func(x, y *uuid.UUID) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	return a.Status == b.Status && a.AttemptsUsed == b.AttemptsUsed && eqID(a.CurrentAttemptID, b.CurrentAttemptID) &&
		eqID(a.BestAttemptID, b.BestAttemptID) && eq(a.Score, b.Score) && eq(a.MaxScore, b.MaxScore) && a.RapidGuessCount == b.RapidGuessCount
}

// recompute derives a submission from the attempts of the assignment (idempotent).
func (h Assignments) recompute(ctx context.Context, a write.AssignmentRow, s *write.SubmissionRow) error {
	attempts, err := h.Store.AttemptsOfContext(ctx, a.ID, s.StudentID)
	if err != nil {
		return err
	}
	graded, err := h.Store.GradedAttempts(ctx, a.ID, s.StudentID)
	if err != nil {
		return err
	}
	s.AttemptsUsed, s.CurrentAttemptID = min(len(attempts), assignment.MaxAttempts), nil
	for _, x := range attempts {
		if x.Status == "in_progress" {
			id := x.AttemptID
			s.CurrentAttemptID = &id
			break
		}
	}
	scored := make([]assignment.Scored, len(graded))
	for i, g := range graded {
		scored[i] = assignment.Scored{AttemptID: g.AttemptID, Score: g.Score, Max: g.MaxScore, SubmittedAt: g.SubmittedAt}
	}
	if s.Status == "excused" {
		return nil
	}
	if pick, ok := assignment.Pick(a.ScorePolicy, scored); ok {
		id := uuid.UUID(pick.AttemptID)
		score, maxScore := pick.Score, pick.Max
		s.BestAttemptID, s.Score, s.MaxScore = &id, &score, &maxScore
		for _, g := range graded {
			if g.AttemptID == id {
				s.RapidGuessCount = g.RapidGuesses
			}
		}
		first := graded[0].SubmittedAt
		s.SubmittedAt = &first
		s.Status = "submitted"
		if window(a, s).IsLate(first) {
			s.Status = "late"
		}
		if s.CompletedAt == nil {
			at := h.now()
			s.CompletedAt = &at
		}
		return nil
	}
	switch {
	case s.CurrentAttemptID != nil:
		s.Status = "in_progress"
	case s.Status == "in_progress":
		s.Status = "not_started"
		if a.Status != string(assignment.StatusOpen) && a.Status != string(assignment.StatusScheduled) {
			s.Status = "missed"
		}
	}
	return nil
}

func (h Assignments) submissionUpdated(ctx context.Context, a write.AssignmentRow, s write.SubmissionRow) error {
	if h.Metrics != nil {
		h.Metrics.SubmissionUpdated(s.Status)
	}
	return h.publish(ctx, EventAssignmentSubmissionUpdated, a, map[string]any{"assignment_id": a.ID, "school_id": a.SchoolID,
		"student_id": s.StudentID, "status": s.Status, "attempts_used": s.AttemptsUsed, "best_attempt_id": s.BestAttemptID, "score": s.Score,
		"max_score": s.MaxScore, "rapid_guess_count": s.RapidGuessCount})
}

// ---------- placement changes (ADR-0095: the school and grade of a student)

// PlacementChange is the school and grade of a student after student.placement_changed (or student.synced of module
// identity, read back from ref.students): Active false when the student is no longer an
// active student of the school.
type PlacementChange struct {
	StudentID uuid.UUID
	SchoolID  uuid.UUID
	Grade     int
	Active    bool
}

// SyncPlacement adds a student who joins a grade of a school to the scheduled and open assignments targeting that grade
// (ADR-0095 mục 3, F041-AC-16): not when the target excludes the student; a student already targeted is left as is; a
// student who leaves the grade or the school keeps the submissions (excused by the school when needed).
func (h Assignments) SyncPlacement(ctx context.Context, e PlacementChange) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SyncAssignmentPlacement")
	defer func() { done(err) }()
	if !e.Active || e.Grade < 1 || e.Grade > 9 {
		return nil
	}
	as, err := h.Store.OpenGradeAssignments(ctx, e.SchoolID, e.Grade)
	if err != nil {
		return err
	}
	grade := e.Grade
	for _, open := range as {
		a, err := h.Store.Assignment(ctx, open.ID, false)
		if err != nil {
			return err
		}
		if a == nil || excluded(*a, grade, e.StudentID) {
			continue
		}
		n, err := h.Store.InsertTargetStudents(ctx, *a, []write.TargetStudent{{StudentID: e.StudentID, Grade: &grade, Source: "grade_added"}})
		if err != nil {
			return err
		}
		if n > 0 {
			s, err := h.Store.Submission(ctx, a.ID, e.StudentID, false)
			if err != nil || s == nil {
				return err
			}
			if err := h.submissionUpdated(ctx, *a, *s); err != nil {
				return err
			}
		}
	}
	return nil
}

// excluded reports whether the grade target of the grade excludes the student (exclude wins).
func excluded(a write.AssignmentRow, grade int, student uuid.UUID) bool {
	for _, t := range a.Targets {
		if t.Type == assignment.TargetGrade && t.Grade != nil && *t.Grade == grade && slices.Contains(t.Exclude, student) {
			return true
		}
	}
	return false
}

// SyncStudent re-applies the placement of a student read from ref.students (student.synced of module identity).
func (h Assignments) SyncStudent(ctx context.Context, studentID uuid.UUID) error {
	pl, err := h.Store.Placement(ctx, studentID)
	if err != nil || pl == nil {
		return err
	}
	return h.SyncPlacement(ctx, PlacementChange{StudentID: studentID, SchoolID: pl.SchoolID, Grade: pl.Grade, Active: true})
}

// AccessRevoked excuses a student whose account is locked before the due date of the open assignments (F041-AC-16).
func (h Assignments) AccessRevoked(ctx context.Context, studentID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.AssignmentAccessRevoked")
	defer func() { done(err) }()
	now := h.now()
	as, err := h.Store.OpenAssignmentsOfStudent(ctx, studentID)
	if err != nil {
		return err
	}
	for _, a := range as {
		s, err := h.Store.Submission(ctx, a.ID, studentID, true)
		if err != nil {
			return err
		}
		if s == nil || s.Status == "submitted" || s.Status == "late" || s.Status == "excused" || !now.Before(window(a, s).DueAt) {
			continue
		}
		s.Status = "excused"
		if err := h.Store.SetException(ctx, a.TenantID, a.SchoolID, a.ID, studentID, "excuse", nil, "account_locked", studentID); err != nil {
			return err
		}
		if err := h.Store.UpsertSubmission(ctx, *s); err != nil {
			return err
		}
		if err := h.submissionUpdated(ctx, a, *s); err != nil {
			return err
		}
	}
	return nil
}
