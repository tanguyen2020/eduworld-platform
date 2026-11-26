package analytics

import (
	"time"

	"github.com/google/uuid"
)

// Mapped is one analytics event derived from a domain event for one student (Student nil for tenant-level events such
// as chat.daily_stats). Props hold catalog keys only (GAME_ANALYTICS §3.1: no score, no rating, no content, no balance).
type Mapped struct {
	EventName     string
	Student       *uuid.UUID
	Tenant        *uuid.UUID // per-student tenant of a cross-tenant event (match results); nil = envelope tenant
	SessionID     *uuid.UUID // game session (session_key) when the event carries it
	Platform      string
	ClientVersion string
	Props         map[string]any
}

// Events lists the domain events the analytics consumer reads, by producer stream (GAME_ANALYTICS §3.1).
var Events = map[string][]string{
	"game": {"game_session.started", "game_session.ended", "world.area_entered", "world.area_unlocked", "world.discovery_found",
		"world.area_completed", "story.chapter_completed", "avatar.created", "quest.accepted", "quest.step_completed",
		"quest.completed", "quest.abandoned", "challenge.started", "challenge.completed", "boss.encounter_started",
		"boss.encounter_completed", "xp.awarded", "level.reached", "currency.credited", "currency.debited",
		"shop.item_purchased", "achievement.unlocked", "collection.completed", "event.participation_started",
		"event.milestone_claimed"},
	"competition": {"party.activity_started", "match.started", "match.completed", "duel.sent", "duel.accepted"},
	"learning":    {"activity.submitted"},
	"chat":        {"chat.daily_stats"},
}

type payload map[string]any

func (p payload) str(k string) string {
	s, _ := p[k].(string)
	return s
}

func (p payload) id(k string) *uuid.UUID {
	if id, err := uuid.Parse(p.str(k)); err == nil && id != uuid.Nil {
		return &id
	}
	return nil
}

func (p payload) num(k string) (float64, bool) {
	n, ok := p[k].(float64)
	return n, ok
}

func (p payload) time(k string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, p.str(k))
	return t, err == nil
}

// copyKeys copies the listed keys that are present (strings, numbers, booleans only).
func (p payload) copyKeys(keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		switch v := p[k].(type) {
		case string, float64, bool:
			out[k] = v
		}
	}
	return out
}

func seconds(p payload, from, to string) (float64, bool) {
	a, ok1 := p.time(from)
	b, ok2 := p.time(to)
	if !ok1 || !ok2 || b.Before(a) {
		return 0, false
	}
	return b.Sub(a).Seconds(), true
}

func (p payload) idsOf(k string) []uuid.UUID {
	raw, _ := p[k].([]any)
	var out []uuid.UUID
	for _, x := range raw {
		s, _ := x.(string)
		if id, err := uuid.Parse(s); err == nil && id != uuid.Nil {
			out = append(out, id)
		}
	}
	return out
}

func one(name string, p payload, props map[string]any) []Mapped {
	st := p.id("student_id")
	if st == nil {
		return nil
	}
	return []Mapped{{EventName: name, Student: st, Props: props}}
}

// Map turns a verified domain event into analytics events (F080-AC-05). Unknown events and payloads without the student
// map to nothing (the consumer counts them). late is analytics_late_night_window (late_night prop of session_end).
func Map(eventType string, raw map[string]any, late Window) []Mapped {
	out := mapEvent(eventType, raw, late)
	for i := range out {
		if Social(out[i].EventName, out[i].Props) {
			props := map[string]any{"social": true}
			for k, v := range out[i].Props {
				props[k] = v
			}
			out[i].Props = props
		}
	}
	return out
}

func mapEvent(eventType string, raw map[string]any, late Window) []Mapped {
	p := payload(raw)
	switch eventType {
	case "game_session.started":
		out := one("session_start", p, map[string]any{})
		for i := range out {
			out[i].SessionID, out[i].Platform, out[i].ClientVersion = p.id("game_session_id"), p.str("platform"), p.str("client_version")
		}
		return out
	case "game_session.ended":
		props := p.copyKeys("end_reason")
		if ms, ok := p.num("duration_ms"); ok && ms >= 0 {
			props["active_seconds"] = float64(int64(ms / 1000))
			if end, ok := p.time("ended_at"); ok {
				props["late_night"] = late.Intersects(end.Add(-time.Duration(ms)*time.Millisecond), end)
			}
		}
		out := one("session_end", p, props)
		for i := range out {
			out[i].SessionID, out[i].Platform, out[i].ClientVersion = p.id("game_session_id"), p.str("platform"), p.str("client_version")
		}
		return out
	case "world.area_entered":
		return one("area_enter", p, p.copyKeys("area_id", "from_area_id"))
	case "world.area_unlocked":
		return one("area_unlock", p, p.copyKeys("area_id"))
	case "world.discovery_found":
		return one("discovery", p, p.copyKeys("area_id", "kind", "completion_pct"))
	case "world.area_completed":
		return one("area_complete", p, p.copyKeys("area_id", "completion_pct"))
	case "story.chapter_completed":
		return one("story_chapter_complete", p, p.copyKeys("chapter_code", "region_code", "order"))
	case "avatar.created":
		return one("avatar_create", p, p.copyKeys("preset_code"))
	case "quest.accepted":
		props := p.copyKeys("quest_id", "quest_version_id", "quest_type")
		props["party"] = p.id("party_id") != nil
		return one("quest_accept", p, props)
	case "quest.step_completed":
		return one("quest_step", p, p.copyKeys("quest_id", "quest_version_id", "step_id"))
	case "quest.completed":
		props := p.copyKeys("quest_id", "quest_version_id", "quest_type", "story_chapter_code")
		props["party"] = p.id("party_id") != nil
		if s, ok := seconds(p, "started_at", "completed_at"); ok {
			props["duration_s"] = float64(int64(s))
		}
		return one("quest_complete", p, props)
	case "quest.abandoned":
		return one("quest_abandon", p, p.copyKeys("quest_id", "quest_version_id", "quest_type"))
	case "challenge.started":
		return one("challenge_start", p, p.copyKeys("challenge_id", "challenge_version_id", "difficulty"))
	case "challenge.completed":
		props := p.copyKeys("challenge_id", "challenge_version_id", "difficulty", "status")
		if s, ok := seconds(p, "started_at", "ended_at"); ok {
			props["duration_s"] = float64(int64(s))
		}
		return one("challenge_result", p, props)
	case "boss.encounter_started":
		members := p.idsOf("member_ids")
		props := p.copyKeys("boss_id", "boss_version_id")
		props["party_size"] = float64(max(len(members), 1))
		return each("boss_start", members, props)
	case "boss.encounter_completed":
		var members []uuid.UUID
		if cs, ok := p["contributions"].([]any); ok {
			for _, c := range cs {
				if m, ok := c.(map[string]any); ok {
					if id := payload(m).id("student_id"); id != nil {
						members = append(members, *id)
					}
				}
			}
		}
		props := p.copyKeys("boss_id", "boss_version_id", "result")
		props["party_size"] = float64(max(len(members), 1))
		return each("boss_result", members, props)
	case "xp.awarded":
		return one("xp_award", p, p.copyKeys("source_code", "amount", "capped"))
	case "level.reached":
		return one("level_up", p, p.copyKeys("to_level"))
	case "currency.credited":
		return one("currency_credit", p, p.copyKeys("reason_code", "delta", "capped"))
	case "currency.debited":
		return one("currency_debit", p, p.copyKeys("reason_code", "delta"))
	case "shop.item_purchased":
		return one("shop_purchase", p, p.copyKeys("item_id", "price"))
	case "achievement.unlocked":
		return one("achievement_unlock", p, p.copyKeys("achievement_id"))
	case "collection.completed":
		return one("collection_complete", p, p.copyKeys("collection_id"))
	case "event.participation_started":
		return one("event_join", p, p.copyKeys("event_id"))
	case "event.milestone_claimed":
		props := p.copyKeys("event_id")
		props["goal_code"] = p.str("milestone_id")
		return one("event_goal_complete", p, props)
	case "party.activity_started":
		members := p.idsOf("member_ids")
		props := map[string]any{"party_size": float64(len(members)), "activity_kind": p.str("activity_type")}
		return each("party_start", members, props)
	case "match.started":
		return []Mapped{{EventName: "match_start", Props: p.copyKeys("match_id", "mode")}}
	case "match.completed":
		return matchResults(p)
	case "duel.sent", "duel.accepted":
		var members []uuid.UUID
		for _, k := range []string{"challenger_id", "opponent_id"} {
			if id := p.id(k); id != nil {
				members = append(members, *id)
			}
		}
		return each("duel", members, map[string]any{})
	case "activity.submitted":
		ctx := p.str("context_type")
		if ctx != "quest" && ctx != "challenge" && ctx != "minigame" {
			return nil
		}
		return one("learning_activity_done", p, map[string]any{"activity_type": p.str("activity_type"), "context": ctx})
	case "chat.daily_stats":
		return []Mapped{{EventName: "chat_daily_volume", Props: p.copyKeys("stat_date", "channel_type", "messages", "masked", "blocked")}}
	}
	return nil
}

func each(name string, members []uuid.UUID, props map[string]any) []Mapped {
	out := make([]Mapped, 0, len(members))
	for _, m := range members {
		id := m
		out = append(out, Mapped{EventName: name, Student: &id, Props: props})
	}
	return out
}

func matchResults(p payload) []Mapped {
	rs, _ := p["results"].([]any)
	var out []Mapped
	for _, r := range rs {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		rp := payload(m)
		st := rp.id("student_id")
		if st == nil {
			continue
		}
		outcome := rp.str("outcome")
		if p.str("status") == "cancelled" {
			outcome = "abandoned"
		}
		props := map[string]any{"mode": p.str("mode"), "outcome": outcome, "team": p.str("matchmaking_type") != "solo"}
		out = append(out, Mapped{EventName: "match_result", Student: st, Tenant: rp.id("tenant_id"), Props: props})
	}
	return out
}

// Social events count for social_play_rate (party activity, team match, group boss, duel — GAME_ANALYTICS §4.4).
func Social(name string, props map[string]any) bool {
	switch name {
	case "party_start", "duel":
		return true
	case "boss_start":
		n, _ := props["party_size"].(float64)
		return n > 1
	case "match_result":
		t, _ := props["team"].(bool)
		return t
	}
	return false
}
