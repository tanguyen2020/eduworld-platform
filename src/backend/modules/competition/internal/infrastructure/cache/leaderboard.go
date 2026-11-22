package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
)

// LeaderboardStore implements port.LeaderboardStore on Valkey sorted sets (VALKEY_DESIGN §3.1, Rebuildable): key
// ew:lb:<tenant_id>:<leaderboard_id>:<scope_key>:<period_key>, member = student_id, score = value·1e7 + tick
// (ties: who reached the value first). The :meta hash marks a board built from the ledgers; the set
// ew:lb:rebuild queues the boards to rebuild.
type LeaderboardStore struct{ C valkey.Client }

var _ port.LeaderboardStore = LeaderboardStore{}

const lbPrefix = "ew:lb:"

// LeaderboardKey is the sorted set of a board period.
func LeaderboardKey(k port.BoardKey) string {
	return lbPrefix + k.Tenant.String() + ":" + k.Board.String() + ":" + k.Scope + ":" + k.Period
}

func metaKey(k port.BoardKey) string { return LeaderboardKey(k) + ":meta" }

const rebuildQueue = lbPrefix + "rebuild"

func encodeKey(k port.BoardKey) string {
	return k.Tenant.String() + "|" + k.Board.String() + "|" + k.Scope + "|" + k.Period
}

func decodeKey(s string) (port.BoardKey, bool) {
	p := strings.Split(s, "|")
	if len(p) != 4 {
		return port.BoardKey{}, false
	}
	t, err1 := uuid.Parse(p[0])
	b, err2 := uuid.Parse(p[1])
	if err1 != nil || err2 != nil || p[2] == "" || p[3] == "" {
		return port.BoardKey{}, false
	}
	return port.BoardKey{Tenant: t, Board: b, Scope: p[2], Period: p[3]}, true
}

func ttlArg(d time.Duration) string { return strconv.FormatInt(max(d.Milliseconds(), 1000), 10) }

func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// incr: KEYS zset, meta; ARGV delta, tick, member, ttl ms. -1 = not built (dropped).
var incrScript = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[2]) == 0 then return -1 end
local cur = redis.call('ZSCORE', KEYS[1], ARGV[3])
local v = 0
if cur then v = math.floor(tonumber(cur) / 10000000) end
v = v + tonumber(ARGV[1])
if v <= 0 then redis.call('ZREM', KEYS[1], ARGV[3]) else
  redis.call('ZADD', KEYS[1], string.format('%.0f', v * 10000000 + tonumber(ARGV[2])), ARGV[3]) end
redis.call('PEXPIRE', KEYS[1], ARGV[4])
redis.call('PEXPIRE', KEYS[2], ARGV[4])
return 1`)

// set: KEYS zset, meta; ARGV value, tick, member, ttl ms.
var setScript = valkey.NewLuaScript(`
if redis.call('EXISTS', KEYS[2]) == 0 then return -1 end
redis.call('ZADD', KEYS[1], string.format('%.0f', tonumber(ARGV[1]) * 10000000 + tonumber(ARGV[2])), ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
redis.call('PEXPIRE', KEYS[2], ARGV[4])
return 1`)

func tickArg(t float64) string { return strconv.FormatInt(int64(t), 10) }

// Incr implements port.LeaderboardStore.
func (s LeaderboardStore) Incr(ctx context.Context, k port.BoardKey, student uuid.UUID, delta, tick float64, ttl time.Duration) (bool, error) {
	n, err := incrScript.Exec(ctx, s.C, []string{LeaderboardKey(k), metaKey(k)}, []string{fnum(delta), tickArg(tick), student.String(), ttlArg(ttl)}).AsInt64()
	if err != nil {
		return false, fmt.Errorf("leaderboard incr: %w", err)
	}
	return n == 1, nil
}

// Set implements port.LeaderboardStore.
func (s LeaderboardStore) Set(ctx context.Context, k port.BoardKey, student uuid.UUID, value, tick float64, ttl time.Duration) (bool, error) {
	n, err := setScript.Exec(ctx, s.C, []string{LeaderboardKey(k), metaKey(k)}, []string{fnum(value), tickArg(tick), student.String(), ttlArg(ttl)}).AsInt64()
	if err != nil {
		return false, fmt.Errorf("leaderboard set: %w", err)
	}
	return n == 1, nil
}

// Remove implements port.LeaderboardStore.
func (s LeaderboardStore) Remove(ctx context.Context, k port.BoardKey, student uuid.UUID) error {
	if err := s.C.Do(ctx, s.C.B().Zrem().Key(LeaderboardKey(k)).Member(student.String()).Build()).Error(); err != nil {
		return fmt.Errorf("leaderboard remove: %w", err)
	}
	return nil
}

// Built implements port.LeaderboardStore.
func (s LeaderboardStore) Built(ctx context.Context, k port.BoardKey) (bool, error) {
	n, err := s.C.Do(ctx, s.C.B().Exists().Key(metaKey(k)).Build()).AsInt64()
	if err != nil {
		return false, fmt.Errorf("leaderboard built: %w", err)
	}
	return n == 1, nil
}

// Count implements port.LeaderboardStore.
func (s LeaderboardStore) Count(ctx context.Context, k port.BoardKey) (int, error) {
	n, err := s.C.Do(ctx, s.C.B().Zcard().Key(LeaderboardKey(k)).Build()).AsInt64()
	if err != nil {
		return 0, fmt.Errorf("leaderboard count: %w", err)
	}
	return int(n), nil
}

// Range implements port.LeaderboardStore.
func (s LeaderboardStore) Range(ctx context.Context, k port.BoardKey, from, to int) ([]ldb.Ranked, error) {
	from = max(from, 1)
	if to < from {
		return nil, nil
	}
	zs, err := s.C.Do(ctx, s.C.B().Zrange().Key(LeaderboardKey(k)).Min(strconv.Itoa(from-1)).Max(strconv.Itoa(to-1)).Rev().Withscores().Build()).AsZScores()
	if err != nil {
		return nil, fmt.Errorf("leaderboard range: %w", err)
	}
	out := make([]ldb.Ranked, 0, len(zs))
	for i, z := range zs {
		id, err := uuid.Parse(z.Member)
		if err != nil {
			continue
		}
		out = append(out, ldb.Ranked{Student: id, Rank: from + i, Value: ldb.ValueOf(z.Score)})
	}
	return out, nil
}

// RankOf implements port.LeaderboardStore.
func (s LeaderboardStore) RankOf(ctx context.Context, k port.BoardKey, student uuid.UUID) (*ldb.Ranked, error) {
	key := LeaderboardKey(k)
	rank, err := s.C.Do(ctx, s.C.B().Zrevrank().Key(key).Member(student.String()).Build()).AsInt64()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leaderboard rank: %w", err)
	}
	score, err := s.C.Do(ctx, s.C.B().Zscore().Key(key).Member(student.String()).Build()).AsFloat64()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leaderboard score: %w", err)
	}
	return &ldb.Ranked{Student: student, Rank: int(rank) + 1, Value: ldb.ValueOf(score)}, nil
}

// Scores implements port.LeaderboardStore (ZMSCORE: the friends of a student, ≤ friends_max).
func (s LeaderboardStore) Scores(ctx context.Context, k port.BoardKey, students []uuid.UUID) (map[uuid.UUID]port.BoardMember, error) {
	out := map[uuid.UUID]port.BoardMember{}
	if len(students) == 0 {
		return out, nil
	}
	members := make([]string, len(students))
	for i, id := range students {
		members[i] = id.String()
	}
	vals, err := s.C.Do(ctx, s.C.B().Zmscore().Key(LeaderboardKey(k)).Member(members...).Build()).ToArray()
	if err != nil {
		return nil, fmt.Errorf("leaderboard scores: %w", err)
	}
	for i, v := range vals {
		sc, err := v.AsFloat64()
		if err != nil {
			continue
		}
		out[students[i]] = port.BoardMember{Student: students[i], Value: ldb.ValueOf(sc), Tick: sc - ldb.ValueOf(sc)*ldb.TickSpan}
	}
	return out, nil
}

// Members implements port.LeaderboardStore.
func (s LeaderboardStore) Members(ctx context.Context, k port.BoardKey) ([]port.BoardMember, error) {
	zs, err := s.C.Do(ctx, s.C.B().Zrange().Key(LeaderboardKey(k)).Min("0").Max("-1").Rev().Withscores().Build()).AsZScores()
	if err != nil {
		return nil, fmt.Errorf("leaderboard members: %w", err)
	}
	out := make([]port.BoardMember, 0, len(zs))
	for _, z := range zs {
		id, err := uuid.Parse(z.Member)
		if err != nil {
			continue
		}
		v := ldb.ValueOf(z.Score)
		out = append(out, port.BoardMember{Student: id, Value: v, Tick: z.Score - v*ldb.TickSpan})
	}
	return out, nil
}

// Replace implements port.LeaderboardStore: the members go into a temporary set renamed over the board, then the meta
// hash marks it built (readers never see a half-built board).
func (s LeaderboardStore) Replace(ctx context.Context, k port.BoardKey, members []port.BoardMember, ttl time.Duration) error {
	key := LeaderboardKey(k)
	tmp := key + ":tmp"
	cmds := valkey.Commands{s.C.B().Del().Key(tmp).Build()}
	for i := 0; i < len(members); i += 500 {
		z := s.C.B().Zadd().Key(tmp).ScoreMember()
		for _, m := range members[i:min(i+500, len(members))] {
			z = z.ScoreMember(ldb.Score(m.Value, float64(int64(m.Tick))), m.Student.String())
		}
		cmds = append(cmds, z.Build())
	}
	if len(members) > 0 {
		cmds = append(cmds, s.C.B().Rename().Key(tmp).Newkey(key).Build(), s.C.B().Pexpire().Key(key).Milliseconds(ttl.Milliseconds()).Build())
	} else {
		cmds = append(cmds, s.C.B().Del().Key(key).Build())
	}
	cmds = append(cmds, s.C.B().Hset().Key(metaKey(k)).FieldValue().FieldValue("rebuilt_at", time.Now().UTC().Format(time.RFC3339)).
		FieldValue("members", strconv.Itoa(len(members))).Build(), s.C.B().Pexpire().Key(metaKey(k)).Milliseconds(ttl.Milliseconds()).Build())
	for _, r := range s.C.DoMulti(ctx, cmds...) {
		if err := r.Error(); err != nil {
			return fmt.Errorf("leaderboard replace: %w", err)
		}
	}
	return nil
}

// QueueRebuild implements port.LeaderboardStore.
func (s LeaderboardStore) QueueRebuild(ctx context.Context, k port.BoardKey) error {
	if err := s.C.Do(ctx, s.C.B().Sadd().Key(rebuildQueue).Member(encodeKey(k)).Build()).Error(); err != nil {
		return fmt.Errorf("leaderboard queue rebuild: %w", err)
	}
	return nil
}

// PopRebuilds implements port.LeaderboardStore.
func (s LeaderboardStore) PopRebuilds(ctx context.Context, n int) ([]port.BoardKey, error) {
	vals, err := s.C.Do(ctx, s.C.B().Spop().Key(rebuildQueue).Count(int64(n)).Build()).AsStrSlice()
	if valkey.IsValkeyNil(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leaderboard pop rebuilds: %w", err)
	}
	out := make([]port.BoardKey, 0, len(vals))
	for _, v := range vals {
		if k, ok := decodeKey(v); ok {
			out = append(out, k)
		}
	}
	return out, nil
}

// BuiltBoards implements port.LeaderboardStore (SCAN of the meta keys).
func (s LeaderboardStore) BuiltBoards(ctx context.Context, n int) ([]port.BoardKey, error) {
	var out []port.BoardKey
	cursor := uint64(0)
	for {
		e, err := s.C.Do(ctx, s.C.B().Scan().Cursor(cursor).Match(lbPrefix+"*:meta").Count(500).Build()).AsScanEntry()
		if err != nil {
			return nil, fmt.Errorf("leaderboard scan: %w", err)
		}
		for _, key := range e.Elements {
			rest := strings.TrimSuffix(strings.TrimPrefix(key, lbPrefix), ":meta")
			// <tenant>:<board>:<scope…>:<period>: the scope key may hold ':' (school:gN)
			p := strings.Split(rest, ":")
			if len(p) < 4 {
				continue
			}
			t, err1 := uuid.Parse(p[0])
			b, err2 := uuid.Parse(p[1])
			if err1 != nil || err2 != nil {
				continue
			}
			out = append(out, port.BoardKey{Tenant: t, Board: b, Scope: strings.Join(p[2:len(p)-1], ":"), Period: p[len(p)-1]})
			if len(out) >= n {
				return out, nil
			}
		}
		if e.Cursor == 0 {
			return out, nil
		}
		cursor = e.Cursor
	}
}
