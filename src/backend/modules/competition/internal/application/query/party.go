package query

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// InvitationsLimit bounds listMyInvitations (no cursor in the contract): party_pending_invites_max keeps the pending
// ones at 20 [Proposed F100].
const InvitationsLimit = 100

// Parties are the read use cases of the party (F100 US1, US2): always the party or invitations of the calling student
// in the session tenant; other parties are indistinguishable from unknown ones (404, F100-AC-06).
type Parties struct {
	Read     read.ReadSession
	Reader   read.PartyReader
	Presence port.PartyPresence // nil: presence unknown
	Now      func() time.Time

	// F101: lobby state (Ready, leader grace), running room, eligible activities.
	Lobby           port.LobbyStore     // nil: lobby not shown
	LobbyReader     read.LobbyReader    // nil: no running room, no eligible activities
	Game            port.GameActivities // nil: quest/boss not offered
	PrivateCapacity int
}

func (q Parties) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

func caller(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *p.TenantID, p.UserID, nil
}

func (q Parties) view(ctx context.Context, tenant uuid.UUID, p *party.Party) (view.Party, error) {
	cards, err := q.Reader.Cards(ctx, tenant, p.MemberIDs())
	if err != nil {
		return view.Party{}, err
	}
	var online map[uuid.UUID]bool
	if q.Presence != nil {
		if online, err = q.Presence.Online(ctx, p.MemberIDs()); err != nil {
			slog.WarnContext(ctx, "competition: party presence unavailable", "error", err)
			online = nil
		}
	}
	v := view.NewParty(*p, cards, online)
	if q.Lobby != nil {
		l, err := q.Lobby.Load(ctx, p.ID)
		if err != nil {
			slog.WarnContext(ctx, "competition: lobby state unavailable", "error", err) // rebuilt: Ready false (F101-AC-06)
			l = party.Lobby{}
		}
		v.Lobby = &l
	}
	if q.LobbyReader != nil && p.Status == party.StatusInActivity {
		if v.Room, err = q.LobbyReader.RunningRoom(ctx, tenant, p.ID); err != nil {
			return view.Party{}, err
		}
	}
	return v, nil
}

// GetMyParty implements getMyParty: the active party of the caller, 404 PARTY_NOT_FOUND when none.
func (q Parties) GetMyParty(ctx context.Context) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetMyParty")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return res, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		p, err := q.Reader.ActiveParty(ctx, tenant, me)
		if err != nil {
			return err
		}
		if p == nil {
			return problem.New(problem.PartyNotFound, "")
		}
		res, err = q.view(ctx, tenant, p)
		return err
	})
	return res, err
}

// GetParty implements getParty (F100-AC-06): members of the party only; anyone else, another school or tenant → 404.
func (q Parties) GetParty(ctx context.Context, id uuid.UUID) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetParty")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return res, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		p, err := q.Reader.Party(ctx, tenant, id)
		if err != nil {
			return err
		}
		if p == nil || p.RequireMember(me) != nil {
			return problem.New(problem.PartyNotFound, "")
		}
		res, err = q.view(ctx, tenant, p)
		return err
	})
	return res, err
}

// ListMyInvitations implements listMyInvitations (F100-AC-15): the invitations addressed to the caller only; the
// inviter as a public card.
func (q Parties) ListMyInvitations(ctx context.Context, status string) (res []view.Invitation, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListMyInvitations")
	defer func() { done(err) }()
	tenant, me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	switch status {
	case "", party.InvitationPending, party.InvitationAccepted, party.InvitationRejected, party.InvitationExpired, party.InvitationCancelled:
	default:
		return nil, problem.Invalid("status", problem.SubInvalid, "unknown invitation status")
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Reader.Invitations(ctx, tenant, me, status, q.now(), InvitationsLimit)
		if err != nil {
			return err
		}
		inviters := make([]uuid.UUID, 0, len(rows))
		for _, r := range rows {
			inviters = append(inviters, r.Invitation.InviterID)
		}
		cards, err := q.Reader.Cards(ctx, tenant, inviters)
		if err != nil {
			return err
		}
		res = make([]view.Invitation, 0, len(rows))
		for _, r := range rows {
			c, ok := cards[r.Invitation.InviterID]
			if !ok {
				c = port.Card{UserID: r.Invitation.InviterID, Level: 1}
			}
			res = append(res, view.Invitation{Invitation: r.Invitation, PartyName: r.PartyName, Inviter: c})
		}
		return nil
	})
	return res, err
}
