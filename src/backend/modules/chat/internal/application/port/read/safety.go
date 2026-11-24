package read

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CaseRow is a moderation case (chat.moderation_cases, F121): metadata only — the evidence content is read through
// SafetyReader.CaseMessages by the assignee or a team lead.
type CaseRow struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	TenantName       *string // ref.tenants name (code until a name is projected), F122 T064
	TenantType       *string // ref.tenants tenant_type (school, unit, common)
	Status           string
	Severity         string
	Source           string
	Category         *string
	ChannelID        *uuid.UUID
	ChannelType      *string
	TargetType       string
	TargetID         uuid.UUID
	SchoolIDs        []uuid.UUID
	Subjects         []uuid.UUID
	Assignee         *uuid.UUID
	AssignedVia      *string
	ReportCount      int
	FirstResponseDue time.Time
	SLADue           time.Time
	FirstResponseAt  *time.Time
	EscalatedAt      *time.Time
	HoldSanctionID   *uuid.UUID
	SchoolNotifiedAt *time.Time
	SchoolAckAt      *time.Time
	AcknowledgedBy   *uuid.UUID
	ContactOutcome   *string
	ContactPath      string
	ResolvedAt       *time.Time
	SourceEventID    *uuid.UUID
	OncallPageLevel  *string    // F124: last on-call level paged (primary, secondary, lead, system_admin)
	LastPagedAt      *time.Time // F124
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

// ActionRow is one entry of the case history (chat.moderation_actions).
type ActionRow struct {
	ID         uuid.UUID
	Action     string
	ActorID    *uuid.UUID
	ActorKind  string
	Note       *string
	SanctionID *uuid.UUID
	MessageID  *uuid.UUID
	At         time.Time
}

// ReportRow is a report of a case (chat.reports).
type ReportRow struct {
	ID         uuid.UUID
	ReporterID uuid.UUID
	SchoolID   *uuid.UUID
	Reason     string
	Note       *string
	Status     string
	CreatedAt  time.Time
}

// EvidenceRow is an immutable evidence snapshot of a case (chat.moderation_evidence).
type EvidenceRow struct {
	ID        uuid.UUID
	CaseID    uuid.UUID
	ChannelID *uuid.UUID
	Snapshot  json.RawMessage
	CreatedAt time.Time
}

// CaseMessage is a message of a case: the evidence copy or a context message of the channel (original content).
type CaseMessage struct {
	MessageID       uuid.UUID  `json:"message_id"`
	ChannelID       uuid.UUID  `json:"channel_id"`
	ChannelType     string     `json:"channel_type,omitempty"`
	Seq             int64      `json:"seq"`
	SenderID        uuid.UUID  `json:"sender_id"`
	SenderSchoolID  *uuid.UUID `json:"sender_school_id,omitempty"`
	Kind            string     `json:"kind"`
	Body            *string    `json:"body,omitempty"`
	BodyDisplay     *string    `json:"body_display,omitempty"`
	QuickMessageID  *string    `json:"quick_message_id,omitempty"`
	Revision        int        `json:"revision"`
	EditedAt        *time.Time `json:"edited_at,omitempty"`
	ModerationState string     `json:"moderation_state"`
	RecalledAt      *time.Time `json:"recalled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	// Revisions are the earlier versions and held edit proposals of the message (F118, evidence of an edit).
	Revisions []CaseRevision `json:"revisions,omitempty"`
}

// CaseRevision is one row of chat.message_revisions in an evidence snapshot.
type CaseRevision struct {
	Revision    int       `json:"revision"`
	State       string    `json:"state"`
	Body        *string   `json:"body,omitempty"`
	BodyDisplay *string   `json:"body_display,omitempty"`
	Decision    string    `json:"decision"`
	EditedAt    time.Time `json:"edited_at"`
}

// SanctionDetail is a sanction of chat.sanctions as the Safety Team sees it.
type SanctionDetail struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     *uuid.UUID
	UserID       uuid.UUID
	CaseID       *uuid.UUID
	Type         string
	Scope        string
	ChannelID    *uuid.UUID
	Reason       string
	ReasonCode   string
	IssuedByKind string
	IssuedBy     *uuid.UUID
	StartsAt     time.Time
	EndsAt       *time.Time
	RevokedAt    *time.Time
	RevokedBy    *uuid.UUID
	RevokeReason *string
	LiftedAt     *time.Time
	CreatedAt    time.Time
	Version      int64
}

// NotificationStudent is a student of a school notification (ids and role only; names are resolved by the portal).
type NotificationStudent struct {
	StudentID uuid.UUID `json:"student_id"`
	Role      string    `json:"role"`
}

// NotificationRow is a school safety notification (chat.school_safety_notifications).
type NotificationRow struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	CaseID           uuid.UUID
	SchoolID         uuid.UUID
	Severity         string
	Category         *string
	Students         []NotificationStudent
	MeasuresTaken    *string
	Summary          string
	SchoolNotifiedAt time.Time
	AckDueAt         time.Time
	ReminderCount    int
	LastRemindedAt   *time.Time
	SchoolAckAt      *time.Time
	AcknowledgedBy   *uuid.UUID
	ContactOutcome   *string
	AckNote          *string
	CreatedAt        time.Time
	Version          int64
}

// CaseFilter filters the cross-tenant queue (listModerationCases).
type CaseFilter struct {
	TenantIDs []uuid.UUID // empty: every tenant
	Status    string
	Severity  string
	Assignee  string // me, unassigned, any
	Me        uuid.UUID
	Category  string
	After     *CaseCursor
	Limit     int
}

// CaseCursor is the keyset position of the queue (severity, sla_due_at, case_id).
type CaseCursor struct {
	Severity string
	Due      time.Time
	ID       uuid.UUID
}

// SanctionFilter filters listSanctions.
type SanctionFilter struct {
	TenantIDs []uuid.UUID
	UserID    *uuid.UUID
	Active    *bool
	Now       time.Time
	After     *Cursor
	Limit     int
}

// NotificationFilter filters the school notifications (platform list: tenants; school list: one tenant and schools).
type NotificationFilter struct {
	TenantIDs    []uuid.UUID
	SchoolIDs    []uuid.UUID // nil: every school of the tenants
	Acknowledged *bool
	Overdue      *bool
	CreatedFrom  *time.Time // created_at ≥ (listSchoolSafetyNotifications created_from, F122 T066)
	Now          time.Time
	After        *Cursor
	Limit        int
}

// OpenCount is one bucket of eduworld_moderation_cases_open{severity,assigned}.
type OpenCount struct {
	Severity string
	Assigned bool
	Count    int
	// OldestUnclaimed is the creation time of the oldest open case without assignee (zero when none).
	OldestUnclaimed time.Time
}

// CaseTenant is one tenant having moderation cases (listModerationCaseTenants): name from ref.tenants (code until a
// name is projected; nil when the tenant is not projected).
type CaseTenant struct {
	TenantID uuid.UUID
	Name     *string
	Type     *string
}

// SafetyReader is the read side of the Safety Team (F121). Callers run it inside a read session of the tenant, or
// under tenancy.Bypass(ctx, "moderation_case") for the cross-tenant queue (explicit tenant and deleted_at conditions
// in every statement, R27).
type SafetyReader interface {
	Cases(ctx context.Context, f CaseFilter) ([]CaseRow, error)
	Case(ctx context.Context, id uuid.UUID) (*CaseRow, error)
	ActionsOf(ctx context.Context, tenant, id uuid.UUID, limit int) ([]ActionRow, error)
	ReportsOf(ctx context.Context, tenant, id uuid.UUID, limit int) ([]ReportRow, error)
	EvidenceOf(ctx context.Context, tenant, id uuid.UUID) ([]EvidenceRow, error)
	CaseNotifications(ctx context.Context, tenant, id uuid.UUID) ([]NotificationRow, error)
	// ChannelWindow returns up to before messages up to and after messages after seq in the channel (both message
	// tables), oldest first; seq 0 addresses the newest messages.
	ChannelWindow(ctx context.Context, tenant, channel uuid.UUID, seq int64, before, after int) ([]CaseMessage, error)
	// ChannelExists returns the type of a live channel (ok false: purged or unknown).
	ChannelExists(ctx context.Context, tenant, channel uuid.UUID) (string, bool, error)
	Sanctions(ctx context.Context, f SanctionFilter) ([]SanctionDetail, error)
	Notifications(ctx context.Context, f NotificationFilter) ([]NotificationRow, error)
	// CaseTenants lists the tenants having at least one live case after the tenant id after (keyset, tenant_id order),
	// with the projected tenant name — under tenancy.Bypass(moderation_case) only (F122 T064).
	CaseTenants(ctx context.Context, after *uuid.UUID, limit int) ([]CaseTenant, error)
	// OpenCounts returns the open cases (not resolved) per severity and assignment, every tenant (platform job).
	OpenCounts(ctx context.Context) ([]OpenCount, error)
	// OverdueNotifications counts the unacknowledged notifications past ack_due_at at now (every tenant).
	OverdueNotifications(ctx context.Context, now time.Time) (int, error)
}
