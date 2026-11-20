// Package resume keeps the resume buffer of a logical connection and the results of processed commands in Valkey
// (F090 US2, research R3), so a client resumes on any node (no sticky session, NFR-SCAL-003):
//   - ew:gateway:rt:resume:<sha256(token)>       LIST "<seq>|<server envelope>" of non-droppable messages (≤ max)
//   - ew:gateway:rt:resume:<sha256(token)>:meta  HASH {user_id, game_session_id, last_seq, trimmed_seq, subs}
//   - ew:gateway:rt:cmd:<user_id>:<cmd_id>        STRING result of system.command.acked/failed (or "pending")
//
// Every key lives resume_window. The token is stored hashed; the buffer holds server envelopes only (ids, no PII).
package resume

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"
)

// Store is the resume buffer.
type Store struct {
	C      valkey.Client
	Window time.Duration // resume_window
	Max    int           // messages kept per logical connection
}

// Meta describes a logical connection.
type Meta struct {
	UserID        uuid.UUID
	GameSessionID uuid.UUID
	LastSeq       int64    // highest buffered seq
	TrimmedSeq    int64    // highest seq dropped from the buffer by the size limit (0 = none)
	Subs          []string // topics granted by modules (≤ 64)
}

// Entry is one buffered message.
type Entry struct {
	Seq  int64
	Data []byte
}

func hashed(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func listKey(token string) string { return "ew:gateway:rt:resume:" + hashed(token) }
func metaKey(token string) string { return listKey(token) + ":meta" }
func cmdKey(user uuid.UUID, id string) string {
	return "ew:gateway:rt:cmd:" + user.String() + ":" + id
}

// NewToken returns "<issued ms base36>.<256 random bits>": the time lets a missing buffer be told expired from
// unknown (F090-AC-15) without storing anything else.
func NewToken(now time.Time) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strconv.FormatInt(now.UnixMilli(), 36) + "." + base64.RawURLEncoding.EncodeToString(b), nil
}

// IssuedAt returns the issue time encoded in a token (false when malformed).
func IssuedAt(token string) (time.Time, bool) {
	ts, rnd, ok := strings.Cut(token, ".")
	if !ok || len(rnd) != 43 {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(ts, 36, 64)
	if err != nil || ms <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(ms), true
}

func (s Store) ttl() int64 { return int64(s.Window/time.Second) + 1 }

// Open creates the meta of a new logical connection.
func (s Store) Open(ctx context.Context, token string, m Meta) error {
	k := metaKey(token)
	for _, r := range s.C.DoMulti(ctx,
		s.C.B().Hset().Key(k).FieldValue().FieldValue("user_id", m.UserID.String()).
			FieldValue("game_session_id", m.GameSessionID.String()).FieldValue("last_seq", "0").
			FieldValue("trimmed_seq", "0").FieldValue("subs", "[]").Build(),
		s.C.B().Expire().Key(k).Seconds(s.ttl()).Build()) {
		if err := r.Error(); err != nil {
			return fmt.Errorf("resume: open: %w", err)
		}
	}
	return nil
}

var appendScript = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[2]) == 0 then return 0 end
redis.call('RPUSH', KEYS[1], ARGV[1] .. '|' .. ARGV[2])
local last = tonumber(redis.call('HGET', KEYS[2], 'last_seq') or '0')
if tonumber(ARGV[1]) > last then redis.call('HSET', KEYS[2], 'last_seq', ARGV[1]) end
while redis.call('LLEN', KEYS[1]) > tonumber(ARGV[3]) do
  local v = redis.call('LPOP', KEYS[1])
  local seq = string.match(v, '^(%d+)|')
  if seq then redis.call('HSET', KEYS[2], 'trimmed_seq', seq) end
end
redis.call('EXPIRE', KEYS[1], ARGV[4])
redis.call('EXPIRE', KEYS[2], ARGV[4])
return 1`)

// Append buffers one non-droppable server envelope (no-op once the buffer was deleted).
func (s Store) Append(ctx context.Context, token string, seq int64, data []byte) error {
	return appendScript.Exec(ctx, s.C, []string{listKey(token), metaKey(token)},
		[]string{strconv.FormatInt(seq, 10), string(data), strconv.Itoa(s.Max), strconv.FormatInt(s.ttl(), 10)}).Error()
}

// SaveSubs records the topics of the logical connection (restored on resume).
func (s Store) SaveSubs(ctx context.Context, token string, subs []string) error {
	sorted := append([]string(nil), subs...)
	sort.Strings(sorted)
	b, _ := json.Marshal(sorted)
	return s.C.Do(ctx, s.C.B().Hset().Key(metaKey(token)).FieldValue().FieldValue("subs", string(b)).Build()).Error()
}

// ErrNotFound means the buffer does not exist (expired, deleted, never issued).
var ErrNotFound = errors.New("resume: buffer not found")

// Load returns the meta and the buffered messages of token.
func (s Store) Load(ctx context.Context, token string) (Meta, []Entry, error) {
	res := s.C.DoMulti(ctx, s.C.B().Hgetall().Key(metaKey(token)).Build(), s.C.B().Lrange().Key(listKey(token)).Start(0).Stop(-1).Build())
	m, err := res[0].AsStrMap()
	if err != nil {
		return Meta{}, nil, err
	}
	if len(m) == 0 {
		return Meta{}, nil, ErrNotFound
	}
	items, err := res[1].AsStrSlice()
	if err != nil && !valkey.IsValkeyNil(err) {
		return Meta{}, nil, err
	}
	var meta Meta
	meta.UserID, _ = uuid.Parse(m["user_id"])
	meta.GameSessionID, _ = uuid.Parse(m["game_session_id"])
	meta.LastSeq, _ = strconv.ParseInt(m["last_seq"], 10, 64)
	meta.TrimmedSeq, _ = strconv.ParseInt(m["trimmed_seq"], 10, 64)
	_ = json.Unmarshal([]byte(m["subs"]), &meta.Subs)
	entries := make([]Entry, 0, len(items))
	for _, it := range items {
		seq, data, ok := strings.Cut(it, "|")
		n, err := strconv.ParseInt(seq, 10, 64)
		if !ok || err != nil {
			continue
		}
		entries = append(entries, Entry{Seq: n, Data: []byte(data)})
	}
	return meta, entries, nil
}

// Delete invalidates token (revocation, replacement by another device, session ended — F090-AC-24, AC-25, AC-32).
func (s Store) Delete(ctx context.Context, token string) error {
	return s.C.Do(ctx, s.C.B().Del().Key(listKey(token), metaKey(token)).Build()).Error()
}

const pending = "pending"

// Reserve marks command id of user as in flight. When the id was already seen in resume_window it returns the
// stored result (nil while the first attempt is still in flight) and false (F090-AC-16).
func (s Store) Reserve(ctx context.Context, user uuid.UUID, id string) (prev []byte, reserved bool, err error) {
	k := cmdKey(user, id)
	err = s.C.Do(ctx, s.C.B().Set().Key(k).Value(pending).Nx().ExSeconds(s.ttl()).Build()).Error()
	if err == nil {
		return nil, true, nil
	}
	if !valkey.IsValkeyNil(err) {
		return nil, false, err
	}
	v, err := s.C.Do(ctx, s.C.B().Get().Key(k).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if v == pending {
		return nil, false, nil
	}
	return []byte(v), false, nil
}

// Complete stores the result of command id (its type and payload, replayed for a duplicate).
func (s Store) Complete(ctx context.Context, user uuid.UUID, id string, result []byte) error {
	return s.C.Do(ctx, s.C.B().Set().Key(cmdKey(user, id)).Value(string(result)).ExSeconds(s.ttl()).Build()).Error()
}

// Release forgets a reservation that produced no result (the client may retry).
func (s Store) Release(ctx context.Context, user uuid.UUID, id string) error {
	return s.C.Do(ctx, s.C.B().Del().Key(cmdKey(user, id)).Build()).Error()
}
