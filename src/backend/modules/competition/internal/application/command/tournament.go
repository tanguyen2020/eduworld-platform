package command

import (
	"context"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	mm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/matchmaking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament/bracket"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Audit actions of the tournaments (F107-AC-04, AC-14, AC-18, AC-20, AC-29; AGENTS R10).
const (
	AuditChallengeCreated      = "challenge.created"
	AuditChallengeCancelled    = "challenge.cancelled"
	AuditTournamentPublished   = "tournament.published"
	AuditTournamentCancelled   = "tournament.cancelled"
	AuditEntriesConfirmed      = "tournament.entries_confirmed"
	AuditBracketGenerated      = "tournament.bracket_generated"
	AuditNodeOverridden        = "tournament.node_overridden"
	AuditNodeConfirmed         = "tournament.node_confirmed"
	AuditTournamentCompleted   = "tournament.completed"
	AuditTournamentRewarded    = "tournament.rewarded"
	AuditTenantDataAssigned    = "tenant_data.assigned"
	AuditTenantDataUnassigned  = "tenant_data.unassigned"
	EventTournamentScheduled   = "tournament.match_scheduled"
	EventTournamentCompleted   = "tournament.completed"
	EventTournamentRewarded    = "tournament.rewarded"
	rtTournamentMatchScheduled = "tournament.match.scheduled"
	rtTournamentBracketUpdated = "tournament.bracket.updated"
	// JobTournamentScheduler names the platform scope of the job tournament-scheduler.
	JobTournamentScheduler = "tournament-scheduler"
	// TimedQuizRoomSize is the largest timed quiz room (COMPETITION_SYSTEM §3.4, F105-AC-19).
	TimedQuizRoomSize = 40
)

// TournamentMatches is what the tournaments need from matchmaking (F104, same module): the direct proposal of a bracket
// node with its ready check, who accepted it, and the timed quiz rooms of a grade challenge.
type TournamentMatches interface {
	CreateDirectProposal(ctx context.Context, in DirectInput) (uuid.UUID, error)
	DirectAcceptance(ctx context.Context, id uuid.UUID) (map[uuid.UUID]bool, bool, bool, error)
	CreateChallengeMatch(ctx context.Context, in ChallengeMatchInput) error
}

// TournamentRules reads the Ranking Rules in effect (seeding by rating: default rating and provisional limit, F106).
type TournamentRules interface {
	Rules(ctx context.Context, at time.Time) (ranking.Rules, uuid.UUID, error)
}

// TournamentPoints writes the Competition Points of the tournaments (source_type tournament, F106 ledger).
type TournamentPoints interface {
	SeasonAt(ctx context.Context, at time.Time) (*write.SeasonRow, error)
	InsertPoints(ctx context.Context, e write.PointEntry) (bool, error)
	SeasonPoints(ctx context.Context, tenant, student, season uuid.UUID) (int, error)
}

// Tournaments holds the F107 use cases: grade challenges and school tournaments from templates, registration, check-in,
// brackets, schedule, results, overrides, rewards and the template assignments.
type Tournaments struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.TournamentRepository
	Events      write.EventPublisher
	Audit       write.Auditor
	Screener    port.TextScreener // nil: every challenge creation is 503 (never fail-open)
	Matches     TournamentMatches
	Ratings     port.RatingReader
	Rules       TournamentRules
	Points      TournamentPoints
	Push        port.PartyRealtime // nil: no realtime push
	Cards       CardReader         // nicknames of the entries (ref.player_profiles)
	Metrics     port.TournamentMetrics
	Timers      trn.Timers // platform defaults of tournament_timers (configuration)
	Now         func() time.Time
}

func (h Tournaments) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Tournaments) later(ctx context.Context, fn func(ctx context.Context)) error {
	if h.AfterCommit == nil {
		fn(ctx)
		return nil
	}
	return h.AfterCommit(ctx, fn)
}

func (h Tournaments) send(ctx context.Context, to []uuid.UUID, typ string, payload any) {
	if h.Push != nil && len(to) > 0 {
		_ = h.Push.Send(ctx, to, typ, payload) // best effort: the bracket stays readable over REST
	}
}

// trnError maps the domain errors of the tournaments to problems.
func trnError(err error) error {
	var ve challenge.ValidationError
	var mve mm.ValidationError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ve):
		return problem.Invalid(ve.Field, problem.SubInvalid, ve.Reason)
	case errors.Is(err, challenge.ErrTemplate):
		return problem.Invalid("template_id", problem.SubInvalid, "is not a usable template (no class challenge, ADR-0095)")
	case errors.As(err, &mve):
		return problem.Invalid(mve.Field, problem.SubInvalid, mve.Message)
	case errors.Is(err, trn.ErrInvalidTransition), errors.Is(err, trn.ErrNodeState), errors.Is(err, trn.ErrNextStarted):
		return problem.New(problem.InvalidStateTransition, "")
	case errors.Is(err, trn.ErrNotInNode):
		return problem.Invalid("winner_entry_id", problem.SubInvalid, "is one of the two entries of the node")
	case errors.Is(err, bracket.ErrTooFew):
		return problem.Invalid("entries", problem.SubInvalid, "fewer confirmed entries than the format needs")
	case errors.Is(err, bracket.ErrManualOrder):
		return problem.Invalid("entry_order", problem.SubInvalid, "lists every confirmed entry once")
	case errors.Is(err, bracket.ErrUnknownPolicy):
		return problem.Invalid("seeding_policy", problem.SubInvalid, "is rating, random or manual")
	}
	return err
}

// organizer is the caller of an organizer route: portal staff with lms.challenge.write (school_admin of its schools,
// System Admin in the target tenant — ADR-0072); a student or staff without it is 403 PERMISSION_DENIED (audited).
func organizer(ctx context.Context) (authz.Principal, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return p, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStaff || p.Channel == authz.ChannelGame || p.TenantID == nil {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(authz.PermLmsChallengeWrite)})
		return p, uuid.Nil, problem.New(problem.PermissionDenied, "")
	}
	tenant := *p.TenantID
	if !p.Has(authz.PermLmsChallengeWrite) {
		return p, uuid.Nil, authz.Require(ctx, p, authz.PermLmsChallengeWrite, authz.Resource{TenantID: &tenant})
	}
	return p, tenant, nil
}

// Covers reports whether the lms.challenge.write grants of p cover school (a tenant-wide grant covers every school).
func Covers(p authz.Principal, school *uuid.UUID) bool {
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, authz.PermLmsChallengeWrite) {
			continue
		}
		if g.SchoolID == nil || (school != nil && *g.SchoolID == *school) {
			return true
		}
	}
	return false
}

// ChallengeInput is ChallengeCreate.
type ChallengeInput struct {
	TenantID             *uuid.UUID
	TemplateID           uuid.UUID
	QuestionSetVersionID *uuid.UUID
	Request              challenge.Request
}

// CreateChallenge implements createChallenge (F107-AC-01…AC-05): the organizer of the school (school outside its grants
// or of another tenant: 404), a published template assigned to the tenant (403 RESOURCE_NOT_ASSIGNED), the request
// inside the template (422), every listed/excluded student of the school, the hours of the school covering the match,
// the name screened by module chat (flagged: 422 naming the field; down: 503, nothing stored); the challenge pins the
// template version and creates its tournament (draft) or its timed quiz (scheduled). Idempotent (Idempotency-Key),
// audit challenge.created.
func (h Tournaments) CreateChallenge(ctx context.Context, in ChallengeInput) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateChallenge")
	defer func() { done(err) }()
	p, tenant, err := organizer(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case p.IsSystemAdmin && in.TenantID == nil:
		return nil, problem.Invalid("tenant_id", problem.SubRequired, "is required for a System Admin (target tenant)")
	case in.TenantID != nil && *in.TenantID != tenant:
		return nil, problem.New(problem.TenantMismatch, "")
	}
	school := in.Request.SchoolID
	if !Covers(p, &school) {
		return nil, problem.New(problem.NotFound, "")
	}
	now := h.now()
	var tpl *write.TemplateRow
	var plan challenge.Plan
	var grades []int
	var tplan *challenge.TournamentPlan
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		ok, err := h.Repo.SchoolActive(ctx, tenant, school)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.NotFound, "")
		}
		if tpl, err = h.Repo.Template(ctx, tenant, in.TemplateID); err != nil {
			return err
		}
		if tpl == nil {
			return problem.New(problem.NotFound, "")
		}
		if !tpl.Assigned {
			return problem.New(problem.ResourceNotAssigned, "")
		}
		if plan, err = in.Request.Validate(tpl.Template, now); err != nil {
			return err
		}
		if grades, err = h.checkStudents(ctx, tenant, school, plan, tpl.Template, now); err != nil {
			return err
		}
		at := []time.Time{plan.StartsAt}
		if tpl.Template.Format == challenge.FormatTimedQuiz {
			at = append(at, plan.EndsAt.Add(-time.Second))
		}
		open, err := h.Repo.SchoolOpen(ctx, tenant, school, at)
		if err != nil {
			return err
		}
		if !open {
			return problem.Invalid("starts_at", problem.SubInvalid, "is outside the access hours of the school")
		}
		if tpl.Template.Format == challenge.FormatTournament {
			tp, err := challenge.TournamentOf(tpl.Template, plan, tpl.Template.TimersOr(h.Timers), now)
			if err != nil {
				return err
			}
			tplan = &tp
		}
		return nil
	}); err != nil {
		return nil, trnError(err)
	}
	if err := h.screen(ctx, tenant, plan.Name); err != nil {
		return nil, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c := write.ChallengeRow{ID: ids.New(), Tenant: tenant, School: school, TemplateConfigID: tpl.ConfigID, TemplateVersionID: tpl.VersionID,
			Kind: tpl.Template.Kind, Format: tpl.Template.Format, Name: plan.Name, Grade: plan.Grade, Listed: plan.Listed, Exclude: plan.Exclude,
			SubjectID: plan.SubjectID, QuestionSetVersionID: in.QuestionSetVersionID, Params: in.Request.Params, StartsAt: plan.StartsAt,
			EndsAt: plan.EndsAt, JoinOpensAt: plan.JoinOpensAt, Status: write.ChallengeScheduled, CreatedBy: p.UserID, CreatedAt: now, Version: 1}
		if c.Params == nil {
			c.Params = map[string]any{}
		}
		if err := h.Repo.InsertChallenge(ctx, c); err != nil {
			return err
		}
		if tplan != nil {
			t := tournamentRow(c, tpl, *tplan, grades)
			if err := h.Repo.InsertTournament(ctx, t); err != nil {
				return err
			}
			c.TournamentID = &t.ID
			if err := h.Repo.UpdateChallenge(ctx, c); err != nil {
				return err
			}
			c.Version++
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditChallengeCreated, TargetType: "challenge", TargetID: c.ID.String(),
			Result: audit.ResultSuccess, SchoolID: school.String(), Metadata: map[string]any{"template_id": tpl.ConfigID,
				"template_version_id": tpl.VersionID, "kind": c.Kind, "format": c.Format, "tournament_id": c.TournamentID}}); err != nil {
			return err
		}
		aud, err := h.audience(ctx, c)
		if err != nil {
			return err
		}
		out = view.Challenge(c, len(aud), 0)
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

// checkStudents checks the listed and excluded students belong to the school (F107-AC-05: another school → 422) and
// returns the grades of the tournament: the grade target and the grades of the listed students, within the template.
func (h Tournaments) checkStudents(ctx context.Context, tenant, school uuid.UUID, plan challenge.Plan, t challenge.Template, now time.Time) ([]int, error) {
	all := append(append([]uuid.UUID(nil), plan.Listed...), plan.Exclude...)
	info, err := h.Repo.Students(ctx, tenant, all, now)
	if err != nil {
		return nil, err
	}
	set := map[int]bool{}
	if plan.Grade != nil {
		set[*plan.Grade] = true
	}
	for i, id := range all {
		s, ok := info[id]
		if !ok || s.School != school {
			field := "targets"
			if i >= len(plan.Listed) {
				field = "exclude_student_ids"
			}
			return nil, problem.Invalid(field, problem.SubInvalid, "every student belongs to the school of the challenge")
		}
		if i < len(plan.Listed) && slices.Contains(t.GradeLevels, s.Grade) {
			set[s.Grade] = true
		}
	}
	// the grade target first (the grade of a grade tournament, grade_level of the contract), then the other grades.
	grades := make([]int, 0, len(set))
	for g := range set {
		if plan.Grade == nil || g != *plan.Grade {
			grades = append(grades, g)
		}
	}
	sort.Ints(grades)
	if plan.Grade != nil && set[*plan.Grade] {
		grades = append([]int{*plan.Grade}, grades...)
	}
	if len(grades) == 0 {
		return nil, problem.Invalid("targets", problem.SubInvalid, "no grade of the audience is a grade of the template")
	}
	return grades, nil
}

// screen checks the name typed by the school with the chat facade screenText (R15): flagged → 422 on name (no word
// returned); screening unavailable → 503, nothing stored (never fail-open).
func (h Tournaments) screen(ctx context.Context, tenant uuid.UUID, name string) error {
	if h.Screener == nil {
		return problem.New(problem.Unavailable, "text screening is unavailable").Retry()
	}
	bad, err := h.Screener.Screen(ctx, tenant, "vi", []string{name})
	if err != nil {
		return problem.New(problem.Unavailable, "text screening is unavailable").Retry()
	}
	if len(bad) > 0 {
		return problem.Invalid("name", problem.SubInvalid, "contains words that are not allowed")
	}
	return nil
}

func tournamentRow(c write.ChallengeRow, tpl *write.TemplateRow, tp challenge.TournamentPlan, grades []int) write.TournamentRow {
	t := write.TournamentRow{ID: ids.New(), Tenant: c.Tenant, OwnerScope: trn.ScopeSchool, OwnerID: c.School, ChallengeID: &c.ID,
		TemplateConfigID: tpl.ConfigID, RuleVersionID: tpl.VersionID, Name: c.Name, Format: tp.Format, ParticipantType: tp.ParticipantType,
		TeamSize: tp.TeamSize, Mode: tp.Mode, SubjectID: c.SubjectID, Grades: grades, RegOpenAt: tp.RegOpenAt, RegCloseAt: tp.RegCloseAt,
		CheckInOpensAt: tp.CheckInOpensAt, StartsAt: tp.StartsAt, MaxEntries: tp.MaxEntries, MinEntries: tp.MinEntries,
		SeedingPolicy: tp.SeedingPolicy, AutoConfirm: tp.AutoConfirm, RewardRules: tpl.Template.RewardRuleRefs, RoundGap: tp.RoundGap,
		Status: trn.StatusDraft, CreatedAt: c.CreatedAt, Version: 1}
	school := c.School
	t.School = &school
	if s := tpl.Template.RewardSummary; s != "" {
		t.RewardSummary = &s
	}
	if c.Params != nil {
		if v, err := challenge.FilterParams(tpl.Template, c.Params); err == nil {
			t.TopicIDs = v.TopicIDs
		}
	}
	return t
}

// audience returns the effective audience of a challenge now (F107-AC-05): (grade students ∪ listed active students of
// the school) − excluded.
func (h Tournaments) audience(ctx context.Context, c write.ChallengeRow) ([]uuid.UUID, error) {
	var grade []uuid.UUID
	if c.Grade != nil {
		var err error
		if grade, err = h.Repo.GradeStudents(ctx, c.Tenant, c.School, *c.Grade); err != nil {
			return nil, err
		}
	}
	listed := []uuid.UUID{}
	if len(c.Listed) > 0 {
		info, err := h.Repo.Students(ctx, c.Tenant, c.Listed, h.now())
		if err != nil {
			return nil, err
		}
		for _, id := range c.Listed {
			if s, ok := info[id]; ok && s.Active && s.School == c.School {
				listed = append(listed, id)
			}
		}
	}
	return challenge.Audience(grade, listed, c.Exclude), nil
}

// CancelChallenge implements cancelChallenge (F107-AC-06): a scheduled or running challenge of a school of the organizer
// becomes cancelled with its reason (the row stays, R21), with its tournament when it can still be cancelled (a
// tournament under way or over is 422); completed or cancelled → 422 INVALID_STATE_TRANSITION. Audit challenge.cancelled.
func (h Tournaments) CancelChallenge(ctx context.Context, id uuid.UUID, reason string) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CancelChallenge")
	defer func() { done(err) }()
	p, tenant, err := organizer(ctx)
	if err != nil {
		return nil, err
	}
	if err := reasonOK(reason); err != nil {
		return nil, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Repo.Challenge(ctx, tenant, id, true)
		if err != nil {
			return err
		}
		if c == nil || !Covers(p, &c.School) {
			return problem.New(problem.NotFound, "")
		}
		if c.Status != write.ChallengeScheduled && c.Status != write.ChallengeRunning {
			return trn.ErrInvalidTransition
		}
		if c.TournamentID != nil {
			t, err := h.Repo.Tournament(ctx, tenant, *c.TournamentID, true)
			if err != nil {
				return err
			}
			if t != nil && t.Status != trn.StatusCancelled {
				if err := h.cancelTournament(ctx, t, reason, now); err != nil {
					return err
				}
			}
		}
		r := reason
		c.Status, c.CancelReason = write.ChallengeCancelled, &r
		if err := h.Repo.UpdateChallenge(ctx, *c); err != nil {
			return err
		}
		c.Version++
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditChallengeCancelled, TargetType: "challenge", TargetID: c.ID.String(),
			Result: audit.ResultSuccess, SchoolID: c.School.String(), Reason: reason, Metadata: map[string]any{
				"template_version_id": c.TemplateVersionID, "tournament_id": c.TournamentID}}); err != nil {
			return err
		}
		out = view.Challenge(*c, 0, 0)
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

func reasonOK(reason string) error {
	if r := []rune(reason); len(r) == 0 || len(r) > 1000 {
		return problem.Invalid("reason", problem.SubRequired, "is 1 to 1000 characters")
	}
	return nil
}

// JoinResult is ChallengeJoinResult.
type JoinResult struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	MatchID     uuid.UUID `json:"match_id"`
	RoomID      uuid.UUID `json:"room_id"`
	StartsAt    time.Time `json:"starts_at"`
}

// ChallengeRoomID is the match (= room) id of room k of a timed-quiz challenge.
func ChallengeRoomID(challengeID uuid.UUID, k int) uuid.UUID {
	return uuid.NewSHA1(challengeID, []byte{'r', 'o', 'o', 'm', byte(k)})
}

// RoomOf returns the room index of student in an audience sorted by id (rooms of at most 40 students), -1 when absent.
func RoomOf(audience []uuid.UUID, student uuid.UUID) int {
	i := slices.Index(audience, student)
	if i < 0 {
		return -1
	}
	return i / TimedQuizRoomSize
}

// JoinChallenge implements joinChallenge (F107-AC-27): a student of the audience inside the join window (from
// join_opens_at to ends_at) gets the timed-quiz room of the challenge (match_id = room_id) and enters it with
// battle.state.request; outside the window → 422 INVALID_STATE_TRANSITION; outside the audience → 404. Before starts_at
// the room is the one the audience gives now (the rooms are created at starts_at with the audience of that moment).
func (h Tournaments) JoinChallenge(ctx context.Context, id uuid.UUID) (out JoinResult, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.JoinChallenge")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return out, err
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Repo.Challenge(ctx, tenant, id, false)
		if err != nil {
			return err
		}
		if c == nil || c.Format != challenge.FormatTimedQuiz {
			return problem.New(problem.NotFound, "")
		}
		info, err := h.Repo.Students(ctx, tenant, []uuid.UUID{me}, now)
		if err != nil {
			return err
		}
		s, ok := info[me]
		if !ok || !s.Active || !challenge.InAudience(me, s.School, s.Grade, c.School, c.Grade, c.Listed, c.Exclude) {
			return problem.New(problem.NotFound, "")
		}
		if c.Status == write.ChallengeCancelled || c.Status == write.ChallengeCompleted || now.Before(c.JoinOpensAt) || !now.Before(c.EndsAt) {
			return trn.ErrInvalidTransition
		}
		var room uuid.UUID
		if c.Status == write.ChallengeRunning {
			// the rooms exist: the one holding the student (a student who joined the audience after the start has none).
			m, err := h.Repo.RoomOf(ctx, tenant, c.MatchIDs, me)
			if err != nil {
				return err
			}
			if m == nil {
				return problem.New(problem.NotFound, "")
			}
			room = *m
		} else {
			aud, err := h.audience(ctx, *c)
			if err != nil {
				return err
			}
			k := RoomOf(aud, me)
			if k < 0 {
				return problem.New(problem.NotFound, "")
			}
			room = ChallengeRoomID(c.ID, k)
		}
		out = JoinResult{ChallengeID: c.ID, MatchID: room, RoomID: room, StartsAt: c.StartsAt.UTC()}
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

// PublishTournament implements publishTournament (F107-AC-07): draft → published by the organizer of its school; audit
// tournament.published.
func (h Tournaments) PublishTournament(ctx context.Context, id uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.PublishTournament")
	defer func() { done(err) }()
	err = h.organizerTournament(ctx, id, func(ctx context.Context, p authz.Principal, t *write.TournamentRow) error {
		if err := trn.Move(t.Status, trn.StatusPublished); err != nil {
			return err
		}
		t.Status = trn.StatusPublished
		if !t.RegOpenAt.After(h.now()) {
			t.Status = trn.StatusRegistrationOpen // its registration window is already open
		}
		if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
			return err
		}
		t.Version++
		if err := h.Audit.Record(ctx, audit.Event{Action: AuditTournamentPublished, TargetType: "tournament", TargetID: t.ID.String(),
			Result: audit.ResultSuccess, SchoolID: schoolOf(t), Metadata: map[string]any{"rule_version_id": t.RuleVersionID,
				"challenge_id": t.ChallengeID}}); err != nil {
			return err
		}
		out = view.Tournament(*t, 0, nil, false)
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

func schoolOf(t *write.TournamentRow) string {
	if t.School == nil {
		return ""
	}
	return t.School.String()
}

// CancelTournament implements cancelTournament (F107-AC-07): from draft, published, registration_open or scheduled with a
// reason; audit tournament.cancelled.
func (h Tournaments) CancelTournament(ctx context.Context, id uuid.UUID, reason string) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CancelTournament")
	defer func() { done(err) }()
	if err := reasonOK(reason); err != nil {
		return nil, err
	}
	err = h.organizerTournament(ctx, id, func(ctx context.Context, p authz.Principal, t *write.TournamentRow) error {
		if err := h.cancelTournament(ctx, t, reason, h.now()); err != nil {
			return err
		}
		out = view.Tournament(*t, 0, nil, false)
		return idempotency.SetResult(ctx, out)
	})
	return out, trnError(err)
}

func (h Tournaments) cancelTournament(ctx context.Context, t *write.TournamentRow, reason string, now time.Time) error {
	if !trn.Cancellable(t.Status) {
		return trn.ErrInvalidTransition
	}
	r := reason
	t.Status, t.CancelReason = trn.StatusCancelled, &r
	if err := h.Repo.UpdateTournament(ctx, *t); err != nil {
		return err
	}
	t.Version++
	return h.Audit.Record(ctx, audit.Event{Action: AuditTournamentCancelled, TargetType: "tournament", TargetID: t.ID.String(),
		Result: audit.ResultSuccess, SchoolID: schoolOf(t), Reason: reason, Metadata: map[string]any{"rule_version_id": t.RuleVersionID,
			"challenge_id": t.ChallengeID, "at": now}})
}

// organizerTournament runs fn on a tournament of a school of the organizer, locked in one transaction (404 otherwise).
func (h Tournaments) organizerTournament(ctx context.Context, id uuid.UUID, fn func(ctx context.Context, p authz.Principal, t *write.TournamentRow) error) error {
	p, tenant, err := organizer(ctx)
	if err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Repo.Tournament(ctx, tenant, id, true)
		if err != nil {
			return err
		}
		if t == nil || !Covers(p, t.School) {
			return problem.New(problem.NotFound, "")
		}
		return fn(ctx, p, t)
	})
}
