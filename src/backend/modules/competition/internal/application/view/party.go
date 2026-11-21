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
		if v.present {
			mb["presence"] = PresenceOffline
			if v.Online[m.StudentID] {
				mb["presence"] = PresenceOnline
			}
		}
		members = append(members, mb)
	}
	return map[string]any{"party_id": p.ID, "name": p.Name, "leader_id": p.LeaderID, "party_type": p.Type, "visibility": p.Visibility,
		"invitation_policy": p.InvitationPolicy, "max_members": p.MaxMembers, "status": p.Status, "members": members,
		"state_version": p.StateVersion, "created_at": p.CreatedAt.UTC(), "updated_at": p.UpdatedAt.UTC()}
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

// MemberState is a PartyMemberState of the realtime contract (ready is owned by F101: false here).
func MemberState(m party.Member, c port.Card, online bool) map[string]any {
	status := PresenceDisconnected
	if online {
		status = PresenceOnline
	}
	return map[string]any{"player": CardBody(c), "role": m.Role, "status": status, "ready": false, "joined_at": ms(m.JoinedAt)}
}

// Snapshot is the payload of party.state.snapshot.
func (v Party) Snapshot() map[string]any {
	p := v.Party
	members := make([]map[string]any, 0, len(p.Members))
	for _, m := range p.Members {
		members = append(members, MemberState(m, card(v.Cards, m.StudentID), v.Online[m.StudentID]))
	}
	return map[string]any{"party_id": p.ID, "state_version": p.StateVersion, "name": p.Name, "party_type": p.Type,
		"visibility": p.Visibility, "leader_id": p.LeaderID, "max_members": p.MaxMembers, "status": p.Status, "members": members}
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
