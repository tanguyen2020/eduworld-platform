package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Battle engine of module competition (F105): the application side of the match rooms. The room host
// (infrastructure/battlehost) holds each room in memory under its lease and calls these use cases for every effect
// with I/O — the question plan and the grading of module learning (outside any transaction, R29), the durable result,
// the abort, memberships and integrity signals — always in the TenantScope of the match (R8).

// Audit actions of the battle engine (AGENTS R10).
const (
	AuditResultRecorded    = "match_result.recorded"
	AuditMatchAborted      = "match.aborted"
	AuditRestrictionCreate = "competition_restriction.created"
)

// Results of eduworld_match_results_total (MONITORING §3).
const (
	MatchResultRecorded  = "recorded"
	MatchResultDuplicate = "duplicate_rejected"
)

// Errors of the battle engine.
var (
	// ErrMatchUnknown: no match of the tenant with this id.
	ErrMatchUnknown = errors.New("battle: unknown match")
	// ErrMatchOver: the match is completed, cancelled or aborted.
	ErrMatchOver = errors.New("battle: match over")
	// ErrMatchRunning: the match room was opened already (its host or checkpoint has it).
	ErrMatchRunning = errors.New("battle: match room already opened")
	// ErrMatchAborted: the room could not open (plan failure) and the match was aborted.
	ErrMatchAborted = errors.New("battle: match aborted")
)

// BattleInfo is what the room host keeps of a match next to its room (and in its checkpoint).
type BattleInfo struct {
	MatchID          uuid.UUID             `json:"match_id"`
	Tenant           uuid.UUID             `json:"tenant_id"`
	Mode             string                `json:"mode"`
	Type             string                `json:"matchmaking_type"`
	SubjectID        uuid.UUID             `json:"subject_id"`
	TopicID          *uuid.UUID            `json:"topic_id,omitempty"`
	Grade            int                   `json:"grade_level"`
	Ranked           bool                  `json:"ranked"`
	IsBot            bool                  `json:"is_bot"`
	TournamentID     *uuid.UUID            `json:"tournament_id,omitempty"`
	TournamentNodeID *uuid.UUID            `json:"tournament_node_id,omitempty"`
	RulesetVersionID uuid.UUID             `json:"ruleset_version_id"`
	RoomType         string                `json:"room_type"`
	AbandonPenalty   battle.AbandonPenalty `json:"abandon_penalty"`
	// LossStreak is battle.loss_streak_encouragement of the pinned configuration (0: no encouragement, old checkpoints).
	LossStreak int `json:"loss_streak_encouragement,omitempty"`
}

// CardReader reads the public cards of students (ref.player_profiles: nickname only, R16).
type CardReader interface {
	Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error)
}

// Battles holds the use cases of the battle engine.
type Battles struct {
	UoW      write.UnitOfWork
	Repo     write.BattleRepository
	Plans    port.QuestionPlans
	Cards    CardReader
	Events   write.EventPublisher
	Audit    write.Auditor
	Metrics  port.BattleMetrics // nil: no metrics
	Now      func() time.Time
	Instance string // recorded_by of the results (EW_INSTANCE_ID)
}

func (h Battles) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Battles) tx(ctx context.Context, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return h.UoW.InTenantTx(ctx, sc, fn)
}

// onBehalf is the context of a facade call made for a student: its Principal and the TenantScope of the match (the
// learning record is the student's own, R29).
func onBehalf(ctx context.Context, tenant, student uuid.UUID) (context.Context, error) {
	t := tenant
	p := authz.NewPrincipal(authz.PrincipalData{UserID: student, UserType: authz.UserTypeStudent, TenantID: &t})
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(authz.WithPrincipal(ctx, p), sc), nil
}

// Open opens the room of a match created by matchmaking (match.created, F105-AC-01): pins the question plan of module
// learning before the countdown and records the room. A plan failure aborts the match (no rating, no abandon).
func (h Battles) Open(ctx context.Context, matchID, tenant uuid.UUID) (info BattleInfo, room *battle.Room, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.OpenMatchRoom")
	defer func() { done(err) }()
	var m *write.BattleMatch
	var payload json.RawMessage
	var cards map[uuid.UUID]port.Card
	if err := h.tx(ctx, tenant, func(ctx context.Context) error {
		var err error
		if m, err = h.Repo.BattleMatch(ctx, matchID); err != nil || m == nil {
			return err
		}
		if payload, err = h.Repo.ConfigPayload(ctx, m.RulesetVersionID); err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(m.Participants))
		for i, p := range m.Participants {
			ids[i] = p.StudentID
		}
		cards, err = h.Cards.Cards(ctx, tenant, ids)
		return err
	}); err != nil {
		return info, nil, err
	}
	if m == nil {
		return info, nil, ErrMatchUnknown
	}
	switch m.Status {
	case "lobby":
	case "running":
		return info, nil, ErrMatchRunning
	default:
		return info, nil, ErrMatchOver
	}
	tournament := m.Mode == battle.ModeTournament || m.TournamentNodeID != nil
	info = BattleInfo{MatchID: m.ID, Tenant: tenant, Mode: m.Mode, Type: m.Type, SubjectID: m.SubjectID, TopicID: m.TopicID, Grade: m.Grade, Ranked: m.Ranked,
		IsBot: m.IsBot, TournamentID: m.TournamentID, TournamentNodeID: m.TournamentNodeID, RulesetVersionID: m.RulesetVersionID,
		RoomType: "battle"}
	if m.Mode == battle.ModeCoop {
		info.RoomType = "coop"
	}
	abort := func(cause error) (BattleInfo, *battle.Room, error) {
		if err := h.abortMatch(ctx, info, m.Participants, battle.EndPlanFailure); err != nil {
			return info, nil, err
		}
		return info, nil, fmt.Errorf("%w: %w", ErrMatchAborted, cause)
	}
	if len(m.TenantIDs) != 1 || m.TenantIDs[0] != tenant {
		// A platform match spans tenants: its plan needs the platform question pool of F107 [Proposed F105].
		return abort(errors.New("battle: platform matches are not played by F105"))
	}
	cfg, err := battle.ParseConfig(payload)
	if err != nil {
		return abort(err)
	}
	info.AbandonPenalty = cfg.AbandonPenalty
	info.LossStreak = cfg.LossStreak
	rules, err := cfg.RulesFor(m.Mode, m.Grade, tournament)
	if err != nil {
		return abort(err)
	}
	students := make([]uuid.UUID, len(m.Participants))
	for i, p := range m.Participants {
		students[i] = p.StudentID
	}
	plan, err := h.Plans.Create(ctx, port.PlanRequest{MatchID: m.ID, Mode: m.Mode, Grade: m.Grade, SubjectID: m.SubjectID, TopicID: m.TopicID,
		Count: rules.QuestionCount, TimeS: int(rules.TimeLimit / time.Second), Participants: students})
	if err != nil {
		return abort(err)
	}
	spec := battle.Spec{MatchID: m.ID, PlanID: plan.ID, PlanVersion: plan.Version, Grade: m.Grade, Ranked: m.Ranked, IsBot: m.IsBot,
		Tournament: tournament, Rules: rules}
	if m.IsBot {
		b := cfg.BotFor(m.Grade)
		spec.Bot = &b
	}
	pinned := make([]map[string]any, 0, len(plan.Items))
	for _, it := range plan.Items {
		limit := time.Duration(it.TimeS) * time.Second
		if limit <= 0 {
			limit = rules.TimeLimit
		}
		inst := battle.InstanceID(m.ID, it.Index)
		spec.Items = append(spec.Items, battle.Item{Index: it.Index, Instance: inst, Type: it.Type, Limit: limit, Render: it.Render})
		mult := 1
		if it.Index == len(plan.Items)-1 && rules.LastMultiplier > 1 {
			mult = rules.LastMultiplier
		}
		pinned = append(pinned, map[string]any{"index": it.Index, "question_instance_id": inst, "question_version_id": it.QuestionVersionID,
			"time_limit_ms": limit.Milliseconds(), "points_multiplier": mult})
	}
	for _, p := range m.Participants {
		nick := ""
		if c, ok := cards[p.StudentID]; ok && c.Nickname != nil {
			nick = *c.Nickname
		}
		spec.Players = append(spec.Players, battle.PlayerSpec{ID: p.StudentID, Tenant: p.TenantID, School: p.SchoolID, Team: p.TeamNo, Nickname: nick})
	}
	now := h.now()
	room, err = battle.New(spec, now)
	if err != nil {
		return abort(err)
	}
	items, _ := json.Marshal(map[string]any{"plan_id": plan.ID, "items": pinned})
	var school *uuid.UUID
	if len(m.Participants) > 0 && m.ScopeKind == "school" {
		s := m.Participants[0].SchoolID
		school = &s
	}
	if err := h.tx(ctx, tenant, func(ctx context.Context) error {
		if err := h.Repo.PinPlan(ctx, m.ID, plan.ID, plan.Version, items); err != nil {
			return err
		}
		return h.Repo.OpenRoom(ctx, write.BattleRoomRow{RoomID: m.ID, TenantID: tenant, RoomType: info.RoomType, Mode: m.Mode, SchoolID: school,
			Capacity: max(len(m.Participants), 1), MatchID: m.ID, At: now})
	}); err != nil {
		return info, nil, err
	}
	if h.Metrics != nil {
		h.Metrics.Rooms(info.RoomType, 1)
	}
	return info, room, nil
}

// Started records the first question shown (status running) and publishes match.started with the participants and
// teams for the chat channels match/team (always on, ADR-0020 §6, F105-AC-26).
func (h Battles) Started(ctx context.Context, info BattleInfo, room *battle.Room, at time.Time) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.StartMatch")
	defer func() { done(err) }()
	parts := make([]map[string]any, 0, len(room.Players))
	for _, p := range room.Players {
		parts = append(parts, map[string]any{"student_id": p.ID, "tenant_id": p.Tenant, "school_id": p.School, "team_no": p.Team})
	}
	return h.tx(ctx, info.Tenant, func(ctx context.Context) error {
		if err := h.Repo.MarkRunning(ctx, info.MatchID, at); err != nil {
			return err
		}
		t := info.Tenant
		return h.Events.Publish(ctx, write.Event{Type: "match.started", SubjectType: "match", SubjectID: info.MatchID.String(),
			PartitionKey: info.MatchID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"match_id": info.MatchID,
				"room_id": info.MatchID, "mode": info.Mode, "matchmaking_type": info.Type, "ranked": info.Ranked, "is_bot": info.IsBot,
				"subject_id": info.SubjectID, "grade_level": info.Grade, "question_count": len(room.Items), "question_plan_version": room.PlanVersion,
				"tournament_node_id": info.TournamentNodeID, "started_at": at, "participants": parts}})
	})
}

// Score grades one answer with module learning on behalf of the student (F105-AC-04, AC-09): learning records it as
// attempt_answer mode competition, idempotent by (plan, index, student). Never inside a transaction (R29).
func (h Battles) Score(ctx context.Context, info BattleInfo, plan uuid.UUID, p battle.Pending) (correct bool, err error) {
	start := time.Now()
	defer func() {
		if h.Metrics != nil {
			r := "ok"
			if err != nil {
				r = "error"
			}
			h.Metrics.ScoreCall(time.Since(start), r)
		}
	}()
	sctx, err := onBehalf(ctx, info.Tenant, p.Student)
	if err != nil {
		return false, err
	}
	return h.Plans.Score(sctx, plan, p.Index, p.Student, p.Response, p.ReceivedAt)
}

// Reveal reads the correct answer and explanation of a closed question from module learning (F105-AC-05).
func (h Battles) Reveal(ctx context.Context, info BattleInfo, plan uuid.UUID, index int, closedAt time.Time) (battle.Reveal, error) {
	sctx, err := authz.EventScope(info.Tenant)
	if err != nil {
		return battle.Reveal{}, err
	}
	return h.Plans.Reveal(tenancy.WithScope(ctx, sctx), plan, index, closedAt)
}

// Finish implements RecordMatchResult (F105-AC-07, AC-08, AC-10, AC-16): in one transaction the result row (once per
// match), the outcomes of the participants, the match, the room, the outbox match.completed, the audit and the abandon
// penalties. A second call writes nothing and returns write.ErrResultRecorded (MATCH_RESULT_ALREADY_RECORDED).
func (h Battles) Finish(ctx context.Context, info BattleInfo, room *battle.Room) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RecordMatchResult")
	defer func() { done(err) }()
	res := room.Result()
	at := res.EndedAt
	if at.IsZero() {
		at = h.now()
	}
	summary := map[string]any{"questions": res.Questions, "plan_id": room.PlanID, "plan_version": room.PlanVersion}
	if res.Bot != nil {
		summary["bot"] = map[string]any{"label": "robot", "score": res.Bot.Score, "correct": res.Bot.Correct}
	}
	if res.GroupGoal != nil {
		summary["grade_goal"] = res.GroupGoal
	}
	results := make([]map[string]any, 0, len(res.Players))
	for _, p := range res.Players {
		results = append(results, map[string]any{"student_id": p.ID, "tenant_id": p.Tenant, "school_id": p.School, "team_no": p.Team,
			"outcome": p.Outcome, "score": p.Score, "correct_count": p.Correct})
	}
	err = h.tx(ctx, info.Tenant, func(ctx context.Context) error {
		if err := h.Repo.RecordResult(ctx, write.MatchOutcome{Result: res, RuleID: info.RulesetVersionID, TenantIDs: room.Tenants(),
			Summary: summary, RecordedBy: h.recorder(), At: at}); err != nil {
			return err
		}
		if err := h.Repo.CloseRoom(ctx, info.MatchID, at); err != nil {
			return err
		}
		t := info.Tenant
		if err := h.Events.Publish(ctx, write.Event{Type: "match.completed", SubjectType: "match", SubjectID: info.MatchID.String(),
			PartitionKey: info.MatchID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"match_id": info.MatchID,
				"room_id": info.MatchID, "mode": info.Mode, "matchmaking_type": info.Type, "ranked": info.Ranked, "rated": res.Rated,
				"is_bot": info.IsBot, "subject_id": info.SubjectID, "grade_level": info.Grade, "status": res.Status, "end_reason": res.EndReason,
				"winner_team": res.WinnerTeam, "rule_version_id": info.RulesetVersionID, "tournament_id": info.TournamentID,
				"tournament_node_id": info.TournamentNodeID, "ended_at": at, "results": results}}); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditResultRecorded, TargetType: "match", TargetID: info.MatchID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"mode": info.Mode, "ranked": info.Ranked, "rated": res.Rated,
				"participants": len(res.Players), "end_reason": res.EndReason, "status": res.Status}}); err != nil {
			return err
		}
		return h.abandonPenalties(ctx, info, res, at)
	})
	if errors.Is(err, write.ErrResultRecorded) {
		if h.Metrics != nil {
			h.Metrics.MatchResult(info.Mode, MatchResultDuplicate)
		}
		return problem.New(problem.MatchResultAlreadyRecorded, "").Wrap(err)
	}
	if err == nil && h.Metrics != nil {
		h.Metrics.MatchResult(info.Mode, MatchResultRecorded)
		h.Metrics.Rooms(info.RoomType, -1)
		for _, p := range res.Players {
			if p.Outcome == battle.OutcomeAbandoned || p.Outcome == battle.OutcomeForfeit {
				h.Metrics.Abandon(info.Mode)
			}
		}
	}
	return err
}

// Encouragements returns the private encouragement of each player whose recorded result ends a loss streak of
// loss_streak_encouragement matches (COMPETITION_SYSTEM §12, delta F109 2026-11-27): practise the subject (topic) of
// the match. It reads the durable outcomes after RecordMatchResult committed, so a retry after a failover gives the same
// answer; it never writes and never blocks the end of the match (the room host sends battle.match.ended without it on
// an error).
func (h Battles) Encouragements(ctx context.Context, info BattleInfo, room *battle.Room) (out map[uuid.UUID]battle.Encouragement, err error) {
	if info.LossStreak <= 0 || room == nil || info.Mode == battle.ModeTimedQuiz || info.Mode == battle.ModeCoop {
		return nil, nil
	}
	res := room.Result()
	if res.Status != "completed" {
		return nil, nil
	}
	ctx, done := observability.UseCase(ctx, "competition.command.MatchEncouragements")
	defer func() { done(err) }()
	err = h.tx(ctx, info.Tenant, func(ctx context.Context) error {
		for _, p := range res.Players {
			if p.Tenant != info.Tenant || !battle.Lost(p.Outcome) {
				continue
			}
			outcomes, err := h.Repo.RecentOutcomes(ctx, p.ID, info.LossStreak)
			if err != nil {
				return err
			}
			if !battle.LossStreak(outcomes, info.LossStreak) {
				continue
			}
			if out == nil {
				out = map[uuid.UUID]battle.Encouragement{}
			}
			out[p.ID] = battle.LossStreakEncouragement(info.SubjectID, info.TopicID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h Battles) recorder() string {
	if h.Instance == "" {
		return "core-api"
	}
	if len(h.Instance) > 64 {
		return h.Instance[:64]
	}
	return h.Instance
}

// abandonPenalties restricts ranked play of a student who abandoned abandon_penalty.count ranked matches in
// abandon_penalty.window_h (F105-AC-16, MULTIPLAYER_GAMEPLAY §9.1): only ranked matches count [Proposed F105]; matches
// cancelled or aborted never do.
func (h Battles) abandonPenalties(ctx context.Context, info BattleInfo, res battle.Result, at time.Time) error {
	pen := info.AbandonPenalty
	if pen.Count <= 0 || !info.Ranked || info.IsBot || res.Status != "completed" {
		return nil
	}
	for _, p := range res.Players {
		if p.Outcome != battle.OutcomeAbandoned {
			continue
		}
		n, err := h.Repo.Abandons(ctx, p.ID, at.Add(-time.Duration(pen.WindowH)*time.Hour))
		if err != nil {
			return err
		}
		if n < pen.Count {
			continue
		}
		if until, err := h.Repo.RestrictedUntil(ctx, p.ID, "abandon_penalty", at); err != nil || until != nil {
			return err
		}
		ends := at.Add(time.Duration(pen.RestrictMin) * time.Minute)
		id, err := h.Repo.InsertRestriction(ctx, write.Restriction{TenantID: p.Tenant, SchoolID: p.School, StudentID: p.ID, Kind: "abandon_penalty",
			Reason: fmt.Sprintf("abandon_penalty: %d abandoned ranked matches in %d h", n, pen.WindowH), StartsAt: at, EndsAt: ends})
		if err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditRestrictionCreate, TargetType: "competition_restriction", TargetID: id.String(),
			Result: audit.ResultSuccess, SchoolID: p.School.String(), Metadata: map[string]any{"kind": "abandon_penalty", "student_id": p.ID,
				"abandons": n, "ends_at": ends, "match_id": info.MatchID}}); err != nil {
			return err
		}
	}
	return nil
}

// Abort records a match aborted by a server failure (F105-AC-15): no result, no rating, no abandon; match.aborted with
// the participants for the chat channels.
func (h Battles) Abort(ctx context.Context, info BattleInfo, room *battle.Room, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AbortMatch")
	defer func() { done(err) }()
	parts := make([]write.BattleParticipant, 0, len(room.Players))
	for _, p := range room.Players {
		parts = append(parts, write.BattleParticipant{StudentID: p.ID, TenantID: p.Tenant, SchoolID: p.School, TeamNo: p.Team})
	}
	return h.abortMatch(ctx, info, parts, reason)
}

// AbortUnhosted aborts a match whose room cannot be restored (no checkpoint after a failover, F105-AC-15).
func (h Battles) AbortUnhosted(ctx context.Context, matchID, tenant uuid.UUID) (BattleInfo, error) {
	var m *write.BattleMatch
	if err := h.tx(ctx, tenant, func(ctx context.Context) error {
		var err error
		m, err = h.Repo.BattleMatch(ctx, matchID)
		return err
	}); err != nil || m == nil {
		return BattleInfo{}, err
	}
	info := BattleInfo{MatchID: m.ID, Tenant: tenant, Mode: m.Mode, Type: m.Type, SubjectID: m.SubjectID, Grade: m.Grade, Ranked: m.Ranked,
		IsBot: m.IsBot, RulesetVersionID: m.RulesetVersionID, RoomType: "battle"}
	return info, h.abortMatch(ctx, info, m.Participants, battle.EndServerFailure)
}

func (h Battles) abortMatch(ctx context.Context, info BattleInfo, parts []write.BattleParticipant, reason string) error {
	at := h.now()
	ps := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		ps = append(ps, map[string]any{"student_id": p.StudentID, "tenant_id": p.TenantID, "school_id": p.SchoolID, "team_no": p.TeamNo})
	}
	err := h.tx(ctx, info.Tenant, func(ctx context.Context) error {
		changed, err := h.Repo.AbortMatch(ctx, info.MatchID, reason, at)
		if err != nil || !changed {
			return err
		}
		if err := h.Repo.CloseRoom(ctx, info.MatchID, at); err != nil {
			return err
		}
		if len(ps) > 0 {
			t := info.Tenant
			if err := h.Events.Publish(ctx, write.Event{Type: "match.aborted", SubjectType: "match", SubjectID: info.MatchID.String(),
				PartitionKey: info.MatchID.String(), TenantID: &t, OccurredAt: at, Payload: map[string]any{"match_id": info.MatchID,
					"room_id": info.MatchID, "mode": info.Mode, "reason": reason, "aborted_at": at, "participants": ps}}); err != nil {
				return err
			}
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditMatchAborted, TargetType: "match", TargetID: info.MatchID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"mode": info.Mode, "reason": reason, "participants": len(ps)}})
	})
	if err == nil && h.Metrics != nil {
		h.Metrics.Aborted(reason)
	}
	return err
}

// Memberships records the session memberships of the room (F105-AC-14, AC-22, MULTIPLAYER_GAMEPLAY §10).
func (h Battles) Memberships(ctx context.Context, info BattleInfo, room *battle.Room, ms []battle.Membership, at time.Time) error {
	if len(ms) == 0 {
		return nil
	}
	schools := map[uuid.UUID]uuid.UUID{}
	for _, p := range room.Players {
		schools[p.ID] = p.School
	}
	return h.tx(ctx, info.Tenant, func(ctx context.Context) error {
		for _, m := range ms {
			var err error
			switch m.Kind {
			case battle.MemberJoin:
				err = h.Repo.JoinRoom(ctx, info.MatchID, info.RoomType, info.MatchID, schools[m.Student], m.Student, m.Session, at)
			case battle.MemberLeave:
				err = h.Repo.LeaveRoom(ctx, info.MatchID, m.Student, m.Session, m.Reason, at)
			case battle.MemberDisconnect:
				err = h.Repo.Disconnected(ctx, info.MatchID, m.Student)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Signals records the integrity signals of the room (F105-AC-03, AC-24): to be reviewed, never an automatic sanction.
func (h Battles) Signals(ctx context.Context, info BattleInfo, room *battle.Room, ss []battle.Signal, at time.Time) error {
	if len(ss) == 0 {
		return nil
	}
	schools := map[uuid.UUID]uuid.UUID{}
	for _, p := range room.Players {
		schools[p.ID] = p.School
	}
	err := h.tx(ctx, info.Tenant, func(ctx context.Context) error {
		for _, s := range ss {
			ev := map[string]any{"question_index": s.Index}
			for k, v := range s.Evidence {
				ev[k] = v
			}
			if err := h.Repo.InsertSignal(ctx, write.IntegritySignalRow{TenantID: info.Tenant, SchoolID: schools[s.Student], StudentID: s.Student,
				MatchID: info.MatchID, Type: s.Type, Score: 1, Evidence: ev, At: at}); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && h.Metrics != nil {
		for _, s := range ss {
			h.Metrics.Signal(s.Type)
		}
	}
	return err
}
