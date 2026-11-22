package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/points"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Events of F106 (specs/events/payloads/competition) and the realtime message of the tier.
const (
	EventRatingUpdated    = "rating.updated"
	EventPointsAwarded    = "competition_points.awarded"
	EventPeriodClosed     = "leaderboard.period_closed"
	EventSeasonStarted    = "season.started"
	EventSeasonEnded      = "season.ended"
	RealtimeTierUpdated   = "competition.tier.updated"
	AuditPeriodClosed     = "leaderboard.period_closed"
	AuditSeasonStarted    = "season.started"
	AuditSeasonEnded      = "season.ended"
	sourceMatch           = "match"
	contributionTeamMatch = "team_match"
	matchmakingTypeFriend = "friend"
	matchStatusCompleted  = "completed"
)

// ErrStudentNotProjected: the placement of a student is not projected yet (identity event late) — the event is retried.
var ErrStudentNotProjected = errors.New("ranking: student not projected yet")

// MatchLine is one participant of match.completed.
type MatchLine struct {
	Student uuid.UUID
	Tenant  uuid.UUID
	School  uuid.UUID
	Team    int
	Outcome string
	Correct int
}

// MatchCompleted is the payload of match.completed (owner F105) as F106 reads it.
type MatchCompleted struct {
	MatchID      uuid.UUID
	Mode         string
	Type         string
	Ranked       bool
	Rated        bool
	IsBot        bool
	SubjectID    uuid.UUID
	Grade        int
	Status       string
	TournamentID *uuid.UUID
	EndedAt      time.Time
	Results      []MatchLine
}

// Counts reports whether the match changes ratings and Competition Points (F106-AC-03): rated (ranked, completed, not a
// bot nor a tournament match, not a server failure — F105), not a friend duel, a rated mode, every participant in the
// event tenant.
func (m MatchCompleted) Counts(tenant uuid.UUID) bool {
	if !m.Rated || !m.Ranked || m.IsBot || m.Status != matchStatusCompleted || m.TournamentID != nil || m.Type == matchmakingTypeFriend ||
		!ranking.Rated(m.Mode) || len(m.Results) < 2 {
		return false
	}
	for _, r := range m.Results {
		if r.Tenant != tenant {
			return false
		}
	}
	return true
}

// Ranking holds the F106 write use cases fed by events: ApplyMatchRating and AwardMatchPoints (match.completed),
// RecordEffort (activity.graded). They run in the transaction of the consumer (processed_events, ADR-0042); the Valkey
// boards and the realtime tier are updated after the commit.
type Ranking struct {
	Repo        write.RankingRepository
	Boards      Boards
	Events      write.EventPublisher
	Push        port.TierPush // nil: no realtime
	Metrics     port.RankingMetrics
	AfterCommit func(ctx context.Context, fn func(ctx context.Context)) error
}

type tierPush struct {
	student uuid.UUID
	payload map[string]any
}

// ApplyMatchRating implements F106-AC-01…AC-05: every participant of a rated match is updated once by Glicko-2 against
// its opponent (team battle: the virtual opponent of the other team) with the Ranking Rules in effect at the end of the
// match; rating, rating_changes (unique (match_id, student_id)) and the outbox rating.updated in the transaction.
func (h Ranking) ApplyMatchRating(ctx context.Context, tenant uuid.UUID, m MatchCompleted) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.ApplyMatchRating")
	defer func() { done(err) }()
	if !m.Counts(tenant) {
		return 0, nil
	}
	rules, rv, err := h.Repo.Rules(ctx, m.EndedAt)
	if err != nil {
		return 0, err
	}
	season, err := h.Repo.SeasonAt(ctx, m.EndedAt)
	if err != nil || season == nil {
		return 0, errors.Join(errors.New("ranking: no season"), err)
	}
	lines := make([]MatchLine, 0, len(m.Results))
	students := make([]uuid.UUID, 0, len(m.Results))
	for _, r := range m.Results {
		if _, ok := ranking.Score(r.Outcome); ok {
			lines = append(lines, r)
			students = append(students, r.Student)
		}
	}
	cur, err := h.Repo.LockRatings(ctx, tenant, students, m.Mode, m.SubjectID, season.ID)
	if err != nil {
		return 0, err
	}
	before := make(map[uuid.UUID]write.RatingRow, len(lines))
	for _, l := range lines {
		r, ok := cur[l.Student]
		if !ok {
			r = write.RatingRow{Tenant: tenant, School: l.School, Student: l.Student, Mode: m.Mode, Subject: m.SubjectID, Season: season.ID,
				Grade: m.Grade, Rating: rules.Glicko2.Initial(), RuleVersion: rv}
			prev, err := h.Repo.PreviousRating(ctx, tenant, l.Student, m.Mode, m.SubjectID, season.StartsAt)
			if err != nil {
				return 0, err
			}
			if prev != nil {
				r.Rating = rules.SeasonReset(prev.Rating)
			}
		}
		before[l.Student] = r
	}
	var pushes []tierPush
	var sets []Increment
	for _, l := range lines {
		var others []ranking.Rating
		for _, o := range lines {
			if o.Team != l.Team {
				others = append(others, before[o.Student].Rating)
			}
		}
		if len(others) == 0 {
			continue
		}
		score, _ := ranking.Score(l.Outcome)
		opp := ranking.Virtual(others)
		b := before[l.Student]
		a := b
		a.Rating = ranking.Update(rules.Glicko2, b.Rating, []ranking.Opponent{{R: opp.R, RD: opp.RD, Score: score}})
		a.Games = b.Games + 1
		at := m.EndedAt
		a.LastPlayed = &at
		a.School = l.School
		a.Grade = h.grade(ctx, tenant, l.Student, m.Grade)
		a.RuleVersion = rv
		provisional := rules.IsProvisional(a.Games, a.Rating.RD)
		a.Tier = nil
		if !provisional {
			t := rules.Tiers.TierOf(a.Rating.R, deref(b.Tier))
			p := rules.Tiers.Peak(deref(b.Peak), t)
			a.Tier, a.Peak = &t, &p
		}
		ok, err := h.Repo.InsertRatingChange(ctx, m.MatchID, b, a)
		if err != nil {
			return n, err
		}
		if !ok {
			continue // rated already (F106-AC-02)
		}
		if err := h.Repo.SaveRating(ctx, a); err != nil {
			return n, err
		}
		t := tenant
		if err := h.Events.Publish(ctx, write.Event{Type: EventRatingUpdated, SubjectType: "rating", SubjectID: l.Student.String(),
			PartitionKey: l.Student.String(), TenantID: &t, OccurredAt: m.EndedAt, Payload: map[string]any{"match_id": m.MatchID,
				"student_id": l.Student, "school_id": l.School, "mode": m.Mode, "subject_id": m.SubjectID, "season_id": season.ID,
				"rating": round2(a.Rating.R), "rd": round2(a.Rating.RD), "games_played": a.Games, "provisional": provisional,
				"tier_code": a.Tier, "rule_version_id": rv}}); err != nil {
			return n, err
		}
		pushes = append(pushes, tierPush{student: l.Student, payload: map[string]any{"mode": m.Mode, "subject_id": m.SubjectID,
			"season_id": season.ID, "tier_code": a.Tier, "provisional": provisional, "provisional_games_required": rules.Provisional.MinGames,
			"games_played": a.Games}})
		sets = append(sets, Increment{Metric: ldb.MetricRating, Tenant: tenant, Member: ldb.Member{Tenant: tenant, School: l.School,
			Grade: a.Grade}, Student: l.Student, Subject: &m.SubjectID, Mode: &m.Mode, Amount: float64(int64(a.Rating.R + 0.5)), At: m.EndedAt,
			Set: true, Remove: provisional})
		n++
		if h.Metrics != nil {
			h.Metrics.RatingUpdated(m.Mode)
		}
	}
	plan, err := h.Boards.Plan(ctx, season, sets)
	if err != nil {
		return n, err
	}
	return n, h.after(ctx, func(ctx context.Context) {
		h.Boards.Apply(ctx, plan)
		for _, p := range pushes {
			if h.Push == nil {
				break
			}
			if err := h.Push.Send(ctx, []uuid.UUID{p.student}, RealtimeTierUpdated, p.payload); err != nil {
				slog.WarnContext(ctx, "competition: tier not pushed", "error", err)
			}
		}
	})
}

func (h Ranking) after(ctx context.Context, fn func(ctx context.Context)) error {
	if h.AfterCommit == nil {
		fn(ctx)
		return nil
	}
	return h.AfterCommit(ctx, fn)
}

// grade is the grade of the student now (ref.students, ADR-0095), the grade of the match when not projected.
func (h Ranking) grade(ctx context.Context, tenant, student uuid.UUID, def int) int {
	if s, err := h.Repo.Student(ctx, tenant, student); err == nil && s != nil && s.Grade >= 1 {
		return s.Grade
	}
	return def
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }

// AwardMatchPoints implements F106-AC-10, AC-11 (TC-CMP-009): the Competition Points of every participant of a rated
// match by the point table of the Ranking Rules, capped per day, appended to the ledger (unique (source_type,
// source_id, student_id)) with competition_points.awarded; team battles also credit team_points. The boards follow
// after the commit.
func (h Ranking) AwardMatchPoints(ctx context.Context, tenant uuid.UUID, m MatchCompleted) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AwardMatchPoints")
	defer func() { done(err) }()
	if !m.Counts(tenant) {
		return 0, nil
	}
	rules, rv, err := h.Repo.Rules(ctx, m.EndedAt)
	if err != nil {
		return 0, err
	}
	season, err := h.Repo.SeasonAt(ctx, m.EndedAt)
	if err != nil || season == nil {
		return 0, errors.Join(errors.New("ranking: no season"), err)
	}
	topic, err := h.Repo.MatchTopic(ctx, m.MatchID)
	if err != nil {
		return 0, err
	}
	tiers, err := h.tiersBefore(ctx, tenant, m, season.ID)
	if err != nil {
		return 0, err
	}
	day := ldb.DayKey(m.EndedAt)
	var incs []Increment
	for _, l := range m.Results {
		if _, ok := ranking.Score(l.Outcome); !ok {
			continue
		}
		d, err := h.Repo.PointDay(ctx, tenant, l.Student, day)
		if err != nil {
			return n, err
		}
		award := points.ForMatch(rules.Points, points.Match{Outcome: l.Outcome, Correct: l.Correct,
			Upset: l.Outcome == "win" && upset(rules.Tiers, tiers, m.Results, l)}, d)
		grade := h.grade(ctx, tenant, l.Student, m.Grade)
		mode := m.Mode
		subject := m.SubjectID
		e := write.PointEntry{Tenant: tenant, School: l.School, Student: l.Student, Grade: grade, Season: season.ID, Subject: &subject,
			Topic: topic, Mode: &mode, Award: award, SourceType: sourceMatch, SourceID: m.MatchID, RuleVersion: rv, Day: day, At: m.EndedAt}
		ok, err := h.Repo.InsertPoints(ctx, e)
		if err != nil {
			return n, err
		}
		if !ok {
			continue
		}
		n++
		if award.Capped && h.Metrics != nil {
			h.Metrics.PointsCapped()
		}
		balance, err := h.Repo.SeasonPoints(ctx, tenant, l.Student, season.ID)
		if err != nil {
			return n, err
		}
		t := tenant
		if err := h.Events.Publish(ctx, write.Event{Type: EventPointsAwarded, SubjectType: "student", SubjectID: l.Student.String(),
			PartitionKey: l.Student.String(), TenantID: &t, OccurredAt: m.EndedAt, Payload: map[string]any{"student_id": l.Student,
				"school_id": l.School, "season_id": season.ID, "source_type": sourceMatch, "source_id": m.MatchID, "mode": m.Mode,
				"subject_id": m.SubjectID, "points": award.Amount, "balance_after": balance, "capped": award.Capped,
				"day_key": day.Format("2006-01-02"), "rule_version_id": rv}}); err != nil {
			return n, err
		}
		member := ldb.Member{Tenant: tenant, School: l.School, Grade: grade}
		incs = append(incs, Increment{Metric: ldb.MetricCompetitionPoints, Tenant: tenant, Member: member, Student: l.Student, Subject: &subject,
			Topic: topic, Mode: &mode, Amount: float64(award.Amount), At: m.EndedAt})
		if m.Mode == ranking.ModeTeamBattle && award.Amount > 0 {
			ok, err := h.Repo.InsertContribution(ctx, write.Contribution{Tenant: tenant, School: l.School, Student: l.Student, Grade: grade,
				Metric: ldb.MetricTeamPoints, Amount: award.Amount, SourceType: contributionTeamMatch, SourceID: m.MatchID.String(), Day: day,
				RuleVersion: &rv, At: m.EndedAt})
			if err != nil {
				return n, err
			}
			if ok {
				incs = append(incs, Increment{Metric: ldb.MetricTeamPoints, Tenant: tenant, Member: member, Student: l.Student,
					Amount: float64(award.Amount), At: m.EndedAt})
			}
		}
	}
	plan, err := h.Boards.Plan(ctx, season, incs)
	if err != nil {
		return n, err
	}
	return n, h.after(ctx, func(ctx context.Context) { h.Boards.Apply(ctx, plan) })
}

// tiersBefore returns the tier of each participant before the match: rating_changes.before when the rating consumer
// ran first, else the current rating (not updated by this match yet).
func (h Ranking) tiersBefore(ctx context.Context, tenant uuid.UUID, m MatchCompleted, season uuid.UUID) (map[uuid.UUID]*string, error) {
	out, err := h.Repo.BeforeTiers(ctx, m.MatchID)
	if err != nil {
		return nil, err
	}
	var missing []uuid.UUID
	for _, r := range m.Results {
		if _, ok := out[r.Student]; !ok {
			missing = append(missing, r.Student)
		}
	}
	if len(missing) == 0 || !ranking.Rated(m.Mode) {
		return out, nil
	}
	cur, err := h.Repo.LockRatings(ctx, tenant, missing, m.Mode, m.SubjectID, season)
	if err != nil {
		return nil, err
	}
	for _, s := range missing {
		if r, ok := cur[s]; ok {
			out[s] = r.Tier
		} else {
			out[s] = nil
		}
	}
	return out, nil
}

// upset: the winner had a tier and beat a higher tier (1v1: the opponent; team: the mean tier rank of the other team,
// every opponent ranked). Provisional students have no tier, so no upset.
func upset(t ranking.Tiers, tiers map[uuid.UUID]*string, all []MatchLine, me MatchLine) bool {
	mine := tiers[me.Student]
	if mine == nil {
		return false
	}
	sum, n := 0, 0
	for _, o := range all {
		if o.Team == me.Team {
			continue
		}
		ot := tiers[o.Student]
		if ot == nil {
			return false
		}
		sum += t.Rank(*ot)
		n++
	}
	return n > 0 && float64(sum)/float64(n) > float64(t.Rank(*mine))
}

// ActivityGraded is the part of activity.graded (owner learning) that earns effort_points.
type ActivityGraded struct {
	Attempt    uuid.UUID
	Student    uuid.UUID
	Review     bool
	RapidGuess bool
	At         time.Time
}

// RecordEffort implements F106-AC-16 (effort_points): the effort of a graded activity by the Ranking Rules (rapid guesses
// earn nothing, 10 activities and 25 points a day) credited to the school and grade of the student at that time
// (ADR-0095), in leaderboard_contributions (unique per source).
func (h Ranking) RecordEffort(ctx context.Context, tenant uuid.UUID, a ActivityGraded) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RecordContribution")
	defer func() { done(err) }()
	st, err := h.Repo.Student(ctx, tenant, a.Student)
	if err != nil {
		return 0, err
	}
	if st == nil {
		return 0, ErrStudentNotProjected
	}
	if !st.Active {
		return 0, nil
	}
	rules, rv, err := h.Repo.Rules(ctx, a.At)
	if err != nil {
		return 0, err
	}
	day := ldb.DayKey(a.At)
	d, err := h.Repo.EffortDay(ctx, tenant, a.Student, day)
	if err != nil {
		return 0, err
	}
	award := ldb.Effort(rules.Effort, ldb.Activity{Review: a.Review, RapidGuess: a.RapidGuess}, d)
	member := ldb.Member{Tenant: tenant, School: st.School, Grade: st.Grade}
	var incs []Increment
	for _, c := range []write.Contribution{
		{SourceType: award.ActivitySource, SourceID: a.Attempt.String(), Amount: award.Activity},
		{SourceType: ldb.SourceActiveDay, SourceID: "day:" + day.Format("2006-01-02"), Amount: award.ActiveDay},
	} {
		if c.Amount <= 0 {
			continue
		}
		c.Tenant, c.School, c.Student, c.Grade, c.Metric, c.Day, c.RuleVersion, c.At = tenant, st.School, a.Student, st.Grade,
			ldb.MetricEffortPoints, day, &rv, a.At
		ok, err := h.Repo.InsertContribution(ctx, c)
		if err != nil {
			return n, err
		}
		if ok {
			n++
			incs = append(incs, Increment{Metric: ldb.MetricEffortPoints, Tenant: tenant, Member: member, Student: a.Student,
				Amount: float64(c.Amount), At: a.At})
		}
	}
	if len(incs) == 0 {
		return n, nil
	}
	season, err := h.Repo.SeasonAt(ctx, a.At)
	if err != nil {
		return n, err
	}
	plan, err := h.Boards.Plan(ctx, season, incs)
	if err != nil {
		return n, fmt.Errorf("effort boards: %w", err)
	}
	return n, h.after(ctx, func(ctx context.Context) { h.Boards.Apply(ctx, plan) })
}
