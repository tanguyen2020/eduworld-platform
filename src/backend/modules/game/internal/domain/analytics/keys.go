package analytics

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"

	"github.com/google/uuid"
)

// Key is one version of the secret analytics key (secret store, rotated every analytics_player_key_rotation_months).
type Key struct {
	Version int
	Secret  []byte
}

// KeyRing holds the analytics keys; the highest version is current, older ones are kept to anonymise rows written under
// them (F080-AC-07).
type KeyRing struct {
	keys []Key
}

// ErrNoKey is returned when no analytics key is configured.
var ErrNoKey = errors.New("analytics: no player key configured")

// NewKeyRing validates the keys (version ≥ 1, secret ≥ 32 bytes, unique versions).
func NewKeyRing(keys ...Key) (KeyRing, error) {
	if len(keys) == 0 {
		return KeyRing{}, ErrNoKey
	}
	seen := map[int]bool{}
	for _, k := range keys {
		if k.Version < 1 || len(k.Secret) < 32 || seen[k.Version] {
			return KeyRing{}, errors.New("analytics: player keys need a unique version ≥ 1 and at least 32 bytes")
		}
		seen[k.Version] = true
	}
	ks := append([]Key(nil), keys...)
	sort.Slice(ks, func(i, j int) bool { return ks[i].Version > ks[j].Version })
	return KeyRing{keys: ks}, nil
}

// Current is the version used for new rows.
func (r KeyRing) Current() Key {
	if len(r.keys) == 0 {
		return Key{}
	}
	return r.keys[0]
}

// All returns every key, newest first.
func (r KeyRing) All() []Key { return append([]Key(nil), r.keys...) }

// Pseudonym is HMAC-SHA256(id, secret) in hex (64 chars): player_key of a student id, session_key of a game session id
// (GAME_ANALYTICS A-4). The id is never stored.
func Pseudonym(k Key, id uuid.UUID) string {
	m := hmac.New(sha256.New, k.Secret)
	_, _ = m.Write(id[:])
	return hex.EncodeToString(m.Sum(nil))
}

// clientNamespace derives the event id of a client event from its player and client_event_id (dedup per player).
var clientNamespace = uuid.MustParse("6f1b8c1e-4d1f-5b7a-9a7e-0f80a0a1b2c3")

// ClientEventID is the events_raw event id of a telemetry event: UUIDv5(player_key | client_event_id), so a resent event
// is stored once (F080-AC-01) and a client cannot collide with another player's events.
func ClientEventID(playerKey string, clientEventID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(clientNamespace, []byte(playerKey+"|"+clientEventID.String()))
}

// StudentEventID is the event id of the row of one student of a multi-student domain event (match, boss, party).
func StudentEventID(eventID, student uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(eventID, student[:])
}
