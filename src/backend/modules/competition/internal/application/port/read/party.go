// Package read holds the read-context ports of the party use cases (ADR-0087): read models only, under
// entx.ReadSession; no writes, no events.
package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// ReadSession runs fn in a READ ONLY transaction with the scope of ctx (entx.ReadSession).
type ReadSession interface {
	Read(ctx context.Context, fn func(ctx context.Context) error) error
}

// Invitation is a row of listMyInvitations with the name of its party.
type Invitation struct {
	Invitation party.Invitation
	PartyName  string
}

// PartyReader reads parties and invitations of the session tenant (RLS plus explicit tenant_id, deleted_at).
type PartyReader interface {
	// ActiveParty returns the active party of student, nil when none.
	ActiveParty(ctx context.Context, tenant, student uuid.UUID) (*party.Party, error)
	// Party returns a party with its active members, nil when unknown in tenant.
	Party(ctx context.Context, tenant, id uuid.UUID) (*party.Party, error)
	// Cards returns the public cards of the students of tenant.
	Cards(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]port.Card, error)
	// Invitations lists the invitations addressed to invitee, newest first, at most limit; status "" = every status,
	// "pending" = pending and not expired at now.
	Invitations(ctx context.Context, tenant, invitee uuid.UUID, status string, now time.Time, limit int) ([]Invitation, error)
}
