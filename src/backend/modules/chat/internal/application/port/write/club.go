package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
)

// ClubMember is one active membership of a club (social.club_members).
type ClubMember struct {
	ClubID   uuid.UUID
	UserID   uuid.UUID
	SchoolID *uuid.UUID
	Role     string
	JoinedAt time.Time
}

// ClubRequest is a join request (social.club_join_requests).
type ClubRequest struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ClubID      uuid.UUID
	RequesterID uuid.UUID
	Status      string
	CreatedAt   time.Time
}

// ClubInvitation is an invitation (social.club_invitations).
type ClubInvitation struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ClubID    uuid.UUID
	InviterID uuid.UUID
	InviteeID uuid.UUID
	Status    string
	ExpiresAt time.Time
}

// ClubGoal is a weekly goal (social.club_goals).
type ClubGoal struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ClubID    uuid.UUID
	Title     *string
	Metric    string
	SubjectID *uuid.UUID
	Target    int
	Progress  int
	WeekStart time.Time
	Status    string
}

// ClubEvent is a club event (social.club_events).
type ClubEvent struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	ClubID               uuid.UUID
	Title                string
	Description          *string
	Kind                 string
	TournamentTemplateID *uuid.UUID
	StartsAt             time.Time
	EndsAt               *time.Time
}

// ClubStore writes the clubs of one tenant in the transaction of a club use case (F120 US4): explicit SQL with tenant
// and deleted_at conditions on top of RLS (R27); soft delete only (R21).
type ClubStore interface {
	// SchoolCreation returns student_club_creation of a projected school (disabled when unknown).
	SchoolCreation(ctx context.Context, tenant, school uuid.UUID) (club.Creation, error)
	// CreateClub inserts c.
	CreateClub(ctx context.Context, c club.Club) error
	// Club returns a live club of tenant (locked FOR UPDATE when lock), nil when unknown.
	Club(ctx context.Context, tenant, id uuid.UUID, lock bool) (*club.Club, error)
	// SaveClub stores status, name, description, icon, member_count, chat_channel_id, approval/archive fields (version+1).
	SaveClub(ctx context.Context, c club.Club, actor uuid.UUID, now time.Time) error
	// Member returns the active membership of user (nil: none).
	Member(ctx context.Context, tenant, clubID, user uuid.UUID) (*ClubMember, error)
	// AddMember inserts an active membership.
	AddMember(ctx context.Context, tenant uuid.UUID, m ClubMember, source uuid.UUID) error
	// EndMember sets left_at/left_reason of the active membership (false: none).
	EndMember(ctx context.Context, tenant, clubID, user uuid.UUID, at time.Time, reason string) (bool, error)
	// EndAll ends every active membership of a club (archive) and returns the users.
	EndAll(ctx context.Context, tenant, clubID uuid.UUID, at time.Time, reason string) ([]uuid.UUID, error)
	// SetRole changes the role of an active member (false: none).
	SetRole(ctx context.Context, tenant, clubID, user uuid.UUID, role string) (bool, error)
	// CountClubs counts the clubs (not archived) user is an active member of.
	CountClubs(ctx context.Context, tenant, user uuid.UUID) (int, error)
	// CreateRequest inserts a pending join request (false: a pending one exists).
	CreateRequest(ctx context.Context, r ClubRequest) (bool, error)
	// Request returns a join request of tenant locked for update (nil: unknown).
	Request(ctx context.Context, tenant, id uuid.UUID) (*ClubRequest, error)
	// PendingRequest returns the pending request of requester for a club (nil: none).
	PendingRequest(ctx context.Context, tenant, clubID, requester uuid.UUID) (*ClubRequest, error)
	// DecideRequest stores the decision of a pending request.
	DecideRequest(ctx context.Context, tenant, id uuid.UUID, status string, by uuid.UUID, at time.Time) (bool, error)
	// CreateInvitation inserts a pending invitation (false: a pending one exists for the invitee; existing returned).
	CreateInvitation(ctx context.Context, inv ClubInvitation) (*ClubInvitation, bool, error)
	// Invitation returns an invitation of tenant locked for update (nil: unknown).
	Invitation(ctx context.Context, tenant, id uuid.UUID) (*ClubInvitation, error)
	// DecideInvitation stores the decision of a pending invitation.
	DecideInvitation(ctx context.Context, tenant, id uuid.UUID, status string, at time.Time) (bool, error)
	// CreateGoal inserts a goal.
	CreateGoal(ctx context.Context, g ClubGoal, actor uuid.UUID) error
	// CreateEvent inserts an event.
	CreateEvent(ctx context.Context, e ClubEvent, actor uuid.UUID) error
	// GoalsOf returns the active goals of metric of week whose clubs (active) user is an active member of, locked.
	GoalsOf(ctx context.Context, tenant, user uuid.UUID, metric string, week time.Time) ([]ClubGoal, error)
	// Contribute adds delta to the contribution of user (false: event already counted for that goal).
	Contribute(ctx context.Context, tenant, goal, user uuid.UUID, delta int, event uuid.UUID) (bool, error)
	// AdvanceGoal adds delta to progress, completes it at target; returns the goal after the change.
	AdvanceGoal(ctx context.Context, tenant, goal uuid.UUID, delta int, at time.Time) (ClubGoal, error)
}
