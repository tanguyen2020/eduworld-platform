package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

func withScope(ctx context.Context, s tenancy.Scope) context.Context {
	return tenancy.WithScope(ctx, s)
}

// locked runs fn on the locked encounter of tenant (nil when unknown or of another tenant), then applies its effect.
func (h Bosses) locked(ctx context.Context, tenant, id uuid.UUID, at time.Time,
	fn func(ctx context.Context, e *boss.Encounter, row port.BossRow) (boss.Effect, error)) (*transition, error) {
	var tr *transition
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		e, err := h.Store.Lock(ctx, tenant, id)
		if err != nil {
			return err
		}
		if e == nil {
			return errEncounterUnknown
		}
		row, err := h.version(ctx, e)
		if err != nil {
			return err
		}
		ef, err := fn(ctx, e, row)
		if err != nil {
			return err
		}
		tr, err = h.apply(ctx, e, row, ef, at)
		return err
	})
	return tr, err
}

// errEncounterUnknown is an encounter not found in the tenant of the caller (never revealing whether it exists).
var errEncounterUnknown = errors.New("boss: encounter unknown")

// wsProblem maps an unknown encounter of a WS command to PERMISSION_DENIED (F103-AC-27: existence never revealed).
func wsProblem(err error) error {
	if errors.Is(err, errEncounterUnknown) {
		return problem.New(problem.PermissionDenied, "")
	}
	return bossProblem(err)
}

// Ready implements boss.encounter.ready (F103-AC-04): the caller (from the session, never the payload) must be a
// present member; when every present member is ready (and at least party_size.min), the first phase starts, the
// question plans of every member are created (per_grade) and each member receives its own questions.
func (h Bosses) Ready(ctx context.Context, encounterID uuid.UUID, ready bool) (out partyrules.EncounterView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.BossReady")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	at := h.now()
	tr, err := h.locked(ctx, tenant, encounterID, at, func(ctx context.Context, e *boss.Encounter, row port.BossRow) (boss.Effect, error) {
		return e.SetReady(row.Version, student, ready, at)
	})
	if err != nil {
		return out, wsProblem(err)
	}
	h.pushed(ctx, tr, at)
	return tr.view, nil
}

// AnswerInput is the payload of boss.answer.submit (the student comes from the session).
type AnswerInput struct {
	EncounterID        uuid.UUID
	PhaseID            string
	RequirementID      string
	QuestionInstanceID string
	Response           json.RawMessage
}

// AnswerReply is the reply of boss.answer.submit — to the student who answered only (F103-AC-09).
type AnswerReply struct {
	QuestionInstanceID string `json:"question_instance_id"`
	IsCorrect          bool   `json:"is_correct"`
	Counted            bool   `json:"counted"`
	Duplicate          bool   `json:"duplicate,omitempty"`
}

// Answer implements boss.answer.submit (F103-AC-05, AC-06, AC-09): the student is the one of the session; the answer is
// stamped with the server time; a closed phase or round is BOSS_PHASE_CLOSED, a non-member ROOM_NOT_MEMBER, an
// encounter of another tenant PERMISSION_DENIED — the encounter is unchanged. Module learning grades it and records it
// as the learning record of that student (scorePlanAnswer, idempotent by (plan, index, student)); the same question
// counts once. A correct answer counted adds to the contribution and the energy; a wrong one takes nothing away; an
// answer faster than rapid_guess_min_response_ms counts nothing and is flagged (audit boss.integrity_flagged).
func (h Bosses) Answer(ctx context.Context, in AnswerInput) (out AnswerReply, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.BossAnswer")
	defer func() {
		done(err)
		result := "ok"
		switch {
		case err != nil:
			result = string(codeOf(err))
		case out.Duplicate:
			result = "duplicate"
		case !out.Counted:
			result = "not_counted"
		case out.IsCorrect:
			result = "correct"
		default:
			result = "wrong"
		}
		h.metric(func(m BossMetrics) { m.Answer(result) })
	}()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	planID, index, ok := partyrules.ParseInstance(in.QuestionInstanceID)
	if !ok || len(in.PhaseID) > 64 || len(in.RequirementID) > 64 {
		return out, problem.Invalid("question_instance_id", problem.SubInvalid, "not a question of this encounter")
	}
	out.QuestionInstanceID = in.QuestionInstanceID
	received := h.now()
	// 1. Check the question against the locked encounter (and apply a deadline already passed).
	var expired *transition
	var prev *port.BossAnswerRow
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		e, err := h.Store.Lock(ctx, tenant, in.EncounterID)
		if err != nil {
			return err
		}
		if e == nil {
			return errEncounterUnknown
		}
		m, ok := e.Member(student)
		if !ok || !m.Present() {
			return boss.ErrNotMember
		}
		row, err := h.version(ctx, e)
		if err != nil {
			return err
		}
		if e.Status == boss.StatusPhaseActive && e.PhaseDeadline != nil && !received.Before(*e.PhaseDeadline) {
			if expired, err = h.apply(ctx, e, row, e.Timeout(row.Version, received), received); err != nil {
				return err
			}
			return nil
		}
		if e.Status != boss.StatusPhaseActive || e.PhaseID != in.PhaseID {
			return boss.ErrPhaseClosed
		}
		p, err := h.Store.PlanByID(ctx, tenant, planID)
		if err != nil {
			return err
		}
		switch {
		case p == nil || p.EncounterID != e.ID || p.StudentID != student:
			return problem.New(problem.PermissionDenied, "not a question of this student")
		case p.PhaseID != e.PhaseID || p.RetryNo != e.Retries:
			return boss.ErrPhaseClosed
		case p.RequirementID != in.RequirementID || index >= len(p.Items):
			return problem.Invalid("requirement_id", problem.SubInvalid, "not the requirement of the question")
		}
		prev, err = h.Store.Answer(ctx, tenant, planID, index, student)
		return err
	})
	if expired != nil {
		h.pushed(ctx, expired, received)
		return out, problem.New(problem.BossPhaseClosed, "")
	}
	if err != nil {
		return out, wsProblem(err)
	}
	if prev != nil {
		out.IsCorrect, out.Counted, out.Duplicate = prev.IsCorrect, prev.Counted, true
		return out, nil
	}
	// 2. Grading by module learning, outside the transaction (learning record of the student, ADR-0016).
	score, err := h.Learning.ScoreBossAnswer(ctx, planID, index, student, in.Response, received)
	if errors.Is(err, port.ErrBossLearningUnavailable) {
		return out, problem.New(problem.Unavailable, "module learning unavailable").Wrap(err).Retry()
	}
	if err != nil {
		return out, err
	}
	// 3. Record once and apply to the encounter.
	var tr *transition
	closed := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		e, err := h.Store.Lock(ctx, tenant, in.EncounterID)
		if err != nil || e == nil {
			if err == nil {
				err = errEncounterUnknown
			}
			return err
		}
		row, err := h.version(ctx, e)
		if err != nil {
			return err
		}
		inRound := e.Status == boss.StatusPhaseActive && e.PhaseID == in.PhaseID && e.PhaseDeadline != nil && received.Before(*e.PhaseDeadline)
		if inRound {
			p, err := h.Store.PlanByID(ctx, tenant, planID)
			if err != nil {
				return err
			}
			inRound = p != nil && p.RetryNo == e.Retries
		}
		counted := inRound && score.IsCorrect && !score.Late && !score.RapidGuess
		inserted, err := h.Store.InsertAnswer(ctx, port.BossAnswerRow{ID: newID(), TenantID: tenant, EncounterID: e.ID, StudentID: student,
			PlanID: planID, Index: index, PhaseID: in.PhaseID, RetryNo: e.Retries, RequirementID: in.RequirementID, IsCorrect: score.IsCorrect,
			Counted: counted, RapidGuess: score.RapidGuess, ReceivedAt: received})
		if err != nil {
			return err
		}
		if !inserted {
			prev, err := h.Store.Answer(ctx, tenant, planID, index, student)
			if err != nil || prev == nil {
				return err
			}
			out.IsCorrect, out.Counted, out.Duplicate = prev.IsCorrect, prev.Counted, true
			return nil
		}
		out.IsCorrect, out.Counted = score.IsCorrect, counted
		closed = !inRound
		if score.RapidGuess {
			if err := h.Audit.Record(ctx, audit.Event{Action: "boss.integrity_flagged", TargetType: "boss_encounter", TargetID: e.ID.String(),
				Result: audit.ResultSuccess, Metadata: map[string]any{"signal": "rapid_guess", "phase_id": in.PhaseID}}); err != nil {
				return err
			}
		}
		if !counted {
			return nil
		}
		r, err := h.Store.Round(ctx, tenant, e.ID, e.PhaseID, e.Retries)
		if err != nil {
			return err
		}
		items, err := h.itemsOf(ctx, e)
		if err != nil {
			return err
		}
		round := boss.Round{Items: items, Correct: r}
		e.RecordCorrect(row.Version, round, student)
		tr, err = h.apply(ctx, e, row, e.Evaluate(row.Version, round, received), received)
		return err
	})
	if err != nil {
		return out, wsProblem(err)
	}
	if h.Realtime != nil {
		_ = h.Realtime.BossMessage(ctx, student, MsgBossEvaluated, map[string]any{"encounter_id": in.EncounterID,
			"question_instance_id": in.QuestionInstanceID, "is_correct": out.IsCorrect, "counted": out.Counted}, received)
	}
	h.pushed(ctx, tr, received)
	if closed {
		return out, problem.New(problem.BossPhaseClosed, "")
	}
	return out, nil
}

// Leave implements leaveBossEncounter (F103-AC-12): a member leaves, its contributions and learning records stay;
// below party_size.min the encounter is abandoned without reward. Not a member (or another tenant) → 404.
func (h Bosses) Leave(ctx context.Context, encounterID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.LeaveBossEncounter")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return err
	}
	at := h.now()
	tr, err := h.locked(ctx, tenant, encounterID, at, func(ctx context.Context, e *boss.Encounter, row port.BossRow) (boss.Effect, error) {
		if _, ok := e.Member(student); !ok {
			return boss.Effect{}, errEncounterUnknown
		}
		ef, err := e.Leave(row.Version, student, boss.LeftByPlayer, at)
		if err != nil {
			return ef, err
		}
		return ef, setResult(ctx, nil)
	})
	if errors.Is(err, errEncounterUnknown) {
		return problem.New(problem.NotFound, "encounter not found")
	}
	if err != nil {
		return bossProblem(err)
	}
	h.pushed(ctx, tr, at)
	return nil
}

// MemberLeft is the boss part of the consumer of party.member_left / party.member_kicked / party.disbanded (F103-AC-12):
// the student (every member when student is Nil: disbanded) leaves the live encounters of the party; contributions
// stay. Runs in the consumer transaction.
func (h Bosses) MemberLeft(ctx context.Context, tenant, party, student uuid.UUID, reason string) error {
	ids, err := h.Store.LiveOfParty(ctx, tenant, party)
	if err != nil {
		return err
	}
	at := h.now()
	for _, id := range ids {
		e, err := h.Store.Lock(ctx, tenant, id)
		if err != nil || e == nil {
			if err != nil {
				return err
			}
			continue
		}
		row, err := h.version(ctx, e)
		if err != nil {
			return err
		}
		var ef boss.Effect
		for _, m := range e.PresentMembers() {
			if student != uuid.Nil && m.StudentID != student {
				continue
			}
			le, err := e.Leave(row.Version, m.StudentID, reason, at)
			if err != nil {
				return err
			}
			ef.Ended, ef.Started, ef.PhaseStarted = ef.Ended || le.Ended, ef.Started || le.Started, ef.PhaseStarted || le.PhaseStarted
			ef.MembersChange = true
		}
		if !ef.MembersChange {
			continue
		}
		tr, err := h.apply(ctx, e, row, ef, at)
		if err != nil {
			return err
		}
		h.after(ctx, func(ctx context.Context) { h.pushed(ctx, tr, at) })
	}
	return nil
}

// OnTimer is the timer of the room of an encounter on the lease owner (pkg/roomhost): the ready deadline (F103-AC-04,
// AC-12), the phase and total deadlines (AC-08), the reconnect grace of the members (AC-11), the missing plans of the
// round (retries). done = the encounter ended (the room is forgotten).
func (h Bosses) OnTimer(ctx context.Context, tenant, id uuid.UUID) (bool, error) {
	ctx, done := observability.UseCase(ctx, "game.command.BossTimer")
	var err error
	defer func() { done(err) }()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return true, nil
	}
	ctx = withScope(ctx, sc)
	at := h.now()
	plans := false
	tr, err := h.locked(ctx, tenant, id, at, func(ctx context.Context, e *boss.Encounter, row port.BossRow) (boss.Effect, error) {
		if !e.Live() {
			return boss.Effect{}, nil
		}
		v := row.Version
		var ef boss.Effect
		switch e.Status {
		case boss.StatusLobby:
			ef = e.ReadyTimeout(v, at)
		case boss.StatusPhaseActive:
			ef = e.Timeout(v, at)
		}
		if e.Live() && e.Status == boss.StatusPhaseActive && h.Presence != nil {
			ids := make([]uuid.UUID, 0, len(e.Members))
			for _, m := range e.PresentMembers() {
				ids = append(ids, m.StudentID)
			}
			online, err := h.Presence.Online(ctx, tenant, ids, at.Add(-h.Config.HeartbeatTimeout))
			if err != nil {
				return ef, err
			}
			pe := e.Presence(v, online, h.grace(v), at)
			ef.Ended, ef.MembersChange = ef.Ended || pe.Ended, ef.MembersChange || pe.MembersChange
		}
		if e.Live() && e.Status == boss.StatusPhaseActive && !ef.PhaseStarted {
			have, err := h.Store.Plans(ctx, tenant, e.ID, e.PhaseID, e.Retries)
			if err != nil {
				return ef, err
			}
			p, _ := v.Phase(e.PhaseIndex)
			plans = len(have) < len(e.PresentMembers())*len(p.Requirements)
		}
		return ef, nil
	})
	if errors.Is(err, errEncounterUnknown) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	h.pushed(ctx, tr, at)
	if plans {
		h.preparePlans(ctx, tenant, id)
	}
	return tr == nil || !tr.e.Live(), nil
}

// RoomCheckpoint is the checkpoint of the room of an encounter (takeover, F076 pkg/roomhost): its next due time on the
// server clock, false when it ended.
func (h Bosses) RoomCheckpoint(ctx context.Context, tenant, id uuid.UUID) (time.Time, bool, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return time.Time{}, false, nil //nolint:nilerr // a room without a valid tenant can never run: it ends instead of retrying forever
	}
	ctx = withScope(ctx, sc)
	var due time.Time
	live := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		e, err := h.Store.Lock(ctx, tenant, id)
		if err != nil || e == nil {
			return err
		}
		live = e.Live()
		due = e.NextDue(h.Config.CheckEvery, h.now())
		return nil
	})
	return due, live, err
}
