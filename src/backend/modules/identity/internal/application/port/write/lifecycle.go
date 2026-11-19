package write

import (
	"context"

	"github.com/google/uuid"
)

// LifecycleTarget is the account a lifecycle use case acts on: the user (global) and, for a student, the
// student row of the tenant of the scope (nil outside that tenant through RLS).
type LifecycleTarget struct {
	UserID          uuid.UUID
	UserType        string
	Username        string
	Status          string
	PrimaryTenantID *uuid.UUID
	StudentTenantID *uuid.UUID
	StudentSchoolID *uuid.UUID
	GradeLevel      int
	Version         int64
}

// StatusChange is one row of identity.user_status_history.
type StatusChange struct {
	UserID   uuid.UUID
	TenantID *uuid.UUID
	From, To string
	Reason   string
	Source   string // staff | sis | cleanup | restore | anonymize | retention
	By       *uuid.UUID
}

// LifecycleRepository reads and writes the status of accounts (F016).
type LifecycleRepository interface {
	Target(ctx context.Context, userID uuid.UUID) (*LifecycleTarget, error)
	// SetStatus moves the user from → to (users.version + 1); 409 CONFLICT when the status changed meanwhile.
	SetStatus(ctx context.Context, userID uuid.UUID, from, to string) error
	RecordStatusChange(ctx context.Context, c StatusChange) error
	// KeycloakSubject returns the subject of the user's active identity at one of issuers ("" when none).
	KeycloakSubject(ctx context.Context, userID uuid.UUID, issuers []string) (string, error)
}
