package command

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Boss events (specs/events/payloads/game/boss.*.v1) and realtime messages (AsyncAPI boss.*).
const (
	EventBossEncounterStarted   = "boss.encounter_started"
	EventBossPhaseCompleted     = "boss.phase_completed"
	EventBossEncounterCompleted = "boss.encounter_completed"
	MsgBossUpdated              = "boss.encounter.updated"
	MsgBossPhaseStarted         = "boss.phase.started"
	MsgBossPhaseCompleted       = "boss.phase.completed"
	MsgBossEnded                = "boss.encounter.ended"
	MsgBossQuestions            = "boss.question.assigned"
	MsgBossEvaluated            = "boss.answer.evaluated"
)

// BossConfig is the configuration of the boss engine (CONFIG_DEFAULTS through infrastructure/config).
type BossConfig struct {
	Grace            time.Duration // boss_reconnect_grace (120 s): ready deadline, default reconnect grace of a version
	CheckEvery       time.Duration // period of the room checks (presence, deadlines)
	PlanRetries      int           // retries of the plans of a round before abandoning (2, edge case of the spec)
	AttemptsPerDay   int           // challenge_attempts_per_day (5)
	HeartbeatTimeout time.Duration // a member is online while its game session heartbeat is within it
	Reset            quest.Reset   // quest_reset_time: day of the encounters
	QuestionCount    int           // questions drawn per requirement (cap; the activity gives its natural count)
}

// BossMetrics observes the boss engine; nil = not observed.
type BossMetrics interface {
	Answer(result string)
	PhaseRetry()
	Ended(outcome string)
	Started(mode string)
}

// Bosses are the boss encounter use cases (F103 US1): createBossEncounter (solo), startPartyActivity (boss), the NATS
// commands boss.encounter.ready and boss.answer.submit, leaveBossEncounter, the party membership consumers and the
// timer of the room host. The server is the only authority for phases, energy, contributions and results: answers
// are graded by module learning (scorePlanAnswer) and recorded as the learning record of the student who answered
// (ADR-0016); every state change of an encounter happens with its row locked.
type Bosses struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Content     port.BossContent
	World       port.WorldContent
	Quests      port.QuestContent
	Store       write.BossStore
	Events      write.EventPublisher
	Audit       write.Auditor
	Learning    port.BossLearning
	Realtime    port.BossRealtime  // nil: no push
	Presence    port.OnlineMembers // nil: every member counts as online
	Room        port.GameRoom      // nil: no room host (the timers then run only from the commands)
	Names       port.NicknameReader
	Metrics     BossMetrics
	Config      BossConfig
	Now         func() time.Time
}

func (h Bosses) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Bosses) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
		return
	}
	fn(ctx)
}

func (h Bosses) metric(fn func(m BossMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

// bossProblem maps the domain errors.
func bossProblem(err error) error {
	switch {
	case errors.Is(err, boss.ErrNotMember):
		return problem.New(problem.RoomNotMember, "")
	case errors.Is(err, boss.ErrPhaseClosed):
		return problem.New(problem.BossPhaseClosed, "")
	case errors.Is(err, boss.ErrInvalidState):
		return problem.New(problem.InvalidStateTransition, err.Error())
	}
	return err
}

// version returns the pinned version of an encounter.
func (h Bosses) version(ctx context.Context, e *boss.Encounter) (port.BossRow, error) {
	row, err := h.Content.BossVersion(ctx, e.VersionID)
	if err != nil {
		return port.BossRow{}, err
	}
	if row == nil {
		return port.BossRow{}, problem.New(problem.NotFound, "boss version not found")
	}
	return *row, nil
}

func (h Bosses) grace(v boss.Version) time.Duration { return v.Grace(h.Config.Grace) }

// CreateSolo implements createBossEncounter (F103-AC-01): mode solo only (a party enters through the Party Lobby: 422);
// the boss must be assigned to the tenant (404), playable solo, of the grade of the student and of an open area (403
// CHALLENGE_NOT_ELIGIBLE), below challenge_attempts_per_day (409 CHALLENGE_ATTEMPT_LIMIT). The encounter is in lobby,
// pinned to the current version; its deadlines run on the server clock. Idempotency-Key is the httpx middleware.
func (h Bosses) CreateSolo(ctx context.Context, bossID uuid.UUID, mode string) (out partyrules.EncounterView, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CreateBossEncounter")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	if mode != boss.EncounterSolo {
		return out, problem.Invalid("mode", problem.SubInvalid, "a party enters a boss through the Party Lobby (startPartyActivity)")
	}
	at := h.now()
	var e *boss.Encounter
	var row port.BossRow
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if ok, err := h.Content.BossAssigned(ctx, tenant, bossID); err != nil || !ok {
			if err != nil {
				return err
			}
			return problem.New(problem.NotFound, "boss not found")
		}
		r, err := h.Content.CurrentBoss(ctx, bossID)
		if err != nil {
			return err
		}
		if r == nil {
			return problem.New(problem.NotFound, "boss not found")
		}
		row = *r
		v := row.Version
		ms, err := partyrules.Members(ctx, h.World, h.Quests, tenant, []uuid.UUID{student})
		if err != nil {
			return err
		}
		if !ms[0].Valid() {
			return problem.New(problem.NotFound, "student not found")
		}
		if !v.Solo() {
			return notEligible("party_required")
		}
		period := partyrules.BossPeriod(h.Config.Reset, at)
		use, err := h.Store.Usage(ctx, tenant, student, bossID, period)
		if err != nil {
			return err
		}
		c := partyrules.BossCheck(v, ms[0], use, h.Config.AttemptsPerDay)
		switch {
		case c.Reason == partyrules.ReasonAttempts:
			e := problem.New(problem.ChallengeAttemptLimit, "hãy quay lại vào ngày mai").With("max_attempts", h.Config.AttemptsPerDay)
			if end := h.Config.Reset.PeriodEnd(quest.RecurrenceDaily, at); end != nil {
				e = e.With("retry_after", end.UTC().Format(time.RFC3339))
			}
			return e
		case !c.Eligible:
			return notEligible(c.Reason)
		}
		e = boss.New(boss.Start{ID: newID(), TenantID: tenant, SchoolID: ms[0].Student.SchoolID, BossID: row.BossID, VersionID: row.VersionID,
			Mode: boss.EncounterSolo, Members: []boss.Member{{StudentID: student, SchoolID: ms[0].Student.SchoolID, Grade: ms[0].Student.Grade}},
			PeriodKey: period, Grace: h.grace(v), At: at})
		if _, err := h.Store.InsertEncounter(ctx, e); err != nil {
			return err
		}
		names, err := h.names(ctx, e)
		if err != nil {
			return err
		}
		out = partyrules.View(partyrules.ViewInput{E: e, Row: row, Names: names, Now: at})
		return setResult(ctx, out)
	})
	if err != nil {
		return out, err
	}
	h.metric(func(m BossMetrics) { m.Started(boss.EncounterSolo) })
	h.opened(ctx, e, row, out, at)
	return out, nil
}

// opened announces the room of a new encounter and pushes its state to the members (after commit).
func (h Bosses) opened(ctx context.Context, e *boss.Encounter, row port.BossRow, view partyrules.EncounterView, at time.Time) {
	if h.Room != nil {
		_ = h.Room.Open(ctx, e.TenantID, e.ID, e.NextDue(h.Config.CheckEvery, at))
	}
	h.broadcast(ctx, e, MsgBossUpdated, partyrules.Updated(view), at)
	_ = row
}

func (h Bosses) names(ctx context.Context, e *boss.Encounter) (map[uuid.UUID]string, error) {
	if h.Names == nil {
		return map[uuid.UUID]string{}, nil
	}
	ids := make([]uuid.UUID, len(e.Members))
	for i, m := range e.Members {
		ids[i] = m.StudentID
	}
	return h.Names.Nicknames(ctx, e.TenantID, ids)
}

// broadcast sends one message to every present member (best effort).
func (h Bosses) broadcast(ctx context.Context, e *boss.Encounter, typ string, payload map[string]any, at time.Time) {
	if h.Realtime == nil {
		return
	}
	for _, m := range e.Members {
		if m.Present() {
			_ = h.Realtime.BossMessage(ctx, m.StudentID, typ, payload, at)
		}
	}
}

// PartyStart is startPartyActivity of the facade (F101 research R2).
type PartyStart struct {
	TenantID        uuid.UUID
	PartyActivityID uuid.UUID
	PartyID         uuid.UUID
	LeaderID        uuid.UUID
	Type            string // quest, boss
	Ref             string // code of the quest or boss
	Members         []uuid.UUID
}

// PartyStarted is PartyActivityStarted.
type PartyStarted struct {
	ExternalRef uuid.UUID
	RoomType    string // boss, private_instance
	AreaID      string
	SceneRef    string
}

// notEligibleMembers is 422 PARTY_ACTIVITY_NOT_ELIGIBLE with the reason codes of the members (no student data, R16).
func notEligibleMembers(cs []partyrules.Check) error {
	var ms []map[string]any
	for _, c := range cs {
		if !c.Eligible {
			ms = append(ms, map[string]any{"student_id": c.StudentID.String(), "reason_code": c.Reason, "requirement": c.Requirement})
		}
	}
	return problem.New(problem.PartyActivityNotEligible, "").With("members", ms)
}

// membersOfTenant refuses a member that is not a student of the TenantScope of the call (F103-AC-26): 422 VALIDATION_FAILED.
func membersOfTenant(ms []partyrules.Member) error {
	for _, m := range ms {
		if m.Student == nil {
			return problem.Invalid("/members", problem.SubInvalid, "a member is not a student of the tenant of the call")
		}
	}
	return nil
}

// StartParty creates the encounter of a party activity (F103-AC-03): idempotent by party_activity_id (the same
// encounter again); the boss must be assigned to the tenant and playable by a party (404 otherwise); every member is
// checked again (422 PARTY_ACTIVITY_NOT_ELIGIBLE) and the member count must be within party_size. Members of another
// tenant than the TenantScope are 422 VALIDATION_FAILED (F103-AC-26) — checked by the facade before.
func (h Bosses) StartParty(ctx context.Context, in PartyStart) (out PartyStarted, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.StartPartyBoss")
	defer func() { done(err) }()
	at := h.now()
	var e *boss.Encounter
	var row port.BossRow
	created := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		prev, err := h.Store.ByPartyActivity(ctx, in.TenantID, in.PartyActivityID)
		if err != nil {
			return err
		}
		r, err := h.Content.BossByCode(ctx, in.Ref)
		if err != nil {
			return err
		}
		if prev != nil {
			if r, err = h.Content.BossVersion(ctx, prev.VersionID); err != nil {
				return err
			}
			e, row = prev, *r
			return nil
		}
		if r == nil {
			return problem.New(problem.NotFound, "boss not found")
		}
		if ok, err := h.Content.BossAssigned(ctx, in.TenantID, r.BossID); err != nil || !ok {
			if err != nil {
				return err
			}
			return problem.New(problem.NotFound, "boss not found")
		}
		row = *r
		v := row.Version
		if !v.Party() {
			return problem.New(problem.PartyActivityNotEligible, "the boss is solo only")
		}
		if n := len(in.Members); n < v.SizeMin() || n > v.SizeMax() {
			return problem.New(problem.PartyActivityNotEligible, "the party size is outside party_size").
				With("min_members", v.SizeMin()).With("max_members", v.SizeMax())
		}
		ms, err := partyrules.Members(ctx, h.World, h.Quests, in.TenantID, in.Members)
		if err != nil {
			return err
		}
		if err := membersOfTenant(ms); err != nil {
			return err
		}
		period := partyrules.BossPeriod(h.Config.Reset, at)
		checks := make([]partyrules.Check, len(ms))
		ok := true
		members := make([]boss.Member, 0, len(ms))
		for i, m := range ms {
			use, err := h.Store.Usage(ctx, in.TenantID, m.ID, row.BossID, period)
			if err != nil {
				return err
			}
			checks[i] = partyrules.BossCheck(v, m, use, h.Config.AttemptsPerDay)
			ok = ok && checks[i].Eligible
			if m.Valid() {
				members = append(members, boss.Member{StudentID: m.ID, SchoolID: m.Student.SchoolID, Grade: m.Student.Grade})
			}
		}
		if !ok {
			return notEligibleMembers(checks)
		}
		pa, party, leader := in.PartyActivityID, in.PartyID, in.LeaderID
		school := members[0].SchoolID
		for _, m := range members {
			if m.StudentID == leader {
				school = m.SchoolID
			}
		}
		e = boss.New(boss.Start{ID: newID(), TenantID: in.TenantID, SchoolID: school, BossID: row.BossID, VersionID: row.VersionID,
			Mode: boss.EncounterParty, PartyID: &party, PartyActivityID: &pa, LeaderID: &leader, Members: members, PeriodKey: period,
			Grace: h.grace(v), At: at})
		inserted, err := h.Store.InsertEncounter(ctx, e)
		if err != nil {
			return err
		}
		if !inserted { // a concurrent call with the same party_activity_id won
			if e, err = h.Store.ByPartyActivity(ctx, in.TenantID, in.PartyActivityID); err != nil || e == nil {
				if err == nil {
					err = problem.New(problem.Conflict, "the party activity changed meanwhile")
				}
				return err
			}
			return nil
		}
		created = true
		return nil
	})
	if err != nil {
		return out, err
	}
	scene := row.Version.VisualRef
	if scene == "" {
		scene = "boss/" + row.Version.Code
	}
	out = PartyStarted{ExternalRef: e.ID, RoomType: "boss", AreaID: row.Version.AreaID, SceneRef: scene}
	if created {
		h.metric(func(m BossMetrics) { m.Started(boss.EncounterParty) })
		h.opened(ctx, e, row, partyrules.View(partyrules.ViewInput{E: e, Row: row, Now: at}), at)
	}
	return out, nil
}

// transition is what a locked change of an encounter produced for the pushes after commit.
type transition struct {
	e    *boss.Encounter
	row  port.BossRow
	ef   boss.Effect
	view partyrules.EncounterView
}

// apply saves a changed encounter and writes its events (in the transaction): boss.encounter_started at the first
// phase, boss.phase_completed for a cleared phase, the result once (finish).
func (h Bosses) apply(ctx context.Context, e *boss.Encounter, row port.BossRow, ef boss.Effect, at time.Time) (*transition, error) {
	if ef.Ended {
		if err := h.finish(ctx, e, row, at); err != nil {
			return nil, err
		}
	}
	if err := h.Store.Save(ctx, e); err != nil {
		return nil, err
	}
	t := e.TenantID
	if ef.Started {
		ids := make([]uuid.UUID, 0, len(e.Members))
		for _, m := range e.PresentMembers() {
			ids = append(ids, m.StudentID)
		}
		if err := h.Events.Publish(ctx, write.Event{Type: EventBossEncounterStarted, SubjectType: "boss_encounter", SubjectID: e.ID.String(),
			PartitionKey: e.ID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"encounter_id": e.ID, "boss_id": e.BossID,
				"boss_version_id": e.VersionID, "mode": e.Mode, "party_id": e.PartyID, "party_activity_id": e.PartyActivityID,
				"member_ids": ids, "started_at": e.StartedAt, "deadline_at": e.Deadline}}); err != nil {
			return nil, err
		}
	}
	if ef.PhaseCleared != "" {
		if err := h.Events.Publish(ctx, write.Event{Type: EventBossPhaseCompleted, SubjectType: "boss_encounter", SubjectID: e.ID.String(),
			PartitionKey: e.ID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"encounter_id": e.ID, "boss_version_id": e.VersionID,
				"party_id": e.PartyID, "phase_id": ef.PhaseCleared, "phase_index": ef.ClearedIndex, "retry_no": e.Retries,
				"energy_remaining": e.Energy, "completed_at": at}}); err != nil {
			return nil, err
		}
	}
	round := boss.Round{}
	if e.Status == boss.StatusPhaseActive {
		r, err := h.Store.Round(ctx, e.TenantID, e.ID, e.PhaseID, e.Retries)
		if err != nil {
			return nil, err
		}
		round.Correct = r
		round.Items, err = h.itemsOf(ctx, e)
		if err != nil {
			return nil, err
		}
	}
	names, err := h.names(ctx, e)
	if err != nil {
		return nil, err
	}
	return &transition{e: e, row: row, ef: ef, view: partyrules.View(partyrules.ViewInput{E: e, Row: row, Round: round, Names: names, Now: at})}, nil
}

// itemsOf returns the item counts of the plans of the current round.
func (h Bosses) itemsOf(ctx context.Context, e *boss.Encounter) (map[uuid.UUID]map[string]int, error) {
	ps, err := h.Store.Plans(ctx, e.TenantID, e.ID, e.PhaseID, e.Retries)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]map[string]int{}
	for _, p := range ps {
		if out[p.StudentID] == nil {
			out[p.StudentID] = map[string]int{}
		}
		out[p.StudentID][p.RequirementID] = len(p.Items)
	}
	return out, nil
}

// finish fixes the result in the transaction of the end (F103-AC-10): eligibility (domain), first defeat of the
// eligible members, boss.encounter_completed with party_activity_id and the rewards the Reward Engine grants
// (idempotent by (encounter_id, student_id) through the XP ledger source key), the audit record.
func (h Bosses) finish(ctx context.Context, e *boss.Encounter, row port.BossRow, at time.Time) error {
	v := row.Version
	contribs := make([]map[string]any, 0, len(e.Members))
	for i := range e.Members {
		m := &e.Members[i]
		el := m.Eligible != nil && *m.Eligible
		if el {
			use, err := h.Store.Usage(ctx, e.TenantID, m.StudentID, e.BossID, "")
			if err != nil {
				return err
			}
			m.FirstDefeat = !use.Defeated
		}
		contribs = append(contribs, map[string]any{"student_id": m.StudentID, "eligible": el, "first_defeat": m.FirstDefeat})
	}
	rewards := func(rs []boss.Reward) []boss.Reward {
		if rs == nil {
			return []boss.Reward{}
		}
		return rs
	}
	var subject any
	if v.SubjectID != "" {
		subject = v.SubjectID
	}
	t := e.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: EventBossEncounterCompleted, SubjectType: "boss_encounter", SubjectID: e.ID.String(),
		PartitionKey: e.ID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"encounter_id": e.ID, "boss_id": e.BossID,
			"boss_version_id": e.VersionID, "mode": e.Mode, "party_id": e.PartyID, "party_activity_id": e.PartyActivityID, "subject_id": subject,
			"result": e.Result(), "outcome": e.Outcome, "end_reason": e.EndReason, "phases_cleared": e.PhasesCleared, "phase_count": len(v.Phases),
			"contributions": contribs, "defeat_rewards": rewards(v.DefeatRewards), "first_defeat_rewards": rewards(v.FirstDefeatRewards),
			"ended_at": at}}); err != nil {
		return err
	}
	outcome := e.Outcome
	h.after(ctx, func(context.Context) { h.metric(func(m BossMetrics) { m.Ended(outcome) }) })
	return h.Audit.Record(ctx, audit.Event{Action: "boss.encounter_completed", TargetType: "boss_encounter", TargetID: e.ID.String(),
		Result: audit.ResultSuccess, Metadata: map[string]any{"boss_id": e.BossID, "boss_version_id": e.VersionID, "outcome": e.Outcome,
			"end_reason": e.EndReason, "party_activity_id": e.PartyActivityID, "members": len(e.Members)}})
}

// pushed sends the messages of a committed transition: the shared state, the phase messages, the end (eligible to the
// member itself only) — and prepares the question plans of a new round.
func (h Bosses) pushed(ctx context.Context, tr *transition, at time.Time) {
	if tr == nil {
		return
	}
	e, ef := tr.e, tr.ef
	if ef.PhaseCleared != "" {
		var next any
		if e.Status == boss.StatusPhaseActive {
			next = e.PhaseIndex
		}
		h.broadcast(ctx, e, MsgBossPhaseCompleted, map[string]any{"encounter_id": e.ID, "phase_id": ef.PhaseCleared,
			"phase_index": ef.ClearedIndex, "energy_remaining_pct": tr.view.EnergyPct, "next_phase_index": next}, at)
	}
	if ef.PhaseStarted && e.Status == boss.StatusPhaseActive {
		h.metric(func(m BossMetrics) {
			if ef.Retry {
				m.PhaseRetry()
			}
		})
		p, _ := tr.row.Version.Phase(e.PhaseIndex)
		payload := map[string]any{"encounter_id": e.ID, "phase_id": e.PhaseID, "phase_index": e.PhaseIndex, "retry": ef.Retry}
		if e.PhaseDeadline != nil {
			payload["deadline_at"] = e.PhaseDeadline.UTC().Format(time.RFC3339Nano)
		}
		if p.Presentation != nil && p.Presentation.PhaseClearAnimation != "" {
			payload["presentation_ref"] = p.Presentation.PhaseClearAnimation
		}
		h.broadcast(ctx, e, MsgBossPhaseStarted, payload, at)
	}
	h.broadcast(ctx, e, MsgBossUpdated, partyrules.Updated(tr.view), at)
	if ef.Ended {
		if h.Realtime != nil {
			for _, m := range e.Members {
				_ = h.Realtime.BossMessage(ctx, m.StudentID, MsgBossEnded, map[string]any{"encounter_id": e.ID, "outcome": e.Outcome,
					"phases_cleared": e.PhasesCleared, "phase_count": len(tr.row.Version.Phases), "eligible": m.Eligible != nil && *m.Eligible}, at)
			}
		}
		if h.Room != nil {
			_ = h.Room.Close(ctx, e.ID)
		}
		return
	}
	if ef.PhaseStarted {
		h.preparePlans(ctx, e.TenantID, e.ID)
	}
}

// preparePlans creates the missing question plans of the current round (one per present member and requirement,
// per_grade by the grade of the member, research R1) outside any transaction, stores them and sends each member its
// own questions (boss.question.assigned, without answers). A failure of module learning is counted on the encounter:
// past the retries the encounter is abandoned without reward and without counting an attempt (edge case).
func (h Bosses) preparePlans(ctx context.Context, tenant, id uuid.UUID) {
	if h.Learning == nil {
		return
	}
	type need struct {
		student uuid.UUID
		grade   int
		req     boss.Requirement
	}
	var e *boss.Encounter
	var row port.BossRow
	var needs []need
	err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if e, err = h.Store.Lock(ctx, tenant, id); err != nil || e == nil || e.Status != boss.StatusPhaseActive {
			e = nil
			return err
		}
		if row, err = h.version(ctx, e); err != nil {
			return err
		}
		have, err := h.Store.Plans(ctx, tenant, id, e.PhaseID, e.Retries)
		if err != nil {
			return err
		}
		p, _ := row.Version.Phase(e.PhaseIndex)
		for _, m := range e.PresentMembers() {
			for _, q := range p.Requirements {
				if !slices.ContainsFunc(have, func(r port.BossPlanRow) bool { return r.StudentID == m.StudentID && r.RequirementID == q.RequirementID }) {
					needs = append(needs, need{student: m.StudentID, grade: m.Grade, req: q})
				}
			}
		}
		return nil
	})
	if err != nil || e == nil || len(needs) == 0 {
		return
	}
	phase, retry, deadline := e.PhaseID, e.Retries, e.PhaseDeadline
	failed := false
	created := map[uuid.UUID][]port.BossPlanRow{}
	for _, n := range needs {
		a, ok := n.req.ActivityFor(n.grade)
		aid, perr := uuid.Parse(a.ActivityID)
		if !ok || perr != nil {
			failed = true
			continue
		}
		key := partyrules.PlanKey(id, phase, retry, n.student, n.req.RequirementID)
		plan, err := h.Learning.CreateBossPlan(h.asMember(ctx, tenant, n.student), port.BossQuestionPlan{PlanKey: key, EncounterID: id,
			StudentID: n.student, GradeLevel: n.grade, ActivityID: aid, ActivityVersion: a.ActivityVersion, QuestionCount: h.questionCount()})
		if err != nil {
			failed = true
			continue
		}
		pr := port.BossPlanRow{ID: newID(), TenantID: tenant, EncounterID: id, PhaseID: phase, RetryNo: retry, StudentID: n.student,
			RequirementID: n.req.RequirementID, PlanID: plan.PlanID, Items: plan.Items, ExpiresAt: plan.ExpiresAt, CreatedAt: h.now()}
		if err := h.UoW.Do(ctx, func(ctx context.Context) error { return h.Store.InsertPlan(ctx, pr) }); err != nil {
			failed = true
			continue
		}
		created[n.student] = append(created[n.student], pr)
	}
	at := h.now()
	if h.Realtime != nil {
		for student, ps := range created {
			for _, pr := range ps {
				qs := make([]map[string]any, 0, len(pr.Items))
				for _, it := range pr.Items {
					qs = append(qs, partyrules.QuestionRender(pr.PlanID, it))
				}
				payload := map[string]any{"encounter_id": id, "phase_id": phase, "requirement_id": pr.RequirementID, "questions": qs}
				if deadline != nil {
					payload["deadline_at"] = deadline.UTC().Format(time.RFC3339Nano)
				}
				_ = h.Realtime.BossMessage(ctx, student, MsgBossQuestions, payload, at)
			}
		}
	}
	if !failed {
		return
	}
	var tr *transition
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		e, err := h.Store.Lock(ctx, tenant, id)
		if err != nil || e == nil || e.Status != boss.StatusPhaseActive || e.PhaseID != phase || e.Retries != retry {
			return err
		}
		row, err := h.version(ctx, e)
		if err != nil {
			return err
		}
		ef := e.PlanFailed(h.Config.PlanRetries, at)
		tr, err = h.apply(ctx, e, row, ef, at)
		return err
	})
	if err == nil && tr != nil && tr.ef.Ended {
		h.pushed(ctx, tr, at)
	}
}

func (h Bosses) questionCount() int {
	if h.Config.QuestionCount > 0 {
		return h.Config.QuestionCount
	}
	return 10
}

// asMember is the context of a call to module learning on behalf of a member: its Principal and the TenantScope of the
// encounter (pkg/modcall forwards both; the learning record belongs to that student).
func (h Bosses) asMember(ctx context.Context, tenant, student uuid.UUID) context.Context {
	t := tenant
	p := authz.NewPrincipal(authz.PrincipalData{UserID: student, UserType: authz.UserTypeStudent, TenantID: &t})
	if sc, err := authz.ScopeFor(p); err == nil {
		return withScope(authz.WithPrincipal(ctx, p), sc)
	}
	return ctx
}
