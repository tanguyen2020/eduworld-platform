package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// publishExam publishes an exam.* event, except for a session of an efficacy wave (F035-AC-15, research R1: mastery,
// rollups, XP, quests and notifications consume exam.*; the efficacy session is blocked at the source).
func (h Exams) publishExam(ctx context.Context, wave *uuid.UUID, typ, subjectType string, subject, tenant uuid.UUID, payload any) error {
	if wave != nil {
		return nil
	}
	return h.publish(ctx, typ, subjectType, subject, tenant, payload)
}

// WaveOf is the efficacy wave of a session of the session tenant (nil for an ordinary session); Attempts.EfficacyWave.
func (h Exams) WaveOf(ctx context.Context, sessionID uuid.UUID) (*uuid.UUID, error) {
	s, err := h.Store.Session(ctx, sessionID, false)
	if err != nil || s == nil {
		return nil, err
	}
	return s.EfficacyWaveID, nil
}

// EfficacySessionInput is what F035 needs to create the exam session of one class in an efficacy wave.
type EfficacySessionInput struct {
	WaveID      uuid.UUID
	TenantID    uuid.UUID
	SchoolID    uuid.UUID
	ClassID     uuid.UUID
	Title       string
	Form        write.FormVersion // the platform blueprint version of the form
	Pool        []selection.Candidate
	WindowStart time.Time
	WindowEnd   time.Time
	DurationSec int // 0 = the duration of the blueprint
	Students    []uuid.UUID
	CreatedBy   uuid.UUID
}

// ErrEfficacyPool: the pool of the form does not hold every fixed question (an instrument unpublished meanwhile).
var ErrEfficacyPool = errors.New("efficacy: the form questions are not all published")

// CreateEfficacySession creates the exam session of a class in an efficacy wave (F035-AC-12) in the transaction of
// ctx, whose session tenant is the tenant of the class: the fixed settings of LEARNING_EFFICACY_PLAN §3.2 (custom, not
// official, feedback never, manual release never done, no answers nor explanations, one attempt), the fixed paper of
// the form, the students given, efficacy_wave_id set, no exam.* event, audit exam_session.created.
func (h Exams) CreateEfficacySession(ctx context.Context, in EfficacySessionInput) (write.ExamSessionRow, error) {
	var row write.ExamSessionRow
	var body blueprintBody
	if err := json.Unmarshal(in.Form.Body, &body); err != nil {
		return row, problem.Invalid("/form", problem.SubInvalid, "the form cannot be read")
	}
	def, err := blueprint.Parse(body.Definition)
	if err != nil {
		return row, problem.Invalid("/form", problem.SubInvalid, "the form definition cannot be read")
	}
	if !body.Efficacy || in.Form.SubjectID == nil {
		return row, problem.Invalid("/form", problem.SubInvalid, "the form is not an efficacy instrument")
	}
	b := loadedBlueprint{ItemID: in.Form.ItemID, VersionID: in.Form.VersionID, SubjectID: *in.Form.SubjectID, Body: body, Def: def}
	if len(in.Form.GradeLevels) > 0 {
		g := in.Form.GradeLevels[0]
		b.Grade = &g
	}
	f, r, never, one := false, exam.ReleaseManual, exam.FeedbackNever, 1
	no := false
	typ := string(exam.TypeCustom)
	s := h.settings(ExamSessionInput{Title: in.Title, WindowStart: in.WindowStart, WindowEnd: in.WindowEnd, Type: &typ, IsOfficial: &f,
		FeedbackTiming: &never, ResultRelease: &r, MaxAttempts: &one, ShowCorrectAnswers: &no, ShowExplanations: &no}, b)
	if in.DurationSec > 0 {
		s.DurationSec = in.DurationSec
	}
	s.Strategy, s.VariantCount = exam.StrategyFixed, 1
	if err := examViolations(s.Validate()); err != nil {
		return row, err
	}
	if err := h.checkMatrix(b, s.FeedbackTiming, in.Pool); err != nil {
		return row, err
	}
	ps, err := paper.SessionPapers(b.Def, in.Pool, s.Strategy, s.VariantCount, randomSeed(), s.IsOfficial, nil)
	if err != nil {
		var pe *paper.PoolError
		if errors.As(err, &pe) {
			return row, ErrEfficacyPool
		}
		return row, err
	}
	now := h.now()
	status := exam.InitialStatus(s.WindowStart, now)
	target, _ := json.Marshal(map[string]any{"targets": []map[string]any{{"target_type": "class", "target_id": in.ClassID}},
		"student_count": len(in.Students)})
	sections := make([]map[string]any, 0, len(b.Def.Sections))
	for _, sec := range b.Def.Sections {
		sections = append(sections, map[string]any{"section_id": sec.ID, "title": sectionTitle(sec.Title), "shuffle_items": sec.ShuffleItems})
	}
	sj, _ := json.Marshal(sections)
	wave := in.WaveID
	row = write.ExamSessionRow{ID: ids.New(), TenantID: in.TenantID, SchoolID: in.SchoolID, SubjectID: b.SubjectID, GradeLevel: b.Grade,
		Title: s.Title, Type: string(s.Type), IsOfficial: false, BlueprintID: b.ItemID, BlueprintVersionID: b.VersionID, WindowStart: s.WindowStart,
		WindowEnd: s.WindowEnd, DurationSec: s.DurationSec, Target: target, Sections: sj, Audience: exam.AudienceAssigned, Strategy: s.Strategy,
		VariantCount: 1, FeedbackTiming: s.FeedbackTiming, ResultRelease: s.ResultRelease, ShowCorrect: false, ShowExplanations: false,
		LateStartPolicy: s.LateStartPolicy, Accommodations: json.RawMessage(`[]`), MaxAttempts: 1, ScorePolicy: s.ScorePolicy,
		Status: string(status), CreatedBy: in.CreatedBy, CreatedByRole: "system_admin", Version: 1, CreatedAt: now, EfficacyWaveID: &wave}
	if len(ps) > 0 {
		row.QuestionCount, row.TotalPoints = len(ps[0].Items), ps[0].TotalPoints
	}
	if status == exam.StatusOpen {
		row.OpenedAt = &now
	}
	if err := h.Store.InsertSession(ctx, row); err != nil {
		return row, err
	}
	rows := make([]write.ExamPaperRow, len(ps))
	for i, pp := range ps {
		rows[i] = write.ExamPaperRow{ID: ids.New(), TenantID: row.TenantID, SessionID: row.ID, VariantNo: pp.VariantNo, Seed: pp.Seed, Items: pp.Items,
			TotalPoints: pp.TotalPoints, ExpectedP: pp.ExpectedP, BalanceWarning: pp.BalanceWarning, GeneratorVersion: paper.GeneratorVersion}
	}
	if err := h.Store.InsertPapers(ctx, rows); err != nil {
		return row, err
	}
	parts := make([]write.ExamParticipationRow, len(in.Students))
	for i, x := range in.Students {
		parts[i] = write.ExamParticipationRow{SessionID: row.ID, StudentID: x, TenantID: row.TenantID, SchoolID: row.SchoolID,
			EffectiveDurationSec: s.DurationSec}
	}
	if err := h.Store.InsertParticipations(ctx, parts); err != nil {
		return row, err
	}
	return row, h.audit(ctx, "exam_session.created", row, "", map[string]any{"blueprint_version_id": b.VersionID, "exam_type": row.Type,
		"student_count": len(in.Students), "variant_count": len(ps), "efficacy_wave_id": wave})
}

// efficacyLocked refuses a staff operation on a session of an efficacy wave (F035-AC-16, LEARNING_EFFICACY_PLAN §8):
// papers, report and exam-session regrade are never available to tenant roles.
func efficacyLocked(s *write.ExamSessionRow) error {
	if s != nil && s.EfficacyWaveID != nil {
		return problem.New(problem.PermissionDenied, "an efficacy exam session is managed by the efficacy study")
	}
	return nil
}

// contentScope is the context reading the platform efficacy instruments of an efficacy session (F035): those question
// versions are never assigned to a tenant, so the tenant-scoped reads (delivery, grading) lift the hybrid filter under
// the audited bypass efficacy_study; RLS still limits learning_app to the tenant rows plus the published platform efficacy
// instruments (migration F035). Any other session reads with ctx unchanged.
func contentScope(ctx context.Context, wave *uuid.UUID, tenant uuid.UUID) (context.Context, error) {
	if wave == nil {
		return ctx, nil
	}
	return tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonEfficacyStudy, tenancy.WithTenantIDs(tenant))
}

// platformInstrument reports a platform blueprint version flagged efficacy_instrument (read under the platform scope).
func (h Exams) platformInstrument(ctx context.Context, versionID uuid.UUID) (bool, error) {
	if h.Instruments == nil || h.Asg.Read == nil {
		return false, nil
	}
	ps, err := authz.SystemPlatformScope("learning-reference-check")
	if err != nil {
		return false, err
	}
	eff := false
	err = h.Asg.Read.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
		f, err := h.Instruments.FormVersion(ctx, versionID)
		if err != nil || f == nil {
			return err
		}
		var b blueprintBody
		_ = json.Unmarshal(f.Body, &b)
		eff = b.Efficacy
		return nil
	})
	return eff, err
}
