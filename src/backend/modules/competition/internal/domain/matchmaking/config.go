// Package matchmaking is the pure domain of matchmaking (F104, MATCHMAKING §2–§9): the hard constraints H1–H7 that
// partition queues (never relaxed), the expanding rating window, the cost of a pair, the greedy pairing with the
// team snake draft, the ready check outcome and the friend duel. No Valkey, no SQL: adapters sit behind ports.
package matchmaking

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Modes that go through a queue (COMPETITION_SYSTEM §2): coop_challenge and timed_quiz are party/instance modes,
// tournament pairs come from the bracket (CreateDirectProposal).
const (
	ModeBattle1v1  = "battle_1v1"
	ModeTeamBattle = "team_battle"
	ModeTournament = "tournament"
)

// Matchmaking types (MATCHMAKING §3, FR-MM-001). friend is a duel only, tournament a direct proposal only, bot the
// practice match (MATCHMAKING §9).
const (
	TypeRandom     = "random"
	TypeSkill      = "skill"
	TypeGrade      = "grade"
	TypeSubject    = "subject"
	TypeTopic      = "topic"
	TypeTeam       = "team"
	TypeFriend     = "friend"
	TypeTournament = "tournament"
	TypeBot        = "bot"
)

// KnownTypes are the values of matchmaking_type accepted by the contract.
var KnownTypes = []string{TypeRandom, TypeSkill, TypeGrade, TypeSubject, TypeTopic, TypeFriend, TypeTeam, TypeTournament}

// WindowStep is one row of mm_rating_window: from AfterS seconds of wait the window is Width (nil: unbounded).
type WindowStep struct {
	AfterS int      `json:"after_s"`
	Width  *float64 `json:"width"`
}

// ModeConfig is one queueable mode of the configuration.
type ModeConfig struct {
	Mode        string   `json:"mode"`
	MinGrade    int      `json:"min_grade"`
	Types       []string `json:"matchmaking_types"`
	RankedTypes []string `json:"ranked_types"`
	TeamSize    int      `json:"team_size,omitempty"`
	MinTeamSize int      `json:"min_team_size,omitempty"`
	MaxTeamSize int      `json:"max_team_size,omitempty"`
}

// SubjectConfig limits a subject to grade levels and, when TopicIDs is not empty, to topics.
type SubjectConfig struct {
	SubjectID   uuid.UUID   `json:"subject_id"`
	GradeLevels []int       `json:"grade_levels"`
	TopicIDs    []uuid.UUID `json:"topic_ids,omitempty"`
}

// TicketRate is mm_ticket_rate: tickets per hour, cooldown after DeclineLimit declines/timeouts in DeclineWindowS.
type TicketRate struct {
	PerHour        int `json:"per_hour"`
	DeclineLimit   int `json:"decline_limit"`
	DeclineWindowS int `json:"decline_window_s"`
	CooldownS      int `json:"cooldown_s"`
}

// GradeWindow is grade_window_g6_9: in grade bands 6–7 and 8–9, ±Levels grade after AfterS seconds of wait.
type GradeWindow struct {
	AfterS int `json:"after_s"`
	Levels int `json:"levels"`
}

// CostWeights are the weights of the soft scoring (MATCHMAKING §2.2).
type CostWeights struct {
	Rating         float64 `json:"rating"`
	RD             float64 `json:"rd"`
	RecentOpponent float64 `json:"recent_opponent"`
	SameClass      float64 `json:"same_class"`
}

// Rating is a Glicko-2 rating on the 1500 scale (glicko2_params).
type Rating struct {
	Mu float64 `json:"mu"`
	RD float64 `json:"rd"`
}

// Config is the published matchmaking configuration (competition.game_config_versions, type matchmaking-configs):
// platform only, read by F104, authored by F108 (MATCHMAKING §5.1). VersionID is the ruleset version of a match.
type Config struct {
	VersionID             uuid.UUID       `json:"-"`
	SchemaVersion         int             `json:"schema_version"`
	RatingWindow          []WindowStep    `json:"rating_window"`
	ReadyCheckS           int             `json:"ready_check_s"`
	BotAfterS             int             `json:"bot_after_s"`
	TicketTTLS            int             `json:"ticket_ttl_s"`
	TicketsPerUser        int             `json:"tickets_per_user"`
	TicketRate            TicketRate      `json:"ticket_rate"`
	GradeWindow           GradeWindow     `json:"grade_window_g6_9"`
	DisconnectCancelS     int             `json:"disconnect_cancel_s"`
	BucketThreshold       int             `json:"bucket_threshold"`
	Batch                 int             `json:"batch"`
	CostWeights           CostWeights     `json:"cost_weights"`
	RecentOpponentWindowS int             `json:"recent_opponent_window_s"`
	DefaultRating         Rating          `json:"default_rating"`
	Modes                 []ModeConfig    `json:"modes"`
	Subjects              []SubjectConfig `json:"subjects"`
}

// ErrConfig is an invalid configuration.
var ErrConfig = errors.New("matchmaking: invalid configuration")

// Parse decodes and validates a configuration payload.
func Parse(b []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, errors.Join(ErrConfig, err)
	}
	return c, c.Validate()
}

// Validate checks the ranges of the configuration.
func (c Config) Validate() error {
	bad := func(what string) error { return fmt.Errorf("%w: %s", ErrConfig, what) }
	if c.SchemaVersion != 1 {
		return bad("schema_version")
	}
	if len(c.RatingWindow) == 0 || c.RatingWindow[0].AfterS != 0 {
		return bad("rating_window must start at 0 s")
	}
	for i := 1; i < len(c.RatingWindow); i++ {
		if c.RatingWindow[i].AfterS <= c.RatingWindow[i-1].AfterS {
			return bad("rating_window steps must grow")
		}
	}
	switch {
	case c.ReadyCheckS < 5 || c.ReadyCheckS > 60:
		return bad("ready_check_s")
	case c.BotAfterS < 30:
		return bad("bot_after_s")
	case c.TicketTTLS < 60 || c.TicketTTLS > 600:
		return bad("ticket_ttl_s")
	case c.TicketsPerUser != 1:
		return bad("tickets_per_user")
	case c.TicketRate.PerHour < 1 || c.TicketRate.DeclineLimit < 1 || c.TicketRate.DeclineWindowS < 1 || c.TicketRate.CooldownS < 1:
		return bad("ticket_rate")
	case c.GradeWindow.AfterS < 0 || c.GradeWindow.Levels < 0 || c.GradeWindow.Levels > 1:
		return bad("grade_window_g6_9")
	case c.Batch < 2 || c.BucketThreshold < c.Batch:
		return bad("batch/bucket_threshold")
	case c.DefaultRating.Mu <= 0 || c.DefaultRating.RD <= 0:
		return bad("default_rating")
	case len(c.Modes) == 0:
		return bad("modes")
	}
	for _, m := range c.Modes {
		if m.Mode != ModeBattle1v1 && m.Mode != ModeTeamBattle {
			return bad("mode " + m.Mode + " is not queueable")
		}
		if m.MinGrade < 1 || m.MinGrade > 9 || len(m.Types) == 0 {
			return bad("mode " + m.Mode)
		}
		for _, t := range m.Types {
			if !queueable(t) || (m.Mode == ModeTeamBattle) != (t == TypeTeam) {
				return bad("matchmaking type " + t + " of " + m.Mode)
			}
		}
		if m.Mode == ModeTeamBattle && (m.MinTeamSize < 2 || m.MaxTeamSize > 4 || m.TeamSize < m.MinTeamSize || m.TeamSize > m.MaxTeamSize) {
			return bad("team sizes")
		}
	}
	return nil
}

func queueable(t string) bool {
	return slices.Contains([]string{TypeRandom, TypeSkill, TypeGrade, TypeSubject, TypeTopic, TypeTeam}, t)
}

// Mode returns the configuration of a queueable mode.
func (c Config) Mode(mode string) (ModeConfig, bool) {
	for _, m := range c.Modes {
		if m.Mode == mode {
			return m, true
		}
	}
	return ModeConfig{}, false
}

// Ranked reports whether a match of mode and type counts for rating (never for bot, duel or random).
func (c Config) Ranked(mode, typ string) bool {
	m, ok := c.Mode(mode)
	return ok && slices.Contains(m.RankedTypes, typ)
}

// Window is window(t) of mm_rating_window for a wait (MATCHMAKING §2.2); +Inf once rating is ignored.
func (c Config) Window(wait time.Duration) float64 {
	w := math.Inf(1)
	for _, s := range c.RatingWindow {
		if wait >= time.Duration(s.AfterS)*time.Second {
			w = math.Inf(1)
			if s.Width != nil {
				w = *s.Width
			}
		}
	}
	return w
}

// Widened reports a window wider than the first step (search_widened of the ticket).
func (c Config) Widened(wait time.Duration) bool {
	return len(c.RatingWindow) > 1 && wait >= time.Duration(c.RatingWindow[1].AfterS)*time.Second
}

// Durations of the configuration.
func (c Config) ReadyCheck() time.Duration { return time.Duration(c.ReadyCheckS) * time.Second }
func (c Config) BotAfter() time.Duration   { return time.Duration(c.BotAfterS) * time.Second }
func (c Config) TicketTTL() time.Duration  { return time.Duration(c.TicketTTLS) * time.Second }
func (c Config) DisconnectCancel() time.Duration {
	return time.Duration(c.DisconnectCancelS) * time.Second
}
func (c Config) DeclineWindow() time.Duration {
	return time.Duration(c.TicketRate.DeclineWindowS) * time.Second
}
func (c Config) Cooldown() time.Duration { return time.Duration(c.TicketRate.CooldownS) * time.Second }
func (c Config) RecentWindow() time.Duration {
	return time.Duration(c.RecentOpponentWindowS) * time.Second
}

// SubjectsFor returns the subjects configured for a grade (empty: the configuration does not limit subjects).
func (c Config) SubjectsFor(grade int) []uuid.UUID {
	var out []uuid.UUID
	for _, s := range c.Subjects {
		if slices.Contains(s.GradeLevels, grade) {
			out = append(out, s.SubjectID)
		}
	}
	return out
}

// subjectAllowed reports whether subject (and topic) is available for grade.
func (c Config) subjectAllowed(grade int, subject uuid.UUID, topic *uuid.UUID) (subjectOK, topicOK bool) {
	if len(c.Subjects) == 0 {
		return true, true
	}
	for _, s := range c.Subjects {
		if s.SubjectID != subject || !slices.Contains(s.GradeLevels, grade) {
			continue
		}
		if topic == nil || len(s.TopicIDs) == 0 {
			return true, true
		}
		return true, slices.Contains(s.TopicIDs, *topic)
	}
	return false, false
}
