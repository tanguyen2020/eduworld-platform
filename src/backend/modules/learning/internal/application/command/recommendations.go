package command

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// EventRecommendationGenerated is the domain event of a changed recommendation list (payload
// learning.recommendation_generated.v1, owner F032).
const EventRecommendationGenerated = "learning.recommendation_generated"

// Statuses of a recommendation (F032 data-model §2).
const (
	recActive     = "active"
	recStarted    = "started"
	recCompleted  = "completed"
	recDismissed  = "dismissed"
	recExpired    = "expired"
	recSuperseded = "superseded"
)

// Recommendations computes the automatic study recommendations of students (F032 US6, LEARNING_ANALYTICS §6) with the
// rule engine, in the write context of a consumer or job (TenantScope of the student's tenant).
type Recommendations struct {
	UoW       write.UnitOfWork
	Store     write.RecommendationStore
	Wrong     write.WrongQuestionStore
	Reader    write.SelfStudyReader
	Masteries write.MasteryStore
	Delivery  port.QuestionDelivery
	Students  port.StudentDirectory
	Events    write.EventPublisher
	Engine    *recommendation.Engine
	Config    PracticeConfig
	Clock     func() time.Time
	Metrics   SelfStudyMetrics
	// Sources fill the snapshot with the data of the rules of other features (F041 R1, F042 R5).
	Sources []SnapshotSource
	// Experience resolves the live experiment arms of the student (F035-AC-24); nil = no experiment.
	Experience ExperienceResolver
}

func (h Recommendations) now() time.Time { return now(h.Clock).UTC() }

// AttemptTrigger is a graded or re-evaluated attempt of a student: it completes the recommendation it followed and counts
// a skip for the active recommendations started before it and not followed.
type AttemptTrigger struct {
	StudentID        uuid.UUID
	AttemptID        uuid.UUID
	RecommendationID *uuid.UUID
	StartedAt        time.Time
	Graded           bool
}

// OnAttempt runs after the mastery of a graded (or re-evaluated) attempt was updated, in the same transaction
// (F032-AC-30): the followed recommendation becomes completed, then the list is computed again.
func (h Recommendations) OnAttempt(ctx context.Context, t AttemptTrigger) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.RecomputeStudentRecommendations")
	defer func() { done(err) }()
	if !t.Graded && h.Wrong != nil {
		if err := h.clearReEvaluated(ctx, t.AttemptID); err != nil {
			return err
		}
	}
	if t.Graded {
		r, err := h.Store.RecommendationByAttempt(ctx, t.AttemptID)
		if err != nil {
			return err
		}
		if r != nil && r.Status == recStarted {
			r.Status = recCompleted
			if _, err := h.Store.UpdateRecommendation(ctx, *r); err != nil {
				return err
			}
		}
	}
	return h.Recompute(ctx, t.StudentID, &t)
}

// Recompute evaluates the rules for a student and stores the result (F032-AC-29…AC-31): a candidate matching an active
// recommendation keeps it (same recommendation_id), new candidates are inserted, active recommendations no rule proposes
// any more are superseded, expired ones expire, a target skipped Skips times is dismissed and never proposed again.
// learning.recommendation_generated is published when the active list changed.
func (h Recommendations) Recompute(ctx context.Context, studentID uuid.UUID, trigger *AttemptTrigger) error {
	school, grade, err := h.Students.StudentSchool(ctx, studentID)
	if err != nil || school == nil {
		return err
	}
	snap, err := h.snapshot(ctx, studentID, grade)
	if err != nil {
		return err
	}
	cfg := h.Config
	blockedKeys, err := h.Store.BlockedTargets(ctx, studentID, cfg.Skips)
	if err != nil {
		return err
	}
	cands := h.Engine.Run(snap.Snapshot, func(c recommendation.Candidate) bool { return slices.Contains(blockedKeys, c.Key()) })
	// F035-AC-24: max_recommendations of a live recommendation_rule arm keeps the first candidates only.
	stored := snap.Params.Limit(cfg.Stored)
	if len(cands) > stored {
		cands = cands[:stored]
	}
	active, err := h.Store.ActiveRecommendations(ctx, studentID)
	if err != nil {
		return err
	}
	now := h.now()
	changed := false
	byKey := map[string]int{}
	for i, c := range cands {
		byKey[c.RuleCode+"|"+c.Key()] = i
	}
	kept := map[int]bool{}
	var list []write.RecommendationRow
	for _, r := range active {
		key := r.RuleCode + "|" + r.Kind + "|" + r.TargetType + "|" + r.TargetID.String()
		i, ok := byKey[key]
		switch {
		case !now.Before(r.ExpiresAt):
			r.Status = recExpired
		case !ok:
			r.Status = recSuperseded
		default:
			kept[i] = true
			c := cands[i]
			r.Priority, r.ObjectiveIDs, r.ReasonParams = c.Priority, c.ObjectiveIDs, reasonParams(c.Params)
			if trigger != nil && trigger.StartedAt.After(r.GeneratedAt) && (trigger.RecommendationID == nil || *trigger.RecommendationID != r.ID) {
				r.SkipCount++
				if r.SkipCount >= cfg.Skips {
					r.Status = recDismissed
				}
			}
		}
		if r.Status != recActive {
			r.ClosedAt = &now
			changed = true
		}
		if _, err := h.Store.UpdateRecommendation(ctx, r); err != nil {
			return err
		}
		if r.Status == recActive {
			list = append(list, r)
		}
	}
	for i, c := range cands {
		if kept[i] || len(list) >= stored {
			continue
		}
		r := write.RecommendationRow{ID: ids.New(), TenantID: snap.TenantID, SchoolID: *school, StudentID: studentID, RuleCode: c.RuleCode,
			Kind: c.Kind, TargetType: c.TargetType, TargetID: c.TargetID, SubjectID: c.SubjectID, Grade: c.Grade, ObjectiveIDs: c.ObjectiveIDs,
			Reason: c.Reason, ReasonCode: c.ReasonCode, ReasonParams: reasonParams(c.Params), Priority: c.Priority, Status: recActive,
			GeneratedAt: now, ExpiresAt: now.Add(cfg.RecommendationTTL)}
		if err := h.Store.InsertRecommendation(ctx, r); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.RecommendationGenerated(c.RuleCode)
		}
		list = append(list, r)
		changed = true
	}
	if !changed {
		return nil
	}
	return h.generated(ctx, snap.TenantID, studentID, list, now)
}

// clearReEvaluated takes out of the review list the questions a re-evaluation turned correct (F032 edge case).
func (h Recommendations) clearReEvaluated(ctx context.Context, attemptID uuid.UUID) error {
	entries, err := h.Wrong.AttemptEntries(ctx, attemptID)
	if err != nil || len(entries) == 0 {
		return err
	}
	correct, err := h.Reader.ReEvaluatedCorrect(ctx, attemptID)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !slices.Contains(correct, e.QuestionVersionID) || !e.Entry.Clear() {
			continue
		}
		if _, err := h.Wrong.UpdateEntry(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func reasonParams(p map[string]string) json.RawMessage {
	if len(p) == 0 {
		return json.RawMessage(`{}`)
	}
	b, _ := json.Marshal(p)
	return b
}

// generated publishes learning.recommendation_generated with the active list in priority order (no reason text, R14).
func (h Recommendations) generated(ctx context.Context, tenant, studentID uuid.UUID, list []write.RecommendationRow, at time.Time) error {
	if h.Events == nil {
		return nil
	}
	slices.SortStableFunc(list, func(a, b write.RecommendationRow) int { return a.Priority - b.Priority })
	idsOut, rules, refs := []uuid.UUID{}, []string{}, []map[string]any{}
	objectives := []uuid.UUID{}
	for _, r := range list {
		idsOut = append(idsOut, r.ID)
		rules = append(rules, r.RuleCode)
		refs = append(refs, map[string]any{"kind": r.Kind, "target_type": r.TargetType, "target_id": r.TargetID})
		for _, o := range r.ObjectiveIDs {
			if !slices.Contains(objectives, o) && len(objectives) < 100 {
				objectives = append(objectives, o)
			}
		}
	}
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: EventRecommendationGenerated, SubjectType: "student", SubjectID: studentID.String(),
		TenantID: &t, Payload: map[string]any{"student_id": studentID, "recommendation_ids": idsOut, "rule_codes": rules,
			"objective_ids": objectives, "content_refs": refs, "generated_at": at}})
}

// studentSnapshot is the snapshot of the rules and the tenant of the student.
type studentSnapshot struct {
	recommendation.Snapshot
	TenantID uuid.UUID
}

// snapshot reads the learning state of a student: the objectives of the grade (and of every objective with mastery)
// with their prerequisites, the mastery after decay, the questions available to the tenant per objective, the due
// reviews and the latest diagnostics.
func (h Recommendations) snapshot(ctx context.Context, studentID uuid.UUID, grade *int) (studentSnapshot, error) {
	out := studentSnapshot{Snapshot: recommendation.Snapshot{Now: h.now(), Grade: grade}}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		out.TenantID, _ = s.SessionTenantID()
	}
	cfg, err := h.Masteries.ActiveModel(ctx)
	if err != nil {
		return out, err
	}
	m, err := ModelOf(cfg)
	if err != nil {
		return out, err
	}
	var arm ExperienceArm
	if h.Experience != nil && out.TenantID != uuid.Nil {
		// F035-AC-24: the recommendations of a student of a live arm follow the arm model and rule_params.
		if arm, err = h.Experience.Experience(ctx, out.TenantID, studentID); err != nil {
			return out, err
		}
		out.Params = arm.Rules
	}
	rows, err := h.Reader.StudentMasteries(ctx, studentID)
	if err != nil {
		return out, err
	}
	byObjective := map[uuid.UUID]write.MasteryRow{}
	var withMastery []uuid.UUID
	for _, r := range rows {
		byObjective[r.ObjectiveID] = r
		withMastery = append(withMastery, r.ObjectiveID)
	}
	var infos []write.ObjectiveInfo
	if grade != nil {
		if infos, err = h.Reader.Objectives(ctx, write.ObjectiveFilter{Grade: grade, WithPrerequisites: true}); err != nil {
			return out, err
		}
	}
	if len(withMastery) > 0 {
		more, err := h.Reader.Objectives(ctx, write.ObjectiveFilter{ObjectiveIDs: withMastery, WithPrerequisites: true})
		if err != nil {
			return out, err
		}
		infos = append(infos, more...)
	}
	pool, err := h.Delivery.Pool(ctx, nil)
	if err != nil {
		return out, err
	}
	available := map[uuid.UUID]bool{}
	for _, c := range pool {
		if c.ExamOnly {
			continue
		}
		for _, o := range c.ObjectiveIDs {
			available[o] = true
		}
	}
	seen, topics := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, o := range infos {
		if seen[o.ID] {
			continue
		}
		seen[o.ID] = true
		ob := recommendation.Objective{ID: o.ID, Code: o.Code, Name: o.Name, TopicID: o.TopicID, SubjectID: o.SubjectID, Grade: o.Grade,
			Prerequisites: o.Prerequisites, Level: "not_started", StoredLevel: "not_started", Available: available[o.ID]}
		if r, ok := byObjective[o.ID]; ok {
			mm, st := arm.Mastery(m, o.ID, r.State)
			v, level := mm.Effective(st, out.Now)
			stored := r.Level
			if arm.Model != nil && st.LastEvidenceAt != nil {
				_, stored = mm.Effective(st, *st.LastEvidenceAt)
			}
			ob.Level, ob.Value, ob.StoredLevel, ob.StoredValue = string(level), v, string(stored), st.Value
			ob.EvidenceCount, ob.LastEvidenceAt, ob.PriorSource, ob.PriorValue = st.EvidenceCount, st.LastEvidenceAt, st.PriorSource,
				st.PriorValue
		}
		out.Objectives = append(out.Objectives, ob)
		if !topics[o.TopicID] {
			topics[o.TopicID] = true
			out.Topics = append(out.Topics, recommendation.Topic{ID: o.TopicID, SubjectID: o.SubjectID, Grade: o.Grade, SortOrder: o.TopicOrder,
				Name: o.TopicName})
		}
	}
	due, err := h.Wrong.DueEntries(ctx, studentID, nil, out.Now, 500)
	if err != nil {
		return out, err
	}
	bySubject := map[uuid.UUID]*recommendation.WrongDue{}
	for _, e := range due {
		if e.SubjectID == nil {
			continue
		}
		w, ok := bySubject[*e.SubjectID]
		if !ok {
			w = &recommendation.WrongDue{SubjectID: *e.SubjectID}
			bySubject[*e.SubjectID] = w
		}
		w.Count++
		for _, o := range e.ObjectiveIDs {
			if !slices.Contains(w.ObjectiveIDs, o) && len(w.ObjectiveIDs) < recommendation.MaxObjectives {
				w.ObjectiveIDs = append(w.ObjectiveIDs, o)
			}
		}
	}
	for _, w := range bySubject {
		out.WrongDue = append(out.WrongDue, *w)
	}
	slices.SortFunc(out.WrongDue, func(a, b recommendation.WrongDue) int { return compareID(a.SubjectID, b.SubjectID) })
	if out.LastDiagnostic, err = h.Reader.LastDiagnostics(ctx, studentID); err != nil {
		return out, err
	}
	for _, src := range h.Sources {
		if err := src.Fill(ctx, studentID, &out.Snapshot); err != nil {
			return out, err
		}
	}
	return out, nil
}

func compareID(a, b uuid.UUID) int {
	for i := range a {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

// NightlyWindowDays: the nightly job recomputes the students with evidence in the last 180 days, an active
// recommendation or an active review entry [Proposed].
const NightlyWindowDays = 180

// RecommendationReport counts one run of the job recommendation-nightly.
type RecommendationReport struct {
	Students, Failed int
}

// RunNightly is the job recommendation-nightly (F032-AC-30, R4 decay and R7 warm-up need no new event): it lists the
// students with learning data or active recommendations of every tenant under tenancy.Bypass(platform_job), then
// recomputes each one in the scope of its tenant and its own transaction.
func (h Recommendations) RunNightly(ctx context.Context, limit int) (rep RecommendationReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.RecommendationNightly")
	defer func() { done(err) }()
	if limit <= 0 {
		limit = 5000
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
	if err != nil {
		return rep, err
	}
	var students []write.StudentRef
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		students, err = h.Store.RecomputeStudents(ctx, h.now().AddDate(0, 0, -NightlyWindowDays), limit)
		return err
	}); err != nil {
		return rep, err
	}
	for _, s := range students {
		sc, err := authz.EventScope(s.TenantID)
		if err != nil {
			return rep, err
		}
		if err := h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
			return h.Recompute(ctx, s.StudentID, nil)
		}); err != nil {
			rep.Failed++
			continue
		}
		rep.Students++
	}
	return rep, nil
}
