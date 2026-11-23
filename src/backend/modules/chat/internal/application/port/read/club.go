package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
)

// ClubFilter selects a page of clubs: a student sees its clubs (Mine) or the active clubs of its scope (Schools; nil =
// the whole tenant of a unit with cross-school social); staff sees the clubs of the schools of its grant.
type ClubFilter struct {
	Tenant  uuid.UUID
	Member  *uuid.UUID  // clubs this student is an active member of
	Schools []uuid.UUID // nil: every school of the tenant
	Status  []string    // empty: any
	Type    *club.Type  // nil: any
	After   *ClubCursor // keyset (created_at, club_id) descending
	Limit   int
	Viewer  *uuid.UUID // my_role of this student
}

// ClubCursor is the keyset of the club pages.
type ClubCursor struct {
	At time.Time
	ID uuid.UUID
}

// ClubRow is a club with the role of the viewer.
type ClubRow struct {
	Club   club.Club
	MyRole string
}

// ClubMemberRow is one active member with its public card (nickname only, R16).
type ClubMemberRow struct {
	UserID   uuid.UUID
	Nickname *string
	Avatar   *string
	Role     string
	JoinedAt time.Time
}

// ClubRequestRow is one pending join request with the requester nickname.
type ClubRequestRow struct {
	ID          uuid.UUID
	ClubID      uuid.UUID
	RequesterID uuid.UUID
	Nickname    *string
	Status      string
	CreatedAt   time.Time
}

// ClubGoalRow is a goal with the contribution of the viewer.
type ClubGoalRow struct {
	ID             uuid.UUID
	ClubID         uuid.UUID
	Title          *string
	Metric         string
	SubjectID      *uuid.UUID
	Target         int
	Progress       int
	MyContribution int
	WeekStart      time.Time
	Status         string
}

// ClubEventRow is one club event.
type ClubEventRow struct {
	ID          uuid.UUID
	ClubID      uuid.UUID
	Title       string
	Description *string
	Kind        string
	StartsAt    time.Time
	EndsAt      *time.Time
	CreatedAt   time.Time
}

// ClubReader reads the clubs of one tenant (explicit SQL, tenant + deleted_at on top of RLS, R27).
type ClubReader interface {
	Clubs(ctx context.Context, f ClubFilter) ([]ClubRow, error)
	Club(ctx context.Context, tenant, id uuid.UUID, viewer *uuid.UUID) (*ClubRow, error)
	Members(ctx context.Context, tenant, clubID uuid.UUID, after *ClubCursor, limit int) ([]ClubMemberRow, error)
	Requests(ctx context.Context, tenant, clubID uuid.UUID, after *ClubCursor, limit int) ([]ClubRequestRow, error)
	Goals(ctx context.Context, tenant, clubID, viewer uuid.UUID) ([]ClubGoalRow, error)
	Events(ctx context.Context, tenant, clubID uuid.UUID, after *ClubCursor, limit int) ([]ClubEventRow, error)
}
