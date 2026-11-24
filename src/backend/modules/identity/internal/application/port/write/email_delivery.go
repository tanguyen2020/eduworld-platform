package write

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EmailDelivery is one email to one staff member (identity.email_deliveries, F124): the recipient id, never the address.
type EmailDelivery struct {
	ID            uuid.UUID
	TenantID      *uuid.UUID // nil for platform staff (on-call, break-glass)
	SourceEventID uuid.UUID
	Template      string
	RecipientID   uuid.UUID
	Params        map[string]string
	Locale        string
	Status        string
	Attempts      int
	NextAttemptAt *time.Time
	CreatedAt     time.Time
}

// EmailDeliveryRepository stores the deliveries and their status (insert + status updates only, never deleted).
type EmailDeliveryRepository interface {
	// Enqueue inserts a pending delivery; false when (source event, recipient, template) already exists (F124-AC-10).
	Enqueue(ctx context.Context, d EmailDelivery) (bool, error)
	// Claim returns up to limit pending deliveries due at now and leases them until now+lease (FOR UPDATE SKIP LOCKED),
	// so concurrent senders never send one twice in a lease.
	Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]EmailDelivery, error)
	// Finish sets the final status (sent, failed, skipped) with the attempts and an error code (SMTP reply code only).
	Finish(ctx context.Context, id uuid.UUID, status string, attempts int, code *string, at time.Time) error
	// Retry records a failed attempt and the next attempt.
	Retry(ctx context.Context, id uuid.UUID, attempts int, next time.Time, code string) error
}

// StaffContact is the current contact of a staff member, read when sending (address never stored, R14).
type StaffContact struct {
	UserID uuid.UUID
	Email  string
	Locale string
}

// StaffDirectory finds the recipients of the staff emails and re-checks them when sending (F124-AC-07, AC-11): live,
// active staff accounts with a valid UserRole (deleted_at IS NULL, valid_from ≤ now < valid_to).
type StaffDirectory interface {
	// TenantRole lists the staff with role in tenant (school: that school only; nil: any scope of the tenant).
	TenantRole(ctx context.Context, tenant uuid.UUID, role string, school *uuid.UUID) ([]uuid.UUID, error)
	// PlatformPermission lists the platform staff holding perm through a platform role other than exceptRole ("" = any).
	PlatformPermission(ctx context.Context, perm, exceptRole string) ([]uuid.UUID, error)
	// PlatformRole lists the platform staff with role.
	PlatformRole(ctx context.Context, role string) ([]uuid.UUID, error)
	// Contact returns the contact of an active, live staff account (nil when locked, deactivated, deleted or without email).
	Contact(ctx context.Context, user uuid.UUID) (*StaffContact, error)
	// SchoolName returns the name of a live school of tenant ("" when unknown).
	SchoolName(ctx context.Context, tenant, school uuid.UUID) (string, error)
}
