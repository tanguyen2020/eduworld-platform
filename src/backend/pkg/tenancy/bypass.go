package tenancy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
)

// BypassReason is the closed list of reasons to lift the tenant/soft-delete filter (ADR-0089 §3,
// MULTITENANCY §4.4). Adding a reason requires amending ADR-0089; the bypassreason analyzer rejects
// any value that is not one of these constants.
type BypassReason string

// Closed list of bypass reasons. The comment is the DB role each reason runs with.
const (
	ReasonSystemAdminCrossTenant BypassReason = "system_admin_cross_tenant" // <module>_platform
	ReasonModerationCase         BypassReason = "moderation_case"           // chat_platform
	ReasonBreakGlass             BypassReason = "break_glass"               // chat_platform
	ReasonTrash                  BypassReason = "trash"                     // <module>_platform
	ReasonAuthLogin              BypassReason = "auth_login"                // SECURITY DEFINER identity.memberships_for_user
	ReasonTenantClone            BypassReason = "tenant_clone"              // <module>_platform
	ReasonInactiveCleanup        BypassReason = "inactive_cleanup"          // <module>_platform
	ReasonReEvaluation           BypassReason = "re_evaluation"             // learning_platform
	ReasonEfficacyStudy          BypassReason = "efficacy_study"            // learning_platform
	ReasonPlatformJob            BypassReason = "platform_job"              // <module>_platform
	ReasonPlatformAnalytics      BypassReason = "platform_analytics"        // <module>_platform, rollup tables only
)

var reasons = []BypassReason{
	ReasonSystemAdminCrossTenant, ReasonModerationCase, ReasonBreakGlass, ReasonTrash, ReasonAuthLogin,
	ReasonTenantClone, ReasonInactiveCleanup, ReasonReEvaluation, ReasonEfficacyStudy, ReasonPlatformJob,
	ReasonPlatformAnalytics,
}

// Reasons returns the closed list.
func Reasons() []BypassReason { return append([]BypassReason(nil), reasons...) }

// Valid reports whether r belongs to the closed list.
func (r BypassReason) Valid() bool {
	for _, x := range reasons {
		if r == x {
			return true
		}
	}
	return false
}

// Mode of the bypassed access (audit metadata).
type Mode string

const (
	ModeRead  Mode = "read"
	ModeWrite Mode = "write"
)

// ErrBypassReason is returned for an empty or unknown reason.
var ErrBypassReason = errors.New("tenancy: bypass reason is not in the closed list")

// Hooks wires audit and metrics; set once at startup (binaries) or per test.
type Hooks struct {
	Recorder   audit.Recorder
	Registerer prometheus.Registerer
}

var (
	hooksMu  sync.RWMutex
	hooks    Hooks
	counter  *prometheus.CounterVec
	counterO sync.Once
)

// BypassCounter is eduworld_tenant_filter_bypass_total{module,reason} (MONITORING).
func BypassCounter() *prometheus.CounterVec {
	counterO.Do(func() {
		counter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eduworld_tenant_filter_bypass_total",
			Help: "Number of tenant/soft-delete filter bypasses by module and reason (ADR-0089).",
		}, []string{"module", "reason"})
	})
	return counter
}

// Configure sets the audit recorder and registers the bypass metric.
func Configure(h Hooks) {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	hooks = h
	if h.Registerer != nil {
		_ = h.Registerer.Register(BypassCounter())
	}
}

func currentHooks() Hooks {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return hooks
}

type bypassOptions struct {
	tenantIDs []uuid.UUID
	mode      Mode
}

// BypassOption configures audit metadata of a bypass.
type BypassOption func(*bypassOptions)

// WithTenantIDs records which tenants are touched.
func WithTenantIDs(ids ...uuid.UUID) BypassOption {
	return func(o *bypassOptions) { o.tenantIDs = append(o.tenantIDs, ids...) }
}

// WithMode records read or write access (default read).
func WithMode(m Mode) BypassOption { return func(o *bypassOptions) { o.mode = m } }

type bypassKey struct{}

// Bypass lifts the tenant/soft-delete filter for ctx after validating the reason, recording audit
// cross_tenant.accessed and incrementing eduworld_tenant_filter_bypass_total{module,reason}.
// It fails closed: an audit failure denies the bypass.
func Bypass(ctx context.Context, reason BypassReason, opts ...BypassOption) (context.Context, error) {
	if !reason.Valid() {
		return ctx, fmt.Errorf("%w: %q", ErrBypassReason, string(reason))
	}
	o := bypassOptions{mode: ModeRead}
	for _, f := range opts {
		f(&o)
	}
	if err := recordCrossTenant(ctx, reason, o.tenantIDs, o.mode); err != nil {
		return ctx, err
	}
	BypassCounter().WithLabelValues(ModuleFrom(ctx), string(reason)).Inc()
	return context.WithValue(ctx, bypassKey{}, reason), nil
}

// RecordCrossTenantScope records the audit of a CrossTenantScope use (called by authz).
func RecordCrossTenantScope(ctx context.Context, s CrossTenantScope, mode Mode) error {
	if err := recordCrossTenant(ctx, s.reason, s.ids, mode); err != nil {
		return err
	}
	BypassCounter().WithLabelValues(ModuleFrom(ctx), string(s.reason)).Inc()
	return nil
}

func recordCrossTenant(ctx context.Context, reason BypassReason, ids []uuid.UUID, mode Mode) error {
	rec := currentHooks().Recorder
	if rec == nil {
		return errors.New("tenancy: no audit recorder configured; bypass denied")
	}
	strIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		strIDs = append(strIDs, id.String())
	}
	err := rec.Record(ctx, audit.Event{
		Action: audit.ActionCrossTenantAccessed,
		Result: audit.ResultSuccess,
		Reason: string(reason),
		Metadata: map[string]any{
			"reason":     string(reason),
			"tenant_ids": strIDs,
			"mode":       string(mode),
		},
	})
	if err != nil {
		return fmt.Errorf("tenancy: audit cross_tenant.accessed: %w", err)
	}
	return nil
}

// Bypassed returns the reason when ctx carries an audited bypass.
func Bypassed(ctx context.Context) (BypassReason, bool) {
	r, ok := ctx.Value(bypassKey{}).(BypassReason)
	return r, ok && r.Valid()
}
