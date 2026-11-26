// Package view builds the contract shapes of the party use cases (OpenAPI Party, PartyInvitation and the AsyncAPI
// party payloads) from the domain and the public cards: nickname, avatar, level and role only — never a real name,
// school or class (R16, F100-AC-06).
package view

import (
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// Presence values of PartyMember.presence (REST) and PartyMemberState.status (realtime).
const (
	PresenceOnline       = "online"
	PresenceOffline      = "offline"
	PresenceDisconnected = "disconnected"
)

// Party is a party with the public cards and presence of its members.
type Party struct {
	Party   party.Party
	Cards   map[uuid.UUID]port.Card
	Online  map[uuid.UUID]bool
	present bool
	// F101: Ready, member status and leader grace of the lobby (nil: not loaded), room of the running session.
	Lobby *party.Lobby
	Room  *uuid.UUID
}

// NewParty builds the view; online nil = presence unknown (not part of the response).
func NewParty(p party.Party, cards map[uuid.UUID]port.Card, online map[uuid.UUID]bool) Party {
	return Party{Party: p, Cards: cards, Online: online, present: online != nil}
}

func card(cards map[uuid.UUID]port.Card, id uuid.UUID) port.Card {
	if c, ok := cards[id]; ok {
		return c
	}
	return port.Card{UserID: id, Level: 1}
}

func nickname(c port.Card) string {
	if c.Nickname != nil {
		return *c.Nickname
	}
	return ""
}

// CardBody is a PlayerCard / PublicPlayer: user_id, nickname, level, avatar_config.
func CardBody(c port.Card) map[string]any {
	m := map[string]any{"user_id": c.UserID, "nickname": nickname(c), "level": max(c.Level, 1)}
	if len(c.Avatar) > 0 && string(c.Avatar) != "null" {
		m["avatar_config"] = c.Avatar
	}
	return m
}

// Body is the Party of the contract (also stored as the idempotent result).
func (v Party) Body() map[string]any {
	p := v.Party
	members := make([]map[string]any, 0, len(p.Members))
	for _, m := range p.Members {
		c := card(v.Cards, m.StudentID)
		mb := map[string]any{"student_id": m.StudentID, "nickname": nickname(c), "role": m.Role, "joined_at": m.JoinedAt.UTC()}
		if len(c.Avatar) > 0 && string(c.Avatar) != "null" {
			mb["avatar_config"] = c.Avatar
		}
		// XP level of the projection ref.player_profiles (delta F102): absent while game has projected nothing for the
		// student — the party read never fails for it.
		if known, ok := v.Cards[m.StudentID]; ok && known.Level >= 1 {
			mb["level"] = known.Level
		}
		if v.present {
			mb["presence"] = PresenceOffline
			if v.Online[m.StudentID] {
				mb["presence"] = PresenceOnline
				if p.Status == party.StatusInActivity {
					mb["presence"] = party.MemberInActivity
				}
			}
		}
		if v.Lobby != nil {
			mb["ready"] = v.Lobby.Member(m.StudentID).Ready
		}
		members = append(members, mb)
	}
	out := map[string]any{"party_id": p.ID, "name": p.Name, "leader_id": p.LeaderID, "party_type": p.Type, "visibility": p.Visibility,
		"invitation_policy": p.InvitationPolicy, "max_members": p.MaxMembers, "status": p.Status, "members": members,
		"state_version": p.StateVersion, "created_at": p.CreatedAt.UTC(), "updated_at": p.UpdatedAt.UTC()}
	if p.Selected != nil {
		out["current_activity"] = Selection(*p.Selected)
	}
	if v.Lobby != nil {
		out["leader_reconnecting"] = v.Lobby.LeaderGoneAt != nil
	}
	if v.Room != nil {
		out["current_room_id"] = *v.Room
	}
	return out
}

// Selection is the PartyActivitySelection of the contract.
func Selection(s party.Selection) map[string]any {
	m := map[string]any{"activity_type": s.Type, "activity_ref": s.Ref}
	if s.GameMode != "" {
		m["game_mode"] = s.GameMode
	}
	return m
}

// ActivityRef is the ActivityRef of the realtime contract.
func ActivityRef(typ, ref string) map[string]any { return map[string]any{"kind": typ, "id": ref} }

// Ineligible is a PartyIneligibleMember list (REST student_id, realtime user_id): reason code and requirement of the
// activity only, never the grade or class of the student (R16).
func Ineligible(cs []party.Check, idKey string) []map[string]any {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		m := map[string]any{idKey: c.StudentID, "reason_code": c.Reason}
		if c.Requirement != "" {
			m["requirement"] = c.Requirement
		}
		out = append(out, m)
	}
	return out
}

// Invitation is a PartyInvitation with the party name and the inviter card.
type Invitation struct {
	Invitation party.Invitation
	PartyName  string
	Inviter    port.Card
}

// Body is the PartyInvitation of the contract (inviter: nickname/avatar only — F100-AC-15).
func (v Invitation) Body() map[string]any {
	i := v.Invitation
	return map[string]any{"invitation_id": i.ID, "party_id": i.PartyID, "party_name": v.PartyName, "inviter": CardBody(v.Inviter),
		"invitee_id": i.InviteeID, "status": i.Status, "created_at": i.CreatedAt.UTC(), "expires_at": i.ExpiresAt.UTC()}
}

// ms is an EpochMs of the realtime contract.
func ms(t time.Time) int64 { return t.UnixMilli() }

// MemberState is a PartyMemberState of the realtime contract (ready false unless the lobby says otherwise, F101).
func MemberState(m party.Member, c port.Card, online bool) map[string]any {
	status := PresenceDisconnected
	if online {
		status = PresenceOnline
	}
	return map[string]any{"player": CardBody(c), "role": m.Role, "status": status, "ready": false, "joined_at": ms(m.JoinedAt)}
}

// Snapshot is the payload of party.state.snapshot (F101-AC-06: members, presence, Ready, selected activity,
// state_version, leader grace and the room of the running session).
func (v Party) Snapshot() map[string]any {
	p := v.Party
	members := make([]map[string]any, 0, len(p.Members))
	for _, m := range p.Members {
		st := MemberState(m, card(v.Cards, m.StudentID), v.Online[m.StudentID])
		if v.Lobby != nil {
			l := v.Lobby.Member(m.StudentID)
			st["ready"] = l.Ready
			if l.Status != "" {
				st["status"] = l.Status
			}
		}
		members = append(members, st)
	}
	out := map[string]any{"party_id": p.ID, "state_version": p.StateVersion, "name": p.Name, "party_type": p.Type,
		"visibility": p.Visibility, "leader_id": p.LeaderID, "max_members": p.MaxMembers, "status": p.Status, "members": members}
	if p.Selected != nil {
		out["current_activity"] = ActivityRef(p.Selected.Type, p.Selected.Ref)
	}
	if v.Lobby != nil {
		out["leader_reconnecting"] = v.Lobby.LeaderGoneAt != nil
	}
	if v.Room != nil {
		out["room_id"] = v.Room.String()
	}
	return out
}

// InvitationReceived is the payload of party.invitation.received (to the invitee).
func InvitationReceived(i party.Invitation, p party.Party, inviter port.Card) map[string]any {
	return map[string]any{"invitation_id": i.ID, "party_id": i.PartyID, "party_type": p.Type, "inviter": CardBody(inviter),
		"member_count": len(p.Members), "expires_at": ms(i.ExpiresAt)}
}

// InvitationUpdated is the payload of party.invitation.updated.
func InvitationUpdated(i party.Invitation) map[string]any {
	return map[string]any{"invitation_id": i.ID, "party_id": i.PartyID, "status": i.Status}
}

// SessionStart is the PartySessionStart of startPartySession (also the idempotent result) and its realtime push.
type SessionStart struct {
	Activity party.Activity
	Members  []uuid.UUID
	Offline  []uuid.UUID
	SceneRef string
}

// Body is the PartySessionStart of the contract.
func (s SessionStart) Body() map[string]any {
	a := s.Activity
	excluded := make([]map[string]any, 0, len(s.Offline))
	for _, id := range s.Offline {
		excluded = append(excluded, map[string]any{"student_id": id, "reason": party.ReasonNotOnline})
	}
	out := map[string]any{"room_id": a.RoomID, "room_type": a.RoomType, "party_activity_id": a.ID, "activity_type": a.Type,
		"activity_ref": a.Ref, "game_session_ref": a.ID, "members": s.Members, "excluded_members": excluded}
	if s.SceneRef != "" {
		out["scene_ref"] = s.SceneRef
	}
	return out
}

// Started is the payload of party.session.started (to the participants).
func (s SessionStart) Started() map[string]any {
	a := s.Activity
	out := map[string]any{"party_id": a.PartyID, "room_id": a.RoomID.String(), "room_type": a.RoomType, "party_activity_id": a.ID,
		"activity": ActivityRef(a.Type, a.Ref), "member_ids": s.Members}
	if s.SceneRef != "" {
		out["scene_ref"] = s.SceneRef
	}
	return out
}

// Ended is the payload of party.session.ended: shared progress only, never a score or a result of a student
// (FR-PTY-013, F101-AC-13).
func Ended(a party.Activity) map[string]any {
	return map[string]any{"party_id": a.PartyID, "party_activity_id": a.ID, "outcome": party.Outcome(a.EndReason), "reason": a.EndReason,
		"summary": party.CleanSummary(a.Summary)}
}

// Eligible is a PartyEligibleActivity of listPartyEligibleActivities.
type Eligible struct {
	Type        string
	Ref         string
	Title       map[string]string
	Eligibility party.Eligibility
}

// Body is the PartyEligibleActivity of the contract (reason codes and requirements only, R16).
func (e Eligible) Body() map[string]any {
	all := len(e.Eligibility.Members) > 0
	members := make([]map[string]any, 0, len(e.Eligibility.Members))
	for _, c := range e.Eligibility.Members {
		m := map[string]any{"student_id": c.StudentID, "eligible": c.Eligible}
		if !c.Eligible {
			all = false
			m["reason_code"] = c.Reason
			if c.Requirement != "" {
				m["requirement"] = c.Requirement
			}
		}
		members = append(members, m)
	}
	out := map[string]any{"activity_type": e.Type, "activity_ref": e.Ref, "min_members": e.Eligibility.MinMembers,
		"max_members": e.Eligibility.MaxMembers, "all_eligible": all, "members": members}
	if len(e.Title) > 0 {
		out["title"] = e.Title
	}
	return out
}
