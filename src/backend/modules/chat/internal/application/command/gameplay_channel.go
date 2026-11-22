package command

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Events of the gameplay channel lifecycle (EVENT_DESIGN §4.9, specs/events/payloads/chat/).
// SeqTTLAfterClose is the TTL of the seq counter of a closed channel (sends are refused once closed).
const SeqTTLAfterClose = 24 * time.Hour

const (
	EventChannelOpened = "chat.channel_opened"
	EventChannelClosed = "chat.channel_closed"
	AuditChannelClosed = "chat.channel.closed"
)

// Reasons a participant of a party/match event is not added (metric label, no PII; F117-AC-07).
const (
	SkipTenantMismatch = "tenant_mismatch"
	SkipNotStudent     = "not_student"
	SkipClosed         = "channel_closed"
	SkipStale          = "stale_event"
)

// GameplayConfig: DefaultRetentionDays is gameplay_chat_retention_days of CONFIG_DEFAULTS (7) used while identity has
// projected no chat_retention setting.
type GameplayConfig struct {
	DefaultRetentionDays int
}

// Gameplay is the lifecycle of the always-on gameplay channels match/team/party (F117 US1, ADR-0020 §6): consumer
// chat.gameplay_channels calls it in the transaction of the competition event. Channel ids are deterministic
// (pkg/chatid, research R1), so every handler is an idempotent upsert that tolerates redelivery and reordering
// (F117-AC-03): a closed channel never gets members again, an older party change never undoes a newer one. Only active
// projected students of the event tenant become members (F117-AC-07); staff never.
type Gameplay struct {
	Channels
	Gameplay write.GameplayRepository
	Events   write.EventPublisher
	Metrics  port.GameplayMetrics // nil: none
	Seq      port.SeqExpirer      // nil: the counter keeps no TTL
	Config   GameplayConfig
}

func (h Gameplay) skipped(ctx context.Context, reason, scope string) {
	slog.InfoContext(ctx, "chat: gameplay participant skipped", "reason", reason, "scope", scope)
	if h.Metrics != nil {
		h.Metrics.Skipped(reason)
	}
}

// retention resolves gameplay_chat_retention_days of tenant: the tenant override when the platform allows it, else the
// platform chat_retention, else the configured default; below the ADR-0033 minimum the minimum applies with a warning
// (F117-AC-15).
func (h Gameplay) retention(ctx context.Context, tenant uuid.UUID) (channel.Retention, error) {
	s, err := h.Gameplay.Retention(ctx, tenant)
	if err != nil {
		return channel.Retention{}, err
	}
	days := h.Config.DefaultRetentionDays
	allowTenant := true
	if len(s.Platform) > 0 && string(s.Platform) != "null" {
		var p struct {
			Days          *int  `json:"gameplay_chat_retention_days"`
			AllowOverride *bool `json:"allow_tenant_override"`
		}
		if json.Unmarshal(s.Platform, &p) == nil {
			if p.Days != nil {
				days = *p.Days
			}
			if p.AllowOverride != nil {
				allowTenant = *p.AllowOverride
			}
		}
	}
	if s.Tenant != nil && allowTenant {
		days = *s.Tenant
	}
	r := channel.ResolveRetention(days)
	if r.Floored {
		slog.WarnContext(ctx, "chat: gameplay_chat_retention_days below the minimum, the minimum applies", "configured", days,
			"minimum", channel.MinGameplayRetentionDays)
		if h.Metrics != nil {
			h.Metrics.RetentionFloored()
		}
	}
	return r, nil
}

func (h Gameplay) opened(ctx context.Context, ch channel.Channel) error {
	members, err := h.Gameplay.ActiveMembers(ctx, ch.TenantID, ch.ID)
	if err != nil {
		return err
	}
	if members == nil {
		members = []uuid.UUID{}
	}
	t := ch.TenantID
	return h.Events.Publish(ctx, write.Event{Type: EventChannelOpened, SubjectType: "chat_channel", SubjectID: ch.ID.String(),
		PartitionKey: ch.ID.String(), TenantID: &t, OccurredAt: h.now(), Payload: map[string]any{"channel_id": ch.ID,
			"channel_type": string(ch.Type), "tenant_id": ch.TenantID, "scope_ref": ch.ScopeRef, "member_ids": members,
			"opened_at": ch.OpenedAt}})
}

// ensure returns the channel id of tenant, creating it open when unknown (created true).
func (h Gameplay) ensure(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, id uuid.UUID, t channel.Type,
	scope map[string]string) (channel.Channel, bool, error) {
	cur, err := h.Repo.Channel(ctx, tenant, id)
	if err != nil || cur != nil {
		if cur != nil {
			return *cur, false, nil
		}
		return channel.Channel{}, false, err
	}
	ch, err := channel.New(id, tenant, school, t, scope, h.now())
	if err != nil {
		return channel.Channel{}, false, err
	}
	created, err := h.Repo.CreateChannel(ctx, ch)
	if err != nil {
		return channel.Channel{}, false, err
	}
	if !created { // a concurrent consumer created it: read it back
		cur, err := h.Repo.Channel(ctx, tenant, id)
		if err != nil {
			return channel.Channel{}, false, err
		}
		if cur == nil { // the id exists in another tenant (RLS hides it): never touched (F117-AC-07)
			return channel.Channel{}, false, ErrProjectionTenantMismatch
		}
		return *cur, false, nil
	}
	if err := h.record(ctx, AuditChannelOpened, ch.ID, map[string]any{"channel_type": string(ch.Type)}); err != nil {
		return channel.Channel{}, false, err
	}
	if h.Metrics != nil {
		h.Metrics.Opened(string(ch.Type))
	}
	return ch, true, nil
}

// add makes user an active member of an open channel; staff, unknown students and other tenants are skipped.
func (h Gameplay) add(ctx context.Context, ch channel.Channel, user uuid.UUID) (bool, error) {
	if !ch.Open() {
		h.skipped(ctx, SkipClosed, string(ch.Type))
		return false, nil
	}
	school, ok, err := h.Repo.StudentSchool(ctx, ch.TenantID, user)
	if err != nil {
		return false, err
	}
	if !ok {
		h.skipped(ctx, SkipNotStudent, string(ch.Type))
		return false, nil
	}
	changed, err := h.Repo.JoinMember(ctx, channel.Member{ChannelID: ch.ID, UserID: user, TenantID: ch.TenantID, SchoolID: &school,
		Role: channel.RoleMember, JoinedAt: h.now()})
	if err != nil || !changed {
		return false, err
	}
	return true, h.record(ctx, AuditMemberAdded, ch.ID, map[string]any{"user_id": user.String()})
}

func (h Gameplay) remove(ctx context.Context, ch channel.Channel, user uuid.UUID) (bool, error) {
	changed, err := h.Repo.LeaveMember(ctx, ch.TenantID, ch.ID, user, h.now())
	if err != nil || !changed {
		return false, err
	}
	return true, h.record(ctx, AuditMemberRemoved, ch.ID, map[string]any{"user_id": user.String()})
}

// close makes ch read-only with purge_after = closed_at + retention of the tenant (F117-AC-05, AC-15).
func (h Gameplay) close(ctx context.Context, ch channel.Channel, at time.Time) error {
	r, err := h.retention(ctx, ch.TenantID)
	if err != nil {
		return err
	}
	changed, err := ch.Close(at, r)
	if err != nil || !changed {
		return err
	}
	if ok, err := h.Gameplay.CloseChannel(ctx, ch); err != nil || !ok {
		return err
	}
	if err := h.record(ctx, AuditChannelClosed, ch.ID, map[string]any{"channel_type": string(ch.Type),
		"purge_after": ch.PurgeAfter.Format(time.RFC3339)}); err != nil {
		return err
	}
	if h.Metrics != nil {
		h.Metrics.Closed(string(ch.Type))
	}
	if h.Seq != nil && h.AfterCommit != nil {
		id := ch.ID
		if err := h.AfterCommit(ctx, func(ctx context.Context) { _ = h.Seq.Expire(ctx, id, SeqTTLAfterClose) }); err != nil {
			return err
		}
	}
	if err := h.invalidate(ctx, ch.TenantID, ch.ID); err != nil {
		return err
	}
	t := ch.TenantID
	return h.Events.Publish(ctx, write.Event{Type: EventChannelClosed, SubjectType: "chat_channel", SubjectID: ch.ID.String(),
		PartitionKey: ch.ID.String(), TenantID: &t, OccurredAt: h.now(), Payload: map[string]any{"channel_id": ch.ID,
			"channel_type": string(ch.Type), "tenant_id": ch.TenantID, "scope_ref": ch.ScopeRef, "closed_at": *ch.ClosedAt,
			"purge_after": *ch.PurgeAfter}})
}

// PartyChange is one party event for the chat (party.created/member_joined/member_left/member_kicked/disbanded).
type PartyChange struct {
	TenantID  uuid.UUID
	PartyID   uuid.UUID
	SchoolID  *uuid.UUID
	StudentID uuid.UUID // the member concerned (leader for party.created); Nil for party.disbanded
	Join      bool
	Disband   bool
	Version   int64 // state_version of the party after the change
	At        time.Time
}

// ApplyParty applies one party event to the channel party (F117-AC-01, AC-03, AC-04, AC-05).
func (h Gameplay) ApplyParty(ctx context.Context, c PartyChange) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApplyPartyChange")
	defer func() { done(err) }()
	return h.UoW.InTenantTx(ctx, mustScope(c.TenantID), func(ctx context.Context) error {
		st, err := h.Gameplay.Party(ctx, c.TenantID, c.PartyID)
		if err != nil {
			return err
		}
		if st == nil {
			st = &write.PartyState{PartyID: c.PartyID, TenantID: c.TenantID, Members: map[uuid.UUID]write.PartyMember{}}
		}
		ch, created, err := h.ensure(ctx, c.TenantID, c.SchoolID, chatid.Party(c.PartyID), channel.Party,
			map[string]string{"party_id": c.PartyID.String()})
		if err != nil {
			return err
		}
		changed := created
		switch {
		case c.Disband:
			st.Disbanded = true
		case st.Disbanded:
			h.skipped(ctx, SkipClosed, string(channel.Party)) // member_joined after party.disbanded (edge case)
		default:
			prev, known := st.Members[c.StudentID]
			if known && prev.V >= c.Version {
				h.skipped(ctx, SkipStale, string(channel.Party))
				break
			}
			st.Members[c.StudentID] = write.PartyMember{V: c.Version, In: c.Join}
			var moved bool
			if c.Join {
				moved, err = h.add(ctx, ch, c.StudentID)
			} else {
				moved, err = h.remove(ctx, ch, c.StudentID)
			}
			if err != nil {
				return err
			}
			changed = changed || moved
		}
		st.Version = max(st.Version, c.Version)
		if err := h.Gameplay.SaveParty(ctx, *st); err != nil {
			return err
		}
		if changed && ch.Open() && !st.Disbanded {
			if err := h.invalidate(ctx, c.TenantID, ch.ID); err != nil {
				return err
			}
			if err := h.opened(ctx, ch); err != nil {
				return err
			}
		}
		if st.Disbanded {
			return h.close(ctx, ch, c.At)
		}
		return nil
	})
}

// Participant is one player of a match event.
type Participant struct {
	StudentID uuid.UUID
	TenantID  uuid.UUID
	SchoolID  uuid.UUID
	TeamNo    int
}

// MatchChange is match.started (Ended false) or match.completed/match.aborted (Ended true).
type MatchChange struct {
	TenantID     uuid.UUID
	MatchID      uuid.UUID
	Participants []Participant
	Ended        bool
	At           time.Time
}

// teams returns the team numbers of a team match (≥ 2 teams, one with ≥ 2 players: team_battle); none for 1v1 or a
// single-team co-op (the match channel is the team).
func teams(ps []Participant) []int {
	size := map[int]int{}
	for _, p := range ps {
		size[p.TeamNo]++
	}
	if len(size) < 2 {
		return nil
	}
	big := false
	for _, n := range size {
		big = big || n >= 2
	}
	if !big {
		return nil
	}
	out := make([]int, 0, len(size))
	for t := range size {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

// ApplyMatch opens the match channel of every player and one team channel per team (match.started, F117-AC-02), or
// closes them (match.completed/aborted, F117-AC-05). Participants of another tenant are skipped (F117-AC-07); an end
// event arriving first creates the channels closed so that a late match.started adds nobody (F117-AC-03).
func (h Gameplay) ApplyMatch(ctx context.Context, m MatchChange) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApplyMatchChange")
	defer func() { done(err) }()
	var ps []Participant
	for _, p := range m.Participants {
		if p.TenantID != m.TenantID {
			h.skipped(ctx, SkipTenantMismatch, string(channel.Match))
			continue
		}
		ps = append(ps, p)
	}
	return h.UoW.InTenantTx(ctx, mustScope(m.TenantID), func(ctx context.Context) error {
		type target struct {
			id    uuid.UUID
			t     channel.Type
			scope map[string]string
			users []uuid.UUID
		}
		all := make([]uuid.UUID, 0, len(ps))
		for _, p := range ps {
			all = append(all, p.StudentID)
		}
		targets := []target{{chatid.Match(m.MatchID), channel.Match, map[string]string{"match_id": m.MatchID.String()}, all}}
		for _, t := range teams(ps) {
			var users []uuid.UUID
			for _, p := range ps {
				if p.TeamNo == t {
					users = append(users, p.StudentID)
				}
			}
			targets = append(targets, target{chatid.Team(m.MatchID, t), channel.Team,
				map[string]string{"match_id": m.MatchID.String(), "team_no": itoa(t)}, users})
		}
		for _, tg := range targets {
			ch, created, err := h.ensure(ctx, m.TenantID, nil, tg.id, tg.t, tg.scope)
			if err != nil {
				return err
			}
			if m.Ended {
				if err := h.close(ctx, ch, m.At); err != nil {
					return err
				}
				continue
			}
			changed := created
			for _, u := range tg.users {
				moved, err := h.add(ctx, ch, u)
				if err != nil {
					return err
				}
				changed = changed || moved
			}
			if changed && ch.Open() {
				if err := h.invalidate(ctx, m.TenantID, ch.ID); err != nil {
					return err
				}
				if err := h.opened(ctx, ch); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
