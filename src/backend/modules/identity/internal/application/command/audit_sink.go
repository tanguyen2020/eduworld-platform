package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Results of eduworld_audit_records_total.
const (
	AuditRecorded       = "recorded"
	AuditDuplicate      = "duplicate"
	AuditInvalid        = "invalid"
	AuditTenantMismatch = "tenant_mismatch"
)

// ErrAuditTenantMismatch: the school of the entry belongs to another tenant than the envelope (F015-AC-05);
// the consumer sends the event to the dead letters and raises a security warning.
var ErrAuditTenantMismatch = errors.New("audit sink: school outside the tenant of the envelope")

// AuditSink implements RecordAuditEntry (F015 US1): validate, sanitize (second layer), check the tenant of
// the school and store once per event id.
type AuditSink struct {
	UoW     write.UnitOfWork
	Repo    write.AuditLogRepository
	Metrics port.AuditMetrics
}

func (h AuditSink) metric(producer, result string) {
	if h.Metrics != nil {
		h.Metrics.Recorded(serviceOf(producer), result)
	}
}

// serviceOf is the module part of a producer (<module>/<component>).
func serviceOf(producer string) string {
	m, _, _ := strings.Cut(producer, "/")
	return m
}

// Record stores one entry. audit.ErrInvalid and ErrAuditTenantMismatch are permanent.
func (h AuditSink) Record(ctx context.Context, e audit.Entry) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RecordAuditEntry")
	defer func() { done(err) }()
	if err := e.Validate(); err != nil {
		h.metric(e.Producer, AuditInvalid)
		return err
	}
	if len(e.Reason) > audit.MaxReason {
		e.Reason = e.Reason[:audit.MaxReason]
	}
	var removed int
	e.Metadata, removed = audit.Sanitize(e.Metadata)
	if removed > 0 && h.Metrics != nil {
		h.Metrics.Sanitized(removed)
	}
	sc, err := authz.SystemPlatformScope("audit-sink")
	if err != nil {
		return err
	}
	var inserted bool
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		if e.SchoolID != nil {
			owner, err := h.Repo.SchoolTenant(ctx, *e.SchoolID)
			if err != nil {
				return err
			}
			if owner == nil || e.TenantID == nil || *owner != *e.TenantID {
				return ErrAuditTenantMismatch
			}
		}
		inserted, err = h.Repo.Insert(ctx, e)
		return err
	})
	switch {
	case errors.Is(err, ErrAuditTenantMismatch):
		h.metric(e.Producer, AuditTenantMismatch)
		slog.WarnContext(ctx, "security: audit entry names a school of another tenant", "event_id", e.ID.String(),
			"producer", e.Producer, "action", e.Action)
		return err
	case err != nil:
		return err
	case inserted:
		h.metric(e.Producer, AuditRecorded)
	default:
		h.metric(e.Producer, AuditDuplicate)
	}
	return nil
}

// Defaults [Proposed] of the audit maintenance (CONFIG_DEFAULTS audit_log_retention, DATA_PRIVACY §2).
const (
	DefaultAuditRetention      = 2 * 365 * 24 * time.Hour
	DefaultAuditMonthsAhead    = 3
	auditPartitionCreateMonths = 13
)

// AuditMaintenance runs the jobs partition-maintainer (monthly partitions of audit.audit_logs and
// identity.login_events) and audit-retention (anonymization in place, never a delete — ADR-0070).
type AuditMaintenance struct {
	Maint     write.AuditMaintenance
	Metrics   port.AuditMetrics
	Retention time.Duration
	Clock     func() time.Time
}

func (h AuditMaintenance) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

// EnsurePartitions creates the partitions from last month through a year ahead and reports a table whose
// next DefaultAuditMonthsAhead months are not all present (F015-AC-03).
func (h AuditMaintenance) EnsurePartitions(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.EnsureAuditPartitions")
	defer func() { done(err) }()
	now := h.now()
	n, err = h.Maint.EnsurePartitions(ctx, now.AddDate(0, -1, 0), auditPartitionCreateMonths)
	if err != nil {
		return 0, err
	}
	for _, table := range []string{"audit.audit_logs", "identity.login_events"} {
		ahead, err := h.Maint.MonthsAhead(ctx, table, now)
		if err != nil {
			return n, err
		}
		if ahead < DefaultAuditMonthsAhead && h.Metrics != nil {
			h.Metrics.PartitionMissing(table)
		}
	}
	return n, nil
}

// Retain anonymizes the entries older than the retention (F015-AC-06).
func (h AuditMaintenance) Retain(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.AuditRetention")
	defer func() { done(err) }()
	r := h.Retention
	if r <= 0 {
		r = DefaultAuditRetention
	}
	c, err := h.Maint.Anonymize(ctx, h.now().Add(-r))
	return int(c), err
}
