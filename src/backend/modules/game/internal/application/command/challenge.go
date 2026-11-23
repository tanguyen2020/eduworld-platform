package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/challengerules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Challenge events (specs/events/payloads/game/challenge.*.v1).
const (
	EventChallengeStarted   = "challenge.started"
	EventChallengeCompleted = "challenge.completed"
)

// ChallengeConfig is the configuration of the Challenge Engine (CONFIG_DEFAULTS through infrastructure/config).
type ChallengeConfig struct {
	Rules         challenge.Rules
	Reset         quest.Reset   // quest_reset_time: day and week of the attempt counters
	LearningRetry time.Duration // Retry-After of nextChallengeActivity when module learning is unavailable
	JobBatch      int           // attempts handled per run of challenge-deadline
}

// ChallengeMetrics observes the Challenge Engine; nil = not observed.
type ChallengeMetrics interface {
	Started(challengeType, difficulty string)
	Completed(status string)
}

// Challenges are the solo challenge use cases of a student (F076): startGameChallenge, nextChallengeActivity,
// abandonGameChallengeAttempt, the consumer of activity.graded (game.challenge_progress), the timer of the room host and
// the job challenge-deadline, and the end of the attempts of a student whose access is revoked or who is cleaned up.
// Score, lives and goal change only from results graded by module learning (ADR-0016, R5); every end writes exactly
// one challenge.completed to the outbox of the same transaction; the Reward Engine (F074) rewards it.
type Challenges struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Content     port.ChallengeContent
	World       port.WorldContent
	Quests      port.QuestContent
	Store       write.ChallengeStore
	Events      write.EventPublisher
	Learning    port.ChallengeLearning // nil: nextChallengeActivity answers 503
	Realtime    port.ChallengeRealtime // nil: no push
	Room        port.ChallengeRoom     // nil: the job challenge-deadline alone ends due attempts
	Metrics     ChallengeMetrics
	Config      ChallengeConfig
	Now         func() time.Time
}

func (h Challenges) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Challenges) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
	}
}

func (h Challenges) metric(fn func(m ChallengeMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

func notEligible(reason string) error {
	return problem.New(problem.ChallengeNotEligible, "").With("reason", reason)
}

// version returns the pinned version of a (always readable: published versions are never deleted).
func (h Challenges) version(ctx context.Context, a *challenge.Attempt) (*port.ChallengeRow, error) {
	row, err := h.Content.ChallengeVersion(ctx, a.VersionID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, problem.New(problem.NotFound, "challenge version not found")
	}
	return row, nil
}

func (h Challenges) studentOf(ctx context.Context, tenant, student uuid.UUID) (*port.WorldStudent, error) {
	st, err := h.World.Student(ctx, tenant, student)
	if err != nil {
		return nil, err
	}
	if st == nil || st.Deleted || st.Status != "active" {
		return nil, problem.New(problem.NotFound, "student not found")
	}
	return st, nil
}

// Start implements startGameChallenge (F076-AC-04…AC-07): the challenge must be assigned to the tenant, published and
// playable solo (404 otherwise, never revealing it); the grade, unlock rule, window and level must fit the student and
// no party is accepted (403 CHALLENGE_NOT_ELIGIBLE); the attempts of the day/period and the cooldown after a failure are
// applied (409 CHALLENGE_ATTEMPT_LIMIT with retry_after); another attempt in progress is 409 CONFLICT. The attempt
// pins the current version, sets deadline_at on the server clock and the lives of the level, writes challenge.started
// and opens its room (timer) after commit. Idempotency-Key is the httpx middleware.
func (h Challenges) Start(ctx context.Context, challengeID uuid.UUID, difficulty string, partyID *uuid.UUID) (out challengerules.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.StartChallenge")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	if challenge.Rank(difficulty) < 0 {
		return out, problem.Invalid("difficulty", problem.SubInvalid, "easy, normal, hard, expert or legendary")
	}
	at := h.now()
	r := h.Config.Rules
	var started *challenge.Attempt
	var row *port.ChallengeRow
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		st, err := h.studentOf(ctx, tenant, student)
		if err != nil {
			return err
		}
		if ok, err := h.Content.Assigned(ctx, tenant, challengeID); err != nil || !ok {
			if err != nil {
				return err
			}
			return problem.New(problem.NotFound, "challenge not found")
		}
		if row, err = h.Content.CurrentChallenge(ctx, challengeID); err != nil {
			return err
		}
		if row == nil || !row.Version.Solo() {
			return problem.New(problem.NotFound, "challenge not found")
		}
		v := row.Version
		if partyID != nil {
			return notEligible(challenge.ReasonParty) // a party plays through the Party Lobby (F103), never here
		}
		cs, err := challengerules.Student(ctx, h.World, h.Quests, tenant, student, st, v.Unlock != nil)
		if err != nil {
			return err
		}
		if ok, why := v.Visible(cs, at); !ok {
			return notEligible(why)
		}
		p, ok := v.Profile(difficulty)
		if !ok || !r.Allowed(v, p, st.Grade) {
			return notEligible(challenge.ReasonDifficulty)
		}
		if active, err := h.Store.ActiveOf(ctx, tenant, student); err != nil || active != nil {
			if err != nil {
				return err
			}
			return problem.New(problem.Conflict, "another challenge is in progress").With("challenge_attempt_id", active.ID.String())
		}
		period := challengerules.PeriodKey(v, h.Config.Reset, at)
		use, err := h.Content.Usage(ctx, tenant, student, challengeID, period)
		if err != nil {
			return err
		}
		if limit := r.MaxAttempts(v); limit > 0 && use.Attempts >= limit {
			e := problem.New(problem.ChallengeAttemptLimit, "hãy quay lại vào kỳ sau").With("max_attempts", limit)
			if end := challengerules.PeriodEnd(v, h.Config.Reset, at); end != nil {
				e = e.With("retry_after", end.UTC().Format(time.RFC3339))
			}
			return e
		}
		if use.LastFailedAt != nil {
			if until := use.LastFailedAt.Add(r.Cooldown(v, use.LastFailedOn)); at.Before(until) {
				return problem.New(problem.ChallengeAttemptLimit, "nghỉ một chút rồi thử lại nhé").With("retry_after", until.UTC().Format(time.RFC3339))
			}
		}
		started = challenge.NewAttempt(challenge.Start{ID: newID(), TenantID: tenant, StudentID: student, SchoolID: st.SchoolID,
			ChallengeID: row.ChallengeID, VersionID: row.VersionID, Type: v.Type, Difficulty: difficulty, PeriodKey: period,
			Lives: r.Lives(v, p), DeadlineAt: r.Deadline(v, st.Grade, at), At: at})
		ok, err = h.Store.Insert(ctx, started)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.Conflict, "another challenge is in progress")
		}
		t := tenant
		if err := h.Events.Publish(ctx, write.Event{Type: EventChallengeStarted, SubjectType: "challenge", SubjectID: challengeID.String(),
			PartitionKey: student.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"challenge_attempt_id": started.ID,
				"challenge_id": started.ChallengeID, "challenge_version_id": started.VersionID, "challenge_type": v.Type, "student_id": student,
				"difficulty": difficulty, "party_id": nil, "period_key": period, "deadline_at": started.DeadlineAt, "started_at": at}}); err != nil {
			return err
		}
		out = challengerules.ViewOf(started, row.VersionNo)
		return setResult(ctx, out)
	})
	if err != nil {
		return out, err
	}
	a, due, typ := started, started.Due(r), started.Type
	h.metric(func(m ChallengeMetrics) { m.Started(typ, difficulty) })
	if h.Room != nil {
		_ = h.Room.Open(ctx, a, due) // best effort: challenge-deadline ends it from PostgreSQL otherwise (F076-AC-19)
	}
	h.push(ctx, a)
	return out, nil
}

func (h Challenges) push(ctx context.Context, a *challenge.Attempt) {
	if h.Realtime == nil || a == nil {
		return
	}
	_ = h.Realtime.ChallengeAttemptUpdated(ctx, a.StudentID, challengerules.Push(a), h.now())
}

// NextView is ChallengeNextActivity.
type NextView struct {
	ChallengeAttempt challengerules.AttemptView `json:"challenge_attempt"`
	Done             bool                       `json:"done"`
	Attempt          *ActivityStart             `json:"attempt,omitempty"`
}

// own returns the attempt of tenant if it belongs to student (404 otherwise: never another student's, F076-AC-03).
func (h Challenges) own(ctx context.Context, tenant, student, id uuid.UUID) (*challenge.Attempt, error) {
	a, err := h.Store.ByID(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	if a == nil || a.StudentID != student {
		return nil, problem.New(problem.NotFound, "challenge attempt not found")
	}
	return a, nil
}

// Next implements nextChallengeActivity (F076-AC-08, AC-09, AC-12, AC-13): the attempt must be in progress and before
// its due time on the server clock (422 INVALID_STATE_TRANSITION); module game opens a learning attempt of the student
// through createAttempt of module learning (mode challenge, context challenge_attempt, the difficulty band of the level,
// the grade of the student, the objectives of the challenge; one item per survival round at the per-item time of the
// level) outside the transaction — its failure answers 503 with Retry-After and changes nothing; then the learning
// attempt is linked to the challenge attempt. The Idempotency-Key is forwarded so a retry opens the same attempt.
func (h Challenges) Next(ctx context.Context, attemptID uuid.UUID) (out NextView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.NextChallengeActivity")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	r := h.Config.Rules
	var req port.ChallengeLearningAttempt
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.own(ctx, tenant, student, attemptID)
		if err != nil {
			return err
		}
		if err := a.CanNext(at, r); err != nil {
			return problem.New(problem.InvalidStateTransition, "the challenge is over")
		}
		row, err := h.version(ctx, a)
		if err != nil {
			return err
		}
		v := row.Version
		ref, ok := a.NextRef(v)
		if !ok {
			return problem.New(problem.Conflict, "every activity of the challenge is open: finish them first")
		}
		activity, err := uuid.Parse(ref.ActivityID)
		if err != nil {
			return problem.New(problem.ItemRefInvalid, "activity_ref")
		}
		st, err := h.studentOf(ctx, tenant, student)
		if err != nil {
			return err
		}
		p, _ := v.Profile(a.Difficulty)
		objectives := make([]uuid.UUID, 0, len(v.ObjectiveIDs))
		for _, o := range v.ObjectiveIDs {
			if id, err := uuid.Parse(o); err == nil {
				objectives = append(objectives, id)
			}
		}
		req = port.ChallengeLearningAttempt{StudentID: student, ActivityID: activity, ChallengeAttemptID: a.ID, DifficultyMin: p.Min,
			DifficultyMax: p.Max, GradeLevel: st.Grade, ObjectiveIDs: objectives}
		if v.Type == challenge.TypeSurvival {
			one, secs := 1, max(r.ItemSeconds(p, st.Grade), 10)
			req.QuestionCount, req.TimeLimitSeconds = &one, &secs
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	if h.Learning == nil {
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning is not wired"), After: h.Config.LearningRetry}
	}
	created, err := h.Learning.CreateChallengeAttempt(ctx, req)
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
		a, err := h.own(ctx, tenant, student, attemptID)
		if err != nil {
			return err
		}
		row, err := h.version(ctx, a)
		if err != nil {
			return err
		}
		if a.Active() {
			if err := a.Opened(created.AttemptID); err != nil {
				return problem.New(problem.InvalidStateTransition, "the challenge has no more activities")
			}
			if err := h.Store.Save(ctx, a); err != nil {
				return err
			}
		}
		out = NextView{ChallengeAttempt: challengerules.ViewOf(a, row.VersionNo), Done: !a.Active(),
			Attempt: &ActivityStart{AttemptID: created.AttemptID, ActivityID: req.ActivityID, ActivityVersionID: created.ContentVersionID,
				Mode: "challenge", Status: created.Status, ContextType: "challenge_attempt", ContextID: a.ID, StartedAt: created.StartedAt,
				DeadlineAt: created.DeadlineAt, ServerTime: at}}
		return setResult(ctx, out)
	})
	return out, err
}

// Abandon implements abandonGameChallengeAttempt (F076-AC-14): an attempt in progress becomes abandoned; the learning
// attempts already recorded stay in module learning; challenge.completed (abandoned). An ended attempt is 422.
func (h Challenges) Abandon(ctx context.Context, attemptID uuid.UUID) (out challengerules.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.AbandonChallenge")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	var ended *challenge.Attempt
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Store.LockStudent(ctx, student); err != nil {
			return err
		}
		a, err := h.own(ctx, tenant, student, attemptID)
		if err != nil {
			return err
		}
		row, err := h.version(ctx, a)
		if err != nil {
			return err
		}
		if _, err := a.Abandon(at, challenge.EndPlayer); err != nil {
			return problem.New(problem.InvalidStateTransition, "the challenge is already over")
		}
		if err := h.finish(ctx, a, row); err != nil {
			return err
		}
		ended, out = a, challengerules.ViewOf(a, row.VersionNo)
		return setResult(ctx, out)
	})
	if err == nil {
		h.closed(ctx, ended)
	}
	return out, err
}

// finish saves an attempt that just ended and writes its single challenge.completed (first_clear: the first succeeded
// attempt of the student on the challenge, every version).
func (h Challenges) finish(ctx context.Context, a *challenge.Attempt, row *port.ChallengeRow) error {
	if a.Status == challenge.StatusSucceeded {
		cleared, err := h.Store.Cleared(ctx, a.TenantID, a.StudentID, a.ChallengeID, a.ID)
		if err != nil {
			return err
		}
		a.FirstClear = !cleared
	}
	if err := h.Store.Save(ctx, a); err != nil {
		return err
	}
	v := row.Version
	p, _ := v.Profile(a.Difficulty)
	mult := p.RewardMultiplier
	if mult <= 0 {
		mult = 1
	}
	objectives := v.ObjectiveIDs
	if objectives == nil {
		objectives = []string{}
	}
	var subject any
	if v.SubjectID != "" {
		subject = v.SubjectID
	}
	t := a.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: EventChallengeCompleted, SubjectType: "challenge", SubjectID: a.ChallengeID.String(),
		PartitionKey: a.StudentID.String(), TenantID: &t, OccurredAt: *a.EndedAt, Payload: map[string]any{"challenge_attempt_id": a.ID,
			"challenge_id": a.ChallengeID, "challenge_version_id": a.VersionID, "challenge_type": a.Type, "student_id": a.StudentID,
			"status": a.Status, "end_reason": a.EndReason, "score": a.Score, "difficulty": a.Difficulty, "reward_multiplier": mult,
			"first_clear": a.FirstClear, "subject_id": subject, "objective_ids": objectives, "activities_completed": a.ActivitiesCompleted,
			"period_key": nullable(a.PeriodKey), "started_at": a.StartedAt, "ended_at": *a.EndedAt}}); err != nil {
		return err
	}
	status := a.Status
	h.after(ctx, func(context.Context) { h.metric(func(m ChallengeMetrics) { m.Completed(status) }) })
	return nil
}

// closed pushes the end of an attempt and closes its room (best effort, after commit).
func (h Challenges) closed(ctx context.Context, a *challenge.Attempt) {
	if a == nil {
		return
	}
	if h.Room != nil {
		_ = h.Room.Close(ctx, a.ID)
	}
	h.push(ctx, a)
}

// ChallengeGraded is activity.graded of module learning (the fields the Challenge Engine reads).
type ChallengeGraded struct {
	TenantID     uuid.UUID
	AttemptID    uuid.UUID
	StudentID    uuid.UUID
	CorrectCount *int
	ItemCount    *int
	Passed       *bool
	OccurredAt   time.Time
}

// ApplyGraded is the consumer game.challenge_progress (F076-AC-10, AC-11): a graded learning attempt opened by an
// attempt in progress of the same student updates score, lives and goal once (a replayed event finds it processed); an
// end writes one challenge.completed; challenge.attempt.updated is pushed after commit. A result of an attempt already
// ended (time out) or graded after deadline_at + deadline_grace changes nothing: the learning record stays in module
// learning. Runs in the consumer transaction.
func (h Challenges) ApplyGraded(ctx context.Context, g ChallengeGraded) error {
	if err := h.Store.LockStudent(ctx, g.StudentID); err != nil {
		return err
	}
	a, err := h.Store.ByLearningAttempt(ctx, g.TenantID, g.AttemptID)
	if err != nil || a == nil || a.StudentID != g.StudentID {
		return err
	}
	row, err := h.version(ctx, a)
	if err != nil {
		return err
	}
	v, r := row.Version, h.Config.Rules
	p, _ := v.Profile(a.Difficulty)
	maxLives := 0
	if l := r.Lives(v, p); l != nil {
		maxLives = *l
	}
	res := challenge.Graded{AttemptID: g.AttemptID, Passed: g.Passed, At: g.OccurredAt}
	if res.At.IsZero() {
		res.At = h.now()
	}
	if g.CorrectCount != nil {
		res.Correct = *g.CorrectCount
	}
	if g.ItemCount != nil {
		res.Items = *g.ItemCount
	}
	o := a.Apply(v, maxLives, r, res)
	if !o.Changed {
		return nil
	}
	if o.Ended {
		if err := h.finish(ctx, a, row); err != nil {
			return err
		}
		h.after(ctx, func(ctx context.Context) { h.closed(ctx, a) })
		return nil
	}
	if err := h.Store.Save(ctx, a); err != nil {
		return err
	}
	h.after(ctx, func(ctx context.Context) { h.push(ctx, a) })
	return nil
}

// Expire ends one attempt of tenant past its due time + deadline_grace on the server clock (F076-AC-13, AC-15): the
// timer of its room (pkg/roomhost) or the job challenge-deadline. done = the attempt is no longer in progress (ended now
// or before). Its own transaction under the event scope of the tenant.
func (h Challenges) Expire(ctx context.Context, tenant, attemptID uuid.UUID) (done bool, err error) {
	ctx, end := observability.UseCase(ctx, "game.command.ExpireChallenge")
	defer func() { end(err) }()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return false, err
	}
	var ended *challenge.Attempt
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		cur, err := h.Content.Attempt(ctx, tenant, attemptID)
		if err != nil || cur == nil {
			done = cur == nil
			return err
		}
		if err := h.Store.LockStudent(ctx, cur.StudentID); err != nil {
			return err
		}
		a, err := h.Store.ByID(ctx, tenant, attemptID)
		if err != nil || a == nil {
			done = a == nil
			return err
		}
		if !a.Active() {
			done = true
			return nil
		}
		if !a.Expire(h.now(), h.Config.Rules) {
			return nil
		}
		row, err := h.version(ctx, a)
		if err != nil {
			return err
		}
		if err := h.finish(ctx, a, row); err != nil {
			return err
		}
		ended, done = a, true
		return nil
	})
	if err == nil && ended != nil {
		h.push(ctx, ended)
	}
	return done, err
}

// ExpireDue is one run of the job challenge-deadline (F076-AC-13, AC-19): the attempts in progress past their due time
// are ended from PostgreSQL, whether or not Valkey and the room host are up. Each attempt in its own transaction after
// a FOR UPDATE re-check, so an attempt the timer ended is never ended twice.
func (h Challenges) ExpireDue(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.ExpireDueChallenges")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("challenge-deadline")
	if err != nil {
		return 0, err
	}
	r := h.Config.Rules
	var refs []write.AttemptRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		refs, err = h.Store.Due(ctx, h.now().Add(-r.DeadlineGrace), r.ResumeWindow, batch)
		return err
	}); err != nil {
		return 0, err
	}
	var errs []error
	for _, ref := range refs {
		ended, err := h.Expire(ctx, ref.TenantID, ref.ID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ended {
			n++
			if h.Room != nil {
				_ = h.Room.Close(ctx, ref.ID)
			}
		}
	}
	return n, errors.Join(errs...)
}

// AbandonForStudent ends as abandoned (student_unavailable) the attempts in progress of a student whose access was
// revoked or who was cleaned up (edge case; consumer game.session_end_on_revoke, its transaction): tenant nil = every
// tenant of the student (platform event). Returns how many ended.
func (h Challenges) AbandonForStudent(ctx context.Context, tenant *uuid.UUID, student uuid.UUID) (int, error) {
	if err := h.Store.LockStudent(ctx, student); err != nil {
		return 0, err
	}
	list, err := h.Store.ActiveOfStudentAnyTenant(ctx, student, tenant)
	if err != nil {
		return 0, err
	}
	at := h.now()
	n := 0
	for _, a := range list {
		row, err := h.version(ctx, a)
		if err != nil {
			return n, err
		}
		if _, err := a.Abandon(at, challenge.EndStudentUnavailable); err != nil {
			continue
		}
		if err := h.finish(ctx, a, row); err != nil {
			return n, err
		}
		n++
		ended := a
		h.after(ctx, func(ctx context.Context) {
			if h.Room != nil {
				_ = h.Room.Close(ctx, ended.ID)
			}
		})
	}
	return n, nil
}

// RoomCheckpoint reads the checkpoint of the room of an attempt (pkg/roomhost takeover, F076-AC-17): its due time on the
// server clock (deadline_at, or the end of attempt_resume_window) and whether it is still in progress. PostgreSQL is the
// source of truth; the registry in Valkey only says which node owns the room.
func (h Challenges) RoomCheckpoint(ctx context.Context, tenant, attemptID uuid.UUID) (due time.Time, active bool, err error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return due, false, err
	}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		a, err := h.Content.Attempt(ctx, tenant, attemptID)
		if err != nil || a == nil {
			return err
		}
		due, active = a.Due(h.Config.Rules), a.Active()
		return nil
	})
	return due, active, err
}
