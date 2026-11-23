package cache

import (
	"context"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
)

// PolicyGens implements port.PolicyGenerations on Valkey counters ew:chat:policygen:platform and
// ew:chat:policygen:<tenant_id> (F120-AC-10, AC-26): raised after the commit of a policy write, read on every effective
// policy lookup so that the cache key changes on every instance at once. Temporary state only (ADR-0004): a lost counter
// starts again at 0 and the cache TTL bounds staleness.
type PolicyGens struct{ C valkey.Client }

var _ port.PolicyGenerations = PolicyGens{}

const policyGenPlatform = "ew:chat:policygen:platform"

func policyGenKey(tenant uuid.UUID) string { return "ew:chat:policygen:" + tenant.String() }

// Generation implements port.PolicyGenerations ("<platform>.<tenant>").
func (g PolicyGens) Generation(ctx context.Context, tenant uuid.UUID) (string, error) {
	vs, err := g.C.Do(ctx, g.C.B().Mget().Key(policyGenPlatform, policyGenKey(tenant)).Build()).ToArray()
	if err != nil {
		return "", unavailable(err)
	}
	out := ""
	for i, v := range vs {
		s, err := v.ToString()
		if err != nil || s == "" {
			s = "0"
		}
		if i > 0 {
			out += "."
		}
		out += s
	}
	return out, nil
}

// Bump implements port.PolicyGenerations (tenant nil: the platform level, every tenant).
func (g PolicyGens) Bump(ctx context.Context, tenant *uuid.UUID) error {
	key := policyGenPlatform
	if tenant != nil {
		key = policyGenKey(*tenant)
	}
	return g.C.Do(ctx, g.C.B().Incr().Key(key).Build()).Error()
}
