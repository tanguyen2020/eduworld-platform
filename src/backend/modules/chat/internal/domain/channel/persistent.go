package channel

import (
	"bytes"
	"strings"

	"github.com/google/uuid"
)

// persistentNamespace is the UUIDv5 namespace of the persistent channels of F120 (world, area, direct, club team). Never
// change it: every stored channel id depends on it. Gameplay channels use pkg/chatid (another namespace).
var persistentNamespace = uuid.MustParse("8b1d5a0e-3c4f-5a6b-8d7e-1f2a3b4c5d6e")

func persistentID(parts ...string) uuid.UUID {
	var b bytes.Buffer
	for i, p := range parts {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(p)
	}
	return uuid.NewSHA1(persistentNamespace, b.Bytes())
}

// Pair orders two students (direct channels are per normalised pair, F120-AC-14).
func Pair(a, b uuid.UUID) (low, high uuid.UUID) {
	if strings.Compare(a.String(), b.String()) <= 0 { // same order as social.NewPair
		return a, b
	}
	return b, a
}

// DirectID is the id of the direct channel of a pair in a tenant (same id whoever opens it).
func DirectID(tenant, a, b uuid.UUID) uuid.UUID {
	low, high := Pair(a, b)
	return persistentID("direct", tenant.String(), low.String(), high.String())
}

// DirectScope is scope_ref of a direct channel.
func DirectScope(a, b uuid.UUID) map[string]string {
	low, high := Pair(a, b)
	return map[string]string{"user_low_id": low.String(), "user_high_id": high.String()}
}

// WorldID is the id of the world channel of a school (scope "school") or of the whole tenant (world_scope tenant, tenant
// unit only — ADR-0054).
func WorldID(tenant uuid.UUID, school *uuid.UUID) uuid.UUID {
	if school == nil {
		return persistentID("world", tenant.String(), "tenant")
	}
	return persistentID("world", tenant.String(), school.String())
}

// WorldScope is scope_ref of a world channel.
func WorldScope(tenant uuid.UUID, school *uuid.UUID) map[string]string {
	if school == nil {
		return map[string]string{"tenant_id": tenant.String()}
	}
	return map[string]string{"school_id": school.String()}
}

// AreaID is the id of the area channel of one hub instance (room_id, created lazily — F120-AC-13).
func AreaID(tenant uuid.UUID, room string) uuid.UUID {
	return persistentID("area", tenant.String(), room)
}

// AreaScope is scope_ref of an area channel.
func AreaScope(room string) map[string]string { return map[string]string{"room_id": room} }

// ClubID is the id of the fixed team channel of a club (scope_ref {club_id}, is_gameplay false — F120-AC-18).
func ClubID(tenant, club uuid.UUID) uuid.UUID {
	return persistentID("club", tenant.String(), club.String())
}

// ClubScope is scope_ref of the team channel of a club.
func ClubScope(club uuid.UUID) map[string]string { return map[string]string{"club_id": club.String()} }

// DirectOf reports whether user is one of the pair of a direct channel.
func (c Channel) DirectOf(user uuid.UUID) bool {
	return c.Type == Direct && (c.ScopeRef["user_low_id"] == user.String() || c.ScopeRef["user_high_id"] == user.String())
}

// Other returns the other student of a direct channel.
func (c Channel) Other(user uuid.UUID) (uuid.UUID, bool) {
	if !c.DirectOf(user) {
		return uuid.Nil, false
	}
	other := c.ScopeRef["user_high_id"]
	if other == user.String() {
		other = c.ScopeRef["user_low_id"]
	}
	id, err := uuid.Parse(other)
	return id, err == nil
}
