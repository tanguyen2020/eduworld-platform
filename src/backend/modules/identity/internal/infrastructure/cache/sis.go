package cache

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// SIS keys and TTLs (SIS_INTEGRATION §5, research F017 R2). The lock and the nonces are temporary: the run is
// recorded in PostgreSQL and the job in JetStream (AGENTS R9).
const (
	SisLockTTL       = 10 * time.Minute // ew:identity:sis:lock:<connection_id>, heartbeat every TTL/3
	SisWebhookSkew   = 5 * time.Minute  // sis_webhook_skew
	SisWebhookNonce  = 10 * time.Minute // sis_webhook_nonce_ttl
	sisLockPrefix    = prefix + "sis:lock:"
	sisNoncePrefix   = prefix + "sis:wh:nonce:"
	sisLockHeartbeat = SisLockTTL / 3
)

// owner-checked extend and release: a lock taken over after expiry is never touched.
var (
	extendScript  = valkey.NewLuaScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`)
	releaseScript = valkey.NewLuaScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)
)

// SisRunLock implements port.RunLock.
type SisRunLock struct {
	C   valkey.Client
	TTL time.Duration // default SisLockTTL
}

var _ port.RunLock = SisRunLock{}

// Acquire takes the lock (SET NX PX) and keeps it alive until release.
func (l SisRunLock) Acquire(ctx context.Context, connectionID uuid.UUID) (func(), error) {
	ttl := l.TTL
	if ttl <= 0 {
		ttl = SisLockTTL
	}
	key, token := sisLockPrefix+connectionID.String(), ids.New().String()
	err := l.C.Do(ctx, l.C.B().Set().Key(key).Value(token).Nx().Px(ttl).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return nil, port.ErrRunLockBusy
	}
	if err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		t := time.NewTicker(max(ttl/3, time.Millisecond))
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				_ = extendScript.Exec(context.WithoutCancel(ctx), l.C, []string{key}, []string{token, strconv.FormatInt(ttl.Milliseconds(), 10)}).Error()
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(stop)
			_ = releaseScript.Exec(context.WithoutCancel(ctx), l.C, []string{key}, []string{token}).Error()
		})
	}, nil
}

// SisWebhookNonces implements port.WebhookNonces (SET NX PX; fail closed: an error is returned, the
// webhook answers 503).
type SisWebhookNonces struct {
	C   valkey.Client
	TTL time.Duration // default SisWebhookNonce
}

var _ port.WebhookNonces = SisWebhookNonces{}

// Claim records the delivery id; false when it was already received within the TTL.
func (n SisWebhookNonces) Claim(ctx context.Context, deliveryID uuid.UUID) (bool, error) {
	ttl := n.TTL
	if ttl <= 0 {
		ttl = SisWebhookNonce
	}
	err := n.C.Do(ctx, n.C.B().Set().Key(sisNoncePrefix+deliveryID.String()).Value("1").Nx().Px(ttl).Build()).Error()
	if valkey.IsValkeyNil(err) {
		return false, nil
	}
	return err == nil, err
}

// Release deletes a claim.
func (n SisWebhookNonces) Release(ctx context.Context, deliveryID uuid.UUID) error {
	return n.C.Do(ctx, n.C.B().Del().Key(sisNoncePrefix+deliveryID.String()).Build()).Error()
}
