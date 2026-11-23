package command

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Quest events (specs/events/payloads/game/quest.*.v1).
const (
	EventQuestAccepted      = "quest.accepted"
	EventQuestStepCompleted = "quest.step_completed"
	EventQuestCompleted     = "quest.completed"
	EventQuestAbandoned     = "quest.abandoned"
)

// Reasons of quest.abandoned.
const (
	AbandonPlayer  = "player"
	AbandonExpired = "expired"
	AbandonFailed  = "failed"
)

// QuestConfig is the configuration of the quest engine (CONFIG_DEFAULTS through infrastructure/config).
type QuestConfig struct {
	MaxActive     int                   // max_active_quests (10)
	DailyPerDay   int                   // daily_quests_per_day (3)
	WeeklyPerWeek int                   // weekly_quests_per_week (3)
	Reset         quest.Reset           // quest_reset_time (04:00 ICT; weekly Monday 04:00)
	PassRule      quest.DefaultPassRule // quest_pass_rule (0.7, 8, 5 per day)
	BlockMax      map[string]int        // learning_block_max_items (g1_3 3, g4_5 5, g6_9 8)
	LearningRetry time.Duration         // Retry-After of startQuestActivity when module learning is unavailable
	JobBatch      int                   // progress handled per job run
	JobInterval   time.Duration         // interval of quest-period-expiry and quest-failure-timer
}

// QuestMetrics observes the quest engine; nil = not observed.
type QuestMetrics interface {
	Accept(result string)
	StepReport(stepType, result string)
	Completed(questType string)
	ProgressLag(seconds float64)
}

// Quests are the quest use cases of a student (F072): acceptQuest, abandonQuest, reportQuestStep, startQuestActivity, the
// consumers of activity.graded, world.discovery_found and game_session.started, and the jobs quest-period-expiry and
// quest-failure-timer. Progress changes only from server facts (ADR-0016); every change writes its quest.* event to the
// outbox of the same transaction (ADR-0042) and pushes quest.progress.updated after commit.
type Quests struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Content     port.QuestContent
	World       port.WorldContent
	Store       write.QuestStore
	Sessions    write.SessionRepository
	Events      write.EventPublisher
	Audit       write.Auditor
	Presence    port.PresenceStore    // nil: every report fails its presence check
	Learning    port.LearningAttempts // nil: startQuestActivity answers 503
	Realtime    port.QuestRealtime    // nil: no push
	Registry    quest.Registry        // nil: quest.DefaultRegistry
	Metrics     QuestMetrics
	Config      QuestConfig
	Now         func() time.Time
}

func (h Quests) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Quests) registry() quest.Registry {
	if h.Registry != nil {
		return h.Registry
	}
	return quest.DefaultRegistry()
}

func (h Quests) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
	}
}

func (h Quests) metric(fn func(m QuestMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

// questProblem maps the domain errors to problem codes (API_DESIGN §4.1).
func questProblem(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, quest.ErrPrecondition):
		return problem.New(problem.StepPreconditionFailed, err.Error())
	case errors.Is(err, quest.ErrInvalidState):
		return problem.New(problem.InvalidStateTransition, err.Error())
	case errors.Is(err, quest.ErrTranslationIncomplete):
		return problem.New(problem.TranslationIncomplete, err.Error())
	case errors.Is(err, quest.ErrRefInvalid):
		return problem.New(problem.ItemRefInvalid, err.Error())
	case errors.Is(err, quest.ErrInvalid):
		return problem.Invalid("payload", problem.SubInvalid, err.Error())
	}
	return err
}

// studentOf returns the projected active student of tenant (404 otherwise).
func (h Quests) studentOf(ctx context.Context, tenant, student uuid.UUID) (*port.WorldStudent, error) {
	st, err := h.World.Student(ctx, tenant, student)
	if err != nil {
		return nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return nil, problem.New(problem.NotFound, "student not found")
	}
	return st, nil
}

func repeatProblem(r string, retry *time.Time) error {
	switch r {
	case quest.RepeatAlreadyCompleted:
		return problem.New(problem.QuestAlreadyCompleted, "")
	case quest.RepeatCooldown:
		return problem.New(problem.QuestCooldown, "").With("retry_after", retry.UTC().Format(time.RFC3339))
	}
	return nil
}

// version returns the pinned version of p (always readable: published versions are never deleted).
func (h Quests) version(ctx context.Context, p *quest.Progress) (quest.Version, error) {
	row, err := h.Content.QuestVersion(ctx, p.VersionID)
	if err != nil {
		return quest.Version{}, err
	}
	if row == nil {
		return quest.Version{}, problem.New(problem.NotFound, "quest version not found")
	}
	return row.Version, nil
}

// Accept implements acceptQuest (F072-AC-02, AC-04…AC-06, AC-25): the quest must be assigned to the tenant, of the
// grade of the student, within its availability and acceptable by the player (404 otherwise, never revealing it); the
// prerequisites and learning requirements must be met (403 QUEST_PREREQUISITE_NOT_MET with the missing ones); an active
// progress of this quest is returned as is; the repeatability, then max_active_quests are applied (409). The new
// progress pins the current version (ADR-0043) and opens its first steps; quest.accepted goes to the outbox.
// Idempotency-Key is the httpx middleware.
func (h Quests) Accept(ctx context.Context, questID uuid.UUID, partyID *uuid.UUID) (out questrules.ProgressView, err error) {
	defer func() {
		result := "ok"
		if err != nil {
			result = string(codeOf(err))
		}
		h.metric(func(m QuestMetrics) { m.Accept(result) })
	}()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		st, err := h.studentOf(ctx, tenant, student)
		if err != nil {
			return err
		}
		// An active progress is returned as is, even when the quest was unassigned since (it completes on its pinned version).
		if p, err := h.Store.Active(ctx, tenant, student, questID); err != nil || p != nil {
			if err != nil {
				return err
			}
			v, err := h.version(ctx, p)
			if err != nil {
				return err
			}
			out = questrules.ViewOf(p, v)
			return setResult(ctx, out)
		}
		if ok, err := h.Content.Assigned(ctx, tenant, questID); err != nil || !ok {
			if err != nil {
				return err
			}
			return problem.New(problem.NotFound, "quest not found")
		}
		row, err := h.Content.CurrentQuest(ctx, questID)
		if err != nil {
			return err
		}
		if row == nil {
			return problem.New(problem.NotFound, "quest not found")
		}
		v := row.Version
		hist, err := h.Content.Histories(ctx, tenant, student)
		if err != nil {
			return err
		}
		f, err := questrules.Facts(ctx, h.World, h.Content, tenant, student, st, hist)
		if err != nil {
			return err
		}
		d := quest.Decide(v, questrules.Student(st), f, at)
		if !d.Visible || !v.Acceptable() {
			return problem.New(problem.NotFound, "quest not found")
		}
		if !d.Met {
			return problem.New(problem.QuestPrerequisiteNotMet, "").With("missing_requirements", d.Missing())
		}
		if err := repeatProblem(quest.CanRepeat(v, questrules.History(hist[questID], v, h.Config.Reset, at), at)); err != nil {
			return err
		}
		n, err := h.Store.CountActive(ctx, tenant, student)
		if err != nil {
			return err
		}
		if h.Config.MaxActive > 0 && n >= h.Config.MaxActive {
			return problem.New(problem.QuestActiveLimit, "").With("max_active_quests", h.Config.MaxActive)
		}
		completed := 0
		if hs := hist[questID]; hs != nil {
			completed = hs.Completed
		}
		p, err := h.start(ctx, *row, st, tenant, student, v.StartTrigger.Kind, partyID, completed+1, at)
		if err != nil {
			return err
		}
		out = questrules.ViewOf(p, v)
		return setResult(ctx, out)
	})
	return out, err
}

// start inserts a new progress on row and writes quest.accepted.
func (h Quests) start(ctx context.Context, row port.QuestRow, st *port.WorldStudent, tenant, student uuid.UUID, trigger string,
	partyID *uuid.UUID, completionNo int, at time.Time) (*quest.Progress, error) {
	v := row.Version
	rec := v.Recurrence()
	p := quest.NewProgress(v, quest.Start{ID: newID(), TenantID: tenant, StudentID: student, SchoolID: st.SchoolID, QuestID: row.QuestID,
		VersionID: row.VersionID, Trigger: trigger, PeriodKey: h.Config.Reset.PeriodKey(rec, at), ExpiresAt: h.Config.Reset.PeriodEnd(rec, at),
		CompletionNo: completionNo, PartyID: partyID, At: at})
	ok, err := h.Store.Insert(ctx, p)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, problem.New(problem.QuestAlreadyCompleted, "a progress of this quest exists for the period")
	}
	t := tenant
	if err := h.Events.Publish(ctx, write.Event{Type: EventQuestAccepted, SubjectType: "quest", SubjectID: row.QuestID.String(),
		PartitionKey: student.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"quest_progress_id": p.ID, "quest_id": p.QuestID,
			"quest_version_id": p.VersionID, "quest_type": v.QuestType, "student_id": student, "start_trigger": trigger,
			"period_key": nullable(p.PeriodKey), "party_id": partyID, "accepted_at": at}}); err != nil {
		return nil, err
	}
	return p, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Abandon implements abandonQuest (F072-AC-07): the active progress of the quest becomes abandoned (404 without one);
// attempts and learning records stay; quest.abandoned (reason player). Accepting again starts a new progress on the
// current version.
func (h Quests) Abandon(ctx context.Context, questID uuid.UUID) (out questrules.ProgressView, err error) {
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	var push map[string]any
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.Store.Active(ctx, tenant, student, questID)
		if err != nil {
			return err
		}
		if p == nil {
			return problem.New(problem.NotFound, "no active progress of this quest")
		}
		v, err := h.version(ctx, p)
		if err != nil {
			return err
		}
		if err := h.end(ctx, p, v, AbandonPlayer, "", "", at); err != nil {
			return err
		}
		out = questrules.ViewOf(p, v)
		push = progressPush(p, "", nil)
		return setResult(ctx, out)
	})
	if err == nil {
		h.push(ctx, student, push, at)
	}
	return out, err
}

// end closes an active progress (abandoned by the player, expired, failed) with quest.abandoned.
func (h Quests) end(ctx context.Context, p *quest.Progress, v quest.Version, reason, stepID, failure string, at time.Time) error {
	var err error
	switch reason {
	case AbandonExpired:
		err = p.Expire(at)
	case AbandonFailed:
		err = p.Fail(at)
	default:
		err = p.Abandon(at)
	}
	if err != nil {
		return questProblem(err)
	}
	if err := h.Store.Save(ctx, p); err != nil {
		return err
	}
	t := p.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: EventQuestAbandoned, SubjectType: "quest", SubjectID: p.QuestID.String(),
		PartitionKey: p.StudentID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"quest_progress_id": p.ID, "quest_id": p.QuestID,
			"quest_version_id": p.VersionID, "quest_type": v.QuestType, "student_id": p.StudentID, "reason": reason, "step_id": nullable(stepID),
			"failure_kind": nullable(failure), "period_key": nullable(p.PeriodKey), "abandoned_at": at}}); err != nil {
		return err
	}
	if reason == AbandonFailed {
		return h.Audit.Record(ctx, audit.Event{Action: "quest.failed", TargetType: "quest_progress", TargetID: p.ID.String(), Result: audit.ResultSuccess,
			Metadata: map[string]any{"quest_id": p.QuestID, "quest_version_id": p.VersionID, "failure_kind": failure}})
	}
	return nil
}

// record saves a changed progress and writes quest.step_completed for every completed step and quest.completed once
// (in the same transaction as the change, F072-AC-19); returns the realtime payload.
func (h Quests) record(ctx context.Context, p *quest.Progress, v quest.Version, outs []quest.Outcome, attemptID *uuid.UUID, at time.Time) (map[string]any, error) {
	if err := h.Store.Save(ctx, p); err != nil {
		return nil, err
	}
	t := p.TenantID
	var last quest.Outcome
	for _, o := range outs {
		if !o.Changed {
			continue
		}
		last = o
		if !o.StepCompleted {
			continue
		}
		s, _ := v.Step(o.StepID)
		if err := h.Events.Publish(ctx, write.Event{Type: EventQuestStepCompleted, SubjectType: "quest", SubjectID: p.QuestID.String(),
			PartitionKey: p.StudentID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"quest_progress_id": p.ID, "quest_id": p.QuestID,
				"quest_version_id": p.VersionID, "student_id": p.StudentID, "step_id": o.StepID, "step_type": s.Type, "attempt_id": attemptID,
				"completed_at": at}}); err != nil {
			return nil, err
		}
		if o.Completed {
			if err := h.completed(ctx, p, v, at); err != nil {
				return nil, err
			}
		}
	}
	return progressPush(p, last.StepID, last.Activated), nil
}

// completed writes quest.completed (rewards are an intent only: the Reward Engine F074 grants them, F072-AC-20) and the
// audit record of the completion.
func (h Quests) completed(ctx context.Context, p *quest.Progress, v quest.Version, at time.Time) error {
	objectives := v.ObjectiveIDs
	if objectives == nil {
		objectives = []string{}
	}
	rewards := v.Rewards
	if rewards == nil {
		rewards = []quest.Reward{}
	}
	var subject any
	if v.SubjectID != "" {
		subject = v.SubjectID
	}
	t := p.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: EventQuestCompleted, SubjectType: "quest", SubjectID: p.QuestID.String(),
		PartitionKey: p.StudentID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"quest_progress_id": p.ID, "quest_id": p.QuestID,
			"quest_version_id": p.VersionID, "quest_type": v.QuestType, "student_id": p.StudentID, "subject_id": subject, "objective_ids": objectives,
			"completion_no": p.CompletionNo, "period_key": nullable(p.PeriodKey), "party_id": p.PartyID, "story_chapter_code": nullable(v.StoryChapterCode),
			"area_id": nullable(v.AreaID), "rewards": rewards, "started_at": p.StartedAt, "completed_at": at}}); err != nil {
		return err
	}
	h.after(ctx, func(context.Context) { h.metric(func(m QuestMetrics) { m.Completed(v.QuestType) }) })
	return h.Audit.Record(ctx, audit.Event{Action: "quest.completed", TargetType: "quest_progress", TargetID: p.ID.String(), Result: audit.ResultSuccess,
		Metadata: map[string]any{"quest_id": p.QuestID, "quest_version_id": p.VersionID, "quest_type": v.QuestType, "completion_no": p.CompletionNo}})
}

// progressPush is the payload of quest.progress.updated.
func progressPush(p *quest.Progress, stepID string, next []string) map[string]any {
	m := map[string]any{"quest_id": p.QuestID, "quest_progress_id": p.ID, "quest_version_id": p.VersionID, "status": p.Status}
	if stepID != "" {
		st := p.Steps[stepID]
		m["step_id"], m["step_status"], m["count"] = stepID, st.Status, st.Count
	}
	if len(next) > 0 {
		m["next_step_ids"] = next
	}
	if p.CompletedAt != nil {
		m["completed_at"] = *p.CompletedAt
	}
	return m
}

func (h Quests) push(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) {
	if h.Realtime == nil || payload == nil {
		return
	}
	_ = h.Realtime.QuestProgressUpdated(ctx, student, payload, at)
}

// ReportInput is QuestStepReport.
type ReportInput struct {
	GameSessionID  uuid.UUID
	AreaID         string
	InteractableID string
	NPCID          string
}

// place is where the presence of the student puts it (empty when none, of another tenant or of another session).
func (h Quests) place(ctx context.Context, tenant uuid.UUID, pres *port.Presence, session uuid.UUID) (quest.Place, error) {
	if pres == nil || pres.TenantID != tenant || pres.GameSessionID != session {
		return quest.Place{}, nil
	}
	a, err := h.World.CurrentArea(ctx, pres.AreaID)
	if err != nil || a == nil {
		return quest.Place{}, err
	}
	pl := quest.Place{AreaID: a.Area.AreaID, NPCs: a.Area.NPCs()}
	for _, it := range a.Area.Layout.Interactables {
		pl.Interactables = append(pl.Interactables, it.ID)
	}
	return pl, nil
}

// Report implements reportQuestStep (F072-AC-09…AC-11, AC-16): the game session must be an active session of the student
// (409), the presence (enterWorldArea, F071) must be in the area of the step in this session and the reported NPC or
// interactable must belong to the step and the area (409 STEP_PRECONDITION_FAILED), the step must be active (422
// INVALID_STATE_TRANSITION). A learning or discover step never completes from a report. A repeated interactable of a
// collect step changes nothing.
func (h Quests) Report(ctx context.Context, questID uuid.UUID, stepID string, in ReportInput) (out questrules.ProgressView, err error) {
	stepType := "unknown"
	defer func() {
		result := "ok"
		if err != nil {
			result = string(codeOf(err))
		}
		h.metric(func(m QuestMetrics) { m.StepReport(stepType, result) })
	}()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	var pres *port.Presence
	if h.Presence != nil {
		if pres, err = h.Presence.Get(ctx, student); err != nil {
			return out, problem.New(problem.Unavailable, "presence store unavailable").Wrap(err).Retry()
		}
	}
	at := h.now()
	var push map[string]any
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Sessions.ByID(ctx, tenant, in.GameSessionID)
		if err != nil {
			return err
		}
		if s == nil || !s.Active() || s.StudentID != student {
			return problem.New(problem.Conflict, "game_session_id is not an active session of the student")
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.Store.Active(ctx, tenant, student, questID)
		if err != nil {
			return err
		}
		if p == nil {
			return problem.New(problem.NotFound, "no active progress of this quest")
		}
		v, err := h.version(ctx, p)
		if err != nil {
			return err
		}
		if st, ok := v.Step(stepID); ok {
			stepType = st.Type
		} else {
			return problem.New(problem.NotFound, "unknown step")
		}
		pl, err := h.place(ctx, tenant, pres, in.GameSessionID)
		if err != nil {
			return err
		}
		o, err := p.Report(v, h.registry(), stepID, quest.Report{AreaID: in.AreaID, NPCID: in.NPCID, InteractableID: in.InteractableID}, pl, at)
		if err != nil {
			return questProblem(err)
		}
		if o.Changed {
			if push, err = h.record(ctx, p, v, []quest.Outcome{o}, nil, at); err != nil {
				return err
			}
		}
		out = questrules.ViewOf(p, v)
		return setResult(ctx, out)
	})
	if err == nil {
		h.push(ctx, student, push, at)
	}
	return out, err
}

// ActivityStart is the Attempt answered by startQuestActivity.
type ActivityStart struct {
	AttemptID         uuid.UUID  `json:"attempt_id"`
	ActivityID        uuid.UUID  `json:"activity_id"`
	ActivityVersionID *uuid.UUID `json:"activity_version_id,omitempty"`
	Mode              string     `json:"mode"`
	Status            string     `json:"status"`
	ContextType       string     `json:"context_type"`
	ContextID         uuid.UUID  `json:"context_id"`
	StartedAt         time.Time  `json:"started_at"`
	DeadlineAt        *time.Time `json:"deadline_at,omitempty"`
	ServerTime        time.Time  `json:"server_time"`
}

// StartActivity implements startQuestActivity (F072-AC-14, AC-17, AC-18): the step must be an active learning step of
// the active progress (409 STEP_PRECONDITION_FAILED / 422 INVALID_STATE_TRANSITION); max_attempts_per_day of the pass
// rule counts the attempts of the step started today (409 QUEST_STEP_ATTEMPT_LIMIT). createAttempt of module learning
// (mode quest, context quest_step, the content pinned in the quest version, Idempotency-Key forwarded) runs outside the
// transaction; its failure answers 503 with Retry-After and changes nothing; then the attempt is linked to the step.
func (h Quests) StartActivity(ctx context.Context, questID uuid.UUID, stepID string, gameSession *uuid.UUID) (out ActivityStart, err error) {
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	day := h.Config.Reset.PeriodKey(quest.RecurrenceDaily, at)
	var req port.LearningAttempt
	var progressID uuid.UUID
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if gameSession != nil {
			s, err := h.Sessions.ByID(ctx, tenant, *gameSession)
			if err != nil {
				return err
			}
			if s == nil || !s.Active() || s.StudentID != student {
				return problem.New(problem.Conflict, "game_session_id is not an active session of the student")
			}
		}
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.Store.Active(ctx, tenant, student, questID)
		if err != nil {
			return err
		}
		if p == nil {
			return problem.New(problem.NotFound, "no active progress of this quest")
		}
		v, err := h.version(ctx, p)
		if err != nil {
			return err
		}
		if _, ok := v.Step(stepID); !ok {
			return problem.New(problem.NotFound, "unknown step")
		}
		s, err := p.CanStartActivity(v, h.registry(), stepID)
		if err != nil {
			return questProblem(err)
		}
		rule := h.Config.PassRule.Effective(s.PassRule)
		n, err := h.Store.CountAttemptsOn(ctx, tenant, p.ID, stepID, day)
		if err != nil {
			return err
		}
		if rule.MaxAttemptsPerDay > 0 && n >= rule.MaxAttemptsPerDay {
			return problem.New(problem.QuestStepAttemptLimit, "hãy quay lại ngày mai").With("max_attempts_per_day", rule.MaxAttemptsPerDay)
		}
		content, err := uuid.Parse(s.ContentRef.ID)
		if err != nil {
			return problem.New(problem.ItemRefInvalid, "content_ref")
		}
		req = port.LearningAttempt{StudentID: student, ContentID: content, QuestProgressID: p.ID, QuestVersionID: p.VersionID, StepID: stepID,
			GameSessionID: gameSession}
		if s.ContentRef.VersionID != "" {
			if vid, err := uuid.Parse(s.ContentRef.VersionID); err == nil {
				req.ContentVersionID = &vid
			}
		}
		progressID = p.ID
		return nil
	})
	if err != nil {
		return out, err
	}
	if h.Learning == nil {
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning is not wired"), After: h.Config.LearningRetry}
	}
	created, err := h.Learning.CreateAttempt(ctx, req)
	if errors.Is(err, port.ErrLearningUnavailable) {
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning unavailable").Wrap(err), After: h.Config.LearningRetry}
	}
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		p, err := h.Store.ByID(ctx, tenant, progressID)
		if err != nil {
			return err
		}
		if p == nil {
			return problem.New(problem.NotFound, "quest progress not found")
		}
		if err := h.Store.InsertAttempt(ctx, write.StepAttempt{ID: newID(), TenantID: tenant, ProgressID: p.ID, StepID: stepID, StudentID: student,
			AttemptID: created.AttemptID, ContentVersionID: created.ContentVersionID, StartedOn: day, Status: write.AttemptInProgress}); err != nil {
			return err
		}
		if !slices.Contains(p.Steps[stepID].AttemptIDs, created.AttemptID.String()) {
			p.AttemptStarted(stepID, created.AttemptID.String())
			if err := h.Store.Save(ctx, p); err != nil {
				return err
			}
		}
		out = ActivityStart{AttemptID: created.AttemptID, ActivityID: req.ContentID, ActivityVersionID: created.ContentVersionID, Mode: "quest",
			Status: created.Status, ContextType: "quest_step", ContextID: p.ID, StartedAt: created.StartedAt, DeadlineAt: created.DeadlineAt, ServerTime: at}
		return setResult(ctx, out)
	})
	return out, err
}

// GradedAttempt is activity.graded of module learning (the fields the quest engine reads).
type GradedAttempt struct {
	TenantID     uuid.UUID
	AttemptID    uuid.UUID
	StudentID    uuid.UUID
	CorrectCount *int
	ItemCount    *int
	Passed       *bool
	OccurredAt   time.Time
}

// ApplyActivityResult is the consumer game.quest_on_activity_graded (F072-AC-15, edge cases): an attempt linked to a
// quest step is evaluated against the pass rule once (a replayed event finds the link already graded); passed completes
// the step (and the quest when the completion rule is met), not passed leaves the step active and pushes the hint of
// the step. A progress no longer active records the result and changes nothing. Runs in the consumer transaction.
func (h Quests) ApplyActivityResult(ctx context.Context, g GradedAttempt) (err error) {
	if err := h.Store.LockStudent(ctx, g.StudentID); err != nil {
		return err
	}
	sa, err := h.Store.AttemptByAttemptID(ctx, g.TenantID, g.AttemptID)
	if err != nil || sa == nil || sa.Status != write.AttemptInProgress || sa.StudentID != g.StudentID {
		return err // not a quest step attempt, already graded (replay) or of another student
	}
	p, err := h.Store.ByID(ctx, g.TenantID, sa.ProgressID)
	if err != nil || p == nil {
		return err
	}
	v, err := h.version(ctx, p)
	if err != nil {
		return err
	}
	s, _ := v.Step(sa.StepID)
	passed := h.Config.PassRule.Effective(s.PassRule).Passes(quest.Graded{CorrectCount: g.CorrectCount, ItemCount: g.ItemCount, Passed: g.Passed})
	at := h.now()
	sa.Status, sa.CorrectCount, sa.ItemCount, sa.GradedAt = write.AttemptNotPassed, g.CorrectCount, g.ItemCount, &at
	if passed {
		sa.Status = write.AttemptPassed
	}
	if err := h.Store.SaveAttempt(ctx, *sa); err != nil {
		return err
	}
	if !p.Active() {
		return nil
	}
	o := p.ActivityResult(v, sa.StepID, passed, at)
	push := progressPush(p, sa.StepID, nil)
	if o.Changed {
		attempt := g.AttemptID
		if push, err = h.record(ctx, p, v, []quest.Outcome{o}, &attempt, at); err != nil {
			return err
		}
	}
	push["passed"] = passed
	if !passed && s.Hint != nil && s.Hint.Complete() {
		push["hint"] = *s.Hint
	}
	if !g.OccurredAt.IsZero() {
		lag := at.Sub(g.OccurredAt).Seconds()
		h.after(ctx, func(context.Context) { h.metric(func(m QuestMetrics) { m.ProgressLag(lag) }) })
	}
	student := p.StudentID
	h.after(ctx, func(ctx context.Context) { h.push(ctx, student, push, at) })
	return nil
}

// Discovery is world.discovery_found (F071).
type Discovery struct {
	TenantID       uuid.UUID
	StudentID      uuid.UUID
	AreaID         string
	InteractableID string
}

// ApplyDiscovery is the consumer game.quest_on_discovery_found (F072-AC-12): a discovery advances every active discover
// step that lists it (once per interactable). Runs in the consumer transaction.
func (h Quests) ApplyDiscovery(ctx context.Context, d Discovery) error {
	if err := h.Store.LockStudent(ctx, d.StudentID); err != nil {
		return err
	}
	ps, err := h.Store.ActiveOfStudent(ctx, d.TenantID, d.StudentID)
	if err != nil {
		return err
	}
	at := h.now()
	for _, p := range ps {
		v, err := h.version(ctx, p)
		if err != nil {
			return err
		}
		outs := p.Discovery(v, h.registry(), d.AreaID, d.InteractableID, at)
		if len(outs) == 0 {
			continue
		}
		push, err := h.record(ctx, p, v, outs, nil, at)
		if err != nil {
			return err
		}
		student := p.StudentID
		h.after(ctx, func(ctx context.Context) { h.push(ctx, student, push, at) })
	}
	return nil
}

// ExpireForGrade ends as expired (quest.abandoned reason expired, no penalty) every active quest of the student that
// does not serve the new grade (F074-AC-25 through command.Progression.ApplyGradeTransition). Runs in the caller's
// transaction; returns how many ended.
func (h Quests) ExpireForGrade(ctx context.Context, tenant, student uuid.UUID, grade int, at time.Time) (int, error) {
	if err := h.Store.LockStudent(ctx, student); err != nil {
		return 0, err
	}
	ps, err := h.Store.ActiveOfStudent(ctx, tenant, student)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range ps {
		v, err := h.version(ctx, p)
		if err != nil {
			return n, err
		}
		if !v.ExpiresOnGradeChange(grade) {
			continue
		}
		if err := h.end(ctx, p, v, AbandonExpired, "", "", at); err != nil {
			return n, err
		}
		n++
		push := progressPush(p, "", nil)
		st := p.StudentID
		h.after(ctx, func(ctx context.Context) { h.push(ctx, st, push, at) })
	}
	return n, nil
}
