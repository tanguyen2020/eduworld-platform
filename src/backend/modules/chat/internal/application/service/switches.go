package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// PlatformLabel is the tenant label of the platform level in eduworld_chat_policy_disabled.
const PlatformLabel = "platform"

// SwitchGauge keeps eduworld_chat_policy_disabled{tenant,step} of this process in line with the configured switches
// (F116-AC-25, MODERATION §12.2): 1 while a step is off at a level (tenant = tenant id, or "platform"), 0 once it is
// back on or expired. Sync re-reads every live switch with a value off in a system platform scope; it runs after each
// switch change, after each run of moderation-switch-expirer and every Interval (a change made by another process —
// the expirer in worker jobs, a command in core-api — is caught up within Interval).
type SwitchGauge struct {
	Read     ReadRunner
	Reader   read.SwitchAdminReader
	Set      func(tenant, step string, off bool)
	Interval time.Duration
	Now      func() time.Time

	mu  sync.Mutex
	off map[[2]string]bool // labels set to 1 by the previous sync
}

func (g *SwitchGauge) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

// Sync sets the gauge from the stored switches.
func (g *SwitchGauge) Sync(ctx context.Context) error {
	if g == nil || g.Set == nil {
		return nil
	}
	sc, err := authz.SystemPlatformScope("moderation-switch-gauge")
	if err != nil {
		return err
	}
	var rows []read.SwitchRow
	if err := g.Read.InTenantRead(ctx, sc, func(ctx context.Context) error {
		rows, err = g.Reader.DisabledSwitches(ctx)
		return err
	}); err != nil {
		return err
	}
	now := g.now()
	next := map[[2]string]bool{}
	for i := range rows {
		label := PlatformLabel
		if rows[i].TenantID != nil {
			label = rows[i].TenantID.String()
		}
		for step, off := range moderation.OffSteps(ToSwitch(&rows[i]), now) {
			if off {
				next[[2]string{label, step}] = true
			}
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for k := range g.off {
		if !next[k] {
			g.Set(k[0], k[1], false)
		}
	}
	for k := range next {
		g.Set(k[0], k[1], true)
	}
	g.off = next
	return nil
}

// Run syncs the gauge every Interval until ctx ends.
func (g *SwitchGauge) Run(ctx context.Context) error {
	iv := g.Interval
	if iv <= 0 {
		iv = moderation.PropagationMax / 2
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for {
		if err := g.Sync(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "chat: moderation switch gauge not synced", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// Tenants returns the tenants this instance holds a catalogue view of (the tenants whose messages it moderates): a
// platform switch change invalidates their cached switches at once.
func (c *Catalogs) Tenants() []uuid.UUID {
	var out []uuid.UUID
	c.tenants.Range(func(k, _ any) bool {
		out = append(out, k.(uuid.UUID))
		return true
	})
	return out
}
