package command

import (
	"context"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Co-op quest events and realtime messages (F103 US2/US3).
const (
	EventQuestPartyCompleted = "quest.party_completed"
	MsgQuestRolesAssigned    = "quest.roles.assigned"
	MsgQuestClueShared       = "quest.clue.shared"
)

// FailPartyDisbanded is the failure_conditions kind of a disbanded party (quest.schema.json).
const FailPartyDisbanded = "party_disbanded"

// PartyQuestConfig is the configuration of the co-op quests (CONFIG_DEFAULTS through infrastructure/config).
type PartyQuestConfig struct {
	Grace            time.Duration // coop_quest_reconnect_grace (120 s)
	CheckEvery       time.Duration // period of the presence checks of a session room
	HeartbeatTimeout time.Duration // a member is online while its game session heartbeat is within it
}

// PartyQuestMetrics observes the co-op quests; nil = not observed.
type PartyQuestMetrics interface {
	StepShared(policy string)
	SessionEnded(outcome string)
}

// PartyQuests are the co-op quest use cases (F103 US2, US3): startPartyActivity (quest), the shared frame steps
// (consumer of quest.step_completed), the collective targets and interdependent steps (consumer of activity.graded),
// the roles and clue cards (assignQuestStepRoles, shareQuestStepClue), the party membership consumers and the presence
// timer of the session room. Learning results are never copied between members (FR-PTY-013): a member completes a
// learning step only from its own graded attempt; the session decides which quest steps count for whom. Lock order:
// the session row, then the members' progress in join order (research R4).
type PartyQuests struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Content     port.QuestContent
	Party       port.PartyQuestContent
	World       port.WorldContent
	Store       write.PartyQuestStore
	Quests      Quests // the quest engine of F072 (progress store, events, pushes)
	Events      write.EventPublisher
	Audit       write.Auditor
	Messages    port.QuestMessages // nil: no push
	Renders     port.ClueRenders   // clue renders (module learning behind the cache); nil: shareQuestStepClue 503
	Presence    port.OnlineMembers // nil: every member counts as online
	Room        port.GameRoom      // nil: no presence timer
	Names       port.NicknameReader
	Metrics     PartyQuestMetrics
	Config      PartyQuestConfig
	Now         func() time.Time
}

func (h PartyQuests) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h PartyQuests) after(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit != nil {
		_ = h.AfterCommit(ctx, fn)
		return
	}
	fn(ctx)
}

func (h PartyQuests) metric(fn func(m PartyQuestMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

func partyQuestProblem(err error) error {
	switch {
	case errors.Is(err, partyquest.ErrNotLeader):
		return problem.New(problem.PartyNotLeader, "")
	case errors.Is(err, partyquest.ErrNotRoleOwner):
		return problem.New(problem.PermissionDenied, "the role is held by another member")
	case errors.Is(err, partyquest.ErrNotMember):
		return problem.New(problem.NotFound, "")
	case errors.Is(err, partyquest.ErrInvalid):
		return problem.Invalid("assignments", problem.SubInvalid, err.Error())
	case errors.Is(err, partyquest.ErrInvalidState):
		return problem.New(problem.Conflict, "the party quest session has ended")
	}
	return err
}

// StartParty creates the shared session of a party quest (F103-AC-13): idempotent by party_activity_id; the quest
// must be assigned to the tenant and played by a party (party_mode ≠ solo), the party within party_size, every member
// eligible (grade, prerequisites, repeatability — 422 PARTY_ACTIVITY_NOT_ELIGIBLE) and below max_active_quests. Each
// member's QuestProgress is the active one (now attached to the party) or a new one pinned to the current version.
func (h PartyQuests) StartParty(ctx context.Context, in PartyStart) (out PartyStarted, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.StartPartyQuest")
	defer func() { done(err) }()
	at := h.now()
	var s *partyquest.Session
	var v quest.Version
	created := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		prev, err := h.Store.ByPartyActivity(ctx, in.TenantID, in.PartyActivityID)
		if err != nil {
			return err
		}
		if prev != nil {
			s = prev
			row, err := h.Content.QuestVersion(ctx, prev.VersionID)
			if err != nil || row == nil {
				return err
			}
			v = row.Version
			return nil
		}
		row, err := h.Party.QuestByCode(ctx, in.Ref)
		if err != nil {
			return err
		}
		if row == nil {
			return problem.New(problem.NotFound, "quest not found")
		}
		if ok, err := h.Content.Assigned(ctx, in.TenantID, row.QuestID); err != nil || !ok {
			if err != nil {
				return err
			}
			return problem.New(problem.NotFound, "quest not found")
		}
		v = row.Version
		if v.PartyMode == "" || v.PartyMode == partyquest.PartySolo {
			return problem.New(problem.PartyActivityNotEligible, "the quest is solo only")
		}
		if v.PartySize != nil && (len(in.Members) < v.PartySize.Min || len(in.Members) > v.PartySize.Max) {
			return problem.New(problem.PartyActivityNotEligible, "the party size is outside party_size").
				With("min_members", v.PartySize.Min).With("max_members", v.PartySize.Max)
		}
		ms, err := partyrules.Members(ctx, h.World, h.Content, in.TenantID, in.Members)
		if err != nil {
			return err
		}
		if err := membersOfTenant(ms); err != nil {
			return err
		}
		checks := make([]partyrules.Check, len(ms))
		ok := true
		for i, m := range ms {
			// A member already playing this quest keeps its progress (it is attached to the party, never restarted).
			if hs := m.Hist[row.QuestID]; m.Valid() && hs != nil && hs.Latest != nil && hs.Latest.Active() {
				checks[i] = partyrules.Check{StudentID: m.ID, Eligible: true}
				continue
			}
			checks[i] = partyrules.QuestCheck(v, m, h.Quests.Config.Reset, row.QuestID, at)
			ok = ok && checks[i].Eligible
		}
		if !ok {
			return notEligibleMembers(checks)
		}
		// Lock the members in a fixed order (no deadlock with the consumers that lock the session first: a new session
		// is not visible to them yet).
		order := slices.Clone(ms)
		sort.Slice(order, func(i, j int) bool { return order[i].ID.String() < order[j].ID.String() })
		party := in.PartyID
		for _, m := range order {
			if err := h.Quests.Store.LockStudent(ctx, m.ID); err != nil {
				return err
			}
			p, err := h.Quests.Store.Active(ctx, in.TenantID, m.ID, row.QuestID)
			if err != nil {
				return err
			}
			if p != nil {
				if p.PartyID == nil || *p.PartyID != party {
					p.PartyID = &party
					if err := h.Quests.Store.Save(ctx, p); err != nil {
						return err
					}
				}
				continue
			}
			n, err := h.Quests.Store.CountActive(ctx, in.TenantID, m.ID)
			if err != nil {
				return err
			}
			if max := h.Quests.Config.MaxActive; max > 0 && n >= max {
				return notEligibleMembers([]partyrules.Check{{StudentID: m.ID, Reason: partyrules.ReasonAttempts,
					Requirement: "max_active_quests"}})
			}
			completed := 0
			if hs := m.Hist[row.QuestID]; hs != nil {
				completed = hs.Completed
			}
			if _, err := h.Quests.start(ctx, *row, m.Student, in.TenantID, m.ID, v.StartTrigger.Kind, &party, completed+1, at); err != nil {
				return err
			}
		}
		school := ms[0].Student.SchoolID
		for _, m := range ms {
			if m.ID == in.LeaderID {
				school = m.Student.SchoolID
			}
		}
		s = partyquest.New(partyquest.Start{ID: newID(), TenantID: in.TenantID, SchoolID: school, PartyID: party,
			PartyActivityID: in.PartyActivityID, LeaderID: in.LeaderID, Version: v, QuestID: row.QuestID, VersionID: row.VersionID,
			Members: in.Members, At: at})
		if err := h.assignActiveRoles(ctx, s, v); err != nil {
			return err
		}
		inserted, err := h.Store.Insert(ctx, s)
		if err != nil {
			return err
		}
		if !inserted {
			if s, err = h.Store.ByPartyActivity(ctx, in.TenantID, in.PartyActivityID); err != nil || s == nil {
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
	out = PartyStarted{ExternalRef: s.ID, RoomType: "private_instance", AreaID: v.AreaID}
	if created {
		if h.Room != nil {
			_ = h.Room.Open(ctx, s.TenantID, s.ID, at.Add(h.checkEvery()))
		}
		h.pushRoles(ctx, s, v, at)
	}
	return out, nil
}

func (h PartyQuests) checkEvery() time.Duration {
	if h.Config.CheckEvery > 0 {
		return h.Config.CheckEvery
	}
	return 5 * time.Second
}

// assignActiveRoles assigns the roles of the interdependent steps active for a present member and not assigned yet
// (F103-AC-20: when the step becomes active).
func (h PartyQuests) assignActiveRoles(ctx context.Context, s *partyquest.Session, v quest.Version) error {
	for _, st := range v.SortedSteps() {
		if st.Type != quest.StepInterdep || s.RolesAssigned(st.ID) {
			continue
		}
		active := false
		for _, id := range s.Present() {
			p, err := h.Quests.Store.Active(ctx, s.TenantID, id, s.QuestID)
			if err != nil {
				return err
			}
			if p != nil && p.Steps[st.ID].Status == quest.StepActive {
				active = true
				break
			}
		}
		if active {
			s.AssignAuto(st)
		}
	}
	return nil
}

// pushRoles sends quest.roles.assigned for every step with roles (role names and nicknames only, never a clue).
func (h PartyQuests) pushRoles(ctx context.Context, s *partyquest.Session, v quest.Version, at time.Time) {
	if h.Messages == nil || len(s.Roles) == 0 {
		return
	}
	names := map[uuid.UUID]string{}
	if h.Names != nil {
		if n, err := h.Names.Nicknames(ctx, s.TenantID, s.Present()); err == nil {
			names = n
		}
	}
	for stepID, r := range s.Roles {
		st, ok := v.Step(stepID)
		if !ok {
			continue
		}
		var as []map[string]any
		for _, role := range st.Roles {
			if id, ok := r.Assignments[role.Role]; ok {
				as = append(as, map[string]any{"role": role.Role, "student_id": id, "nickname": names[id]})
			}
		}
		for _, id := range s.Present() {
			_ = h.Messages.QuestMessage(ctx, id, MsgQuestRolesAssigned, map[string]any{"quest_id": s.QuestID, "step_id": stepID,
				"assignments": as, "my_roles": s.RolesOf(st, id), "pending_leader": r.Pending}, at)
		}
	}
}

// memberProgress locks and returns the active progress of the present members of a session on its quest, in join
// order (the session row is locked by the caller first).
func (h PartyQuests) memberProgress(ctx context.Context, s *partyquest.Session) ([]*quest.Progress, error) {
	var out []*quest.Progress
	for _, id := range s.Present() {
		if err := h.Quests.Store.LockStudent(ctx, id); err != nil {
			return nil, err
		}
		p, err := h.Quests.Store.Active(ctx, s.TenantID, id, s.QuestID)
		if err != nil {
			return nil, err
		}
		if p != nil && p.PartyID != nil && *p.PartyID == s.PartyID {
			out = append(out, p)
		}
	}
	return out, nil
}

// complete completes a step for one member (progress pinned to its own version) and records its events.
func (h PartyQuests) complete(ctx context.Context, p *quest.Progress, stepID string, at time.Time, extra map[string]any) error {
	v, err := h.Quests.version(ctx, p)
	if err != nil {
		return err
	}
	o := p.CompleteStep(v, stepID, at)
	if !o.Changed {
		return nil
	}
	push, err := h.Quests.record(ctx, p, v, []quest.Outcome{o}, nil, at)
	if err != nil {
		return err
	}
	for k, val := range extra {
		push[k] = val
	}
	student := p.StudentID
	h.after(ctx, func(ctx context.Context) { h.Quests.push(ctx, student, push, at) })
	return nil
}

// end closes a session once with quest.party_completed and the audit record (F103-AC-17).
func (h PartyQuests) end(ctx context.Context, s *partyquest.Session, status, reason string, at time.Time) error {
	if !s.End(status, reason, at) {
		return nil
	}
	ids := make([]string, 0, len(s.Members))
	for _, m := range s.Members {
		ids = append(ids, m.StudentID.String())
	}
	t := s.TenantID
	if err := h.Events.Publish(ctx, write.Event{Type: EventQuestPartyCompleted, SubjectType: "party_quest_session", SubjectID: s.ID.String(),
		PartitionKey: s.PartyID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"party_quest_session_id": s.ID,
			"party_activity_id": s.PartyActivityID, "party_id": s.PartyID, "quest_id": s.QuestID, "quest_version_id": s.VersionID,
			"outcome": status, "end_reason": reason, "member_ids": ids, "ended_at": at}}); err != nil {
		return err
	}
	h.after(ctx, func(ctx context.Context) {
		h.metric(func(m PartyQuestMetrics) { m.SessionEnded(status) })
		if h.Room != nil {
			_ = h.Room.Close(ctx, s.ID)
		}
	})
	return h.Audit.Record(ctx, audit.Event{Action: "quest.party_completed", TargetType: "party_quest_session", TargetID: s.ID.String(),
		Result: audit.ResultSuccess, Metadata: map[string]any{"quest_id": s.QuestID, "party_activity_id": s.PartyActivityID, "outcome": status,
			"end_reason": reason}})
}

// checkEnd ends the session completed when every present member completed the quest (F103-AC-17).
func (h PartyQuests) checkEnd(ctx context.Context, s *partyquest.Session, at time.Time) error {
	if !s.Active() {
		return nil
	}
	present := s.Present()
	if len(present) == 0 {
		return h.end(ctx, s, partyquest.StatusAbandoned, partyquest.EndAbandoned, at)
	}
	hist := 0
	for _, id := range present {
		hs, err := h.Content.Histories(ctx, s.TenantID, id)
		if err != nil {
			return err
		}
		if x := hs[s.QuestID]; x != nil && x.Latest != nil && x.Latest.Status == quest.StatusCompleted && x.Latest.PartyID != nil &&
			*x.Latest.PartyID == s.PartyID {
			hist++
		}
	}
	if hist == len(present) {
		return h.end(ctx, s, partyquest.StatusCompleted, partyquest.EndCompleted, at)
	}
	return nil
}

// StepCompleted is the consumer step of quest.step_completed (F103-AC-14, AC-20): a frame step a member completed in a
// shared policy is completed for every present member of the session (once — the propagated completions are
// replays for the session); interdependent steps that became active get their roles; the session ends when every
// member completed the quest. Runs in the consumer transaction (TenantScope of the envelope).
func (h PartyQuests) StepCompleted(ctx context.Context, tenant, progressID uuid.UUID, stepID string) error {
	p, err := h.Store.ProgressRef(ctx, tenant, progressID)
	if err != nil || p == nil || p.PartyID == nil {
		return err
	}
	s, err := h.Store.ActiveFor(ctx, tenant, *p.PartyID, p.QuestID, p.StudentID)
	if err != nil || s == nil {
		return err
	}
	row, err := h.Content.QuestVersion(ctx, s.VersionID)
	if err != nil || row == nil {
		return err
	}
	v := row.Version
	at := h.now()
	if st, ok := v.Step(stepID); ok && s.SharesStep(st) && s.MarkShared(stepID, p.StudentID, at) {
		h.after(ctx, func(context.Context) { h.metric(func(m PartyQuestMetrics) { m.StepShared(s.Policy) }) })
		ps, err := h.memberProgress(ctx, s)
		if err != nil {
			return err
		}
		for _, mp := range ps {
			if mp.StudentID == p.StudentID {
				continue
			}
			if err := h.complete(ctx, mp, stepID, at, map[string]any{"party_progress": map[string]any{"step_id": stepID,
				"shared_completed": true}}); err != nil {
				return err
			}
		}
	}
	before := len(s.Roles)
	if err := h.assignActiveRoles(ctx, s, v); err != nil {
		return err
	}
	if err := h.checkEnd(ctx, s, at); err != nil {
		return err
	}
	if err := h.Store.Save(ctx, s); err != nil {
		return err
	}
	if len(s.Roles) > before {
		h.after(ctx, func(ctx context.Context) { h.pushRoles(ctx, s, v, at) })
	}
	return nil
}

// Graded is activity.graded of a quest step attempt as the co-op quests read it.
type Graded = GradedAttempt

// ActivityGraded is the consumer step of activity.graded for the co-op quests (F103-AC-15, AC-16, AC-23): the attempt
// is the student's own (never copied). A collective step adds the metric of the attempt to the contribution of the
// student and completes the step for the members with min_contribution once the party reached count; an interdependent
// step completes for the members who passed their own final question once the completion of the step is met. Without
// an active session the pass rule of the student applies (F072). Runs in the consumer transaction.
func (h PartyQuests) ActivityGraded(ctx context.Context, g Graded) error {
	sa, err := h.Store.AttemptRef(ctx, g.TenantID, g.AttemptID)
	if err != nil || sa == nil || sa.StudentID != g.StudentID {
		return err
	}
	p, err := h.Store.ProgressRef(ctx, g.TenantID, sa.ProgressID)
	if err != nil || p == nil || p.PartyID == nil {
		return err
	}
	pv, err := h.Quests.version(ctx, p)
	if err != nil {
		return err
	}
	st, ok := pv.Step(sa.StepID)
	if !ok || (st.CollectiveTarget == nil && st.Type != quest.StepInterdep) {
		return nil // the quest engine (F072) applied the pass rule of the student
	}
	passed := h.Quests.Config.PassRule.Effective(st.PassRule).Passes(quest.Graded{CorrectCount: g.CorrectCount, ItemCount: g.ItemCount,
		Passed: g.Passed})
	at := h.now()
	s, err := h.Store.ActiveFor(ctx, g.TenantID, *p.PartyID, p.QuestID, p.StudentID)
	if err != nil {
		return err
	}
	if s == nil || (st.CollectiveTarget != nil && !s.CollectiveStep(st)) {
		if !passed {
			return nil
		}
		if err := h.Quests.Store.LockStudent(ctx, p.StudentID); err != nil {
			return err
		}
		locked, err := h.Quests.Store.ByID(ctx, g.TenantID, p.ID)
		if err != nil || locked == nil {
			return err
		}
		return h.complete(ctx, locked, st.ID, at, nil)
	}
	row, err := h.Content.QuestVersion(ctx, s.VersionID)
	if err != nil || row == nil {
		return err
	}
	v := row.Version
	if sv, ok := v.Step(st.ID); ok {
		st = sv
	}
	var ready []uuid.UUID // members whose step completes now
	if s.CollectiveStep(st) {
		metric := 0
		if g.CorrectCount != nil {
			metric = *g.CorrectCount
		}
		if st.CollectiveTarget.Metric == "completed_items" {
			metric = 0
			if g.ItemCount != nil {
				metric = *g.ItemCount
			}
		}
		totals, added, err := h.Store.AddContribution(ctx, g.TenantID, s.ID, p.StudentID, st.ID, g.AttemptID, metric,
			st.CollectiveTarget.MinContribution)
		if err != nil || !added {
			return err
		}
		sum := 0
		for _, n := range totals {
			sum += n
		}
		s.Collective(st, sum, at)
		if s.CollectiveDone(st.ID) {
			for _, id := range s.Present() {
				if totals[id] >= st.CollectiveTarget.MinContribution {
					ready = append(ready, id)
				}
			}
		}
	} else if passed {
		totals, _, err := h.Store.AddContribution(ctx, g.TenantID, s.ID, p.StudentID, st.ID, g.AttemptID, 1, 1)
		if err != nil {
			return err
		}
		ok := map[uuid.UUID]bool{}
		for id, n := range totals {
			ok[id] = n > 0
		}
		if s.InterdependentDone(st, ok) {
			for _, id := range s.Present() {
				if ok[id] {
					ready = append(ready, id)
				}
			}
		}
	}
	ps, err := h.memberProgress(ctx, s)
	if err != nil {
		return err
	}
	progress := s.CollectiveProgress(st)
	for _, mp := range ps {
		if slices.Contains(ready, mp.StudentID) {
			extra := map[string]any{"party_progress": map[string]any{"step_id": st.ID, "collective_progress": progress, "my_contribution_met": true}}
			if err := h.complete(ctx, mp, st.ID, at, extra); err != nil {
				return err
			}
			continue
		}
		push := progressPush(mp, st.ID, nil)
		push["party_progress"] = map[string]any{"step_id": st.ID, "collective_progress": progress, "my_contribution_met": false}
		student := mp.StudentID
		h.after(ctx, func(ctx context.Context) { h.Quests.push(ctx, student, push, at) })
	}
	if err := h.checkEnd(ctx, s, at); err != nil {
		return err
	}
	return h.Store.Save(ctx, s)
}

// MemberLeft is the co-op quest part of the consumer of party.member_left / party.member_kicked (F103-AC-19): the
// member leaves the active sessions of the party (its QuestProgress and the shared steps already completed stay); its
// roles move to the present members (allow_multi_role); the session ends abandoned when it no longer continues.
func (h PartyQuests) MemberLeft(ctx context.Context, tenant, party, student uuid.UUID, reason string) error {
	ids, err := h.Store.ActiveOfParty(ctx, tenant, party)
	if err != nil {
		return err
	}
	at := h.now()
	for _, id := range ids {
		s, err := h.Store.Lock(ctx, tenant, id)
		if err != nil || s == nil {
			if err != nil {
				return err
			}
			continue
		}
		row, err := h.Content.QuestVersion(ctx, s.VersionID)
		if err != nil || row == nil {
			return err
		}
		if !s.Leave(row.Version, student, reason, at) {
			continue
		}
		if !s.Continues() {
			if err := h.end(ctx, s, partyquest.StatusAbandoned, partyquest.EndAbandoned, at); err != nil {
				return err
			}
		} else if err := h.checkEnd(ctx, s, at); err != nil {
			return err
		}
		if err := h.Store.Save(ctx, s); err != nil {
			return err
		}
		v := row.Version
		h.after(ctx, func(ctx context.Context) { h.pushRoles(ctx, s, v, at) })
	}
	return nil
}

// Disbanded is the co-op quest part of the consumer of party.disbanded (F103-AC-19): failure_conditions
// party_disbanded applies to the active progress of every member (reset_step by default — the active steps restart —
// or fail_quest); without that condition the progress stays as it is. The session ends once (failed with fail_quest,
// abandoned otherwise).
func (h PartyQuests) Disbanded(ctx context.Context, tenant, party uuid.UUID) error {
	ids, err := h.Store.ActiveOfParty(ctx, tenant, party)
	if err != nil {
		return err
	}
	at := h.now()
	for _, id := range ids {
		s, err := h.Store.Lock(ctx, tenant, id)
		if err != nil || s == nil {
			if err != nil {
				return err
			}
			continue
		}
		row, err := h.Content.QuestVersion(ctx, s.VersionID)
		if err != nil || row == nil {
			return err
		}
		fc, declared := failureOf(row.Version, FailPartyDisbanded)
		ps, err := h.memberProgress(ctx, s)
		if err != nil {
			return err
		}
		status := partyquest.StatusAbandoned
		if declared && fc.Policy() == quest.OnFailQuest {
			status = partyquest.StatusFailed
		}
		for _, p := range ps {
			if !declared {
				continue
			}
			v, err := h.Quests.version(ctx, p)
			if err != nil {
				return err
			}
			if fc.Policy() == quest.OnFailQuest {
				if err := h.Quests.end(ctx, p, v, AbandonFailed, fc.StepID, FailPartyDisbanded, at); err != nil {
					return err
				}
			} else if len(p.ResetSteps(v, fc.StepID, at)) > 0 {
				if err := h.Quests.Store.Save(ctx, p); err != nil {
					return err
				}
			}
			push := progressPush(p, fc.StepID, nil)
			student := p.StudentID
			h.after(ctx, func(ctx context.Context) { h.Quests.push(ctx, student, push, at) })
		}
		if err := h.end(ctx, s, status, partyquest.EndDisbanded, at); err != nil {
			return err
		}
		if err := h.Store.Save(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// failureOf returns a declared failure condition of a kind.
func failureOf(v quest.Version, kind string) (quest.FailureCondition, bool) {
	for _, f := range v.FailureConditions {
		if f.Kind == kind {
			return f, true
		}
	}
	return quest.FailureCondition{}, false
}

// OnTimer is the presence timer of the room of a session (F103-AC-19, AC-24): members offline past
// coop_quest_reconnect_grace leave (progress kept, roles moved); the session ends abandoned when it no longer
// continues. done = the session ended.
func (h PartyQuests) OnTimer(ctx context.Context, tenant, id uuid.UUID) (bool, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return true, nil //nolint:nilerr // a room without a valid tenant can never run: it ends instead of retrying forever
	}
	ctx = withScope(ctx, sc)
	at := h.now()
	done := false
	var push func(ctx context.Context)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Lock(ctx, tenant, id)
		if err != nil {
			return err
		}
		if s == nil || !s.Active() {
			done = true
			return nil
		}
		row, err := h.Content.QuestVersion(ctx, s.VersionID)
		if err != nil || row == nil {
			return err
		}
		online := map[uuid.UUID]bool{}
		for _, m := range s.Present() {
			online[m] = true
		}
		if h.Presence != nil {
			if online, err = h.Presence.Online(ctx, tenant, s.Present(), at.Add(-h.Config.HeartbeatTimeout)); err != nil {
				return err
			}
		}
		version := s.StateVersion
		left := s.Presence(row.Version, online, h.Config.Grace, at)
		if len(left) > 0 && !s.Continues() {
			if err := h.end(ctx, s, partyquest.StatusAbandoned, partyquest.EndAbandoned, at); err != nil {
				return err
			}
		}
		if s.StateVersion == version {
			return nil
		}
		done = !s.Active()
		if len(left) > 0 {
			v := row.Version
			push = func(ctx context.Context) { h.pushRoles(ctx, s, v, at) }
		}
		return h.Store.Save(ctx, s)
	})
	if err != nil {
		return false, err
	}
	if push != nil {
		push(ctx)
	}
	return done, nil
}

// RoomCheckpoint is the checkpoint of the room of a session: due every check period while it is active.
func (h PartyQuests) RoomCheckpoint(ctx context.Context, tenant, id uuid.UUID) (time.Time, bool, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return time.Time{}, false, nil //nolint:nilerr // a room without a valid tenant can never run: it ends instead of retrying forever
	}
	ctx = withScope(ctx, sc)
	active := false
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Store.Lock(ctx, tenant, id)
		if err != nil || s == nil {
			return err
		}
		active = s.Active()
		return nil
	})
	return h.now().Add(h.checkEvery()), active, err
}
