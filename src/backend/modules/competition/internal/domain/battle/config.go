// Package battle is the pure domain of the server-authoritative match rooms of module competition (F105,
// REALTIME_ARCHITECTURE §7, COMPETITION_SYSTEM §3–§4, MULTIPLAYER_GAMEPLAY §9): the state machine of a match room, the
// rules of each mode (1v1, team, co-op, timed quiz, tournament, practice bot), the server-side points with the speed
// bonus by grade, the disconnect graces and the fast-answer detector. No clock, no I/O: the room host feeds it the
// server time and the results of module learning, and runs the effects it returns (messages, signals, persistence).
package battle

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Modes of a match (COMPETITION_SYSTEM §2, enum MatchMode).
const (
	Mode1v1        = "battle_1v1"
	ModeTeam       = "team_battle"
	ModeCoop       = "coop_challenge"
	ModeTimedQuiz  = "timed_quiz"
	ModeTournament = "tournament"
)

// ErrModeGrade: the mode is not open to the grade (team battle from grade 4, COMPETITION_SYSTEM §3.2).
var ErrModeGrade = errors.New("battle: mode not open to this grade")

// ErrUnknownMode: not a mode of the battle engine.
var ErrUnknownMode = errors.New("battle: unknown mode")

// Format is battle_1v1_format / the timed quiz format (CONFIG_DEFAULTS): questions, time per question (grades 1–3 have
// their own), points of a right answer, speed bonus and last-question multiplier.
type Format struct {
	Questions      int `json:"questions"`
	TimeS          int `json:"time_s"`
	TimeG13S       int `json:"time_g1_3_s"`
	Points         int `json:"points"`
	BonusMax       int `json:"speed_bonus_max"`
	LastMultiplier int `json:"last_multiplier"`
}

// TeamFormat is team_battle_format: questions, support turn share (percent of the points), minimum grade.
type TeamFormat struct {
	Format
	SupportPercent int `json:"support_percent"`
	MinGrade       int `json:"min_grade"`
}

// TimedQuizFormat is the timed quiz (COMPETITION_SYSTEM §3.4): at most 40 students, top 5, grade goal (grade_goal, ADR-0095) for grades 1–3.
type TimedQuizFormat struct {
	Format
	MaxPlayers   int `json:"max_players"`
	Top          int `json:"top"`
	GoalPercent  int `json:"grade_goal_percent"`
	RankMinGrade int `json:"rank_min_grade"`
}

// CoopFormat is the co-op challenge (COMPETITION_SYSTEM §3.3): questions, time per question, collective target (number
// of questions answered right by at least one member).
type CoopFormat struct {
	Questions int `json:"questions"`
	TimeS     int `json:"time_s"`
	Target    int `json:"target"`
}

// Graces is disconnect_grace and tournament_timers (MULTIPLAYER_GAMEPLAY §9.2): seconds per mode, tournament graces per
// match.
type Graces struct {
	Battle1v1S    int `json:"battle_1v1_s"`
	TeamS         int `json:"team_battle_s"`
	CoopS         int `json:"coop_challenge_s"`
	TournamentS   int `json:"tournament_s"`
	TournamentMax int `json:"tournament_max"`
}

// Timing are the timers of the state machine (REALTIME_ARCHITECTURE §7.1).
type Timing struct {
	AnswerGraceMS int `json:"answer_grace_ms"` // late answers after deadline + 300 ms
	RTTMS         int `json:"rtt_allowance_ms"`
	CountdownS    int `json:"countdown_s"`
	RevealS       int `json:"reveal_s"`
	LobbyWaitS    int `json:"lobby_wait_s"`
	ScoreWaitS    int `json:"score_wait_s"`
	RevealWaitS   int `json:"reveal_wait_s"`
	MaxReplays    int `json:"max_replays"`
}

// FastAnswer is cheat_fast_answer_flag (COMPETITION_SYSTEM §4): faster than the threshold Run times in a row.
type FastAnswer struct {
	G15MS int `json:"g1_5_ms"`
	G69MS int `json:"g6_9_ms"`
	Run   int `json:"run"`
}

// AbandonPenalty is abandon_penalty (MULTIPLAYER_GAMEPLAY §9.1): Count abandoned ranked matches in WindowH hours →
// ranked play restricted for RestrictMin minutes.
type AbandonPenalty struct {
	Count       int `json:"count"`
	WindowH     int `json:"window_h"`
	RestrictMin int `json:"restrict_min"`
}

// BotProfile is one practice bot (MATCHMAKING §9, bot_profiles): accuracy and answer delay range.
type BotProfile struct {
	MinGrade int     `json:"min_grade,omitempty"`
	MaxGrade int     `json:"max_grade,omitempty"`
	Accuracy float64 `json:"accuracy"`
	MinMS    int     `json:"min_ms"`
	MaxMS    int     `json:"max_ms"`
}

// Config is the battle configuration: the `battle` and `bot_profiles` sections of the published matchmaking
// configuration pinned by the match (ruleset_version_id). A missing key takes the CONFIG_DEFAULTS value.
type Config struct {
	SpeedBonusMinGrade int `json:"speed_bonus_min_grade"`
	// LossStreak is loss_streak_encouragement (COMPETITION_SYSTEM §12): lost matches in a row before the encouragement.
	LossStreak     int             `json:"loss_streak_encouragement"`
	Battle1v1      Format          `json:"battle_1v1"`
	Team           TeamFormat      `json:"team_battle"`
	TimedQuiz      TimedQuizFormat `json:"timed_quiz"`
	Coop           CoopFormat      `json:"coop_challenge"`
	Graces         Graces          `json:"disconnect_grace"`
	Timing         Timing          `json:"timing"`
	FastAnswer     FastAnswer      `json:"fast_answer"`
	AbandonPenalty AbandonPenalty  `json:"abandon_penalty"`
	BotProfiles    []BotProfile    `json:"-"`
}

// DefaultConfig is CONFIG_DEFAULTS: battle_1v1_format (7 × 20 s, g1_3 30 s, 100 + bonus ≤ 50, last ×2),
// team_battle_format (10 questions, support 30 %, grade ≥ 4), speed_bonus_min_grade 6, disconnect_grace (1v1 30 s,
// team 45 s, tournament 60 s × 2), cheat_fast_answer_flag (400 ms, g1_5 600 ms, 3 in a row), abandon_penalty
// (3 / 24 h → 15 min), loss_streak_encouragement (3) and the bot of MATCHMAKING §9 (60–85 % accuracy). Timed quiz, co-op and the timers of the state
// machine are [Proposed F105] (spec Clarifications 2026-11-21).
func DefaultConfig() Config {
	f := Format{Questions: 7, TimeS: 20, TimeG13S: 30, Points: 100, BonusMax: 50, LastMultiplier: 2}
	tq := f
	tq.Questions = 10
	return Config{
		SpeedBonusMinGrade: 6,
		LossStreak:         3,
		Battle1v1:          f,
		Team:               TeamFormat{Format: Format{Questions: 10, TimeS: 20, TimeG13S: 30, Points: 100, BonusMax: 50, LastMultiplier: 2}, SupportPercent: 30, MinGrade: 4},
		TimedQuiz:          TimedQuizFormat{Format: tq, MaxPlayers: 40, Top: 5, GoalPercent: 60, RankMinGrade: 4},
		Coop:               CoopFormat{Questions: 12, TimeS: 25, Target: 10},
		Graces:             Graces{Battle1v1S: 30, TeamS: 45, CoopS: 120, TournamentS: 60, TournamentMax: 2},
		Timing: Timing{AnswerGraceMS: 300, RTTMS: 150, CountdownS: 3, RevealS: 3, LobbyWaitS: 15, ScoreWaitS: 3, RevealWaitS: 2,
			MaxReplays: 2},
		FastAnswer:     FastAnswer{G15MS: 600, G69MS: 400, Run: 3},
		AbandonPenalty: AbandonPenalty{Count: 3, WindowH: 24, RestrictMin: 15},
		BotProfiles: []BotProfile{{MinGrade: 1, MaxGrade: 3, Accuracy: 0.6, MinMS: 6000, MaxMS: 18000},
			{MinGrade: 4, MaxGrade: 5, Accuracy: 0.7, MinMS: 4000, MaxMS: 14000}, {MinGrade: 6, MaxGrade: 9, Accuracy: 0.75, MinMS: 3000, MaxMS: 12000}},
	}
}

// ParseConfig reads the battle configuration from a published matchmaking configuration payload: its `battle` object
// and `bot_profiles` array override the defaults key by key.
func ParseConfig(payload []byte) (Config, error) {
	c := DefaultConfig()
	if len(payload) == 0 {
		return c, nil
	}
	var doc struct {
		Battle json.RawMessage `json:"battle"`
		Bots   []BotProfile    `json:"bot_profiles"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return c, fmt.Errorf("battle config: %w", err)
	}
	if len(doc.Battle) > 0 && string(doc.Battle) != "null" {
		if err := json.Unmarshal(doc.Battle, &c); err != nil {
			return c, fmt.Errorf("battle config: %w", err)
		}
	}
	if len(doc.Bots) > 0 {
		c.BotProfiles = doc.Bots
	}
	return c, nil
}

// BotFor returns the bot profile of a grade (the last profile when none matches).
func (c Config) BotFor(grade int) BotProfile {
	for _, b := range c.BotProfiles {
		if grade >= b.MinGrade && grade <= b.MaxGrade {
			return b
		}
	}
	if len(c.BotProfiles) > 0 {
		return c.BotProfiles[len(c.BotProfiles)-1]
	}
	return BotProfile{Accuracy: 0.7, MinMS: 4000, MaxMS: 12000}
}

// Rules are the resolved rules of one match.
type Rules struct {
	Mode           string        `json:"mode"`
	QuestionCount  int           `json:"question_count"`
	TimeLimit      time.Duration `json:"time_limit"`
	CorrectPoints  int           `json:"correct_points"`
	SpeedBonusMax  int           `json:"speed_bonus_max"`
	SpeedBonus     bool          `json:"speed_bonus"`
	LastMultiplier int           `json:"last_multiplier"`
	AnswerGrace    time.Duration `json:"answer_grace"`
	RTT            time.Duration `json:"rtt"`
	Countdown      time.Duration `json:"countdown"`
	RevealFor      time.Duration `json:"reveal_for"`
	LobbyWait      time.Duration `json:"lobby_wait"`
	ScoreWait      time.Duration `json:"score_wait"`
	RevealWait     time.Duration `json:"reveal_wait"`
	MaxReplays     int           `json:"max_replays"`
	Grace          time.Duration `json:"grace"`
	MaxGraces      int           `json:"max_graces"`
	OutOutcome     string        `json:"out_outcome"`
	EndOnAbandon   bool          `json:"end_on_abandon"`
	HoldSeats      bool          `json:"hold_seats"`
	Rotation       bool          `json:"rotation"`
	SupportPercent int           `json:"support_percent"`
	FastAnswer     time.Duration `json:"fast_answer"`
	FastRun        int           `json:"fast_run"`
	MaxPlayers     int           `json:"max_players"`
	Ranking        bool          `json:"ranking"`
	Top            int           `json:"top"`
	GroupGoal      bool          `json:"group_goal"`
	GoalPercent    int           `json:"goal_percent"`
	CoopTarget     int           `json:"coop_target"`
	Collective     bool          `json:"collective"`
}

func secs(s int) time.Duration { return time.Duration(s) * time.Second }

// RulesFor resolves the rules of a match of mode for its (lowest) grade; tournament applies the tournament rules
// (COMPETITION_SYSTEM §10.7): 1v1 or team format, 60 s grace at most twice, forfeit, never rated.
func (c Config) RulesFor(mode string, grade int, tournament bool) (Rules, error) {
	if grade < 1 || grade > 9 {
		return Rules{}, fmt.Errorf("%w: grade %d", ErrModeGrade, grade)
	}
	t := c.Timing
	r := Rules{Mode: mode, AnswerGrace: time.Duration(t.AnswerGraceMS) * time.Millisecond, RTT: time.Duration(t.RTTMS) * time.Millisecond,
		Countdown: secs(t.CountdownS), RevealFor: secs(t.RevealS), LobbyWait: secs(t.LobbyWaitS), ScoreWait: secs(t.ScoreWaitS),
		RevealWait: secs(t.RevealWaitS), MaxReplays: t.MaxReplays, OutOutcome: OutcomeAbandoned, FastRun: c.FastAnswer.Run, MaxPlayers: 2,
		SpeedBonus: grade >= c.SpeedBonusMinGrade}
	r.FastAnswer = time.Duration(c.FastAnswer.G69MS) * time.Millisecond
	if grade <= 5 {
		r.FastAnswer = time.Duration(c.FastAnswer.G15MS) * time.Millisecond
	}
	format := func(f Format) {
		r.QuestionCount, r.CorrectPoints, r.SpeedBonusMax, r.LastMultiplier = f.Questions, f.Points, f.BonusMax, f.LastMultiplier
		r.TimeLimit = secs(f.TimeS)
		if grade <= 3 && f.TimeG13S > 0 {
			r.TimeLimit = secs(f.TimeG13S)
		}
	}
	switch mode {
	case Mode1v1, ModeTournament:
		format(c.Battle1v1)
		r.Grace, r.EndOnAbandon = secs(c.Graces.Battle1v1S), true
	case ModeTeam:
		if grade < c.Team.MinGrade {
			return Rules{}, fmt.Errorf("%w: team battle from grade %d", ErrModeGrade, c.Team.MinGrade)
		}
		format(c.Team.Format)
		r.Grace, r.Rotation, r.SupportPercent, r.MaxPlayers = secs(c.Graces.TeamS), true, c.Team.SupportPercent, 8
	case ModeTimedQuiz:
		format(c.TimedQuiz.Format)
		r.HoldSeats, r.MaxPlayers, r.Top, r.GoalPercent = true, c.TimedQuiz.MaxPlayers, c.TimedQuiz.Top, c.TimedQuiz.GoalPercent
		r.Ranking = grade >= c.TimedQuiz.RankMinGrade
		r.GroupGoal = !r.Ranking
	case ModeCoop:
		r.QuestionCount, r.TimeLimit, r.CorrectPoints, r.LastMultiplier = c.Coop.Questions, secs(c.Coop.TimeS), 100, 1
		r.SpeedBonus, r.Grace, r.Collective, r.GroupGoal, r.CoopTarget, r.MaxPlayers = false, secs(c.Graces.CoopS), true, true, c.Coop.Target, 8
	default:
		return Rules{}, fmt.Errorf("%w: %s", ErrUnknownMode, mode)
	}
	if tournament || mode == ModeTournament {
		if mode == ModeTimedQuiz || mode == ModeCoop {
			return Rules{}, fmt.Errorf("%w: %s is not a tournament format", ErrUnknownMode, mode)
		}
		r.Grace, r.MaxGraces, r.OutOutcome, r.EndOnAbandon = secs(c.Graces.TournamentS), c.Graces.TournamentMax, OutcomeForfeit, true
	}
	if r.QuestionCount < 1 || r.QuestionCount > 30 || r.TimeLimit < 5*time.Second || r.TimeLimit > 120*time.Second {
		return Rules{}, fmt.Errorf("battle config: %s has %d questions of %v", mode, r.QuestionCount, r.TimeLimit)
	}
	return r, nil
}
