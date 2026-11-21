package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
)

// Sanction types that keep a student out of party and shared multiplayer (MODERATION §7).
var restrictsMultiplayer = map[string]bool{"restrict_multiplayer": true, "account_lock": true}

// RestrictsMultiplayer reports a sanction type that keeps a student out of parties.
func RestrictsMultiplayer(typ string) bool { return restrictsMultiplayer[typ] }

// PartySafety applies the safety changes of other modules to parties, inside the transaction the consumer opened with
// the event scope (consumer competition.party_safety, F100-AC-25 and edge cases): blocks cancel the invitations
// between the two students, a multiplayer sanction, a school change, a disabled account or a cleanup take the student
// out of its party and end its invitations.
type PartySafety struct {
	Parties   Parties
	Sanctions write.SanctionWriter
}

// Blocked applies user.blocked: the pending invitations between blocker and blocked are cancelled; the next
// invitations and joins are refused by the block projection (ref.blocks, consumer competition.projection_social).
func (s PartySafety) Blocked(ctx context.Context, tenant, blocker, blocked uuid.UUID) (bool, error) {
	return s.Parties.CancelBetween(ctx, tenant, blocker, blocked)
}

// Sanction applies sanction.applied / sanction.lifted: ref.sanctions keeps the multiplayer sanctions (read by every
// later check); an applied one in force removes the student from its party (left_reason policy).
func (s PartySafety) Sanction(ctx context.Context, tenant, sanction, user uuid.UUID, typ string, until *time.Time, active bool,
	version int64) (bool, error) {
	if !RestrictsMultiplayer(typ) {
		return false, nil
	}
	changed, err := s.Sanctions.Sanction(ctx, tenant, sanction, user, typ, until, active, version)
	if err != nil || !changed || !active || (until != nil && !until.After(s.Parties.now())) {
		return changed, err
	}
	_, err = s.Parties.RemoveByPolicy(ctx, tenant, user, RealtimeAccountDisabled, "sanction_"+typ)
	return true, err
}

// StudentSynced applies student.synced: a student moved to another school or deactivated leaves its party.
func (s PartySafety) StudentSynced(ctx context.Context, tenant, student, school uuid.UUID, status string) (bool, error) {
	if status != "" && status != "active" {
		return s.Parties.RemoveByPolicy(ctx, tenant, student, RealtimeAccountDisabled, "student_"+status)
	}
	ids, err := s.Parties.Repo.MembershipsOf(ctx, tenant, student)
	if err != nil || len(ids) == 0 {
		return false, err
	}
	for _, id := range ids {
		p, err := s.Parties.Repo.Load(ctx, tenant, id, false)
		if err != nil {
			return false, err
		}
		if p != nil && p.Active() && p.SchoolID != school {
			sc, err := s.Parties.scope(ctx, tenant)
			if err != nil {
				return false, err
			}
			if sc.CrossSchool {
				continue
			}
			return s.Parties.RemoveByPolicy(ctx, tenant, student, RealtimeTransferredSchool, "school_changed")
		}
	}
	return false, nil
}

// StatusChanged applies user.status_changed: a locked, deactivated or anonymized account leaves its party. tenant nil
// (platform event): the owning tenant of the projected student.
func (s PartySafety) StatusChanged(ctx context.Context, tenant *uuid.UUID, user uuid.UUID, to string) (bool, error) {
	if to == "active" || to == "pending" {
		return false, nil
	}
	if tenant == nil {
		owner, err := s.Sanctions.OwnerTenant(ctx, user)
		if err != nil || owner == nil {
			return false, err
		}
		tenant = owner
	}
	return s.Parties.RemoveByPolicy(ctx, *tenant, user, RealtimeAccountDisabled, "account_"+to)
}

// CleanedUp applies student.cleaned_up (ADR-0032): the cleaned up student leaves its party.
func (s PartySafety) CleanedUp(ctx context.Context, tenant, student uuid.UUID) (bool, error) {
	return s.Parties.RemoveByPolicy(ctx, tenant, student, RealtimeAccountDisabled, party.LeftPolicy+"_cleanup")
}
