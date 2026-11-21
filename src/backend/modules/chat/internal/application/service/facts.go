// Package service holds the application services shared by the commands and queries of the chat core (F115): the
// cached facts of the send path — channel and members, effective policy (PolicyResolver, T025), curfew, seq floor.
// Reads run in entx.ReadSession through the read ports; nothing here writes the database.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ReadRunner runs fn in a READ ONLY transaction with an explicit scope (entx.ReadSession.InTenantRead).
type ReadRunner interface {
	InTenantRead(ctx context.Context, s tenancy.Scope, fn func(ctx context.Context) error) error
}

// FactsConfig are the freshness of the cached facts (chat_policy_cache, channel facts, curfew) and the platform curfew
// used until identity projects platform_curfew.
type FactsConfig struct {
	FactsFresh     time.Duration
	PolicyCache    time.Duration
	CurfewFresh    time.Duration
	PlatformCurfew policy.Window
	CurfewLocation *time.Location
}

// Facts loads the facts of the send path through the cache: a stale value is served while PostgreSQL is down
// (F115-AC-16); without any value the caller fails with port.ErrUnavailable (fail closed).
type Facts struct {
	Read   ReadRunner
	Reader read.ChatReader
	Cache  port.Cache
	Recent port.RecentBuffer
	Config FactsConfig
}

// Cache keys (logical; the Valkey adapter prefixes ew:chat:).
func channelKey(tenant, id uuid.UUID) string { return "chan:" + tenant.String() + ":" + id.String() }

func policyKey(tenant uuid.UUID, school *uuid.UUID, t channel.Type) string {
	return "policy:" + tenant.String() + ":" + idOr(school) + ":" + string(t)
}

func curfewKey(tenant uuid.UUID, school *uuid.UUID) string {
	return "curfew:" + tenant.String() + ":" + idOr(school)
}

func idOr(id *uuid.UUID) string {
	if id == nil {
		return "-"
	}
	return id.String()
}

func unavailable(err error) error { return fmt.Errorf("%w: %w", port.ErrUnavailable, err) }

func (f Facts) read(ctx context.Context, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	s, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return f.Read.InTenantRead(ctx, s, fn)
}

// Channel returns the facts of a channel of tenant (nil: unknown in the tenant or deleted — never another tenant's:
// the query filters tenant_id and RLS hides the rest, F115-AC-11).
func (f Facts) Channel(ctx context.Context, tenant, id uuid.UUID) (*read.ChannelFacts, error) {
	raw, err := f.Cache.Get(ctx, channelKey(tenant, id), f.Config.FactsFresh, func(ctx context.Context) ([]byte, error) {
		var facts *read.ChannelFacts
		err := f.read(ctx, tenant, func(ctx context.Context) error {
			var err error
			facts, err = f.Reader.ChannelFacts(ctx, tenant, id)
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(facts)
	})
	if err != nil {
		return nil, unavailable(err)
	}
	var facts *read.ChannelFacts
	if err := json.Unmarshal(raw, &facts); err != nil {
		return nil, unavailable(err)
	}
	if facts != nil && facts.Channel.TenantID != tenant { // defence in depth: never serve another tenant's channel
		return nil, nil
	}
	return facts, nil
}

// InvalidateChannel drops the cached facts of a channel (membership changed).
func (f Facts) InvalidateChannel(ctx context.Context, tenant, id uuid.UUID) error {
	return f.Cache.Invalidate(ctx, channelKey(tenant, id))
}

type effectiveEntry struct {
	Denied    bool              `json:"denied"`
	Effective *policy.Effective `json:"effective,omitempty"`
}

// Effective is the PolicyResolver (T025): the effective policy of a channel for a sender of school (school level and
// preset of the sender's school; no class level since ADR-0095), cached chat_policy_cache. policy.ErrDenied when no
// rule allows the channel type (F115-AC-08). The result is the same for every grade (R15).
func (f Facts) Effective(ctx context.Context, tenant uuid.UUID, school *uuid.UUID, ch channel.Channel) (policy.Effective, error) {
	raw, err := f.Cache.Get(ctx, policyKey(tenant, school, ch.Type), f.Config.PolicyCache, func(ctx context.Context) ([]byte, error) {
		var levels policy.Levels
		err := f.read(ctx, tenant, func(ctx context.Context) error {
			var err error
			levels, err = f.Reader.PolicyLevels(ctx, tenant, school)
			return err
		})
		if err != nil {
			return nil, err
		}
		e, err := policy.Intersect(string(ch.Type), ch.Type.AlwaysOn(), levels)
		if err != nil {
			return json.Marshal(effectiveEntry{Denied: true})
		}
		return json.Marshal(effectiveEntry{Effective: &e})
	})
	if err != nil {
		return policy.Effective{}, unavailable(err)
	}
	var entry effectiveEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return policy.Effective{}, unavailable(err)
	}
	if entry.Denied || entry.Effective == nil {
		return policy.Effective{}, policy.ErrDenied
	}
	return *entry.Effective, nil
}

type curfewEntry struct {
	Platform *read.CurfewWindow `json:"platform,omitempty"`
	School   *read.CurfewWindow `json:"school,omitempty"`
}

// Curfew is the effective curfew of a sender of school: platform_curfew (projected, else the configured default) united
// with the curfew_window of the school (ADR-0078, F115-AC-22). A malformed projected value never lifts the curfew.
func (f Facts) Curfew(ctx context.Context, tenant uuid.UUID, school *uuid.UUID) (policy.Curfew, error) {
	raw, err := f.Cache.Get(ctx, curfewKey(tenant, school), f.Config.CurfewFresh, func(ctx context.Context) ([]byte, error) {
		var e curfewEntry
		err := f.read(ctx, tenant, func(ctx context.Context) error {
			var err error
			e.Platform, e.School, err = f.Reader.CurfewWindows(ctx, tenant, school)
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(e)
	})
	if err != nil {
		return nil, unavailable(err)
	}
	var e curfewEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, unavailable(err)
	}
	loc := f.Config.CurfewLocation
	platform := f.Config.PlatformCurfew
	if e.Platform != nil {
		if w, err := policy.NewWindow(e.Platform.Start, e.Platform.End, loc); err == nil && !w.Zero() {
			platform = w
		}
	}
	c := policy.Curfew{platform}
	if e.School != nil {
		if w, err := policy.NewWindow(e.School.Start, e.School.End, loc); err == nil {
			c = append(c, w)
		}
	}
	return c, nil
}

// SeqFloor is the start value of a lost seq counter: GREATEST(channels.last_seq, max seq of the recent buffer)
// (VALKEY_DESIGN §4.9, F115-AC-20). It reads PostgreSQL: without it the send fails closed.
func (f Facts) SeqFloor(ctx context.Context, tenant, id uuid.UUID) (int64, error) {
	var last int64
	if err := f.read(ctx, tenant, func(ctx context.Context) error {
		var err error
		last, err = f.Reader.LastSeq(ctx, tenant, id)
		return err
	}); err != nil {
		return 0, unavailable(err)
	}
	if f.Recent != nil {
		recent, err := f.Recent.List(ctx, id)
		if err != nil {
			return 0, unavailable(err)
		}
		for _, e := range recent {
			last = max(last, e.Seq)
		}
	}
	return last, nil
}
