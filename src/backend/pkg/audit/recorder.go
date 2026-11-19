// Package audit defines the audit recorder port (F002 T007). F002 owns the interface; the outbox-backed
// implementation (event audit.recorded) is F003 T042; module identity persists audit.audit_logs (F015).
// Never put PII, tokens, chat text or answers into Metadata (AGENTS R14).
package audit

import "context"

// Field length limits (DATABASE_DESIGN §3.11, ADR-0041).
const (
	MaxActionLen     = 64
	MaxTargetTypeLen = 64
	MaxTargetIDLen   = 64
	MaxReasonLen     = 500
)

// Result of the audited action.
type Result string

const (
	ResultSuccess Result = "success"
	ResultDenied  Result = "denied"
	ResultFailure Result = "failure"
)

// Common actions recorded by the platform kit.
const (
	ActionAuthzDenied         = "authz.denied"
	ActionCrossTenantAccessed = "cross_tenant.accessed"
)

// Event is one audit entry. Actor, tenant and trace are taken from the context by the recorder.
type Event struct {
	Action     string
	TargetType string
	TargetID   string
	Result     Result
	Reason     string
	Metadata   map[string]any
	// SchoolID is the school the action concerns (UUID string; empty = none): audit.audit_logs.school_id, the
	// filter of School Admins (F015).
	SchoolID string
}

// Recorder records audit events. Implementations must be safe for concurrent use.
type Recorder interface {
	Record(ctx context.Context, e Event) error
}

// RecorderFunc adapts a function to Recorder.
type RecorderFunc func(ctx context.Context, e Event) error

// Record calls f.
func (f RecorderFunc) Record(ctx context.Context, e Event) error { return f(ctx, e) }

// Discard drops events; only for code paths that are not yet wired (never in production wiring).
var Discard Recorder = RecorderFunc(func(context.Context, Event) error { return nil })
