package command

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the efficacy study (F035 plan: Bảo mật, tenant, idempotency, audit).
const (
	ActionAgreementCreated     = "research_agreement.created"
	ActionAgreementRevoked     = "research_agreement.revoked"
	ActionStudyCreated         = "efficacy_study.created"
	ActionStudyUpdated         = "efficacy_study.updated"
	ActionStudyCohortAdded     = "efficacy_study.cohort_added"
	ActionStudyCohortRemoved   = "efficacy_study.cohort_removed"
	ActionStudyActivated       = "efficacy_study.activated"
	ActionStudyCompleted       = "efficacy_study.completed"
	ActionWaveScheduled        = "efficacy_wave.scheduled"
	ActionWaveOpened           = "efficacy_wave.opened"
	ActionWaveClosed           = "efficacy_wave.closed"
	ActionParticipantsWithdraw = "efficacy.participants_withdrawn"
	ActionEfficacyReport       = "efficacy_report.generated"
	ActionDatasetExported      = "efficacy.dataset_exported"
)

// EfficacyMetrics observes the study (eduworld_efficacy_wave_sessions_created_total, eduworld_efficacy_participants{status},
// eduworld_efficacy_report_duration_seconds); nil = not observed.
type EfficacyMetrics interface {
	WaveSessions(n int)
	Participants(status string, n int)
	ReportDuration(seconds float64)
}

// Efficacy holds the write use cases of the efficacy study (F035 US2–US4, US6): research agreements of the tenants, the
// study and its cohorts (school × grade, ADR-0095), the activation, the waves of exam sessions, the participants and their withdrawal, the
// mastery snapshots and the report job. Every use case requires a platform permission (authz.Require first, R7); writes
// into tenants run under the audited bypass efficacy_study.
type Efficacy struct {
	UoW        write.UnitOfWork
	Store      write.EfficacyStore
	Content    write.EfficacyContent
	Exams      *Exams // exam sessions of the waves (F042)
	Exclusions write.ExamExclusions
	Read       Reader // read session of the report job (replica when configured)
	Data       read.EfficacyDataReader
	Keys       PseudonymKeys // key of the pseudonymous dataset (F035 US7)
	Audit      write.Auditor
	Clock      func() time.Time
	Config     efficacy.Config
	Metrics    EfficacyMetrics
	// Experiments is called when an agreement is revoked: the students of the tenant leave every experiment (US5).
	Experiments ExperimentLeaver
}

// Reader is a read session (port read.Session).
type Reader interface {
	Read(ctx context.Context, fn func(ctx context.Context) error) error
}

// ExperimentLeaver takes the students of a tenant out of the running experiments (F035-AC-20).
type ExperimentLeaver interface {
	LeaveTenant(ctx context.Context, tenant uuid.UUID, at time.Time) (int, error)
}

func (h Efficacy) now() time.Time { return now(h.Clock).UTC() }

func (h Efficacy) config() efficacy.Config {
	if h.Config.MinQuestionsPerObjective <= 0 {
		return efficacy.DefaultConfig()
	}
	return h.Config
}

// efficacyAdmin authorizes a platform permission of the study (F035-AC-10): platform staff only, the permission held
// (tenant roles, content_admin and safety_moderator lack it → 403 PERMISSION_DENIED; the platform role is the httpx guard of /admin/).
func efficacyAdmin(ctx context.Context, perm authz.Permission) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return p, err
	}
	if p.UserType != authz.UserTypeStaff || !p.HasPlatformRole() {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

// targetTenant is the tenant of X-Tenant-Id of a System Admin write into a tenant (400 TENANT_HEADER_REQUIRED without it).
func targetTenant(p authz.Principal) (uuid.UUID, error) {
	if p.TenantID == nil {
		return uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
	}
	return *p.TenantID, nil
}

// studyScope is the audited bypass of a use case touching every tenant of a study (ADR-0089 §3, reason efficacy_study).
func studyScope(ctx context.Context, mode tenancy.Mode, tenants ...uuid.UUID) (context.Context, error) {
	return tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonEfficacyStudy, tenancy.WithMode(mode), tenancy.WithTenantIDs(tenants...))
}

func efficacyViolations(vs []efficacy.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	pe := problem.Invalid(vs[0].Field, problem.SubInvalid, vs[0].Reason)
	for _, v := range vs[1:] {
		pe.AddField(v.Field, problem.SubInvalid, v.Reason)
	}
	return pe
}

func stateErr(err error) error {
	if errors.Is(err, efficacy.ErrState) {
		return problem.New(problem.InvalidStateTransition, "the state of the study does not allow this")
	}
	return err
}

func (h Efficacy) auditPlatform(ctx context.Context, action, targetType string, target uuid.UUID, reason string, meta map[string]any) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: targetType, TargetID: target.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: meta})
}

func (h Efficacy) auditTenant(ctx context.Context, tenant uuid.UUID, action, targetType string, target uuid.UUID, reason string,
	meta map[string]any) error {
	return auditIn(ctx, h.Audit, tenant, audit.Event{Action: action, TargetType: targetType, TargetID: target.String(),
		Result: audit.ResultSuccess, Reason: reason, Metadata: meta})
}

// ---------- research agreements (US2, US4)

// AgreementInput is ResearchAgreementCreate.
type AgreementInput struct {
	Ref      string
	Covers   []string
	AgreedOn time.Time
}

// AgreementView is the view of an agreement row.
func AgreementView(a write.AgreementRow) dto.ResearchAgreementView {
	return dto.ResearchAgreementView{AgreementID: a.ID, TenantID: a.TenantID, AgreementRef: a.Ref, Covers: a.Covers,
		AgreedOn: a.AgreedOn.Format(time.DateOnly), Status: a.Status, RecordedBy: a.RecordedBy, RevokedAt: a.RevokedAt, RevokedBy: a.RevokedBy,
		RevokeReason: a.RevokeReason, CreatedAt: a.CreatedAt}
}

// CreateResearchAgreement records the agreement of the target tenant (F035-AC-06): X-Tenant-Id required, a unique
// reference in the tenant (409), 1–2 coverages; audit research_agreement.created.
func (h Efficacy) CreateResearchAgreement(ctx context.Context, in AgreementInput) (out dto.ResearchAgreementView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateResearchAgreement")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	tenant, err := targetTenant(p)
	if err != nil {
		return out, err
	}
	if n := len([]rune(in.Ref)); n < 1 || n > efficacy.MaxRefLen {
		return out, problem.Invalid("/agreement_ref", problem.SubInvalid, "must have 1 to 128 characters")
	}
	if !efficacy.ValidCovers(in.Covers) {
		return out, problem.Invalid("/covers", problem.SubInvalid, "lists efficacy_study and/or experiment once each")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if qa, err := h.Store.TenantIsQA(ctx, tenant); err != nil || qa {
			if err == nil {
				err = problem.Invalid("/tenant_id", problem.SubInvalid, "a QA tenant takes no part in research")
			}
			return err
		}
		if prev, err := h.Store.AgreementByRef(ctx, tenant, in.Ref); err != nil || prev != nil {
			if err == nil {
				err = problem.New(problem.Conflict, "the agreement reference is already recorded for the tenant")
			}
			return err
		}
		a := write.AgreementRow{ID: ids.New(), TenantID: tenant, Ref: in.Ref, Covers: in.Covers, AgreedOn: in.AgreedOn, Status: efficacy.AgreementActive,
			RecordedBy: p.UserID, Version: 1, CreatedAt: h.now()}
		if err := h.Store.InsertAgreement(ctx, a); err != nil {
			return err
		}
		if err := h.auditTenant(ctx, tenant, ActionAgreementCreated, "research_agreement", a.ID, "", map[string]any{"covers": a.Covers}); err != nil {
			return err
		}
		out = AgreementView(a)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// RevokeResearchAgreement revokes an agreement of the target tenant (F035-AC-20): every participant of the tenant leaves
// every running study (as F035-AC-18, scope tenant), the students leave every experiment; audit
// research_agreement.revoked.
func (h Efficacy) RevokeResearchAgreement(ctx context.Context, id uuid.UUID, reason string) (out dto.ResearchAgreementView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RevokeResearchAgreement")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	tenant, err := targetTenant(p)
	if err != nil {
		return out, err
	}
	if !efficacy.ValidReason(reason) {
		return out, problem.Invalid("/reason", problem.SubInvalid, "must have 10 to 1000 characters")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.Store.Agreement(ctx, tenant, id, true)
		if err != nil {
			return err
		}
		if a == nil {
			return problem.New(problem.NotFound, "")
		}
		if a.Status == efficacy.AgreementRevoked {
			return problem.New(problem.Conflict, "the agreement is already revoked")
		}
		now := h.now()
		prev := a.Version
		a.Status, a.RevokedAt, a.RevokedBy, a.RevokeReason = efficacy.AgreementRevoked, &now, &p.UserID, &reason
		if ok, err := h.Store.UpdateAgreement(ctx, *a, prev); err != nil || !ok {
			if err == nil {
				err = problem.New(problem.VersionMismatch, "")
			}
			return err
		}
		affected, cancelled, err := h.withdraw(ctx, p, write.WithdrawScope{TenantID: tenant}, now)
		if err != nil {
			return err
		}
		left := 0
		if h.Experiments != nil {
			if left, err = h.Experiments.LeaveTenant(ctx, tenant, now); err != nil {
				return err
			}
		}
		key := idemKey(ctx)
		if key == "" {
			key = "revoke:" + a.ID.String()
		}
		w := write.WithdrawalRow{ID: ids.New(), TenantID: tenant, Scope: efficacy.ScopeTenant, Reason: reason, Affected: affected,
			CancelledSessions: cancelled, RequestedBy: p.UserID, IdempotencyKey: truncateRunes(key, 64), CreatedAt: now}
		if err := h.Store.InsertWithdrawal(ctx, w); err != nil {
			return err
		}
		if err := h.auditTenant(ctx, tenant, ActionAgreementRevoked, "research_agreement", a.ID, reason, map[string]any{
			"withdrawn_count": affected, "cancelled_exam_session_count": cancelled, "experiment_assignments_removed": left}); err != nil {
			return err
		}
		out = AgreementView(*a)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// ---------- studies (US2)

// StudyInput is EfficacyStudyCreate.
type StudyInput struct {
	efficacy.StudyInput
	IdempotencyKey string
}

// StudyViewOf is the view of a study with its cohorts.
func StudyViewOf(s write.StudyRow, cohorts []write.StudyCohortRow) dto.EfficacyStudyView {
	v := dto.EfficacyStudyView{StudyID: s.ID, Code: s.Code, Title: s.Title, SubjectID: s.SubjectID, GradeLevel: s.GradeLevel,
		ObjectiveIDs: s.ObjectiveIDs, FormA: s.FormA, FormB: s.FormB, MinWeeks: s.MinWeeks, Status: s.Status, CreatedBy: s.CreatedBy,
		CreatedAt: s.CreatedAt, ActivatedAt: s.ActivatedAt, CompletedAt: s.CompletedAt, Version: s.Version, TenantIDs: []uuid.UUID{}}
	if v.ObjectiveIDs == nil {
		v.ObjectiveIDs = []uuid.UUID{}
	}
	pairs := map[int]bool{}
	for _, c := range cohorts {
		v.CohortCount++
		if c.PairNo != nil {
			pairs[*c.PairNo] = true
		}
		if !slices.Contains(v.TenantIDs, c.TenantID) {
			v.TenantIDs = append(v.TenantIDs, c.TenantID)
		}
	}
	v.PairCount = len(pairs)
	return v
}

// checkForms applies F035-AC-07 to the two forms of a study (platform scope: the instruments are platform content).
func (h Efficacy) checkForms(ctx context.Context, s efficacy.StudyInput) error {
	var vs []efficacy.Violation
	for _, f := range []struct {
		field string
		id    uuid.UUID
	}{{"/form_a_blueprint_version_id", s.FormA}, {"/form_b_blueprint_version_id", s.FormB}} {
		form, err := h.form(ctx, f.id, s.GradeLevel)
		if err != nil {
			return err
		}
		vs = append(vs, efficacy.CheckForm(f.field, s, form, h.config().MinQuestionsPerObjective)...)
	}
	return efficacyViolations(vs)
}

// form reads a blueprint version as a form: published, efficacy flag, subject, grade and questions per objective.
func (h Efficacy) form(ctx context.Context, versionID uuid.UUID, grade int) (efficacy.Form, error) {
	var out efficacy.Form
	v, err := h.Content.FormVersion(ctx, versionID)
	if err != nil || v == nil {
		return out, err
	}
	var body struct {
		Definition json.RawMessage `json:"definition"`
		Efficacy   bool            `json:"efficacy_instrument"`
	}
	_ = json.Unmarshal(v.Body, &body)
	out.Published, out.Efficacy = v.Status == "published", body.Efficacy
	if v.SubjectID != nil {
		out.SubjectID = *v.SubjectID
	}
	if len(v.GradeLevels) > 0 {
		g := v.GradeLevels[0]
		if slices.Contains(v.GradeLevels, grade) {
			g = grade
		}
		out.GradeLevel = &g
	}
	var fixed []uuid.UUID
	if def, perr := blueprint.Parse(body.Definition); perr == nil {
		fixed = def.FixedQuestions()
	} else {
		out.Efficacy = false // an unreadable definition is no efficacy form (422 on the field, not an error)
	}
	qs, err := h.Content.InstrumentQuestions(ctx, fixed)
	if err != nil {
		return out, err
	}
	out.PerObjective = map[uuid.UUID]int{}
	for _, id := range fixed {
		q, ok := qs[id]
		if !ok || !q.Efficacy {
			out.Efficacy = false
			continue
		}
		for _, o := range q.ObjectiveIDs {
			out.PerObjective[o]++
		}
	}
	return out, nil
}

// CreateEfficacyStudy creates a draft study (F035-AC-07): the fields, a unique code (409), the two forms checked;
// idempotent by Idempotency-Key; audit efficacy_study.created.
func (h Efficacy) CreateEfficacyStudy(ctx context.Context, in StudyInput) (out dto.EfficacyStudyView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateEfficacyStudy")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	if err := efficacyViolations(in.Validate()); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if in.IdempotencyKey != "" {
			if prev, err := h.Store.StudyByKey(ctx, p.UserID, in.IdempotencyKey); err != nil || prev != nil {
				if err == nil {
					out = StudyViewOf(*prev, nil)
				}
				return err
			}
		}
		if taken, err := h.Store.StudyCodeTaken(ctx, in.Code, uuid.Nil); err != nil || taken {
			if err == nil {
				err = problem.New(problem.Conflict, "the study code is taken")
			}
			return err
		}
		if err := h.checkForms(ctx, in.StudyInput); err != nil {
			return err
		}
		s := write.StudyRow{ID: ids.New(), Code: in.Code, Title: in.Title, SubjectID: in.SubjectID, GradeLevel: in.GradeLevel,
			ObjectiveIDs: in.ObjectiveIDs, FormA: in.FormA, FormB: in.FormB, MinWeeks: in.MinWeeks, Status: efficacy.StudyDraft,
			CreatedBy: p.UserID, IdempotencyKey: truncateRunes(in.IdempotencyKey, 64), Version: 1, CreatedAt: h.now()}
		if s.IdempotencyKey == "" {
			s.IdempotencyKey = s.ID.String()
		}
		if err := h.Store.InsertStudy(ctx, s); err != nil {
			return err
		}
		if err := h.auditPlatform(ctx, ActionStudyCreated, "efficacy_study", s.ID, "", map[string]any{"code": s.Code}); err != nil {
			return err
		}
		out = StudyViewOf(s, nil)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// StudyPatch is EfficacyStudyUpdate (merge patch; nil = unchanged).
type StudyPatch struct {
	Title        *string
	ObjectiveIDs []uuid.UUID
	FormA, FormB *uuid.UUID
	MinWeeks     *int
}

// UpdateEfficacyStudy edits a draft study (If-Match; another status → 422 INVALID_STATE_TRANSITION); the result is
// checked as at creation; audit efficacy_study.updated.
func (h Efficacy) UpdateEfficacyStudy(ctx context.Context, id uuid.UUID, ifMatch int64, in StudyPatch) (out dto.EfficacyStudyView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateEfficacyStudy")
	defer func() { done(err) }()
	if _, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Study(ctx, id, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if s.Version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if s.Status != efficacy.StudyDraft {
			return problem.New(problem.InvalidStateTransition, "only a draft study is edited")
		}
		if in.Title != nil {
			s.Title = *in.Title
		}
		if in.ObjectiveIDs != nil {
			s.ObjectiveIDs = in.ObjectiveIDs
		}
		if in.FormA != nil {
			s.FormA = *in.FormA
		}
		if in.FormB != nil {
			s.FormB = *in.FormB
		}
		if in.MinWeeks != nil {
			s.MinWeeks = *in.MinWeeks
		}
		si := efficacy.StudyInput{Code: s.Code, Title: s.Title, SubjectID: s.SubjectID, GradeLevel: s.GradeLevel, ObjectiveIDs: s.ObjectiveIDs,
			FormA: s.FormA, FormB: s.FormB, MinWeeks: s.MinWeeks}
		if err := efficacyViolations(si.Validate()); err != nil {
			return err
		}
		if err := h.checkForms(ctx, si); err != nil {
			return err
		}
		prev := s.Version
		if ok, err := h.Store.UpdateStudy(ctx, *s, prev); err != nil || !ok {
			if err == nil {
				err = problem.New(problem.VersionMismatch, "")
			}
			return err
		}
		s.Version = prev + 1
		if err := h.auditPlatform(ctx, ActionStudyUpdated, "efficacy_study", s.ID, "", nil); err != nil {
			return err
		}
		cohorts, err := h.Store.StudyCohorts(ctx, s.ID)
		if err != nil {
			return err
		}
		out = StudyViewOf(*s, cohorts)
		return nil
	})
	return out, err
}

// CohortInput is EfficacyStudyCohortCreate: a school of the target tenant (the grade is the grade of the study).
type CohortInput struct {
	SchoolID  uuid.UUID
	PairNo    *int
	Arm       string
	FormOrder string
}

// StudyCohortView is the view of a cohort row.
func StudyCohortView(c write.StudyCohortRow) dto.EfficacyStudyCohortView {
	return dto.EfficacyStudyCohortView{StudyID: c.StudyID, Conflict: c.Conflict, TenantID: c.TenantID, SchoolID: c.SchoolID, GradeLevel: c.GradeLevel,
		PairNo: c.PairNo, Arm: c.Arm, AssignmentMethod: c.Method, FormOrder: c.FormOrder, Status: c.Status, WithdrawnAt: c.WithdrawnAt}
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// AddEfficacyStudyCohort adds a cohort (a school of the target tenant, the grade of the study — ADR-0095 D3) to a draft
// study (F035-AC-08): a school of another tenant → 404, a school without active student of the grade or a tenant without
// an active agreement covering efficacy_study (or a QA tenant) → 422, a cohort in another draft or active study or already
// in this one → 409, a study no longer draft → 422 INVALID_STATE_TRANSITION.
func (h Efficacy) AddEfficacyStudyCohort(ctx context.Context, studyID uuid.UUID, in CohortInput) (out dto.EfficacyStudyCohortView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.AddEfficacyStudyCohort")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return out, err
	}
	tenant, err := targetTenant(p)
	if err != nil {
		return out, err
	}
	c, vs := efficacy.NewCohort(efficacy.Cohort{SchoolID: in.SchoolID, TenantID: tenant, PairNo: in.PairNo, Arm: in.Arm, FormOrder: in.FormOrder})
	if err := efficacyViolations(vs); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Study(ctx, studyID, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if s.Status != efficacy.StudyDraft {
			return problem.New(problem.InvalidStateTransition, "cohorts are added to a draft study only")
		}
		ref, err := h.Store.School(ctx, tenant, in.SchoolID, s.GradeLevel)
		if err != nil {
			return err
		}
		if ref == nil {
			return problem.New(problem.NotFound, "")
		}
		if ref.Students == 0 {
			return problem.Invalid("/school_id", problem.SubInvalid, "the school has no active student of the grade of the study")
		}
		if qa, err := h.Store.TenantIsQA(ctx, tenant); err != nil || qa {
			if err == nil {
				err = problem.Invalid("/school_id", problem.SubInvalid, "a QA tenant takes no part in research")
			}
			return err
		}
		as, err := h.Store.Agreements(ctx, tenant)
		if err != nil {
			return err
		}
		if !efficacy.Covers(agreementsOf(as), efficacy.CoversStudy) {
			return problem.Invalid("/school_id", problem.SubInvalid, "the tenant has no active agreement covering efficacy_study")
		}
		if other, err := h.Store.CohortInOtherStudy(ctx, tenant, in.SchoolID, s.GradeLevel, studyID); err != nil || other {
			if err == nil {
				err = problem.New(problem.Conflict, "the cohort takes part in another running study")
			}
			return err
		}
		if cur, err := h.Store.StudyCohort(ctx, studyID, in.SchoolID, false); err != nil || cur != nil {
			if err == nil {
				err = problem.New(problem.Conflict, "the cohort is already in the study")
			}
			return err
		}
		row := write.StudyCohortRow{StudyID: studyID, SchoolID: in.SchoolID, TenantID: tenant, GradeLevel: s.GradeLevel,
			PairNo: c.PairNo, Arm: optString(c.Arm), Method: c.Method, FormOrder: optString(c.FormOrder), Status: "enrolled", AddedBy: p.UserID}
		if err := h.Store.UpsertStudyCohort(ctx, row); err != nil {
			return err
		}
		if err := h.auditTenant(ctx, tenant, ActionStudyCohortAdded, "efficacy_study", studyID, "", map[string]any{"school_id": in.SchoolID, "grade_level": s.GradeLevel,
			"pair_no": c.PairNo, "arm": c.Arm}); err != nil {
			return err
		}
		out = StudyCohortView(row)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func agreementsOf(rows []write.AgreementRow) []efficacy.Agreement {
	out := make([]efficacy.Agreement, len(rows))
	for i, a := range rows {
		out[i] = efficacy.Agreement{Status: a.Status, Covers: a.Covers}
	}
	return out
}

// RemoveEfficacyStudyCohort removes a cohort (school) of the target tenant from a draft study (soft-delete, ADR-0028);
// a cohort flagged conflict is resolved this way (removed, then added again with its arm).
func (h Efficacy) RemoveEfficacyStudyCohort(ctx context.Context, studyID, schoolID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RemoveEfficacyStudyCohort")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage)
	if err != nil {
		return err
	}
	tenant, err := targetTenant(p)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Study(ctx, studyID, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		c, err := h.Store.StudyCohort(ctx, studyID, schoolID, false)
		if err != nil {
			return err
		}
		if c == nil || c.TenantID != tenant {
			return problem.New(problem.NotFound, "")
		}
		if s.Status != efficacy.StudyDraft {
			return problem.New(problem.InvalidStateTransition, "cohorts are removed from a draft study only")
		}
		if err := h.Store.RemoveStudyCohort(ctx, *c, p.UserID); err != nil {
			return err
		}
		return h.auditTenant(ctx, tenant, ActionStudyCohortRemoved, "efficacy_study", studyID, "", map[string]any{"school_id": schoolID})
	})
}

func studyTenants(cohorts []write.StudyCohortRow) []uuid.UUID {
	var out []uuid.UUID
	for _, c := range cohorts {
		if !slices.Contains(out, c.TenantID) {
			out = append(out, c.TenantID)
		}
	}
	return out
}

// ActivateEfficacyStudy activates a draft study (F035-AC-09): every pair checked, the arms left empty assigned by the
// stable hash of (study_id, pair_no), the form orders alternating by pair; status active; audit efficacy_study.activated.
// An active study is never activated again (422 INVALID_STATE_TRANSITION; the Idempotency-Key replay answers as before).
func (h Efficacy) ActivateEfficacyStudy(ctx context.Context, id uuid.UUID) (out dto.EfficacyStudyView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ActivateEfficacyStudy")
	defer func() { done(err) }()
	if _, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		s, err := h.Store.Study(ctx, id, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if s.Status != efficacy.StudyDraft {
			return problem.New(problem.InvalidStateTransition, "only a draft study is activated")
		}
		rows, err := h.Store.StudyCohorts(ctx, id)
		if err != nil {
			return err
		}
		cohorts := make([]efficacy.Cohort, len(rows))
		for i, r := range rows {
			cohorts[i] = efficacy.Cohort{SchoolID: r.SchoolID, TenantID: r.TenantID, Conflict: r.Conflict, PairNo: r.PairNo, Method: r.Method,
				GradeLevel: r.GradeLevel}
			if r.Arm != nil {
				cohorts[i].Arm = *r.Arm
			}
			if r.FormOrder != nil {
				cohorts[i].FormOrder = *r.FormOrder
			}
		}
		assigned, vs := efficacy.Activate(id, cohorts)
		if err := efficacyViolations(vs); err != nil {
			return err
		}
		for i, c := range assigned {
			r := rows[i]
			arm, order := c.Arm, c.FormOrder
			r.Arm, r.FormOrder, r.Method = &arm, &order, c.Method
			if err := h.Store.UpdateStudyCohort(ctx, r); err != nil {
				return err
			}
			rows[i] = r
		}
		now, prev := h.now(), s.Version
		s.Status, s.ActivatedAt = efficacy.StudyActive, &now
		if ok, err := h.Store.UpdateStudy(ctx, *s, prev); err != nil || !ok {
			if err == nil {
				err = problem.New(problem.VersionMismatch, "")
			}
			return err
		}
		s.Version = prev + 1
		counts := efficacy.ArmCounts(assigned)
		if err := h.auditPlatform(ctx, ActionStudyActivated, "efficacy_study", s.ID, "", map[string]any{"intervention_cohorts": counts[efficacy.ArmIntervention],
			"comparison_cohorts": counts[efficacy.ArmComparison], "validation_cohorts": counts[efficacy.ArmValidation],
			"tenant_ids": studyTenants(rows)}); err != nil {
			return err
		}
		out = StudyViewOf(*s, rows)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// CompleteEfficacyStudy completes an active study once its post wave is closed (422 INVALID_STATE_TRANSITION otherwise);
// audit efficacy_study.completed. The efficacy exam sessions stay unreleased (LEARNING_EFFICACY_PLAN §8).
func (h Efficacy) CompleteEfficacyStudy(ctx context.Context, id uuid.UUID) (out dto.EfficacyStudyView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CompleteEfficacyStudy")
	defer func() { done(err) }()
	if _, err := efficacyAdmin(ctx, authz.PermPlatformEfficacyStudyManage); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Study(ctx, id, true)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		ws, err := h.Store.Waves(ctx, id)
		if err != nil {
			return err
		}
		closed := false
		for _, w := range ws {
			closed = closed || (w.Kind == efficacy.WavePost && w.Status == efficacy.WaveClosed)
		}
		if s.Status != efficacy.StudyActive || !closed {
			return problem.New(problem.InvalidStateTransition, "an active study is completed once its post wave is closed")
		}
		now, prev := h.now(), s.Version
		s.Status, s.CompletedAt = efficacy.StudyCompleted, &now
		if ok, err := h.Store.UpdateStudy(ctx, *s, prev); err != nil || !ok {
			if err == nil {
				err = problem.New(problem.VersionMismatch, "")
			}
			return err
		}
		s.Version = prev + 1
		if err := h.auditPlatform(ctx, ActionStudyCompleted, "efficacy_study", s.ID, "", nil); err != nil {
			return err
		}
		out = StudyViewOf(*s, nil)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}
