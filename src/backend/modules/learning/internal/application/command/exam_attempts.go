package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// deviceHash is the stored form of an exam device token (EXAM_ENGINE §12: the token itself is never stored).
func deviceHash(token *string) *string {
	if token == nil || *token == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(*token))
	h := hex.EncodeToString(sum[:])
	return &h
}

// sessionSections is the section list stored on a session (id, title, shuffle_items).
type sessionSection struct {
	SectionID    string `json:"section_id"`
	Title        string `json:"title"`
	ShuffleItems *bool  `json:"shuffle_items,omitempty"`
}

func sectionsOf(s write.ExamSessionRow) []sessionSection {
	var out []sessionSection
	_ = json.Unmarshal(s.Sections, &out)
	return out
}

func orderDefinition(s write.ExamSessionRow) blueprint.Definition {
	var d blueprint.Definition
	for _, sec := range sectionsOf(s) {
		d.Sections = append(d.Sections, blueprint.Section{ID: sec.SectionID, ShuffleItems: sec.ShuffleItems})
	}
	return d
}

// loadExamAttempt loads an exam attempt of the student of ctx (another student's or another mode → 404, F042-AC-17).
func (h Exams) loadExamAttempt(ctx context.Context, p authz.Principal, id uuid.UUID) (attempt.Attempt, error) {
	a, err := h.Att.Store.Attempt(ctx, id)
	if err != nil {
		return a, err
	}
	if a.StudentID != p.UserID || a.Mode != attempt.ModeExam {
		return attempt.Attempt{}, problem.New(problem.NotFound, "")
	}
	return a, nil
}

// checkDevice refuses a request from a device that no longer holds the attempt (F042-AC-14).
func (h Exams) checkDevice(ctx context.Context, attemptID uuid.UUID, token *string) error {
	info, err := h.Store.AttemptExam(ctx, attemptID)
	if err != nil {
		return err
	}
	if info.DeviceToken != nil {
		if hash := deviceHash(token); hash == nil || *hash != *info.DeviceToken {
			return problem.New(problem.ExamActiveOnOtherDevice, "")
		}
	}
	return nil
}

// StartExamInput is startExamAttempt.
type StartExamInput struct {
	SessionID      uuid.UUID
	DeviceToken    *string
	TakeOver       bool
	AcceptLanguage string
}

// StartExamAttempt starts or resumes the attempt of a student (F042-AC-09…AC-14, AC-27): a student of the audience in the
// window, attempts left; the variant hash(student, session) mod N (per_student: drawn now), items shuffled per section
// and options by the attempt seed, deadline_at on the server clock; an open attempt is returned with its saved answers
// and unchanged deadline, from the device holding it (take_over moves it, audited).
func (h Exams) StartExamAttempt(ctx context.Context, in StartExamInput) (out dto.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.StartExamAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if key != "" {
			if prev, err := h.Att.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
				if err == nil {
					out, err = h.attemptView(ctx, *prev)
				}
				return err
			}
		}
		s, err := h.Store.Session(ctx, in.SessionID, false)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		part, err := h.Store.Participation(ctx, s.ID, p.UserID, true)
		if err != nil {
			return err
		}
		if part == nil {
			return problem.New(problem.ExamNotAssigned, "")
		}
		now := h.now()
		if part.LastAttemptID != nil {
			a, err := h.Att.Store.Attempt(ctx, *part.LastAttemptID)
			if err != nil {
				return err
			}
			if a.Status == attempt.StatusInProgress {
				if err := h.resumeOn(ctx, s, a, in); err != nil {
					return err
				}
				if out, err = h.attemptView(ctx, a); err != nil {
					return err
				}
				return idempotency.SetResult(ctx, out)
			}
		}
		switch {
		case s.Status == string(exam.StatusCancelled) || !now.Before(s.WindowEnd) || s.Status == string(exam.StatusClosed) ||
			s.Status == string(exam.StatusReleased):
			return problem.New(problem.ExamWindowClosed, "")
		case now.Before(s.WindowStart):
			return problem.New(problem.ExamNotOpen, "")
		}
		if part.AttemptsUsed >= s.MaxAttempts {
			return problem.New(problem.ExamMaxAttempts, "")
		}
		if err := h.Att.checkStartRate(ctx, p.UserID); err != nil {
			return err
		}
		pp, err := h.paperOf(ctx, *s, p.UserID, part.AttemptsUsed+1)
		if err != nil {
			return err
		}
		seed := randomSeed()
		ordered := paper.Order(orderDefinition(*s), pp.Items, seed)
		qids := make([]uuid.UUID, len(ordered))
		for i, it := range ordered {
			qids[i] = it.QuestionVersionID
		}
		cctx, err := contentScope(ctx, s.EfficacyWaveID, s.TenantID)
		if err != nil {
			return err
		}
		items, rendered, err := h.Att.pinItems(cctx, qids, seed, false, nil)
		if err != nil {
			return err
		}
		for i := range items {
			items[i].MaxScore = ordered[i].Points
		}
		lang := "vi"
		if displayLanguage(in.AcceptLanguage, json.RawMessage(`{"vi":"x","en":"x"}`), "vi") == "en" {
			lang = "en"
		}
		a := attempt.New(attempt.Start{ID: ids.New(), TenantID: s.TenantID, SchoolID: s.SchoolID, StudentID: p.UserID, Mode: attempt.ModeExam,
			ContextType: examContextType, ContextID: s.ID, ActivityType: examActivityType, SourceKind: examSourceKind,
			SourceVersionID: s.BlueprintVersionID, Seed: seed, Now: now, ActivityFeedback: s.FeedbackTiming, DisplayLanguage: lang,
			IdempotencyKey: attemptKey(key), Items: items})
		deadline := exam.Deadline(now, s.WindowEnd, part.EffectiveDurationSec, s.LateStartPolicy)
		a.DeadlineAt = &deadline
		if err := h.Att.Store.InsertAttempt(ctx, a); err != nil {
			return err
		}
		if err := h.Store.SetAttemptExam(ctx, a.ID, pp.ID, deviceHash(in.DeviceToken)); err != nil {
			return err
		}
		part.Status, part.AttemptsUsed, part.LastAttemptID = exam.ParticipationInProgress, part.AttemptsUsed+1, &a.ID
		if err := h.Store.UpdateParticipation(ctx, *part); err != nil {
			return err
		}
		if err := h.Att.started(ctx, a); err != nil {
			return err
		}
		if err := h.publishExam(ctx, s.EfficacyWaveID, EventExamStarted, "attempt", a.ID, a.TenantID, map[string]any{"attempt_id": a.ID, "exam_session_id": s.ID,
			"school_id": s.SchoolID, "student_id": p.UserID, "exam_paper_id": pp.ID, "exam_type": s.Type, "is_official": s.IsOfficial,
			"assignment_id": nullableID(s.AssignmentID), "started_at": now, "deadline_at": deadline, "attempt_no": part.AttemptsUsed}); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.Started()
		}
		if out, err = h.Att.view(ctx, a, rendered, nil, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// attemptKey is idempotency_key of a start without Idempotency-Key (the column is NOT NULL and unique per student).
func attemptKey(key string) string {
	if key == "" {
		return "exam:" + ids.New().String()
	}
	return key
}

// resumeOn checks the device of an open attempt: the device holding it resumes; another device is 409 unless it takes
// over (audit exam_attempt.device_switched); a first token binds the attempt.
func (h Exams) resumeOn(ctx context.Context, s *write.ExamSessionRow, a attempt.Attempt, in StartExamInput) error {
	info, err := h.Store.AttemptExam(ctx, a.ID)
	if err != nil {
		return err
	}
	want := deviceHash(in.DeviceToken)
	switch {
	case info.DeviceToken == nil:
		if want != nil {
			return h.Store.SetAttemptDevice(ctx, a.ID, want)
		}
		return nil
	case want != nil && *want == *info.DeviceToken:
		return nil
	case !in.TakeOver:
		return problem.New(problem.ExamActiveOnOtherDevice, "")
	}
	if err := h.Store.SetAttemptDevice(ctx, a.ID, want); err != nil {
		return err
	}
	return h.Audit.Record(ctx, audit.Event{Action: "exam_attempt.device_switched", TargetType: "attempt", TargetID: a.ID.String(),
		Result: audit.ResultSuccess, SchoolID: s.SchoolID.String(), Metadata: map[string]any{"exam_session_id": s.ID}})
}

// paperOf is the paper of an attempt: the variant of the student, or a per_student paper drawn for this attempt number.
func (h Exams) paperOf(ctx context.Context, s write.ExamSessionRow, studentID uuid.UUID, attemptNo int) (write.ExamPaperRow, error) {
	if s.Strategy != exam.StrategyPerStudent {
		ps, err := h.Store.Papers(ctx, s.ID)
		if err != nil {
			return write.ExamPaperRow{}, err
		}
		if len(ps) == 0 {
			return write.ExamPaperRow{}, problem.New(problem.QuestionPoolInsufficient, "the session has no paper")
		}
		return ps[exam.Variant(studentID, s.ID, len(ps))-1], nil
	}
	if prev, err := h.Store.StudentPaper(ctx, s.ID, studentID, attemptNo); err != nil || prev != nil {
		if prev == nil {
			return write.ExamPaperRow{}, err
		}
		return *prev, err
	}
	b, err := h.loadBlueprint(ctx, s.BlueprintVersionID)
	if err != nil {
		return write.ExamPaperRow{}, err
	}
	pool, err := h.Att.Delivery.Pool(ctx, &s.SubjectID)
	if err != nil {
		return write.ExamPaperRow{}, err
	}
	seed := exam.PerStudentSeed(s.ID, studentID, attemptNo)
	pp, err := paper.Generate(b.Def, pool, seed, s.IsOfficial)
	if err != nil {
		return write.ExamPaperRow{}, problem.New(problem.QuestionPoolInsufficient, err.Error())
	}
	st, n := studentID, attemptNo
	row := write.ExamPaperRow{ID: ids.New(), TenantID: s.TenantID, SessionID: s.ID, VariantNo: 1, StudentID: &st, AttemptNo: &n, Seed: seed,
		Items: pp.Items, TotalPoints: pp.TotalPoints, GeneratorVersion: paper.GeneratorVersion}
	return row, h.Store.InsertPapers(ctx, []write.ExamPaperRow{row})
}

// attemptView renders an exam attempt with its saved answers (resume, F042-AC-13); never a result.
func (h Exams) attemptView(ctx context.Context, a attempt.Attempt) (dto.AttemptView, error) {
	wave, err := h.WaveOf(ctx, a.ContextID)
	if err != nil {
		return dto.AttemptView{}, err
	}
	if ctx, err = contentScope(ctx, wave, a.TenantID); err != nil {
		return dto.AttemptView{}, err
	}
	latest, err := h.Store.LatestAnswers(ctx, a.ID)
	if err != nil {
		return dto.AttemptView{}, err
	}
	saved := make([]dto.SavedResponseView, 0, len(latest))
	for _, it := range a.Items {
		if r, ok := latest[it.Position]; ok {
			saved = append(saved, dto.SavedResponseView{Position: it.Position, Response: r})
		}
	}
	return h.Att.view(ctx, a, nil, nil, saved, nil)
}

// SaveExamInput is saveExamAnswer.
type SaveExamInput struct {
	AttemptID        uuid.UUID
	Position         int
	Response         json.RawMessage
	AnsweredAtClient *time.Time
	ClientElapsedMS  *int64
	DeviceToken      *string
}

// SaveExamAnswer saves one answer (F042-AC-12, AC-15): append-only, the latest counts, graded at the submit; nothing
// about correctness is returned; after deadline_at + exam_offline_sync_grace (or answered after deadline_at) → 422
// EXAM_TIME_OVER; a submitted attempt → 409 EXAM_ALREADY_SUBMITTED; another device → 409 EXAM_ACTIVE_ON_OTHER_DEVICE.
func (h Exams) SaveExamAnswer(ctx context.Context, in SaveExamInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SaveExamAnswer")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return err
	}
	if len(in.Response) == 0 || string(in.Response) == "null" {
		return problem.New(problem.InvalidResponse, "the response is missing")
	}
	if err := h.Att.checkAnswerRate(ctx, RateAnswer, p.UserID, in.AttemptID); err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadExamAttempt(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		if a.Status != attempt.StatusInProgress {
			return h.rejected(problem.ExamAlreadySubmitted)
		}
		if err := h.checkDevice(ctx, a.ID, in.DeviceToken); err != nil {
			return err
		}
		now := h.now()
		if a.DeadlineAt != nil && !exam.AcceptsAnswer(*a.DeadlineAt, now, in.AnsweredAtClient, h.config().SyncGrace) {
			return h.rejected(problem.ExamTimeOver)
		}
		it, err := a.Item(in.Position)
		if err != nil {
			return problem.Invalid("/position", problem.SubInvalid, "no item at this position")
		}
		server := now.Sub(a.LastActivityAt).Milliseconds()
		if server < 0 {
			server = 0
		}
		row := a.RowVersion
		it.AnswerCount++
		a.LastActivityAt = now
		if err := h.Att.Store.UpdateItem(ctx, *it); err != nil {
			return err
		}
		if err := h.Att.Store.UpdateAttempt(ctx, a, row); err != nil {
			return err
		}
		return h.Att.Store.InsertAnswer(ctx, write.AnswerRow{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID, AttemptID: a.ID,
			Position: in.Position, QuestionVersionID: it.QuestionVersionID, Response: in.Response, AnsweredAt: now, ClientElapsedMS: in.ClientElapsedMS,
			ResponseMS: server, AttemptNo: it.AnswerCount, IsScored: true, IdempotencyKey: "exam:" + ids.New().String(), Ungraded: true})
	})
}

func (h Exams) rejected(code problem.Code) error {
	if h.Metrics != nil {
		h.Metrics.AnswerRejected(string(code))
	}
	return problem.New(code, "")
}

// SubmitExamInput is submitExamAttempt.
type SubmitExamInput struct {
	AttemptID   uuid.UUID
	DeviceToken *string
}

// SubmitExamAttempt submits an attempt once (F042-AC-18): conditional in_progress → submitted on the row version;
// another key after the submit (or after the auto-submit) is 409 EXAM_ALREADY_SUBMITTED, the same key replays the
// stored answer (pkg/httpx); the worker grades it (exam.submitted).
func (h Exams) SubmitExamAttempt(ctx context.Context, in SubmitExamInput) (out dto.ExamSubmitView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SubmitExamAttempt")
	defer func() { done(err) }()
	// MONITORING §3: every student submit with a known exam type by result, the duration of an accepted one.
	started, examType, late := time.Now(), "", false
	defer func() {
		r := submitResult(err)
		if h.Metrics == nil || examType == "" || r == "" {
			return
		}
		if r == SubmitOK && late {
			r = SubmitLate
		}
		h.Metrics.Submitted(examType, r)
		if err == nil {
			h.Metrics.SubmitDuration(examType, time.Since(started))
		}
	}()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadExamAttempt(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		if a.Status != attempt.StatusInProgress {
			if s, serr := h.Store.Session(ctx, a.ContextID, false); serr == nil && s != nil {
				examType = s.Type // the duplicate is counted under its exam type
			}
			return problem.New(problem.ExamAlreadySubmitted, "")
		}
		if err := h.checkDevice(ctx, a.ID, in.DeviceToken); err != nil {
			return err
		}
		reason := exam.ReasonManual
		if a.DeadlineAt != nil && h.now().After(a.DeadlineAt.Add(h.config().SyncGrace)) {
			reason = exam.ReasonAutoTimeout
		}
		late = reason == exam.ReasonAutoTimeout
		typ, ok, err := h.submitAttempt(ctx, a.ID, reason, &p.UserID)
		if typ != "" {
			examType = typ
		}
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.ExamAlreadySubmitted, "")
		}
		if a, err = h.Att.Store.Attempt(ctx, a.ID); err != nil {
			return err
		}
		out = dto.ExamSubmitView{AttemptID: a.ID, Status: string(a.Status)}
		if a.DurationMS != nil {
			out.DurationMS = *a.DurationMS
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// submitAttempt moves an in-progress (or expired) exam attempt to submitted in the transaction of ctx: submitted_at
// capped at deadline_at for a timeout, the participation submitted, exam.submitted for the grading worker. It reports
// false when the attempt was already submitted (a concurrent submit or the auto-submit won).
func (h Exams) submitAttempt(ctx context.Context, id uuid.UUID, reason string, _ *uuid.UUID) (string, bool, error) {
	a, err := h.Att.Store.Attempt(ctx, id)
	if err != nil {
		return "", false, err
	}
	if a.Status != attempt.StatusInProgress && a.Status != attempt.StatusExpired {
		return "", false, nil
	}
	now := h.now()
	end := now
	if reason != exam.ReasonManual && a.DeadlineAt != nil && a.DeadlineAt.Before(now) {
		end = *a.DeadlineAt
	}
	d := max(end.Sub(a.StartedAt).Milliseconds()-a.PausedMS, 0)
	row := a.RowVersion
	a.Status, a.SubmittedAt, a.SubmitReason, a.DurationMS, a.PausedAt, a.LastActivityAt = attempt.StatusSubmitted, &end, &reason, &d, nil, now
	if err := h.Att.Store.UpdateAttempt(ctx, a, row); err != nil {
		if pe, ok := problemOf(err); ok && pe.Code == problem.VersionMismatch {
			return "", false, nil
		}
		return "", false, err
	}
	part, err := h.Store.Participation(ctx, a.ContextID, a.StudentID, true)
	if err != nil {
		return "", false, err
	}
	if part != nil {
		part.Status = exam.ParticipationSubmitted
		if err := h.Store.UpdateParticipation(ctx, *part); err != nil {
			return "", false, err
		}
	}
	s, err := h.Store.Session(ctx, a.ContextID, false)
	if err != nil || s == nil {
		return "", false, err
	}
	latest, err := h.Store.LatestAnswers(ctx, a.ID)
	if err != nil {
		return "", false, err
	}
	if reason != exam.ReasonManual && h.Metrics != nil {
		h.Metrics.AutoSubmitted(reason)
	}
	if s.EfficacyWaveID != nil {
		// F035-AC-15: no exam.submitted (no outbox event at all): the attempt is graded in the submit transaction.
		return s.Type, true, h.gradeAttempt(ctx, a.ID)
	}
	return s.Type, true, h.publishExam(ctx, s.EfficacyWaveID, EventExamSubmitted, "attempt", a.ID, a.TenantID, map[string]any{"attempt_id": a.ID, "exam_session_id": s.ID,
		"school_id": s.SchoolID, "student_id": a.StudentID, "exam_type": s.Type, "is_official": s.IsOfficial,
		"assignment_id": nullableID(s.AssignmentID), "submit_reason": reason, "submitted_at": end, "answered_count": len(latest)})
}

// AutoSubmitReport counts what the job exam.autosubmit did.
type AutoSubmitReport struct{ Submitted int }

// AutoSubmitExpired is the job exam.autosubmit (F042-AC-16, EXAM_ENGINE §7.2, every exam_autosubmit_interval): the
// in-progress exam attempts past deadline_at + exam_offline_sync_grace are submitted with auto_timeout (rows locked
// FOR UPDATE SKIP LOCKED, each attempt once).
func (h Exams) AutoSubmitExpired(ctx context.Context, limit int) (rep AutoSubmitReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ExamAutoSubmit")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = 200
	}
	cutoff := h.now().Add(-h.config().SyncGrace)
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return rep, err
	}
	var due []write.ExamAttemptRef
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		due, err = h.Store.ExpiredAttempts(ctx, cutoff, limit)
		return err
	}); err != nil {
		return rep, err
	}
	for _, d := range due {
		var typ string
		var ok bool
		if err := h.Asg.inTenant(ctx, d.TenantID, func(ctx context.Context) error {
			var err error
			typ, ok, err = h.submitAttempt(ctx, d.AttemptID, exam.ReasonAutoTimeout, nil)
			return err
		}); err != nil {
			return rep, err
		}
		if ok {
			rep.Submitted++
			if h.Metrics != nil {
				h.Metrics.Submitted(typ, ExamAutoSubmitted)
			}
		}
	}
	return rep, nil
}

// GradeExamAttempt grades a submitted exam attempt (worker exam.grade, F042-AC-19, AC-25): every item from its latest
// saved answer with the server grader on the pinned version, points of the paper, the effective evaluation, graded;
// learning events and activity.* for mastery, exam.graded; a 100 % attempt faster than suspicious_speed_flag gets an
// integrity signal (never a change of the result). A graded attempt is left as it is (redelivery).
func (h Exams) GradeExamAttempt(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.GradeExamAttempt")
	defer func() { done(err) }()
	return h.gradeAttempt(ctx, id)
}

// gradeAttempt is GradeExamAttempt in the transaction of ctx.
func (h Exams) gradeAttempt(ctx context.Context, id uuid.UUID) error {
	a, err := h.Att.Store.Attempt(ctx, id)
	if err != nil {
		return err
	}
	if a.Mode != attempt.ModeExam || a.Status != attempt.StatusSubmitted {
		return nil
	}
	s, err := h.Store.Session(ctx, a.ContextID, false)
	if err != nil || s == nil {
		return err
	}
	latest, err := h.Store.LatestAnswers(ctx, a.ID)
	if err != nil {
		return err
	}
	now := h.now()
	times, err := h.Store.LatestResponseMS(ctx, a.ID)
	if err != nil {
		return err
	}
	gctx, err := contentScope(ctx, s.EfficacyWaveID, s.TenantID)
	if err != nil {
		return err
	}
	for i := range a.Items {
		it := &a.Items[i]
		c, sc := false, 0.0
		if resp, ok := latest[it.Position]; ok {
			var pres question.Presentation
			_ = json.Unmarshal(it.Presentation, &pres)
			g, err := h.Att.Delivery.Grade(gctx, it.QuestionVersionID, pres, resp)
			if err != nil {
				if pe, ok := problemOf(err); !ok || pe.Code != problem.InvalidResponse {
					return err
				}
			} else if g.MaxScore > 0 {
				c, sc = g.IsCorrect, activity.Round2(min(max(g.Score, 0), g.MaxScore)/g.MaxScore*it.MaxScore)
			}
			if err := h.Att.learningEvent(ctx, a, EventQuestionAnswered, "qa:"+a.ID.String()+":"+itoa(it.Position), now, it.ObjectiveIDs, "system",
				map[string]any{"attempt_id": a.ID, "position": it.Position, "question_version_id": it.QuestionVersionID, "question_type": it.QuestionType,
					"is_correct": c, "is_scored": true, "score": learningScore(sc, it.MaxScore), "response_ms": times[it.Position], "attempt_no": max(it.AnswerCount, 1),
					"hints_used": 0, "rapid_guess": false}); err != nil {
				return err
			}
		}
		it.FinalIsCorrect, it.FinalScore = &c, &sc
		if err := h.Att.Store.UpdateItem(ctx, *it); err != nil {
			return err
		}
	}
	res := examResult(a)
	row := a.RowVersion
	a.Score, a.MaxScore, a.GradedAt, a.Status = &res.Score, &res.MaxScore, &now, attempt.StatusGraded
	if err := h.Att.Store.UpdateAttempt(ctx, a, row); err != nil {
		if pe, ok := problemOf(err); ok && pe.Code == problem.VersionMismatch {
			return nil
		}
		return err
	}
	evID := ids.New()
	details, _ := json.Marshal(map[string]any{"items": a.ItemResults(), "strategies": res.Strategies})
	if err := h.Att.Store.InsertEvaluation(ctx, write.EvaluationRow{ID: evID, AttemptID: a.ID, Source: "system", Status: "final", Score: res.Score,
		MaxScore: res.MaxScore, Details: details, Effective: true}); err != nil {
		return err
	}
	// The activity.* events (mastery, progress) use the F030 vocabulary: auto_timeout and window_closed are a timeout.
	ga := a
	if *a.SubmitReason != exam.ReasonManual {
		r := attempt.ReasonTimeout
		ga.SubmitReason = &r
	}
	if err := h.Att.graded(ctx, ga, res, evID, nil); err != nil {
		return err
	}
	part, err := h.Store.Participation(ctx, s.ID, a.StudentID, true)
	if err != nil {
		return err
	}
	if part != nil && part.LastAttemptID != nil && *part.LastAttemptID == a.ID {
		part.Status = exam.ParticipationGraded
		if err := h.Store.UpdateParticipation(ctx, *part); err != nil {
			return err
		}
	}
	suspicious, err := h.speedSignal(ctx, a, res)
	if err != nil {
		return err
	}
	visible := exam.ResultsVisible(s.ResultRelease, exam.Status(s.Status))
	var visibleAt *time.Time
	if visible {
		visibleAt = &now
	}
	return h.publishExam(ctx, s.EfficacyWaveID, EventExamGraded, "attempt", a.ID, a.TenantID, map[string]any{"attempt_id": a.ID, "evaluation_id": evID,
		"exam_session_id": s.ID, "school_id": s.SchoolID, "student_id": a.StudentID, "exam_type": s.Type, "is_official": s.IsOfficial,
		"assignment_id": nullableID(s.AssignmentID), "score": res.Score, "max_score": res.MaxScore, "correct_count": res.CorrectCount,
		"item_count": len(a.Items), "result_release": s.ResultRelease, "results_visible_at": visibleAt, "submit_reason": *a.SubmitReason,
		"suspicious_speed": suspicious, "objective_ids": nonNilUUIDs(a.ObjectiveIDs)})
}

// examResult sums the item results (EXAM_ENGINE §8: unanswered = 0, points of the paper).
func examResult(a attempt.Attempt) activity.Result {
	r := activity.Result{Strategies: []string{"auto"}}
	for _, it := range a.Items {
		r.MaxScore += it.MaxScore
		if it.FinalScore != nil {
			r.Score += *it.FinalScore
		}
		if it.FinalIsCorrect != nil && *it.FinalIsCorrect {
			r.CorrectCount++
		}
	}
	r.Score, r.MaxScore = activity.Round2(r.Score), activity.Round2(r.MaxScore)
	return r
}

// speedSignal records suspicious_speed (F042-AC-25, ANTI_CHEAT): every item of an attempt of at least 10 items correct
// in less than suspicious_speed_flag (1.5 s) per item on average. The result is never changed.
func (h Exams) speedSignal(ctx context.Context, a attempt.Attempt, r activity.Result) (bool, error) {
	if len(a.Items) < suspiciousSpeedMinItems || r.CorrectCount != len(a.Items) || a.DurationMS == nil {
		return false, nil
	}
	limit := h.config().SuspiciousSpeedSec * 1000 * float64(len(a.Items))
	if float64(*a.DurationMS) >= limit || h.Att.Progress == nil {
		return float64(*a.DurationMS) < limit, nil
	}
	ev, _ := json.Marshal(map[string]any{"duration_ms": *a.DurationMS, "item_count": len(a.Items), "threshold_ms": int64(limit)})
	return true, h.Att.Progress.InsertIntegritySignal(ctx, write.IntegritySignal{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID,
		StudentID: a.StudentID, AttemptID: a.ID, SignalType: "suspicious_speed", Score: float64(*a.DurationMS), Evidence: ev, CreatedAt: h.now()})
}
