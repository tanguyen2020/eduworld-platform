// Package registry keeps one effective connection per student across gateway nodes (F090-AC-08, AC-28, ADR-0031)
// and the liveness of nodes (VALKEY_DESIGN §3.1, prefix ew:gateway:): ew:gateway:rt:conn:<user_id> HASH {node,
// conn_id, game_session_id, since} written by a Lua script that returns the previous holder, so the later
// registration always wins and the earlier connection is closed with 4409.
package registry

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"
)

// Keys of the gateway (key builder of module realtime: real prefix ew:gateway:).
func connKey(user uuid.UUID) string { return "ew:gateway:rt:conn:" + user.String() }
func aliveKey(node string) string   { return "ew:gateway:rt:node:" + node + ":alive" }
func connsKey(node string) string   { return "ew:gateway:rt:node:" + node + ":conns" }

// NodesKey is the ZSET node → number of connections.
const NodesKey = "ew:gateway:rt:nodes"

// Entry is the holder of a student's connection.
type Entry struct {
	Node          string
	ConnID        string
	GameSessionID string
}

var (
	registerScript = valkey.NewLuaScript(`
local old = redis.call('HMGET', KEYS[1], 'node', 'conn_id', 'game_session_id')
redis.call('DEL', KEYS[1])
redis.call('HSET', KEYS[1], 'node', ARGV[1], 'conn_id', ARGV[2], 'game_session_id', ARGV[3], 'since', ARGV[4])
redis.call('EXPIRE', KEYS[1], ARGV[5])
return old`)
	refreshScript = valkey.NewLuaScript(`
if redis.call('HGET', KEYS[1], 'conn_id') == ARGV[1] then return redis.call('EXPIRE', KEYS[1], ARGV[2]) end
return 0`)
	unregisterScript = valkey.NewLuaScript(`
if redis.call('HGET', KEYS[1], 'conn_id') == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0`)
)

// Registry is the Valkey connection registry.
type Registry struct {
	C   valkey.Client
	TTL time.Duration // presence_ttl
}

func (r Registry) ttl() string { return strconv.FormatInt(int64(r.TTL/time.Second), 10) }

// Register records conn as the connection of user and returns the previous holder (nil when none).
func (r Registry) Register(ctx context.Context, user uuid.UUID, e Entry, since time.Time) (*Entry, error) {
	vals, err := registerScript.Exec(ctx, r.C, []string{connKey(user)},
		[]string{e.Node, e.ConnID, e.GameSessionID, strconv.FormatInt(since.UnixMilli(), 10), r.ttl()}).ToArray()
	if err != nil {
		return nil, fmt.Errorf("registry: register: %w", err)
	}
	s := make([]string, 3)
	for i := 0; i < 3 && i < len(vals); i++ {
		s[i], _ = vals[i].ToString()
	}
	if s[0] == "" || s[1] == "" {
		return nil, nil
	}
	return &Entry{Node: s[0], ConnID: s[1], GameSessionID: s[2]}, nil
}

// Refresh extends the TTL while conn still holds user (heartbeat).
func (r Registry) Refresh(ctx context.Context, user uuid.UUID, conn string) error {
	return refreshScript.Exec(ctx, r.C, []string{connKey(user)}, []string{conn, r.ttl()}).Error()
}

// Unregister removes the entry of user only if conn still holds it.
func (r Registry) Unregister(ctx context.Context, user uuid.UUID, conn string) error {
	return unregisterScript.Exec(ctx, r.C, []string{connKey(user)}, []string{conn}).Error()
}

// Holder returns the current holder of user (tests, diagnostics).
func (r Registry) Holder(ctx context.Context, user uuid.UUID) (*Entry, error) {
	m, err := r.C.Do(ctx, r.C.B().Hgetall().Key(connKey(user)).Build()).AsStrMap()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	return &Entry{Node: m["node"], ConnID: m["conn_id"], GameSessionID: m["game_session_id"]}, nil
}

// NodeAlive marks node alive for 10 s with its connection ids and count (renewed every 3 s by the manager).
func (r Registry) NodeAlive(ctx context.Context, node string, conns []string) error {
	cmds := valkey.Commands{
		r.C.B().Set().Key(aliveKey(node)).Value(strconv.Itoa(len(conns))).ExSeconds(10).Build(),
		r.C.B().Zadd().Key(NodesKey).ScoreMember().ScoreMember(float64(len(conns)), node).Build(),
		r.C.B().Del().Key(connsKey(node)).Build(),
	}
	if len(conns) > 0 {
		cmds = append(cmds, r.C.B().Sadd().Key(connsKey(node)).Member(conns...).Build(),
			r.C.B().Expire().Key(connsKey(node)).Seconds(3600).Build())
	}
	for _, res := range r.C.DoMulti(ctx, cmds...) {
		if err := res.Error(); err != nil {
			return fmt.Errorf("registry: node alive: %w", err)
		}
	}
	return nil
}

// NodeGone removes node on graceful shutdown.
func (r Registry) NodeGone(ctx context.Context, node string) error {
	for _, res := range r.C.DoMulti(ctx, r.C.B().Del().Key(aliveKey(node), connsKey(node)).Build(),
		r.C.B().Zrem().Key(NodesKey).Member(node).Build()) {
		if err := res.Error(); err != nil {
			return err
		}
	}
	return nil
}
