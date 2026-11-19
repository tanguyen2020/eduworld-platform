package dto

import (
	"time"

	"github.com/google/uuid"
)

// AuditLogEntry is the AuditLogEntry schema (metadata filtered at the sink is returned as changes).
type AuditLogEntry struct {
	AuditID    uuid.UUID      `json:"audit_id"`
	TenantID   *uuid.UUID     `json:"tenant_id"`
	SchoolID   *uuid.UUID     `json:"school_id"`
	ActorID    *uuid.UUID     `json:"actor_id"`
	ActorType  string         `json:"actor_type"` // student | staff | service | system
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Result     string         `json:"result"`
	Reason     string         `json:"reason,omitempty"`
	Changes    map[string]any `json:"changes,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
}
