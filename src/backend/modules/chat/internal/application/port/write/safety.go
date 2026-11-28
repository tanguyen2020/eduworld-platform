package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
)

// ReportedMessage is the stored message a report or a signal is about (chat.messages or chat.gameplay_messages).
type ReportedMessage struct {
	ID          uuid.UUID
	ChannelID   uuid.UUID
	ChannelType string
	Seq         int64
	SenderID    uuid.UUID
	SchoolID    *uuid.UUID
	CreatedAt   time.Time
}

// SafetyStudent is a projected student of the tenant (ref.students).
type SafetyStudent struct {
	ID       uuid.UUID
	SchoolID uuid.UUID
	Status   string
	Deleted  bool
}

// NewReport is a report to insert.
type NewReport struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     *uuid.UUID
	ReporterID   uuid.UUID
	TargetType   string
	TargetID     uuid.UUID
	TargetUserID *uuid.UUID
	Reason       string
	Note         *string
	Context      map[string]string
	CaseID       uuid.UUID
	Weight       float64
	EvidenceID   *uuid.UUID
	CreatedAt    time.Time
}

// NewAction is one entry of a case history (append-only).
type NewAction struct {
	TenantID   uuid.UUID
	CaseID     uuid.UUID
	Action     string
	ActorID    *uuid.UUID
	ActorKind  string
	Note       *string
	SanctionID *uuid.UUID
	MessageID  *uuid.UUID
	At         time.Time
}

// NewEvidence is an immutable evidence snapshot.
type NewEvidence struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	CaseID    uuid.UUID
	ReportID  *uuid.UUID
	SchoolID  *uuid.UUID
	ChannelID *uuid.UUID
	Snapshot  json.RawMessage
	SHA256    string
	At        time.Time
}

// SafetySanction is a sanction to insert by the Safety Team or the system (F121).
type SafetySanction struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	SchoolID     *uuid.UUID
	UserID       uuid.UUID
	Type         string
	Scope        string
	ChannelID    *uuid.UUID
	Reason       string
	ReasonCode   string
	CaseID       *uuid.UUID
	IssuedByKind string
	IssuedBy     *uuid.UUID
	StartsAt     time.Time
	EndsAt       *time.Time
}

// SafetyStore writes the Safety Team data inside the UnitOfWork transaction of the tenant (explicit tenant and
// deleted_at conditions, R27). The cross-tenant jobs read their work lists through it under
// tenancy.Bypass(ctx, "platform_job") and then write each item in the scope of its tenant.
type SafetyStore interface {
	// Message returns the stored message id of tenant (both message tables; nil when unknown).
	Message(ctx context.Context, tenant, id uuid.UUID) (*ReportedMessage, error)
	// Member reports a membership row of user in channel (left members included: a report window outlives the party,
	// MODERATION §5) joined before at.
	Member(ctx context.Context, tenant, channel, user uuid.UUID, at time.Time) (bool, error)
	// ChannelType returns the type of a live channel of tenant ("" when unknown).
	ChannelType(ctx context.Context, tenant, channel uuid.UUID) (string, error)
	// Window returns ±n messages around seq in the channel (seq 0: the newest), oldest first, original content.
	Window(ctx context.Context, tenant, channel uuid.UUID, seq int64, before, after int) ([]read.CaseMessage, error)
	Student(ctx context.Context, tenant, id uuid.UUID) (*SafetyStudent, error)
	TenantType(ctx context.Context, tenant uuid.UUID) (string, error)
	// PlayerNickname returns the current nickname of the public card of student (ref.player_profiles), nil when none —
	// the server snapshot of a nickname report (F136-AC-18).
	PlayerNickname(ctx context.Context, tenant, student uuid.UUID) (*string, error)

	// OpenCase returns the open/in_review case of the target created since (locked), nil when none.
	OpenCase(ctx context.Context, tenant uuid.UUID, targetType string, target uuid.UUID, since time.Time) (*read.CaseRow, error)
	// CaseByEvent returns the case opened by a signal event (idempotency of the consumer).
	CaseByEvent(ctx context.Context, tenant, event uuid.UUID) (*read.CaseRow, error)
	// CaseTenant returns the tenant of a live case of any tenant (only under tenancy.Bypass(moderation_case)).
	CaseTenant(ctx context.Context, id uuid.UUID) (*uuid.UUID, error)
	// CaseActions returns the history of a case, oldest first.
	CaseActions(ctx context.Context, tenant, caseID uuid.UUID) ([]read.ActionRow, error)
	// LockCase returns the live case of tenant locked FOR UPDATE (nil when unknown).
	LockCase(ctx context.Context, tenant, id uuid.UUID) (*read.CaseRow, error)
	InsertCase(ctx context.Context, c read.CaseRow) error
	// UpdateCase saves c when the stored version is c.Version - 1 (ErrStale otherwise).
	UpdateCase(ctx context.Context, c read.CaseRow, actor *uuid.UUID) error
	// InsertReport inserts r; when the reporter already reported the target in this case it returns the existing id and
	// false (one report per reporter and case).
	InsertReport(ctx context.Context, r NewReport) (uuid.UUID, bool, error)
	// DismissedReports counts the reports of reporter dismissed since (abuse of reports, MODERATION §5).
	DismissedReports(ctx context.Context, tenant, reporter uuid.UUID, since time.Time) (int, error)
	// SetReportStatus sets the status of the live reports of a case.
	SetReportStatus(ctx context.Context, tenant, caseID uuid.UUID, status string) error
	CaseReports(ctx context.Context, tenant, caseID uuid.UUID) ([]read.ReportRow, error)
	InsertAction(ctx context.Context, a NewAction) error
	InsertEvidence(ctx context.Context, e NewEvidence) error
	CaseEvidence(ctx context.Context, tenant, caseID uuid.UUID) ([]read.EvidenceRow, error)
	// RemoveMessage marks a message removed and soft-deletes it (original content kept as evidence, ADR-0034).
	RemoveMessage(ctx context.Context, tenant, id, by uuid.UUID, at time.Time) (bool, error)

	InsertSanction(ctx context.Context, s SafetySanction) error
	// ActiveHold returns the active hold sanction of user without end (case_hold or block_and_alert_hold), nil if none.
	ActiveHold(ctx context.Context, tenant, user uuid.UUID) (*read.SanctionDetail, error)
	LinkSanction(ctx context.Context, tenant, sanction, caseID uuid.UUID) error
	// LockSanction returns the live sanction of tenant locked FOR UPDATE (nil when unknown).
	LockSanction(ctx context.Context, tenant, id uuid.UUID) (*read.SanctionDetail, error)
	// Revoke ends a sanction before its end (revoked_at, revoked_by, revoke_reason, lifted_at).
	Revoke(ctx context.Context, tenant, id uuid.UUID, by *uuid.UUID, reason string, at time.Time) error
	// MarkLifted records that sanction.lifted was published for an expired sanction.
	MarkLifted(ctx context.Context, tenant, id uuid.UUID, at time.Time) error
	// CaseSanctions returns the sanctions of a case.
	CaseSanctions(ctx context.Context, tenant, caseID uuid.UUID) ([]read.SanctionDetail, error)

	InsertNotification(ctx context.Context, n read.NotificationRow, actor uuid.UUID) error
	// LockNotification returns the live notification of tenant locked FOR UPDATE (nil when unknown).
	LockNotification(ctx context.Context, tenant, id uuid.UUID) (*read.NotificationRow, error)
	UpdateNotification(ctx context.Context, n read.NotificationRow, actor *uuid.UUID) error
	// PendingNotification returns the unacknowledged notification of a case and school (nil when none).
	PendingNotification(ctx context.Context, tenant, caseID, school uuid.UUID) (*read.NotificationRow, error)

	// Work lists of the platform jobs (every tenant; run under tenancy.Bypass(ctx, "platform_job")).
	OverdueCases(ctx context.Context, now time.Time, limit int) ([]read.CaseRow, error)
	DueNotifications(ctx context.Context, now time.Time, limit int) ([]read.NotificationRow, error)
	ExpiredSanctions(ctx context.Context, now time.Time, limit int) ([]read.SanctionDetail, error)
	// StaleHolds returns active holds whose case is resolved (released when the case resolved; safety net).
	StaleHolds(ctx context.Context, limit int) ([]read.SanctionDetail, error)
}
