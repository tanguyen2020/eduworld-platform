package battle

import (
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
)

// Client deltas of F109 (AsyncAPI 2026-11-27): the gameplay chat channel ids of a player, the canonical option key of
// the render payload and the encouragement after a loss streak (COMPETITION_SYSTEM §12).

// ChatChannels are the gameplay chat channels of one player of a match (F117): the match channel, and its team channel
// when the match has teams.
type ChatChannels struct {
	Match uuid.UUID
	Team  *uuid.UUID
}

// Payload is MatchChatChannelIds of the AsyncAPI.
func (c ChatChannels) Payload() map[string]any {
	m := map[string]any{"match": c.Match.String()}
	if c.Team != nil {
		m["team"] = c.Team.String()
	}
	return m
}

// ChatChannelsOf returns the gameplay chat channels of every player of a match from its team numbers (team_no of the
// match events). The ids are the deterministic ids of pkg/chatid that module chat creates when it consumes
// match.started; the team channels follow the rule of module chat (F117): at least two teams and one of them with two
// players or more (team battle) — none for 1v1, a bot match or a timed quiz.
func ChatChannelsOf(match uuid.UUID, teams map[uuid.UUID]int) map[uuid.UUID]ChatChannels {
	size := map[int]int{}
	for _, t := range teams {
		size[t]++
	}
	split := false
	if len(size) >= 2 {
		for _, n := range size {
			split = split || n >= 2
		}
	}
	out := make(map[uuid.UUID]ChatChannels, len(teams))
	for id, t := range teams {
		c := ChatChannels{Match: chatid.Match(match)}
		if split {
			team := chatid.Team(match, t)
			c.Team = &team
		}
		out[id] = c
	}
	return out
}

// chatChannels returns the chat channels of viewer in the room (nil when the viewer is not a player).
func (r *Room) chatChannels(viewer uuid.UUID) map[string]any {
	teams := make(map[uuid.UUID]int, len(r.Players))
	for _, p := range r.Players {
		teams[p.ID] = p.Team
	}
	c, ok := ChatChannelsOf(r.ID, teams)[viewer]
	if !ok {
		return nil
	}
	return c.Payload()
}

// aliasOptions gives every option of a render payload both keys: `id` (canonical, the render payload of module
// learning) and `option_id` (the alias kept for v1 clients of the AsyncAPI QuestionRender).
func aliasOptions(q map[string]any) {
	opts, ok := q["options"].([]any)
	if !ok {
		return
	}
	for _, o := range opts {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		id, hasID := m["id"].(string)
		alias, hasAlias := m["option_id"].(string)
		switch {
		case hasID && !hasAlias:
			m["option_id"] = id
		case hasAlias && !hasID:
			m["id"] = alias
		}
	}
}

// Encouragement codes and their client message keys (AsyncAPI BattleEncouragement): a locale key, never learning
// content.
const (
	EncouragementLossStreak        = "loss_streak_review"
	EncouragementLossStreakMessage = "competition.encouragement.loss_streak_review"
)

// Encouragement is the private encouragement of one player in battle.match.ended (COMPETITION_SYSTEM §12): after a loss
// streak, practise the subject (and topic) of the match.
type Encouragement struct {
	Code        string
	MessageCode string
	SubjectID   uuid.UUID
	TopicID     *uuid.UUID
}

// Payload is BattleEncouragement of the AsyncAPI.
func (e Encouragement) Payload() map[string]any {
	m := map[string]any{"code": e.Code, "message_code": e.MessageCode, "subject_id": e.SubjectID.String()}
	if e.TopicID != nil {
		m["topic_id"] = e.TopicID.String()
	}
	return m
}

// LossStreakEncouragement is the encouragement after a loss streak in subject (topic).
func LossStreakEncouragement(subject uuid.UUID, topic *uuid.UUID) Encouragement {
	return Encouragement{Code: EncouragementLossStreak, MessageCode: EncouragementLossStreakMessage, SubjectID: subject, TopicID: topic}
}

// Lost reports an outcome that is not a win (loss, forfeit, abandoned): a draw, a win or no contest is not lost.
func Lost(outcome string) bool {
	return outcome == OutcomeLoss || outcome == OutcomeForfeit || outcome == OutcomeAbandoned
}

// LossStreak reports n lost matches in a row: outcomes are the latest outcomes of a student, newest first; no_contest
// (timed quiz, cancelled) never counts and never breaks the streak, a win or a draw breaks it.
func LossStreak(outcomes []string, n int) bool {
	if n <= 0 {
		return false
	}
	lost := 0
	for _, o := range outcomes {
		switch {
		case o == OutcomeNoContest || o == "":
			continue
		case Lost(o):
			lost++
			if lost >= n {
				return true
			}
		default:
			return false
		}
	}
	return false
}
