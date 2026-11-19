package job

import (
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
)

// Intervals of the F015 jobs [Proposed].
const (
	AuditPartitionInterval = 6 * time.Hour  // partition-maintainer (audit.audit_logs, identity.login_events)
	AuditRetentionInterval = 24 * time.Hour // audit-retention
)

// AuditPartitions is the job partition-maintainer of module identity.
func AuditPartitions(m command.AuditMaintenance) RunOnce { return m.EnsurePartitions }

// AuditRetention is the job audit-retention.
func AuditRetention(m command.AuditMaintenance) RunOnce { return m.Retain }
