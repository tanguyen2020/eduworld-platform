package party

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// Member statuses of the Party Lobby (AsyncAPI PartyMemberState.status, PARTY_SYSTEM §5).
const (
	MemberOnline       = "online"
	MemberAway         = "away"
	MemberInActivity   = "in_activity"
	MemberDisconnected = "disconnected"
)

// MemberStatus maps a presence status of the realtime gateway (PresenceStatus; "" = no signal or expired) to the
// member status of the lobby.
func MemberStatus(presence string) string {
	switch presence {
	case "online":
		return MemberOnline
	case "away":
		return MemberAway
	case "in_activity", "in_match":
		return MemberInActivity
	}
	return MemberDisconnected
}

// Connected reports a member status that counts as online for the party rules (away and in_activity included).
func Connected(status string) bool { return status != "" && status != MemberDisconnected }

// MemberState is the ephemeral lobby state of one member (Valkey, rebuildable: Ready false, ADR-0004).
type MemberState struct {
	Status string     // last status seen by the sweeper ("" = not seen yet)
	Ready  bool       //
	GoneAt *time.Time // disconnected since (member grace in a session)
}

// Lobby is the ephemeral state of a Party Lobby kept by module competition (Valkey ew:party:<party_id>:lobby).
type Lobby struct {
	Members       map[uuid.UUID]MemberState
	LeaderGoneAt  *time.Time // leader disconnected since (party_leader_grace)
	LastOnline    time.Time  // last time a member was online (party_dormant_timeout); zero = unknown
	BelowMinSince *time.Time // participants online below min_members since (party_activity_min_members_timeout)
}

// Member returns the state of a member (zero value when unknown).
func (l Lobby) Member(id uuid.UUID) MemberState {
	if l.Members == nil {
		return MemberState{}
	}
	return l.Members[id]
}

// Ready returns the Ready flags of the members.
func (l Lobby) Ready() map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for id, m := range l.Members {
		if m.Ready {
			out[id] = true
		}
	}
	return out
}

// AllReady reports that every online member of p is Ready (all_ready of party.member.ready_changed); online nil
// treats every member as online.
func AllReady(p *Party, online, ready map[uuid.UUID]bool) bool {
	n := 0
	for _, m := range p.Members {
		if online != nil && !online[m.StudentID] {
			continue
		}
		n++
		if !ready[m.StudentID] {
			return false
		}
	}
	return n > 0
}

// NextOnlineLeader is the member who takes over a disconnected leader: the online member who joined first
// (PARTY_SYSTEM §9, F101-AC-15); false when nobody else is online (dormant).
func (p *Party) NextOnlineLeader(online map[uuid.UUID]bool) (uuid.UUID, bool) {
	ms := append([]Member(nil), p.Members...)
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].JoinedAt.Before(ms[j].JoinedAt) })
	for _, m := range ms {
		if m.StudentID != p.LeaderID && online[m.StudentID] {
			return m.StudentID, true
		}
	}
	return uuid.Nil, false
}

// Reasons of party.leader_transferred and party.disbanded owned by F101.
const (
	TransferLeaderDisconnected = "leader_disconnected"
	DisbandInactivity          = "inactivity"
)

// Leave reasons of a session membership (MULTIPLAYER_GAMEPLAY §10).
const (
	SessionLeft         = "left"
	SessionDisconnected = "disconnected"
	SessionTimeout      = "timeout"
	SessionRoomClosed   = "room_closed"
)

// DormantDisbands reports the party types that disband after party_dormant_timeout without anyone online
// (PARTY_SYSTEM §9: friend, custom, study_group).
func DormantDisbands(typ string) bool {
	return typ == TypeFriend || typ == TypeCustom || typ == TypeStudyGroup
}

// SweepConfig are the timers of the presence sweeper (CONFIG_DEFAULTS party_leader_grace, party_dormant_timeout,
// disconnect_grace of private_instance/boss, party_activity_min_members_timeout).
type SweepConfig struct {
	LeaderGrace       time.Duration
	DormantTimeout    time.Duration
	MemberGrace       time.Duration
	MinMembersTimeout time.Duration
}

// SweepInput is what one sweep of a party knows: the party, its lobby state, the presence of its members and the
// running activity with the participants whose session membership is still open.
type SweepInput struct {
	Party    *Party
	Lobby    Lobby
	Presence map[uuid.UUID]string // member status (MemberStatus), missing = disconnected
	Activity *Activity            // running activity, nil when none
	Open     map[uuid.UUID]bool   // participants with an open session membership
	Now      time.Time
}

// StatusChange is a member whose status changed since the previous sweep.
type StatusChange struct {
	StudentID uuid.UUID
	Status    string
}

// SweepOutcome is what one sweep decides (F101-AC-14…AC-18); the application applies it.
type SweepOutcome struct {
	Lobby       Lobby
	Changes     []StatusChange
	ReadyReset  []uuid.UUID // Ready reset in the lobby because the member disconnected
	TransferTo  uuid.UUID   // uuid.Nil: no automatic transfer
	Disband     bool        // dormant past party_dormant_timeout
	CloseMember []uuid.UUID // disconnected past the member grace of the room
	EndActivity string      // end reason, "" = keep running
	Dirty       bool        // the lobby state changed
}

func tptr(t time.Time) *time.Time { return &t }

// Sweep evaluates one party (PARTY_SYSTEM §9): member statuses, Ready reset in the lobby, leader grace and automatic
// transfer, dormancy, member grace and minimum members of a running activity.
func Sweep(in SweepInput, cfg SweepConfig) SweepOutcome {
	p, now := in.Party, in.Now
	out := SweepOutcome{Lobby: Lobby{Members: map[uuid.UUID]MemberState{}, LeaderGoneAt: in.Lobby.LeaderGoneAt,
		LastOnline: in.Lobby.LastOnline, BelowMinSince: in.Lobby.BelowMinSince}}
	online := map[uuid.UUID]bool{}
	for _, m := range p.Members {
		id := m.StudentID
		prev := in.Lobby.Member(id)
		st := in.Presence[id]
		if st == "" {
			st = MemberDisconnected
		}
		next := prev
		next.Status = st
		if Connected(st) {
			online[id] = true
			next.GoneAt = nil
		} else {
			if prev.GoneAt == nil {
				next.GoneAt = tptr(now)
			}
			if prev.Ready && p.Status == StatusOpen {
				next.Ready = false
				out.ReadyReset = append(out.ReadyReset, id)
			}
		}
		if prev.Status != st {
			out.Dirty = true
			if prev.Status != "" {
				out.Changes = append(out.Changes, StatusChange{StudentID: id, Status: st})
			}
		}
		if (prev.GoneAt == nil) != (next.GoneAt == nil) {
			out.Dirty = true
		}
		out.Lobby.Members[id] = next
	}
	if len(online) > 0 {
		out.Lobby.LastOnline = now
		out.Dirty = true
	} else if out.Lobby.LastOnline.IsZero() {
		out.Lobby.LastOnline = now
		out.Dirty = true
	}

	// Leader grace, automatic transfer, dormancy.
	if online[p.LeaderID] {
		if out.Lobby.LeaderGoneAt != nil {
			out.Lobby.LeaderGoneAt = nil
			out.Dirty = true
		}
	} else {
		if out.Lobby.LeaderGoneAt == nil {
			out.Lobby.LeaderGoneAt = tptr(now)
			out.Dirty = true
		} else if now.Sub(*out.Lobby.LeaderGoneAt) >= cfg.LeaderGrace {
			if next, ok := p.NextOnlineLeader(online); ok {
				out.TransferTo = next
				out.Lobby.LeaderGoneAt = nil
				out.Dirty = true
			}
		}
	}
	if len(online) == 0 && DormantDisbands(p.Type) && cfg.DormantTimeout > 0 && now.Sub(out.Lobby.LastOnline) >= cfg.DormantTimeout {
		out.Disband = true
		return out
	}

	// Running activity: member grace, everybody gone, minimum members.
	if in.Activity == nil || in.Activity.Status != ActivityRunning {
		if out.Lobby.BelowMinSince != nil {
			out.Lobby.BelowMinSince = nil
			out.Dirty = true
		}
		return out
	}
	open, connected, closedByGrace := 0, 0, false
	for _, m := range p.Members {
		id := m.StudentID
		if !in.Open[id] {
			continue
		}
		s := out.Lobby.Members[id]
		if !online[id] && s.GoneAt != nil && now.Sub(*s.GoneAt) >= cfg.MemberGrace {
			out.CloseMember = append(out.CloseMember, id)
			closedByGrace = true
			continue
		}
		open++
		if online[id] {
			connected++
		}
	}
	switch {
	case open == 0:
		out.EndActivity = EndAllLeft
	case closedByGrace && !in.Activity.AllowContinue:
		out.EndActivity = EndTimeout
	case connected < in.Activity.MinMembers:
		if out.Lobby.BelowMinSince == nil {
			out.Lobby.BelowMinSince = tptr(now)
			out.Dirty = true
		} else if now.Sub(*out.Lobby.BelowMinSince) >= cfg.MinMembersTimeout {
			out.EndActivity = EndTimeout
		}
	default:
		if out.Lobby.BelowMinSince != nil {
			out.Lobby.BelowMinSince = nil
			out.Dirty = true
		}
	}
	return out
}
