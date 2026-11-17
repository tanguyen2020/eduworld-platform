package authz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// cacheVersion is bumped whenever the cached JSON shape changes; other versions are read as a miss.
const cacheVersion = 1

// CacheOptions configures CachedResolver (principal_cache, principal_cache_ttl — CONFIG_DEFAULTS).
type CacheOptions struct {
	Module     string        // key prefix ew:<module>: (Valkey ACL per module, VALKEY_DESIGN §1)
	TTL        time.Duration // principal_cache_ttl (Valkey)
	L1TTL      time.Duration // in-process level (principal_cache: L1 15 s)
	L1Size     int
	Valkey     valkey.Client // nil = L1 only
	Clock      clock.Clock   // nil = system clock
	Registerer prometheus.Registerer
}

type l1Entry struct {
	data    PrincipalData
	expires time.Time
}

// CachedResolver wraps the identity resolver with an in-process LRU and Valkey (F002-AC-09, F002-AC-10).
// Valkey keys: ew:<module>:principal:<user_id>:<tenant_id|platform> (JSON of the stable principal data)
// plus ew:<module>:principal:sub:<hash(iss,sub)> → user_id, because tokens carry no EduWorld user_id, and
// ew:<module>:principal:idx:<user_id> (SET of tenant parts) for user-wide invalidation.
type CachedResolver struct {
	next   PrincipalResolver
	o      CacheOptions
	l1     *lru.Cache[string, l1Entry]
	mu     sync.Mutex
	hits   *prometheus.CounterVec
	prefix string
}

var moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// NewCachedResolver builds the cache.
func NewCachedResolver(next PrincipalResolver, o CacheOptions) (*CachedResolver, error) {
	if next == nil || !moduleNameRe.MatchString(o.Module) || o.TTL <= 0 || o.L1TTL <= 0 || o.L1Size <= 0 {
		return nil, errors.New("authz: cache needs a resolver, a module name, TTL, L1TTL and L1Size")
	}
	if o.Clock == nil {
		o.Clock = clock.System()
	}
	l1, err := lru.New[string, l1Entry](o.L1Size)
	if err != nil {
		return nil, fmt.Errorf("authz: l1: %w", err)
	}
	c := &CachedResolver{next: next, o: o, l1: l1, prefix: "ew:" + o.Module + ":principal:"}
	c.hits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eduworld_principal_cache_hits_total",
		Help: "Principal cache lookups by layer (l1, valkey) and result (hit, miss, error).",
	}, []string{"layer", "result"})
	if o.Registerer != nil {
		if err := o.Registerer.Register(c.hits); err != nil {
			var are prometheus.AlreadyRegisteredError
			if !errors.As(err, &are) {
				return nil, fmt.Errorf("authz: register metric: %w", err)
			}
			c.hits = are.ExistingCollector.(*prometheus.CounterVec)
		}
	}
	return c, nil
}

func tenantPart(t *uuid.UUID) string {
	if t == nil {
		return "platform"
	}
	return t.String()
}

func subjectHash(s SubjectRef) string {
	h := sha256.Sum256([]byte(s.Issuer + "\x00" + s.Subject))
	return hex.EncodeToString(h[:16])
}

// Resolve returns the principal from L1, then Valkey, then the identity resolver. Resolver errors other
// than ErrNotMember become 503 UNAVAILABLE (retryable): the request is never let through (fail closed).
// Per-request fields (ClientID; SessionID, Channel and MFA are set by httpx) are never cached.
func (c *CachedResolver) Resolve(ctx context.Context, req ResolveRequest) (Principal, error) {
	sh := subjectHash(req.Subject)
	tp := tenantPart(req.TenantID)
	l1Key := sh + ":" + tp
	now := c.o.Clock.Now()

	if e, ok := c.l1.Get(l1Key); ok && now.Before(e.expires) {
		c.hits.WithLabelValues("l1", "hit").Inc()
		return c.finish(e.data, req), nil
	}
	c.hits.WithLabelValues("l1", "miss").Inc()

	if c.o.Valkey != nil {
		if d, ok := c.readValkey(ctx, sh, tp); ok {
			c.l1.Add(l1Key, l1Entry{data: d, expires: now.Add(c.o.L1TTL)})
			return c.finish(d, req), nil
		}
	}

	p, err := c.next.Resolve(ctx, req)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			return Principal{}, err
		}
		return Principal{}, problem.New(problem.Unavailable, "identity unavailable").Retry().Wrap(err)
	}
	d := p.PrincipalData
	d.ClientID, d.SessionID, d.Channel, d.MFA = "", "", ChannelUnknown, false
	c.l1.Add(l1Key, l1Entry{data: d, expires: now.Add(c.o.L1TTL)})
	if c.o.Valkey != nil {
		c.writeValkey(ctx, sh, tp, d)
	}
	return c.finish(d, req), nil
}

func (c *CachedResolver) finish(d PrincipalData, req ResolveRequest) Principal {
	d.ClientID = req.ClientID
	return NewPrincipal(d)
}

type envelope struct {
	V int           `json:"v"`
	P PrincipalData `json:"p"`
}

func (c *CachedResolver) readValkey(ctx context.Context, sh, tp string) (PrincipalData, bool) {
	vk := c.o.Valkey
	uid, err := vk.Do(ctx, vk.B().Get().Key(c.prefix+"sub:"+sh).Build()).ToString()
	if err != nil {
		c.valkeyMiss(err)
		return PrincipalData{}, false
	}
	raw, err := vk.Do(ctx, vk.B().Get().Key(c.prefix+uid+":"+tp).Build()).AsBytes()
	if err != nil {
		c.valkeyMiss(err)
		return PrincipalData{}, false
	}
	var e envelope
	if json.Unmarshal(raw, &e) != nil || e.V != cacheVersion || e.P.UserID.String() != uid {
		c.hits.WithLabelValues("valkey", "miss").Inc()
		return PrincipalData{}, false
	}
	c.hits.WithLabelValues("valkey", "hit").Inc()
	return e.P, true
}

func (c *CachedResolver) valkeyMiss(err error) {
	if valkey.IsValkeyNil(err) {
		c.hits.WithLabelValues("valkey", "miss").Inc()
		return
	}
	c.hits.WithLabelValues("valkey", "error").Inc()
}

// writeValkey is best effort: a Valkey error only costs a later miss (R9: Valkey is never the source).
func (c *CachedResolver) writeValkey(ctx context.Context, sh, tp string, d PrincipalData) {
	raw, err := json.Marshal(envelope{V: cacheVersion, P: d})
	if err != nil {
		return
	}
	vk := c.o.Valkey
	uid := d.UserID.String()
	idx := c.prefix + "idx:" + uid
	cmds := valkey.Commands{
		vk.B().Set().Key(c.prefix + "sub:" + sh).Value(uid).Ex(c.o.TTL).Build(),
		vk.B().Set().Key(c.prefix + uid + ":" + tp).Value(valkey.BinaryString(raw)).Ex(c.o.TTL).Build(),
		vk.B().Sadd().Key(idx).Member(tp).Build(),
		vk.B().Expire().Key(idx).Seconds(int64(c.o.TTL/time.Second) + 1).Build(),
	}
	for _, r := range vk.DoMulti(ctx, cmds...) {
		if r.Error() != nil {
			c.hits.WithLabelValues("valkey", "error").Inc()
			return
		}
	}
}

// Invalidate removes the principal of userID in tenantID (nil = every tenant and platform) from L1 and
// Valkey. Consumers of user_role.changed, role.updated, tenant_membership.changed, user.access_revoked
// and tenant.updated call it on every replica; other replicas' L1 converges within L1TTL.
func (c *CachedResolver) Invalidate(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) error {
	c.mu.Lock()
	for _, k := range c.l1.Keys() {
		if e, ok := c.l1.Peek(k); ok && e.data.UserID == userID && (tenantID == nil || tenantPart(e.data.TenantID) == tenantPart(tenantID)) {
			c.l1.Remove(k)
		}
	}
	c.mu.Unlock()
	if c.o.Valkey == nil {
		return nil
	}
	vk := c.o.Valkey
	uid := userID.String()
	idx := c.prefix + "idx:" + uid
	if tenantID != nil {
		tp := tenantPart(tenantID)
		for _, r := range vk.DoMulti(ctx, vk.B().Del().Key(c.prefix+uid+":"+tp).Build(), vk.B().Srem().Key(idx).Member(tp).Build()) {
			if err := r.Error(); err != nil {
				return fmt.Errorf("authz: invalidate principal: %w", err)
			}
		}
		return nil
	}
	parts, err := vk.Do(ctx, vk.B().Smembers().Key(idx).Build()).AsStrSlice()
	if err != nil && !valkey.IsValkeyNil(err) {
		return fmt.Errorf("authz: invalidate principal: %w", err)
	}
	keys := []string{idx}
	for _, tp := range parts {
		keys = append(keys, c.prefix+uid+":"+tp)
	}
	if err := vk.Do(ctx, vk.B().Del().Key(keys...).Build()).Error(); err != nil {
		return fmt.Errorf("authz: invalidate principal: %w", err)
	}
	return nil
}
