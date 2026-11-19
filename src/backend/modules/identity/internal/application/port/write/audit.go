package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/audit"
)

// AuditLogRepository writes the central audit log (audit.audit_logs, F015 US1). It runs in the platform scope
// "audit-sink": entries of every tenant and of the platform arrive through one consumer.
type AuditLogRepository interface {
	// Insert stores the entry once (the key is the event id); inserted is false for a redelivery.
	Insert(ctx context.Context, e audit.Entry) (inserted bool, err error)
	// SchoolTenant returns the tenant of a school (nil when the school does not exist).
	SchoolTenant(ctx context.Context, schoolID uuid.UUID) (*uuid.UUID, error)
}

// AuditMaintenance runs the SECURITY DEFINER maintenance functions of the audit log with the role
// identity_partition_admin (no table privilege of its own).
type AuditMaintenance interface {
	// EnsurePartitions creates the missing monthly partitions from the month of from, months ahead.
	EnsurePartitions(ctx context.Context, from time.Time, months int) (int, error)
	// MonthsAhead counts the consecutive monthly partitions of table from the month of from.
	MonthsAhead(ctx context.Context, table string, from time.Time) (int, error)
	// Anonymize anonymizes in place the rows older than before; returns the rows changed.
	Anonymize(ctx context.Context, before time.Time) (int64, error)
}
