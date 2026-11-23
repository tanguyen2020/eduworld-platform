package consumer

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// MatchRanker is the F106 use case of a completed match (command.Ranking.ApplyMatchRating or AwardMatchPoints).
type MatchRanker func(ctx context.Context, tenant uuid.UUID, m command.MatchCompleted) (int, error)

// MatchCompleted is the consumer of match.completed for ranking (competition.ranking_match) or for the Competition
// Points (competition.points_match): it decodes the payload of F105 and calls one use case in the transaction of
// processed_events (a replayed event changes nothing, F106-AC-02, AC-13).
type MatchCompleted struct{ H MatchRanker }

// RankingMatchOptions returns the options of competition.ranking_match.
func RankingMatchOptions(h MatchCompleted, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.RankingMatchName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.MatchCompletedEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// PointsMatchOptions returns the options of competition.points_match.
func PointsMatchOptions(h MatchCompleted, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.PointsMatchName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.MatchCompletedEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// DecodeMatchCompleted decodes and checks a match.completed payload (match.completed.v1).
func DecodeMatchCompleted(e outbox.Envelope) (command.MatchCompleted, error) {
	var p struct {
		MatchID      uuid.UUID  `json:"match_id"`
		Mode         string     `json:"mode"`
		Type         string     `json:"matchmaking_type"`
		Ranked       bool       `json:"ranked"`
		Rated        bool       `json:"rated"`
		IsBot        bool       `json:"is_bot"`
		SubjectID    uuid.UUID  `json:"subject_id"`
		Grade        int        `json:"grade_level"`
		Status       string     `json:"status"`
		TournamentID *uuid.UUID `json:"tournament_id"`
		EndedAt      time.Time  `json:"ended_at"`
		Results      []struct {
			StudentID uuid.UUID `json:"student_id"`
			TenantID  uuid.UUID `json:"tenant_id"`
			SchoolID  uuid.UUID `json:"school_id"`
			TeamNo    int       `json:"team_no"`
			Outcome   string    `json:"outcome"`
			Correct   int       `json:"correct_count"`
		} `json:"results"`
	}
	if err := decode(e, &p); err != nil {
		return command.MatchCompleted{}, err
	}
	switch {
	case p.MatchID == uuid.Nil || p.SubjectID == uuid.Nil || p.EndedAt.IsZero():
		return command.MatchCompleted{}, invalid(e.EventType, "match_id/subject_id/ended_at")
	case p.Grade < 1 || p.Grade > 9, len(p.Results) == 0 || len(p.Results) > 40, !maxLen(&p.Mode, 64), !maxLen(&p.Type, 64):
		return command.MatchCompleted{}, invalid(e.EventType, "grade_level/results")
	}
	m := command.MatchCompleted{MatchID: p.MatchID, Mode: p.Mode, Type: p.Type, Ranked: p.Ranked, Rated: p.Rated, IsBot: p.IsBot,
		SubjectID: p.SubjectID, Grade: p.Grade, Status: p.Status, TournamentID: p.TournamentID, EndedAt: p.EndedAt.UTC()}
	for _, r := range p.Results {
		if r.StudentID == uuid.Nil || r.TenantID == uuid.Nil || r.SchoolID == uuid.Nil || r.TeamNo < 0 || r.TeamNo > 1 || r.Correct < 0 ||
			!slices.Contains([]string{"win", "loss", "draw", "abandoned", "forfeit", "no_contest"}, r.Outcome) {
			return command.MatchCompleted{}, invalid(e.EventType, "results[]")
		}
		m.Results = append(m.Results, command.MatchLine{Student: r.StudentID, Tenant: r.TenantID, School: r.SchoolID, Team: r.TeamNo,
			Outcome: r.Outcome, Correct: r.Correct})
	}
	return m, nil
}

// Handle implements outbox.Handler.
func (c MatchCompleted) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "match.completed" {
		return nil
	}
	m, err := DecodeMatchCompleted(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil {
		return outbox.Permanent(err)
	}
	_, err = c.H(ctx, tenant, m)
	return err
}

// EffortRecorder is command.Ranking.RecordEffort.
type EffortRecorder func(ctx context.Context, tenant uuid.UUID, a command.ActivityGraded) (int, error)

// LeaderboardEffort is the consumer competition.leaderboard_effort: activity.graded (owner learning) → effort_points
// (F106-AC-16). The payload carries no school nor grade: the use case takes them from ref.students at that time
// (ADR-0095) and retries while the student is not projected yet.
type LeaderboardEffort struct{ H EffortRecorder }

// LeaderboardEffortOptions returns its options.
func LeaderboardEffortOptions(h LeaderboardEffort, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.LeaderboardEffortName, Stream: messaging.LearningStream,
		Subjects: messaging.Subjects("learning", messaging.LeaderboardEffortEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c LeaderboardEffort) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "activity.graded" {
		return nil
	}
	var p struct {
		AttemptID      uuid.UUID  `json:"attempt_id"`
		StudentID      uuid.UUID  `json:"student_id"`
		AssignmentID   *uuid.UUID `json:"assignment_id"`
		Recommendation *uuid.UUID `json:"recommendation_id"`
		RapidGuess     bool       `json:"rapid_guess_flagged"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil || p.AttemptID == uuid.Nil || p.StudentID == uuid.Nil {
		return outbox.Permanent(invalid(e.EventType, "attempt_id/student_id/tenant"))
	}
	_, err = c.H(ctx, tenant, command.ActivityGraded{Attempt: p.AttemptID, Student: p.StudentID,
		Review: p.AssignmentID != nil || p.Recommendation != nil, RapidGuess: p.RapidGuess, At: e.OccurredAt.UTC()})
	// command.ErrStudentNotProjected is retried (backoff, then DLQ): the identity projection catches up.
	return err
}

// XPRecorder is RecordXP (command.Ranking).
type XPRecorder func(ctx context.Context, tenant uuid.UUID, x command.XPAwarded) (int, error)

// LeaderboardXP is the consumer competition.leaderboard_xp: xp.awarded (owner F074) → the xp metric of the friends
// boards (friends_xp_weekly). The payload carries no school nor grade: the use case reads ref.students and retries while
// the student is not projected yet.
type LeaderboardXP struct{ H XPRecorder }

// LeaderboardXPOptions returns its options.
func LeaderboardXPOptions(h LeaderboardXP, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.LeaderboardXPName, Stream: messaging.GameStream,
		Subjects: messaging.Subjects("game", messaging.LeaderboardXPEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c LeaderboardXP) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.EventType != "xp.awarded" {
		return nil
	}
	var p struct {
		EntryID   uuid.UUID `json:"xp_entry_id"`
		StudentID uuid.UUID `json:"student_id"`
		Amount    int       `json:"amount"`
		Reason    string    `json:"reason_code"`
	}
	if err := decode(e, &p); err != nil {
		return outbox.Permanent(err)
	}
	tenant, err := envTenant(e)
	if err != nil || p.EntryID == uuid.Nil || p.StudentID == uuid.Nil {
		return outbox.Permanent(invalid(e.EventType, "xp_entry_id/student_id/tenant"))
	}
	if p.Reason == "adjustment" {
		return nil // compensating entries (merge, adjustment) are not effort of the week
	}
	_, err = c.H(ctx, tenant, command.XPAwarded{Entry: p.EntryID, Student: p.StudentID, Amount: p.Amount, At: e.OccurredAt.UTC()})
	return err
}
