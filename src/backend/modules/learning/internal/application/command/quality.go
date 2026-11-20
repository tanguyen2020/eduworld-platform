package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/examquality"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/itemanalysis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/regulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the calibration (F044-AC-20, AC-21) and the change note of a calibrated version (QUESTION_BANK §9.1).
const (
	ActionCalibrationAccepted  = "content.calibration_accepted"
	ActionCalibrationDismissed = "content.calibration_dismissed"
	CalibrationChangeNote      = "difficulty_calibration"
)

// QualityConfig are the parameters of item analysis, calibration, exam quality and reference levels (CONFIG_DEFAULTS:
// item_analysis_min_responses, difficulty_calibration, exam_reliability_warn, assessment_reference_thresholds; no settings
// reader yet: the registered defaults).
type QualityConfig struct {
	MinResponses int           // item_analysis_min_responses
	Lookback     time.Duration // versions answered since then are recomputed each night [Proposed: 48 h]
	MaxVersions  int           // versions per nightly run [Proposed]
	Calibration  itemanalysis.CalibrationConfig
	Exam         examquality.Config
}

// DefaultQualityConfig returns the CONFIG_DEFAULTS values.
func DefaultQualityConfig() QualityConfig {
	return QualityConfig{MinResponses: itemanalysis.DefaultMinResponses, Lookback: 48 * time.Hour, MaxVersions: 5000,
		Calibration: itemanalysis.DefaultCalibrationConfig(), Exam: examquality.DefaultConfig()}
}

// QualityMetrics observes the jobs (eduworld_item_analysis_questions_total, eduworld_calibration_proposals_total{status},
// eduworld_exam_reliability_low_total).
type QualityMetrics interface {
	ItemAnalysed(n int)
	Proposal(status string)
	ReliabilityLow()
}

// Quality holds the write use cases of F044 US3–US6: item analysis (job item-analysis-nightly), calibration proposals (job
// calibration-weekly, accept/dismiss), the exam quality report (consumer of exam.closed) and the reference levels (job
// reference-levels-nightly, new profile version). Statistics never change a published version (R11).
type Quality struct {
	UoW          write.UnitOfWork
	Items        write.ItemAnalysisStore
	Calibrations write.CalibrationStore
	Exams        write.ExamQualityStore
	References   write.ReferenceStore
	Tenants      interface {
		Tenants(ctx context.Context) ([]uuid.UUID, error)
	} // tenants of the nightly reference job (RollupRepo)
	Questions Questions // the question store and its usage records (new calibrated version)
	Audit     write.Auditor
	Config    QualityConfig
	Clock     func() time.Time
	Metrics   QualityMetrics
}

func (h Quality) now() time.Time { return now(h.Clock).UTC() }

func (h Quality) config() QualityConfig {
	if h.Config.MinResponses <= 0 {
		return DefaultQualityConfig()
	}
	return h.Config
}

func platformJob(ctx context.Context) (context.Context, error) {
	return tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
}

// ---------- item analysis (US4)

// choiceKeys are the options and correct options of a choice version (render_payload.options[].id, answer key).
func choiceKeys(render, key json.RawMessage) ([]string, []string) {
	var r struct {
		Options []struct {
			ID string `json:"id"`
		} `json:"options"`
	}
	var k struct {
		One  string   `json:"correct_option_id"`
		Many []string `json:"correct_option_ids"`
	}
	_ = json.Unmarshal(render, &r)
	_ = json.Unmarshal(key, &k)
	opts := make([]string, 0, len(r.Options))
	for _, o := range r.Options {
		opts = append(opts, o.ID)
	}
	correct := k.Many
	if k.One != "" {
		correct = append(correct, k.One)
	}
	return opts, correct
}

// selectedOptions are the options of a choice response ({option_id} or {option_ids}).
func selectedOptions(resp json.RawMessage) []string {
	var r struct {
		One  string   `json:"option_id"`
		Many []string `json:"option_ids"`
	}
	_ = json.Unmarshal(resp, &r)
	if r.One != "" {
		return append(r.Many, r.One)
	}
	return r.Many
}

// ComputeQuestionStats is the job item-analysis-nightly (F044-AC-16): every version answered within the lookback is
// recomputed from all its first scored answers, per grade and pooled (grade 0); a platform question pools every tenant.
func (h Quality) ComputeQuestionStats(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ItemAnalysis")
	defer func() { done(err) }()
	cfg := h.config()
	bctx, err := platformJob(ctx)
	if err != nil {
		return 0, err
	}
	var versions []write.AnalysedVersion
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		versions, err = h.Items.ChangedVersions(ctx, h.now().Add(-cfg.Lookback), cfg.MaxVersions)
		return err
	})
	if err != nil {
		return 0, err
	}
	at := h.now()
	for _, v := range versions {
		err := h.UoW.Do(bctx, func(ctx context.Context) error {
			answers, err := h.Items.FirstAnswers(ctx, v.QuestionVersionID)
			if err != nil {
				return err
			}
			return h.Items.UpsertQuestionStats(ctx, StatsRows(v, answers, cfg.MinResponses, at))
		})
		if err != nil {
			return n, fmt.Errorf("item analysis of %s: %w", v.QuestionVersionID, err)
		}
		n++
	}
	if h.Metrics != nil {
		h.Metrics.ItemAnalysed(n)
	}
	return n, nil
}

// StatsRows computes the rows of one version: one per grade and the pooled row (grade 0).
func StatsRows(v write.AnalysedVersion, answers []write.FirstAnswer, minN int, at time.Time) []write.QuestionStatRow {
	opts, correct := choiceKeys(v.Render, v.AnswerKey)
	byGrade := map[int][]itemanalysis.Response{}
	for _, a := range answers {
		r := itemanalysis.Response{Correct: a.Correct, Total: a.AttemptScore, TimeMS: a.ResponseMS, RapidGuess: a.RapidGuess,
			HintsUsed: a.HintsUsed, Options: selectedOptions(a.Response)}
		byGrade[a.GradeLevel] = append(byGrade[a.GradeLevel], r)
		if a.GradeLevel != 0 {
			byGrade[0] = append(byGrade[0], r)
		}
	}
	grades := make([]int, 0, len(byGrade))
	for g := range byGrade {
		grades = append(grades, g)
	}
	slices.Sort(grades)
	out := make([]write.QuestionStatRow, 0, len(grades))
	for _, g := range grades {
		s := itemanalysis.Compute(byGrade[g], itemanalysis.Options{MinResponses: minN, OptionIDs: opts, CorrectOptionIDs: correct,
			Difficulty: v.Difficulty})
		out = append(out, write.QuestionStatRow{TenantID: v.TenantID, QuestionID: v.QuestionID, QuestionVersionID: v.QuestionVersionID,
			GradeLevel: g, FirstAttempts: s.FirstAttempts, ExcludedRapidGuess: s.ExcludedRapidGuess, WithHint: s.WithHint, PValue: s.PValue,
			PValueWithHint: s.PValueWithHint, Discrimination: s.Discrimination, DistractorRates: s.DistractorRates, AvgTimeMS: s.AvgTimeMS,
			RapidGuessRate: round4(s.RapidGuessRate), HintUsageRate: round4(s.HintUsageRate), Flags: s.Flags, ComputedAt: at})
	}
	return out
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// ---------- calibration (US5)

// ProposeCalibrations is the job calibration-weekly (F044-AC-19): a pending proposal for every current version whose
// p_value (≥ 100 plain first answers in its main grade) is at least one level away from its declared difficulty, unless
// a proposal is pending or the question is suppressed after two consecutive dismissals.
func (h Quality) ProposeCalibrations(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.CalibrationWeekly")
	defer func() { done(err) }()
	cfg := h.config()
	bctx, err := platformJob(ctx)
	if err != nil {
		return 0, err
	}
	var cands []write.CalibrationCandidate
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		cands, err = h.Calibrations.CalibrationCandidates(ctx, cfg.Calibration.MinResponses)
		return err
	}); err != nil {
		return 0, err
	}
	at := h.now()
	for _, c := range cands {
		err := h.UoW.Do(bctx, func(ctx context.Context) error {
			st, err := h.Calibrations.CalibrationState(ctx, c.QuestionID)
			if err != nil {
				return err
			}
			d, ok := itemanalysis.Propose(c.Difficulty, c.PValue, c.SampleSize, itemanalysis.PriorState{Pending: st.Pending,
				ConsecutiveDismissals: st.ConsecutiveDismissals, SuppressedUntil: st.SuppressedUntil}, cfg.Calibration, at)
			if !ok {
				return nil
			}
			n++
			return h.Calibrations.InsertProposal(ctx, write.ProposalRow{ID: ids.New(), TenantID: c.TenantID, QuestionID: c.QuestionID,
				BasedOnVersionID: c.QuestionVersionID, CurrentDifficulty: c.Difficulty, ProposedDifficulty: d, PValue: c.PValue,
				SampleSize: c.SampleSize, GradeLevel: c.GradeLevel, Grades: c.Grades, ConsecutiveDismissals: st.ConsecutiveDismissals,
				CreatedAt: at})
		})
		if err != nil {
			return n, err
		}
		if h.Metrics != nil {
			h.Metrics.Proposal("pending")
		}
	}
	return n, nil
}

// ProposalView is the CalibrationProposal view of a row.
func ProposalView(p write.ProposalRow) dto.CalibrationProposalView {
	g := p.Grades
	if g == nil {
		g = map[string]any{}
	}
	return dto.CalibrationProposalView{ProposalID: p.ID, QuestionID: p.QuestionID, BasedOnVersionID: p.BasedOnVersionID,
		CurrentDifficulty: p.CurrentDifficulty, ProposedDifficulty: p.ProposedDifficulty, PValue: p.PValue, SampleSize: p.SampleSize,
		GradeLevel: p.GradeLevel, GradeDistribution: g, Status: p.Status, DecidedBy: p.DecidedBy, DecidedAt: p.DecidedAt,
		CreatedVersionID: p.CreatedVersionID, CreatedAt: p.CreatedAt}
}

// decide loads a pending proposal for a reviewer (F044-AC-21): learning_content.review; a tenant reviewer cannot decide on
// a platform question (403, read-only); not pending → 409.
func (h Quality) decide(ctx context.Context, id uuid.UUID) (authz.Principal, write.ProposalRow, error) {
	p, err := authorize(ctx, authz.PermLearningContentReview)
	if err != nil {
		return p, write.ProposalRow{}, err
	}
	pr, err := h.Calibrations.Proposal(ctx, id, false)
	if err != nil {
		return p, pr, err
	}
	owner := contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: pr.TenantID}
	if pr.TenantID == nil {
		owner.Scope = contentmodel.OwnerPlatform
	}
	if err := writable(ctx, p, authz.PermLearningContentReview, owner); err != nil {
		return p, pr, err
	}
	if pr, err = h.Calibrations.Proposal(ctx, id, true); err != nil {
		return p, pr, err
	}
	if pr.Status != "pending" {
		return p, pr, problem.New(problem.InvalidStateTransition, "the proposal was already decided")
	}
	return p, pr, nil
}

func isNotFound(err error) bool {
	var pe *problem.Error
	return errors.As(err, &pe) && pe.Code == problem.NotFound
}

// AcceptCalibration is acceptCalibrationProposal (F044-AC-20, AC-21): a new draft version of the question copied from the
// version the proposal is based on with only the difficulty changed (change_note difficulty_calibration); it goes through
// the normal review/publish workflow (another reviewer approves it, ADR-0043); the published version is untouched; the
// author of the based-on version may not accept (reviewer ≠ author); audit content.calibration_accepted.
func (h Quality) AcceptCalibration(ctx context.Context, id uuid.UUID) (out dto.CalibrationProposalView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.AcceptCalibration")
	defer func() { done(err) }()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		p, pr, err := h.decide(ctx, id)
		if err != nil {
			return err
		}
		v, err := h.Questions.calibratedVersion(ctx, p, pr.QuestionID, pr.BasedOnVersionID, pr.ProposedDifficulty)
		if err != nil {
			return err
		}
		at := h.now()
		st := itemanalysis.Accept(itemanalysis.PriorState{})
		pr.Status, pr.DecidedBy, pr.DecidedAt, pr.CreatedVersionID, pr.ConsecutiveDismissals = "accepted", &p.UserID, &at, &v, st.ConsecutiveDismissals
		if err := h.Calibrations.UpdateProposal(ctx, pr); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: ActionCalibrationAccepted, TargetType: "question", TargetID: pr.QuestionID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"proposal_id": pr.ID.String(), "based_on_version_id": pr.BasedOnVersionID.String(),
				"question_version_id": v.String(), "difficulty_from": pr.CurrentDifficulty, "difficulty_to": pr.ProposedDifficulty}}); err != nil {
			return err
		}
		out = ProposalView(pr)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil && h.Metrics != nil {
		h.Metrics.Proposal("accepted")
	}
	return out, err
}

// DismissCalibration is dismissCalibrationProposal (F044-AC-21): the second consecutive dismissal suppresses the question
// for 90 days (F044-AC-19); audit content.calibration_dismissed.
func (h Quality) DismissCalibration(ctx context.Context, id uuid.UUID) (out dto.CalibrationProposalView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DismissCalibration")
	defer func() { done(err) }()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		p, pr, err := h.decide(ctx, id)
		if err != nil {
			return err
		}
		at := h.now()
		st := itemanalysis.Dismiss(itemanalysis.PriorState{ConsecutiveDismissals: pr.ConsecutiveDismissals}, h.config().Calibration, at)
		pr.Status, pr.DecidedBy, pr.DecidedAt = "dismissed", &p.UserID, &at
		pr.ConsecutiveDismissals, pr.SuppressedUntil = st.ConsecutiveDismissals, st.SuppressedUntil
		if err := h.Calibrations.UpdateProposal(ctx, pr); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: ActionCalibrationDismissed, TargetType: "question", TargetID: pr.QuestionID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"proposal_id": pr.ID.String(), "consecutive_dismissals": pr.ConsecutiveDismissals}}); err != nil {
			return err
		}
		out = ProposalView(pr)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil && h.Metrics != nil {
		h.Metrics.Proposal("dismissed")
	}
	return out, err
}

// calibratedVersion inserts the new draft version (copy of src with the new difficulty) and returns its id.
func (h Questions) calibratedVersion(ctx context.Context, p authz.Principal, questionID, srcID uuid.UUID, difficulty int) (uuid.UUID, error) {
	q, err := h.Store.Question(ctx, questionID)
	if err != nil {
		return uuid.Nil, err
	}
	vs, err := h.Store.QuestionVersions(ctx, q.ID)
	if err != nil {
		return uuid.Nil, err
	}
	var src *dto.QuestionVersion
	maxNo := q.LatestVersionNo
	for i, v := range vs {
		if v.Status == contentmodel.StatusDraft || v.Status == contentmodel.StatusInReview {
			return uuid.Nil, problem.New(problem.Conflict, "the question already has an open draft or in-review version")
		}
		if v.ID == srcID {
			src = &vs[i]
		}
		maxNo = max(maxNo, v.VersionNo)
	}
	if src == nil {
		return uuid.Nil, problem.New(problem.NotFound, "the version of the proposal does not exist")
	}
	if src.AuthorID == p.UserID {
		return uuid.Nil, denied(ctx, "reviewer_is_author")
	}
	v := *src
	note := CalibrationChangeNote
	v.ID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID = ids.New(), maxNo+1, contentmodel.StatusDraft, p.UserID, nil, nil, &srcID
	v.AudioExemptApprovedBy, v.ChangeNote, v.Difficulty, v.Owner = nil, &note, difficulty, q.Owner
	if err := h.Store.InsertQuestionVersion(ctx, v); err != nil {
		return uuid.Nil, err
	}
	if err := h.recordUsages(ctx, v); err != nil {
		return uuid.Nil, err
	}
	q.LatestVersionNo = v.VersionNo
	if _, err := h.Store.UpdateQuestion(ctx, q, q.RowVersion); err != nil {
		return uuid.Nil, err
	}
	if err := h.recordQuestion(ctx, q.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: q.ID.String(),
		Metadata: map[string]any{"question_version_id": v.ID.String(), "version": v.VersionNo, "based_on_version_id": srcID.String(),
			"change_note": CalibrationChangeNote}}); err != nil {
		return uuid.Nil, err
	}
	return v.ID, nil
}

// ---------- exam quality (US6)

// ComputeExamQuality runs on exam.closed (F044-AC-23): for an official session, reliability (KR-20/alpha) and SEM per
// variant with enough students, variant gap, problem items, score distribution; the scores are never changed. The report
// is also computed for a non-official session (analysis of US-LMS-007) without the reliability warning metric.
func (h Quality) ComputeExamQuality(ctx context.Context, sessionID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ComputeExamQuality")
	defer func() { done(err) }()
	cfg := h.config()
	s, err := h.Exams.ExamSessionForQuality(ctx, sessionID)
	if err != nil {
		if isNotFound(err) {
			slog.WarnContext(ctx, "exam quality: session not found", "exam_session_id", sessionID)
			return nil
		}
		return err
	}
	sheets, err := h.Exams.ExamSheets(ctx, sessionID)
	if err != nil {
		return err
	}
	versions := map[uuid.UUID]bool{}
	for _, sh := range sheets {
		for _, it := range sh.Items {
			versions[it.QuestionVersionID] = true
		}
	}
	vids := make([]uuid.UUID, 0, len(versions))
	for v := range versions {
		vids = append(vids, v)
	}
	tags, err := h.Exams.OptionMisconceptions(ctx, vids)
	if err != nil {
		return err
	}
	rep := examquality.Analyze(ExamSheets(sheets, tags), nil, cfg.Exam)
	row := ExamQualityRowOf(s, rep, h.now())
	if !s.IsOfficial {
		row.ReliabilityWarning = false
	}
	if row.ReliabilityWarning && h.Metrics != nil {
		h.Metrics.ReliabilityLow()
	}
	return h.Exams.UpsertExamQuality(ctx, row)
}

// ExamSheets maps the stored sheets to the domain (misconception tags of the wrong options chosen).
func ExamSheets(sheets []write.ExamSheet, tags map[uuid.UUID]map[string]string) []examquality.Sheet {
	out := make([]examquality.Sheet, 0, len(sheets))
	for _, sh := range sheets {
		s := examquality.Sheet{StudentID: sh.StudentID, VariantNo: sh.VariantNo, Total: sh.Score, Max: sh.MaxScore}
		for _, it := range sh.Items {
			var mis []string
			if !it.Correct {
				for _, o := range selectedOptions(it.Response) {
					if t := tags[it.QuestionVersionID][o]; t != "" {
						mis = append(mis, t)
					}
				}
			}
			s.Items = append(s.Items, examquality.ItemScore{QuestionVersionID: it.QuestionVersionID, Score: it.Score, Max: it.MaxScore,
				Correct: it.Correct, RapidGuess: it.RapidGuess, TimeMS: it.ResponseMS, Misconceptions: mis})
		}
		out = append(out, s)
	}
	return out
}

// ExamQualityRowOf is the stored report.
func ExamQualityRowOf(s write.ExamSessionRow, rep examquality.Report, at time.Time) write.ExamQualityRow {
	variants, _ := json.Marshal(nonNil(rep.Variants))
	items, _ := json.Marshal(nonNil(rep.Items))
	var problems []examquality.ItemStats
	for _, it := range rep.Items {
		if it.Problem {
			problems = append(problems, it)
		}
	}
	pj, _ := json.Marshal(nonNil(problems))
	row := write.ExamQualityRow{SessionID: s.ID, TenantID: s.TenantID, SchoolID: s.SchoolID, StudentCount: rep.N, Variants: variants,
		VariantGap: rep.VariantGap, ProblemItems: pj, Items: items, Distribution: rep.Distribution, ReliabilityWarning: rep.ReliabilityWarning,
		ComputedAt: at}
	if rep.N > 0 {
		m, md, pr := math.Round(rep.Mean*100)/100, math.Round(rep.Median*100)/100, round4(rep.PassRate)
		row.Mean, row.Median, row.PassRate = &m, &md, &pr
	}
	return row
}

// ---------- reference levels (US3)

// ComputeReferenceLevels is the job reference-levels-nightly (F044-AC-11, AC-14): for every tenant, the reference level of
// every student × subject of the current period (default calendar) under the active profile of the grade; a profile
// version change writes new rows and keeps those of earlier versions and periods.
func (h Quality) ComputeReferenceLevels(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ReferenceLevels")
	defer func() { done(err) }()
	bctx, err := platformJob(ctx)
	if err != nil {
		return 0, err
	}
	var tenants []uuid.UUID
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		tenants, err = h.Tenants.Tenants(ctx)
		return err
	}); err != nil {
		return 0, err
	}
	period, year := regulation.CurrentPeriod(h.now().In(time.FixedZone("ICT", 7*3600)))
	for _, t := range tenants {
		sc, err := authz.EventScope(t)
		if err != nil {
			return n, err
		}
		k, err := h.ReferenceLevelsFor(tenancy.WithScope(tenancy.WithModule(ctx, module), sc), year, period)
		if err != nil {
			return n, fmt.Errorf("reference levels of tenant %s: %w", t, err)
		}
		n += k
	}
	return n, nil
}

// ReferenceLevelsFor computes the reference levels of the tenant of ctx for one period (F044-AC-11).
func (h Quality) ReferenceLevelsFor(ctx context.Context, year int, period string) (int, error) {
	var n int
	from, to := regulation.PeriodRange(year, period)
	at := h.now()
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		profiles, err := h.References.ActiveProfiles(ctx)
		if err != nil {
			return err
		}
		byCode := map[string]write.AssessmentProfileRow{}
		for _, p := range profiles {
			byCode[p.Code] = p
		}
		students, err := h.References.ReferenceStudents(ctx, regulation.ScopeShare(period))
		if err != nil {
			return err
		}
		sc, _ := tenancy.ScopeFrom(ctx)
		tenant := uuid.Nil
		if ts, ok := sc.(tenancy.TenantScope); ok {
			tenant, _ = ts.SessionTenantID()
		}
		rows := make([]write.ReferenceLevelRow, 0, len(students))
		for _, s := range students {
			prof, ok := byCode[regulation.ProfileFor(s.GradeLevel)]
			if !ok {
				continue
			}
			rules := regulation.DefaultRules()
			_ = json.Unmarshal(prof.Rules, &rules)
			levels, err := h.References.ObjectiveLevels(ctx, s.StudentID, s.Objectives)
			if err != nil {
				return err
			}
			res := regulation.Map(prof.Code, rules, ReferenceCounts(len(s.Objectives), levels, rules.MinEvidence))
			official, low, err := h.References.OfficialScores(ctx, s.StudentID, s.SubjectID, from, to)
			if err != nil {
				return err
			}
			rapid, err := h.References.RapidGuessRatio(ctx, s.StudentID, s.SubjectID, from, to)
			if err != nil {
				return err
			}
			rows = append(rows, write.ReferenceLevelRow{TenantID: tenant, SchoolID: s.SchoolID, ClassID: s.ClassID, StudentID: s.StudentID,
				SubjectID: s.SubjectID, AcademicYear: year, Period: period, ProfileID: prof.ID, ProfileCode: prof.Code, ProfileVersion: prof.Version,
				LevelCode: res.Level, InScope: len(s.Objectives), Coverage: res.Coverage, Proficient: res.ProficientRatio, Mastered: res.MasteredRatio,
				OfficialScore: official, LowReliabilityExamIDs: low, RapidGuessRatio: rapid, ComputedAt: at})
		}
		n = len(rows)
		return h.References.UpsertReferenceLevels(ctx, rows)
	})
	return n, err
}

// ReferenceCounts counts the objectives of the scope by level (covered = ≥ minEvidence evidence).
func ReferenceCounts(inScope int, levels []write.ObjectiveLevel, minEvidence int) regulation.Counts {
	c := regulation.Counts{InScope: inScope}
	for _, l := range levels {
		if l.EvidenceCount < minEvidence {
			continue
		}
		c.Covered++
		switch l.Level {
		case "mastered":
			c.Mastered++
			c.AtLeastProficient++
			c.AtLeastDeveloping++
		case "proficient":
			c.AtLeastProficient++
			c.AtLeastDeveloping++
		case "developing":
			c.AtLeastDeveloping++
		}
	}
	return c
}

// PublishProfileVersion creates a new version of a reference profile with new thresholds and supersedes the active one
// (F044-AC-14): earlier periods keep the version they were computed with.
func (h Quality) PublishProfileVersion(ctx context.Context, code string, rules regulation.Rules, disclaimer map[string]string) (write.AssessmentProfileRow, error) {
	var out write.AssessmentProfileRow
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		profiles, err := h.References.ActiveProfiles(ctx)
		if err != nil {
			return err
		}
		var cur *write.AssessmentProfileRow
		for i := range profiles {
			if profiles[i].Code == code {
				cur = &profiles[i]
			}
		}
		if cur == nil {
			return problem.New(problem.NotFound, "no active profile "+code)
		}
		if err := h.References.SupersedeProfile(ctx, cur.ID); err != nil {
			return err
		}
		raw, _ := json.Marshal(rules)
		if disclaimer == nil {
			disclaimer = cur.Disclaimer
		}
		out = write.AssessmentProfileRow{ID: ids.New(), TenantID: cur.TenantID, Code: code, Version: cur.Version + 1, GradeLevels: cur.GradeLevels,
			Rules: raw, Disclaimer: disclaimer, Status: "active", CreatedAt: h.now()}
		return h.References.InsertProfileVersion(ctx, out)
	})
	return out, err
}
