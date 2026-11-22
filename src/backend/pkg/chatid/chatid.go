// Package chatid derives the deterministic id of a gameplay chat channel (CHAT_SYSTEM §4.1, F117 research R1): a
// UUIDv5 of a fixed EduWorld namespace and "<kind>:<scope_id>[:<team>]". Module competition fills chat_channel_id of a
// party or match with the same function, so a client gets the id at once, while module chat creates the channel when it
// consumes the party/match event (idempotent upsert on that id). Pure Go: no I/O (ADR-0074, pkg/ platform kit).
package chatid

import (
	"strconv"

	"github.com/google/uuid"
)

// Namespace is the UUIDv5 namespace of gameplay chat channels. Never change it: every stored channel id depends on it.
var Namespace = uuid.MustParse("6f0b6c1e-2d4a-5e8b-9c3f-7a1d2e4b5c60")

// Gameplay channel kinds (channel_type of module chat).
const (
	KindParty = "party"
	KindMatch = "match"
	KindTeam  = "team"
)

// ChannelID returns UUIDv5(Namespace, "<kind>:<scope>[:<team>]"); team is empty except for the team channel of a match.
func ChannelID(kind string, scope uuid.UUID, team string) uuid.UUID {
	name := kind + ":" + scope.String()
	if team != "" {
		name += ":" + team
	}
	return uuid.NewSHA1(Namespace, []byte(name))
}

// Party is the channel of a party (party_id).
func Party(partyID uuid.UUID) uuid.UUID { return ChannelID(KindParty, partyID, "") }

// Match is the channel of every player of a match (match_id).
func Match(matchID uuid.UUID) uuid.UUID { return ChannelID(KindMatch, matchID, "") }

// Team is the channel of one team of a match (team_no of the match events: 0 or 1).
func Team(matchID uuid.UUID, teamNo int) uuid.UUID {
	return ChannelID(KindTeam, matchID, strconv.Itoa(teamNo))
}
