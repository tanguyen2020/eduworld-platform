package command

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/assignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Domain events of the exam engine (EVENT_DESIGN §4.3; F042 owns every exam.* payload schema).
const (
	EventExamOpened           = "exam.opened"
	EventExamClosed           = "exam.closed"
	EventExamStarted          = "exam.started"
	EventExamSubmitted        = "exam.submitted"
	EventExamGraded           = "exam.graded"
	EventExamResultsReleased  = "exam.results_released"
	EventExamRegraded         = "exam.regraded"
	examSourceKind            = "exam_paper"
	examActivityType          = "quiz"
	examContextType           = "exam_session"
	defaultExamScoreScale     = 10.0
	defaultExamSyncGrace      = 30 * time.Second
	defaultSuspiciousSpeedSec = 1.5
	suspiciousSpeedMinItems   = 10
)

// ExamConfig holds the parameters of CONFIG_DEFAULTS read by the exam engine (owner EXAM_ENGINE, ANTI_CHEAT).
type ExamConfig struct {
	// SyncGrace is exam_offline_sync_grace (30 s [Confirmed]).
	SyncGrace time.Duration
	// ScoreScale is score_scale (10).
	ScoreScale float64
	// SuspiciousSpeedSec is suspicious_speed_flag (CONFIG_DEFAULTS: < 1.5 s per question on ≥ 10 questions).
	SuspiciousSpeedSec float64
	// GradedFeedback / GradedRelease are graded_assignment_release_default (after_close / after_close).
	GradedFeedback string
	GradedRelease  string
}

// ExamMetrics counts eduworld_exam_attempts_started_total, eduworld_exam_autosubmit_total{reason},
// eduworld_exam_answer_rejected_total{code}, eduworld_exam_regrade_attempts_total{action} and (MONITORING §3)
// eduworld_exam_submissions_total{exam_type,result}, eduworld_exam_submit_duration_seconds{exam_type}; nil = not counted.
type ExamMetrics interface {
	Started()
	AutoSubmitted(reason string)
	// Submitted counts one exam submission: ok, late, duplicate, rejected (student) or auto_submitted (job exam.autosubmit).
	Submitted(examType, result string)
	// SubmitDuration observes one accepted student submit (submitExamAttempt on the server).
	SubmitDuration(examType string, d time.Duration)
	AnswerRejected(code string)
	Regraded(action string)
}

// Exams holds the write use cases of the exam engine (F042): sessions created from a published blueprint with locked
// papers, cancel and release, the window job, the attempts of the students (start, save, submit), the grading worker,
// the auto-submit job and the regrade of a session. Staff use cases authorize explicitly with AuthorizeLMS (R7).
type Exams struct {
	UoW      write.UnitOfWork
	Store    write.ExamStore
	Asg      Assignments // school, targets and their snapshot, blueprint visibility (F041 stores)
	Att      Attempts    // the attempt engine: items, render, grading, learning events
	Events   write.EventPublisher
	Audit    write.Auditor
	Clock    func() time.Time
	Config   ExamConfig
	Balancer paper.VariantBalancer
	// Instruments reads the platform efficacy forms (F035-AC-04: a form is refused with 422, never "not assigned").
	Instruments write.EfficacyContent
	// PValues reads the measured p_value of the pool for the balancer (F044 item analysis); nil = p̂ from the difficulty.
	PValues write.QuestionPValues
	Metrics ExamMetrics
}

func (h Exams) now() time.Time { return now(h.Clock).UTC() }

func (h Exams) config() ExamConfig {
	c := h.Config
	if c.SyncGrace <= 0 {
		c.SyncGrace = defaultExamSyncGrace
	}
	if c.ScoreScale <= 0 {
		c.ScoreScale = defaultExamScoreScale
	}
	if c.SuspiciousSpeedSec <= 0 {
		c.SuspiciousSpeedSec = defaultSuspiciousSpeedSec
	}
	if c.GradedFeedback == "" {
		c.GradedFeedback = exam.FeedbackAfterClose
	}
	if c.GradedRelease == "" {
		c.GradedRelease = exam.ReleaseAfterClose
	}
	return c
}

func (h Exams) audit(ctx context.Context, action string, s write.ExamSessionRow, reason string, meta map[string]any) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "exam_session", TargetID: s.ID.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: meta, SchoolID: s.SchoolID.String()})
}

func (h Exams) publish(ctx context.Context, typ, subjectType string, subject uuid.UUID, tenant uuid.UUID, payload any) error {
	if h.Events == nil {
		return nil
	}
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: subjectType, SubjectID: subject.String(), TenantID: &t, Payload: payload})
}

// ExamSessionInput is ExamSessionCreate.
type ExamSessionInput struct {
	Title              string     `json:"title"`
	BlueprintVersionID uuid.UUID  `json:"assessment_blueprint_version_id"`
	SchoolID           *uuid.UUID `json:"school_id"`
	// SubjectID is the subject of the assignment of the session (not a request field).
	SubjectID          *uuid.UUID              `json:"-"`
	AssignmentID       *uuid.UUID              `json:"assignment_id"`
	Type               *string                 `json:"type"`
	IsOfficial         *bool                   `json:"is_official"`
	Targets            []AssignmentTargetInput `json:"targets"`
	WindowStart        time.Time               `json:"window_start"`
	WindowEnd          time.Time               `json:"window_end"`
	MaxAttempts        *int                    `json:"max_attempts"`
	FeedbackTiming     *string                 `json:"feedback_timing"`
	ResultRelease      *string                 `json:"result_release"`
	DurationSeconds    *int                    `json:"duration_seconds"`
	LateStartPolicy    *string                 `json:"late_start_policy"`
	Audience           *string                 `json:"audience"`
	ShowCorrectAnswers *bool                   `json:"show_correct_answers"`
	ShowExplanations   *bool                   `json:"show_explanations"`
	ScorePolicy        *string                 `json:"score_policy"`
	Accommodations     []exam.Accommodation    `json:"accommodations"`
}

// blueprintBody is the stored body of a blueprint version (F024 docs: definition and the exam fields).
type blueprintBody struct {
	Definition     json.RawMessage `json:"definition"`
	ExamType       string          `json:"exam_type"`
	FeedbackTiming string          `json:"feedback_timing"`
	ResultRelease  string          `json:"result_release"`
	Efficacy       bool            `json:"efficacy_instrument"`
}

// loadedBlueprint is a published blueprint version visible to the tenant.
type loadedBlueprint struct {
	ItemID    uuid.UUID
	VersionID uuid.UUID
	SubjectID uuid.UUID
	Grade     *int
	Body      blueprintBody
	Def       blueprint.Definition
}

// loadBlueprint reads a published blueprint version visible to the tenant (F042-AC-02): unknown → 422, a platform
// version not assigned → 403 RESOURCE_NOT_ASSIGNED, not published → 409 CONTENT_NOT_PUBLISHED, an efficacy instrument
// → 422 (F035 only).
func (h Exams) loadBlueprint(ctx context.Context, versionID uuid.UUID) (loadedBlueprint, error) {
	var out loadedBlueprint
	find := func(ctx context.Context) (*write.VersionAccess, error) {
		return h.Asg.Store.ContentVersion(ctx, string(contentmodel.TypeAssessmentBlueprint), versionID)
	}
	v, err := find(ctx)
	if err != nil {
		return out, err
	}
	if v == nil {
		if ok, err := h.Asg.platformHas(ctx, find); err != nil || ok {
			if err == nil {
				if eff, e := h.platformInstrument(ctx, versionID); e != nil || eff {
					if e == nil {
						e = problem.Invalid("/assessment_blueprint_version_id", problem.SubInvalid, "efficacy instruments are scheduled by F035 only")
					}
					return out, e
				}
				err = problem.New(problem.ResourceNotAssigned, "the platform blueprint is not assigned to the tenant")
			}
			return out, err
		}
		return out, problem.Invalid("/assessment_blueprint_version_id", problem.SubInvalid, "is not a blueprint version visible to the tenant")
	}
	if v.Status != "published" {
		return out, problem.New(problem.ContentNotPublished, "the blueprint version is not published")
	}
	vs, err := h.Att.Docs.DocVersions(ctx, contentmodel.TypeAssessmentBlueprint, v.ItemID)
	if err != nil {
		return out, err
	}
	for _, dv := range vs {
		if dv.ID != versionID {
			continue
		}
		out = loadedBlueprint{ItemID: v.ItemID, VersionID: versionID}
		if dv.SubjectID == nil {
			return out, problem.Invalid("/assessment_blueprint_version_id", problem.SubInvalid, "the blueprint has no subject")
		}
		out.SubjectID = *dv.SubjectID
		if len(dv.GradeLevels) > 0 {
			g := dv.GradeLevels[0]
			out.Grade = &g
		}
		_ = json.Unmarshal(dv.Body, &out.Body)
		if out.Body.Efficacy {
			return out, problem.Invalid("/assessment_blueprint_version_id", problem.SubInvalid, "efficacy instruments are scheduled by F035 only")
		}
		if out.Def, err = blueprint.Parse(out.Body.Definition); err != nil {
			return out, problem.Invalid("/assessment_blueprint_version_id", problem.SubInvalid, "the blueprint definition cannot be read")
		}
		return out, nil
	}
	return out, problem.New(problem.ContentNotPublished, "the blueprint version is not published")
}

// settings resolves the request over the blueprint and the defaults of the exam type (EXAM_ENGINE §2, §3, §5).
func (h Exams) settings(in ExamSessionInput, b loadedBlueprint) exam.Settings {
	typ := exam.Type(b.Body.ExamType)
	if in.Type != nil {
		typ = exam.Type(*in.Type)
	}
	if typ == "" {
		typ = exam.TypeCustom
	}
	d := exam.DefaultsOf(typ)
	s := exam.Settings{Title: in.Title, Type: typ, IsOfficial: d.IsOfficial, WindowStart: in.WindowStart.UTC(), WindowEnd: in.WindowEnd.UTC(),
		DurationSec: b.Def.DurationSec, MaxAttempts: d.MaxAttempts, FeedbackTiming: d.FeedbackTiming, ResultRelease: d.ResultRelease,
		LateStartPolicy: exam.LateUntilWindowEnd, Audience: exam.AudienceAssigned, Strategy: exam.StrategyFixed, VariantCount: 1,
		ScorePolicy: "last", ShowCorrectAnswers: true, ShowExplanations: true, Accommodations: in.Accommodations}
	if v := b.Def.Variants; v != nil && v.Strategy != "" {
		s.Strategy = v.Strategy
		if v.Count > 0 && v.Strategy == exam.StrategyIndependentDraw {
			s.VariantCount = v.Count
		}
	}
	if rr := b.Def.ResultRelease; rr != nil {
		if rr.Mode != "" {
			s.ResultRelease = rr.Mode
		}
		if rr.FeedbackTiming != "" {
			s.FeedbackTiming = rr.FeedbackTiming
		}
		if rr.ShowCorrectAnswers != nil {
			s.ShowCorrectAnswers = *rr.ShowCorrectAnswers
		}
		if rr.ShowExplanations != nil {
			s.ShowExplanations = *rr.ShowExplanations
		}
	}
	if b.Body.ResultRelease != "" {
		s.ResultRelease = b.Body.ResultRelease
	}
	if b.Body.FeedbackTiming != "" {
		s.FeedbackTiming = b.Body.FeedbackTiming
	}
	if ap := b.Def.AttemptPolicy; ap != nil {
		if ap.MaxAttempts > 0 {
			s.MaxAttempts = min(ap.MaxAttempts, exam.MaxAttemptsLimit)
		}
		if ap.ScorePolicy != "" {
			s.ScorePolicy = ap.ScorePolicy
		}
	}
	if s.Strategy == exam.StrategyPerStudent {
		// F042-AC-27: a per_student paper is practice, released at the submit.
		s.ResultRelease, s.FeedbackTiming = exam.ReleaseImmediate, exam.FeedbackAfterSubmit
	}
	if in.IsOfficial != nil {
		s.IsOfficial = *in.IsOfficial
	}
	if in.MaxAttempts != nil {
		s.MaxAttempts = *in.MaxAttempts
	}
	if in.FeedbackTiming != nil {
		s.FeedbackTiming = *in.FeedbackTiming
	}
	if in.ResultRelease != nil {
		s.ResultRelease = *in.ResultRelease
	}
	if in.DurationSeconds != nil {
		s.DurationSec = *in.DurationSeconds
	}
	if in.LateStartPolicy != nil {
		s.LateStartPolicy = *in.LateStartPolicy
	}
	if in.Audience != nil {
		s.Audience = *in.Audience
	}
	if in.ShowCorrectAnswers != nil {
		s.ShowCorrectAnswers = *in.ShowCorrectAnswers
	}
	if in.ShowExplanations != nil {
		s.ShowExplanations = *in.ShowExplanations
	}
	if in.ScorePolicy != nil {
		s.ScorePolicy = *in.ScorePolicy
	}
	return s
}

// examViolations turns violations into one 422 VALIDATION_FAILED naming every field.
func examViolations(vs []exam.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	pe := problem.Invalid(vs[0].Field, problem.SubInvalid, vs[0].Reason)
	for _, v := range vs[1:] {
		pe.AddField(v.Field, problem.SubInvalid, v.Reason)
	}
	return pe
}

// checkMatrix applies EXAM_ENGINE §3 at creation (F042-AC-03): the blueprint consistency (total points, no immediate
// feedback, fixed questions), automatically graded types only and a pool large enough for every rule (× variants).
func (h Exams) checkMatrix(b loadedBlueprint, feedback string, pool []selection.Candidate) error {
	var vs []exam.Violation
	for _, v := range b.Def.Validate(feedback) {
		vs = append(vs, exam.Violation{Field: v.Field, Reason: v.Reason})
	}
	section := map[string]int{}
	for si, s := range b.Def.Sections {
		section[s.ID] = si
		for ri, r := range s.Rules {
			for _, t := range r.Types {
				if !question.Type(t).Known() {
					vs = append(vs, exam.Violation{Field: rulePointer(si, ri) + "/types", Reason: t + " is not graded automatically"})
				}
			}
		}
	}
	for _, st := range b.Def.Stats(pool) {
		if st.Insufficient {
			vs = append(vs, exam.Violation{Field: rulePointer(section[st.SectionID], st.RuleIndex),
				Reason: "the pool has " + strconv.Itoa(st.Available) + " questions, " + strconv.Itoa(st.Required) + " are needed"})
		}
	}
	return examViolations(vs)
}

func rulePointer(section, rule int) string {
	return "/definition/sections/" + strconv.Itoa(section) + "/rules/" + strconv.Itoa(rule)
}

// sessionSchool resolves the school of a new session (ADR-0095: school_id directly, no course) and the subject an
// assignment requires of the blueprint (nil for a session created from createExamSession).
func (h Exams) sessionSchool(ctx context.Context, in ExamSessionInput) (uuid.UUID, *uuid.UUID, error) {
	if in.SchoolID == nil || *in.SchoolID == uuid.Nil {
		return uuid.Nil, nil, problem.Invalid("/school_id", problem.SubRequired, "is required")
	}
	return *in.SchoolID, in.SubjectID, nil
}

// newSession is what createSession needs besides the request: the principal and an optional fixed audience (the
// students of an assignment).
type newSession struct {
	P        authz.Principal
	Students []uuid.UUID
	Balance  *dto.ExamBalanceView // filled with the balancer result when not nil (F044)
}

// createSession creates a session in the transaction of ctx (F042-AC-01…AC-08, AC-31): authorization on the school
// and subject, the audience snapshot as participations with their effective duration, the papers generated and locked,
// audit exam_session.created and exam.opened when the window has started.
func (h Exams) createSession(ctx context.Context, in ExamSessionInput, ns newSession) (write.ExamSessionRow, error) {
	p := ns.P
	var row write.ExamSessionRow
	school, asgSubject, err := h.sessionSchool(ctx, in)
	if err != nil {
		return row, err
	}
	// The school first (404 outside every grant, before the blueprint is read: no existence leak), then the subject.
	if asgSubject != nil {
		if err := AuthorizeLMS(ctx, p, authz.PermLmsAssignmentWrite, school, asgSubject); err != nil {
			return row, err
		}
	} else if holdsPerm(p, authz.PermLmsAssignmentWrite) && !SchoolCovered(p, authz.PermLmsAssignmentWrite, school) {
		_ = authz.Require(ctx, p, authz.PermLmsAssignmentWrite, authz.Resource{TenantID: p.TenantID, SchoolID: &school})
		return row, problem.New(problem.NotFound, "")
	}
	if err := h.Asg.schoolOfTenant(ctx, school); err != nil {
		return row, err
	}
	b, err := h.loadBlueprint(ctx, in.BlueprintVersionID)
	if err != nil {
		return row, err
	}
	if asgSubject != nil && *asgSubject != b.SubjectID {
		return row, problem.Invalid("/content_ref", problem.SubInvalid, "the blueprint is not of the subject of the assignment")
	}
	if err := AuthorizeLMS(ctx, p, authz.PermLmsAssignmentWrite, school, &b.SubjectID); err != nil {
		return row, err
	}
	s := h.settings(in, b)
	if err := examViolations(s.Validate()); err != nil {
		return row, err
	}
	if ns.Students == nil {
		ts := make([]assignment.Target, len(in.Targets))
		for i, t := range in.Targets {
			ts[i] = t.target()
		}
		if err := assignmentViolations(assignment.ValidateTargets(ts)); err != nil {
			return row, err
		}
	}
	targets, err := h.Asg.targets(ctx, school, in.Targets)
	if err != nil {
		return row, err
	}
	students := ns.Students
	if students == nil {
		snap, err := h.Asg.snapshot(ctx, write.AssignmentRow{SchoolID: school, Targets: targets})
		if err != nil {
			return row, err
		}
		for _, x := range snap {
			students = append(students, x.StudentID)
		}
	}
	member := map[uuid.UUID]bool{}
	for _, x := range students {
		member[x] = true
	}
	extra := map[uuid.UUID]int{}
	for i, a := range s.Accommodations {
		if !member[a.StudentID] {
			return row, problem.Invalid("/accommodations/"+strconv.Itoa(i)+"/student_id", problem.SubInvalid, "is not a student of the session")
		}
		extra[a.StudentID] = a.ExtraTimePercent
	}
	pool, err := h.Att.Delivery.Pool(ctx, &b.SubjectID)
	if err != nil {
		return row, err
	}
	if err := h.checkMatrix(b, s.FeedbackTiming, pool); err != nil {
		return row, err
	}
	balancer, err := h.balancer(ctx, s.Strategy, pool)
	if err != nil {
		return row, err
	}
	seed := randomSeed()
	ps, err := paper.SessionPapers(b.Def, pool, s.Strategy, s.VariantCount, seed, s.IsOfficial, balancer)
	if err != nil {
		var pe *paper.PoolError
		if errors.As(err, &pe) {
			return row, problem.New(problem.QuestionPoolInsufficient, pe.Error()).AddField(pe.Pointer(), problem.SubInvalid, "pool insufficient")
		}
		return row, err
	}
	now := h.now()
	status := exam.InitialStatus(s.WindowStart, now)
	target, _ := json.Marshal(map[string]any{"targets": targets, "student_count": len(students)})
	sections := make([]map[string]any, 0, len(b.Def.Sections))
	for _, sec := range b.Def.Sections {
		sections = append(sections, map[string]any{"section_id": sec.ID, "title": sectionTitle(sec.Title), "shuffle_items": sec.ShuffleItems})
	}
	sj, _ := json.Marshal(sections)
	acc, _ := json.Marshal(nonNilAccommodations(s.Accommodations))
	row = write.ExamSessionRow{ID: ids.New(), TenantID: *p.TenantID, SchoolID: school, SubjectID: b.SubjectID, GradeLevel: b.Grade,
		AssignmentID: in.AssignmentID, Title: s.Title, Type: string(s.Type), IsOfficial: s.IsOfficial, BlueprintID: b.ItemID,
		BlueprintVersionID: b.VersionID, WindowStart: s.WindowStart, WindowEnd: s.WindowEnd, DurationSec: s.DurationSec, Target: target,
		Sections: sj, Audience: s.Audience, Strategy: s.Strategy, VariantCount: s.VariantCount, FeedbackTiming: s.FeedbackTiming,
		ResultRelease: s.ResultRelease, ShowCorrect: s.ShowCorrectAnswers, ShowExplanations: s.ShowExplanations, LateStartPolicy: s.LateStartPolicy,
		Accommodations: acc, MaxAttempts: s.MaxAttempts, ScorePolicy: s.ScorePolicy, Status: string(status), CreatedBy: p.UserID,
		CreatedByRole: roleOf(p, school), Version: 1, CreatedAt: now, Navigation: exam.NavigationOf(b.Def.Navigation)}
	if len(ps) > 0 {
		row.QuestionCount, row.TotalPoints = len(ps[0].Items), ps[0].TotalPoints
	} else {
		row.QuestionCount, row.TotalPoints = blueprintCount(b.Def), b.Def.TotalPoints
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
	if ns.Balance != nil && len(ps) > 1 {
		*ns.Balance = balanceView(b.Def, ps)
	}
	if err := h.Store.InsertPapers(ctx, rows); err != nil {
		return row, err
	}
	parts := make([]write.ExamParticipationRow, len(students))
	for i, x := range students {
		parts[i] = write.ExamParticipationRow{SessionID: row.ID, StudentID: x, TenantID: row.TenantID, SchoolID: school,
			EffectiveDurationSec: exam.EffectiveDuration(s.DurationSec, extra[x]), ExtraTimePercent: extra[x]}
	}
	if err := h.Store.InsertParticipations(ctx, parts); err != nil {
		return row, err
	}
	if err := h.audit(ctx, "exam_session.created", row, "", map[string]any{"blueprint_version_id": b.VersionID, "exam_type": row.Type,
		"student_count": len(students), "variant_count": len(ps), "assignment_id": nullableID(row.AssignmentID)}); err != nil {
		return row, err
	}
	if status == exam.StatusOpen {
		return row, h.publishExam(ctx, row.EfficacyWaveID, EventExamOpened, "exam_session", row.ID, row.TenantID, windowPayload(row))
	}
	return row, nil
}

// sectionTitle is the Vietnamese title of a section (i18nText or a plain string), at most 255 characters.
func sectionTitle(raw json.RawMessage) string {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return truncateRunes(plain, 255)
	}
	node, _ := i18ntext.Resolve(raw, "vi", "vi")
	_ = json.Unmarshal(node, &plain)
	return truncateRunes(plain, 255)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func nonNilAccommodations(a []exam.Accommodation) []exam.Accommodation {
	if a == nil {
		return []exam.Accommodation{}
	}
	return a
}

func blueprintCount(d blueprint.Definition) int {
	n := 0
	for _, s := range d.Sections {
		for _, r := range s.Rules {
			n += r.Count
		}
	}
	return n
}

// windowPayload is the payload of exam.opened / exam.closed.
func windowPayload(s write.ExamSessionRow) map[string]any {
	return map[string]any{"exam_session_id": s.ID, "school_id": s.SchoolID, "exam_type": s.Type, "is_official": s.IsOfficial,
		"assignment_id": nullableID(s.AssignmentID), "window_start": s.WindowStart,
		"window_end": s.WindowEnd, "status": s.Status}
}

// ExamSessionViewOf is the staff view of a session.
func ExamSessionViewOf(s write.ExamSessionRow, now time.Time) dto.ExamSessionView {
	var acc []dto.ExamAccommodationView
	_ = json.Unmarshal(s.Accommodations, &acc)
	return dto.ExamSessionView{ExamSessionID: s.ID, Title: s.Title, Type: s.Type, IsOfficial: s.IsOfficial, SubjectID: s.SubjectID,
		GradeLevel: s.GradeLevel, WindowStart: s.WindowStart, WindowEnd: s.WindowEnd, DurationSeconds: s.DurationSec,
		QuestionCount: s.QuestionCount, MaxAttempts: s.MaxAttempts, Status: s.Status, BlueprintVersionID: s.BlueprintVersionID,
		FeedbackTiming: s.FeedbackTiming, ResultRelease: s.ResultRelease, ResultsReleasedAt: s.ReleasedAt, SchoolID: s.SchoolID,
		AssignmentID: s.AssignmentID, Audience: s.Audience, VariantStrategy: s.Strategy, VariantCount: s.VariantCount,
		LateStartPolicy: s.LateStartPolicy, ScorePolicy: s.ScorePolicy, ShowCorrectAnswers: s.ShowCorrect, ShowExplanations: s.ShowExplanations,
		Accommodations: acc, ServerTime: now, CreatedAt: s.CreatedAt, Version: s.Version, Navigation: exam.NavigationOf(s.Navigation)}
}

// CreateExamSession creates an exam session (createExamSession, F042 US1): lms.assignment.write in the school and subject,
// Idempotency-Key, audit exam_session.created.
func (h Exams) CreateExamSession(ctx context.Context, in ExamSessionInput) (out dto.ExamSessionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateExamSession")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	if in.AssignmentID != nil {
		return out, problem.Invalid("/assignment_id", problem.SubInvalid, "the exam session of an assignment is created when the assignment is published")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		bal := &dto.ExamBalanceView{}
		row, err := h.createSession(ctx, in, newSession{P: p, Balance: bal})
		if err != nil {
			return err
		}
		out = ExamSessionViewOf(row, h.now())
		if len(bal.Variants) > 0 {
			out.Balance = bal
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

// CreateForAssignment creates the session of a graded assignment on a blueprint in the publish transaction (F042-AC-31,
// adapter of port.ExamSessionCreator): the students snapshotted by the assignment, its window, the release default of
// graded assignments (graded_assignment_release_default) unless the assignment says otherwise.
func (h Exams) CreateForAssignment(ctx context.Context, in ExamSessionInput, students []uuid.UUID) (uuid.UUID, error) {
	p, err := staffOf(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if students == nil {
		students = []uuid.UUID{}
	}
	row, err := h.createSession(ctx, in, newSession{P: p, Students: students})
	return row.ID, err
}

// CancelExamSession cancels a scheduled or open session (F042-AC-31, adapter of port.ExamSessionCreator for
// cancelAssignment): status cancelled, audit exam_session.cancelled; open attempts are not graded.
func (h Exams) CancelExamSession(ctx context.Context, id uuid.UUID, reason string) error {
	s, err := h.Store.Session(ctx, id, true)
	if err != nil {
		return err
	}
	if s == nil {
		return problem.New(problem.NotFound, "")
	}
	switch exam.Status(s.Status) {
	case exam.StatusCancelled:
		return nil
	case exam.StatusScheduled, exam.StatusOpen:
	default:
		return problem.New(problem.InvalidStateTransition, "a closed exam session is not cancelled")
	}
	prev, now := s.Version, h.now()
	s.Status, s.CancelledAt = string(exam.StatusCancelled), &now
	if reason != "" {
		r := reason
		s.CancelReason = &r
	}
	ok, err := h.Store.UpdateSession(ctx, *s, prev)
	if err != nil {
		return err
	}
	if !ok {
		return problem.New(problem.VersionMismatch, "")
	}
	return h.audit(ctx, "exam_session.cancelled", *s, reason, nil)
}

// ReleaseExamResults releases the results of a manual session whose window closed (releaseExamResults, F042-AC-21):
// lms.result.release in the school and subject, status results_released, exam.results_released and audit; the
// Idempotency-Key replay publishes nothing again.
func (h Exams) ReleaseExamResults(ctx context.Context, id uuid.UUID) (out dto.ExamSessionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ReleaseExamResults")
	defer func() { done(err) }()
	p, err := staffOf(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Session(ctx, id, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if err := AuthorizeLMS(ctx, p, authz.PermLmsResultRelease, s.SchoolID, &s.SubjectID); err != nil {
			return err
		}
		if s.EfficacyWaveID != nil {
			// F035-AC-16: the results of an efficacy session are never released.
			return problem.New(problem.InvalidStateTransition, "the results of an efficacy exam session are never released")
		}
		if !exam.CanRelease(s.ResultRelease, exam.Status(s.Status)) {
			return problem.New(problem.Conflict, "only a closed exam session with manual release is released")
		}
		if err := h.release(ctx, s, &p.UserID); err != nil {
			return err
		}
		out = ExamSessionViewOf(*s, h.now())
		return idempotency.SetResult(ctx, out)
	})
	return out, deniedOutside(ctx, err)
}

// release moves a session to results_released with its event and audit (manual, or after_close at the close).
func (h Exams) release(ctx context.Context, s *write.ExamSessionRow, by *uuid.UUID) error {
	prev, now := s.Version, h.now()
	s.Status, s.ReleasedAt, s.ReleasedBy = string(exam.StatusReleased), &now, by
	ok, err := h.Store.UpdateSession(ctx, *s, prev)
	if err != nil {
		return err
	}
	if !ok {
		return problem.New(problem.VersionMismatch, "")
	}
	s.Version = prev + 1
	if err := h.publishExam(ctx, s.EfficacyWaveID, EventExamResultsReleased, "exam_session", s.ID, s.TenantID, map[string]any{"exam_session_id": s.ID,
		"school_id": s.SchoolID, "assignment_id": nullableID(s.AssignmentID), "released_at": now, "released_by": nullableID(by),
		"mode": s.ResultRelease}); err != nil {
		return err
	}
	return h.audit(ctx, "exam_session.results_released", *s, "", map[string]any{"mode": s.ResultRelease})
}

// ExamWindowReport counts what the job exam-window did.
type ExamWindowReport struct {
	Opened, Closed, AutoSubmitted, Absent int
}

// RunWindows is the job exam-window (F042-AC-16, EXAM_ENGINE §5, §7.2): scheduled sessions whose window started open
// (exam.opened); sessions whose window ended auto-submit their open attempts (window_closed), record the students who
// never started as absent and close (exam.closed); after_close sessions release their results at the close.
func (h Exams) RunWindows(ctx context.Context, limit int) (rep ExamWindowReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ExamWindow")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = 200
	}
	now := h.now()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return rep, err
	}
	var opening, closing []write.DueAssignment
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		if opening, err = h.Store.DueToOpen(ctx, now, limit); err != nil {
			return err
		}
		closing, err = h.Store.DueToClose(ctx, now, limit)
		return err
	}); err != nil {
		return rep, err
	}
	for _, d := range opening {
		if err := h.Asg.inTenant(ctx, d.TenantID, func(ctx context.Context) error { return h.open(ctx, d.ID, now, &rep) }); err != nil {
			return rep, err
		}
	}
	for _, d := range closing {
		if err := h.Asg.inTenant(ctx, d.TenantID, func(ctx context.Context) error { return h.close(ctx, d.ID, now, &rep) }); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

func (h Exams) open(ctx context.Context, id uuid.UUID, now time.Time, rep *ExamWindowReport) error {
	s, err := h.Store.Session(ctx, id, true)
	if err != nil || s == nil || s.Status != string(exam.StatusScheduled) || s.WindowStart.After(now) {
		return err
	}
	prev := s.Version
	s.Status, s.OpenedAt = string(exam.StatusOpen), &now
	ok, err := h.Store.UpdateSession(ctx, *s, prev)
	if err != nil || !ok {
		return err
	}
	rep.Opened++
	return h.publishExam(ctx, s.EfficacyWaveID, EventExamOpened, "exam_session", s.ID, s.TenantID, windowPayload(*s))
}

func (h Exams) close(ctx context.Context, id uuid.UUID, now time.Time, rep *ExamWindowReport) error {
	s, err := h.Store.Session(ctx, id, true)
	if err != nil || s == nil || (s.Status != string(exam.StatusOpen) && s.Status != string(exam.StatusScheduled)) || s.WindowEnd.After(now) {
		return err
	}
	open, err := h.Store.OpenAttempts(ctx, s.ID)
	if err != nil {
		return err
	}
	submitted := 0
	for _, a := range open {
		_, done, err := h.submitAttempt(ctx, a, exam.ReasonWindowClosed, nil)
		if err != nil {
			return err
		}
		if done {
			submitted++
			if h.Metrics != nil {
				h.Metrics.Submitted(s.Type, ExamAutoSubmitted)
			}
		}
	}
	absent, err := h.Store.MarkAbsent(ctx, s.ID)
	if err != nil {
		return err
	}
	prev := s.Version
	s.Status, s.ClosedAt = string(exam.StatusClosed), &now
	ok, err := h.Store.UpdateSession(ctx, *s, prev)
	if err != nil || !ok {
		return err
	}
	s.Version = prev + 1
	rep.Closed++
	rep.AutoSubmitted += submitted
	rep.Absent += absent
	payload := windowPayload(*s)
	payload["absent_count"], payload["auto_submitted_count"] = absent, submitted
	if err := h.publishExam(ctx, s.EfficacyWaveID, EventExamClosed, "exam_session", s.ID, s.TenantID, payload); err != nil {
		return err
	}
	if s.ResultRelease == exam.ReleaseAfterClose {
		return h.release(ctx, s, nil)
	}
	return nil
}

// ReleaseOf is the result release of a session and whether students see its results now (Attempts.ExamRelease).
func (h Exams) ReleaseOf(ctx context.Context, sessionID uuid.UUID) (string, bool, error) {
	s, err := h.Store.Session(ctx, sessionID, false)
	if err != nil || s == nil {
		return exam.ReleaseManual, false, err
	}
	return s.ResultRelease, exam.ResultsVisible(s.ResultRelease, exam.Status(s.Status)), nil
}

// RegradeTarget implements ExamRegradeTarget (F042-AC-28): a live session of the session tenant (404 otherwise) whose papers
// or attempts hold the question version (422 otherwise).
func (h Exams) RegradeTarget(ctx context.Context, sessionID, questionVersionID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	s, err := h.Store.Session(ctx, sessionID, false)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if s == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.NotFound, "")
	}
	if err := efficacyLocked(s); err != nil {
		return uuid.Nil, uuid.Nil, err // F035-AC-16: no regrade of an efficacy session
	}
	ps, err := h.Store.Papers(ctx, s.ID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	held := false
	for _, p := range ps {
		for _, it := range p.Items {
			held = held || it.QuestionVersionID == questionVersionID
		}
	}
	if !held {
		as, err := h.Store.GradedAttempts(ctx, s.ID, questionVersionID)
		if err != nil {
			return uuid.Nil, uuid.Nil, err
		}
		held = len(as) > 0
	}
	if !held {
		return uuid.Nil, uuid.Nil, problem.Invalid("/question_version_id", problem.SubInvalid, "is not a question of the exam session")
	}
	return s.SchoolID, s.SubjectID, nil
}

// Regraded publishes exam.regraded once a request of scope exam_session ran (F042-AC-28).
func (h Exams) Regraded(ctx context.Context, r write.RegradeRequest, attempts []uuid.UUID) error {
	if r.ExamSessionID == nil || r.QuestionVersionID == nil {
		return nil
	}
	s, err := h.Store.Session(ctx, *r.ExamSessionID, false)
	if err != nil || s == nil {
		return err
	}
	if attempts == nil {
		attempts = []uuid.UUID{}
	}
	if h.Metrics != nil {
		for range attempts {
			h.Metrics.Regraded(string(r.Action))
		}
	}
	return h.publishExam(ctx, s.EfficacyWaveID, EventExamRegraded, "exam_session", s.ID, s.TenantID, map[string]any{"exam_session_id": s.ID, "school_id": s.SchoolID,
		"regrade_request_id": r.ID, "question_version_id": *r.QuestionVersionID, "corrected_version_id": nullableID(r.CorrectedVersionID),
		"action": string(r.Action), "affected_attempt_ids": attempts})
}
