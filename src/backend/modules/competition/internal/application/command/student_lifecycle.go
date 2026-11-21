package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Results of the student lifecycle facade operations (StudentLifecycleResult / UserLifecycleResult status).
const (
	LifecycleDone        = "done"
	LifecycleNothingToDo = "nothing_to_do"
)

// PartyLifecycle implements the competition part of the identity sagas (F100 T042, OQ F016-2, F045-3; ADR-0032,
// ADR-0028): the cleanup and the anonymization take the student out of its party and end its invitations (memberships
// stay as history), the anonymization also resets the names of the parties the user led or created (free text typed
// by a student); a restore has nothing to bring back (a party is temporary); the merge moves the active membership of
// the source to the target (same role, the leader follows) or, when the target is already in a party, takes the source
// out. Every step is idempotent: a repeated call finds nothing left to do.
type PartyLifecycle struct{ Parties Parties }

func lifecycleTenant(ctx context.Context, op string) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, problem.New(problem.Internal, op+" needs a tenant scope")
	}
	tenant, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, problem.New(problem.Internal, op+" needs one tenant")
	}
	return tenant, nil
}

func outcome(changed bool) string {
	if changed {
		return LifecycleDone
	}
	return LifecycleNothingToDo
}

// Cleanup answers cleanupStudentData (student cleaned up for inactivity, ADR-0032).
func (h PartyLifecycle) Cleanup(ctx context.Context, student uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CleanupStudentData")
	defer func() { done(err) }()
	tenant, err := lifecycleTenant(ctx, "cleanupStudentData")
	if err != nil {
		return "", err
	}
	err = h.Parties.UoW.Do(ctx, func(ctx context.Context) error {
		changed, err := h.Parties.RemoveByPolicy(ctx, tenant, student, RealtimeAccountDisabled, "inactive_cleanup")
		status = outcome(changed)
		return err
	})
	return status, err
}

// Restore answers restoreStudentData: parties are temporary, nothing comes back.
func (h PartyLifecycle) Restore(ctx context.Context, _ uuid.UUID) (string, error) {
	if _, err := lifecycleTenant(ctx, "restoreStudentData"); err != nil {
		return "", err
	}
	return LifecycleNothingToDo, nil
}

// Anonymize answers anonymizeUserData: out of its party, invitations ended, names of the parties it led or created
// reset to the generated name without nickname.
func (h PartyLifecycle) Anonymize(ctx context.Context, user uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.AnonymizeUserData")
	defer func() { done(err) }()
	tenant, err := lifecycleTenant(ctx, "anonymizeUserData")
	if err != nil {
		return "", err
	}
	err = h.Parties.UoW.Do(ctx, func(ctx context.Context) error {
		removed, err := h.Parties.RemoveByPolicy(ctx, tenant, user, RealtimeAccountDisabled, "anonymized")
		if err != nil {
			return err
		}
		renamed, err := h.Parties.Repo.RenameLed(ctx, tenant, user, party.DefaultName(""))
		status = outcome(removed || renamed > 0)
		return err
	})
	return status, err
}

// Merge answers mergeUserData (user merge saga, idempotent per merge_id): the active membership of from moves to into
// (party.member_left policy + party.member_joined via merge), or from leaves when into is already in a party; the
// pending invitations of from end.
func (h PartyLifecycle) Merge(ctx context.Context, from, into uuid.UUID) (status string, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.MergeUserData")
	defer func() { done(err) }()
	tenant, err := lifecycleTenant(ctx, "mergeUserData")
	if err != nil {
		return "", err
	}
	if from == into {
		return "", problem.Invalid("target_user_id", problem.SubInvalid, "must differ from user_id")
	}
	p := h.Parties
	err = p.UoW.Do(ctx, func(ctx context.Context) error {
		now := p.now()
		fx := &effects{}
		if err := p.Repo.LockStudents(ctx, from, into); err != nil {
			return err
		}
		src, err := p.Repo.ActivePartyOf(ctx, tenant, from)
		if err != nil {
			return err
		}
		dst, err := p.Repo.ActivePartyOf(ctx, tenant, into)
		if err != nil {
			return err
		}
		changed := false
		if src != uuid.Nil && dst == uuid.Nil {
			pt, err := p.load(ctx, tenant, src)
			if err != nil {
				return err
			}
			m, err := pt.Reassign(from, into, now)
			if err != nil {
				return err
			}
			if err := p.Repo.EndMember(ctx, tenant, pt.ID, from, party.LeftPolicy, now); err != nil {
				return err
			}
			if err := p.Repo.AddMember(ctx, pt, m); err != nil {
				return err
			}
			if err := p.Repo.Save(ctx, pt); err != nil {
				return err
			}
			if err := p.history(ctx, pt, histLeft, nil, now, map[string]any{"student_id": from.String(), "reason": party.LeftPolicy}); err != nil {
				return err
			}
			if err := p.publish(ctx, "party.member_left", pt, now, map[string]any{"party_id": pt.ID, "tenant_id": tenant, "student_id": from,
				"reason": party.LeftPolicy, "state_version": pt.StateVersion}); err != nil {
				return err
			}
			if err := p.history(ctx, pt, histJoined, &into, now, map[string]any{"student_id": into.String(), "via": "merge"}); err != nil {
				return err
			}
			if err := p.publish(ctx, "party.member_joined", pt, now, map[string]any{"party_id": pt.ID, "tenant_id": tenant, "student_id": into,
				"role": m.Role, "via": "merge", "state_version": pt.StateVersion, "joined_at": m.JoinedAt}); err != nil {
				return err
			}
			cards, err := p.Directory.Cards(ctx, tenant, []uuid.UUID{into})
			if err != nil {
				return err
			}
			fx.send(pt.MemberIDs(), rtMemberJoined, map[string]any{"party_id": pt.ID, "state_version": pt.StateVersion,
				"member": view.MemberState(m, cardOf(cards, into), false)})
			fx.state(pt, from)
			changed = true
		}
		// from leaves any party left (target already in one) and its pending invitations end.
		removed, err := p.RemoveByPolicy(ctx, tenant, from, RealtimeAccountDisabled, "user_merge")
		if err != nil {
			return err
		}
		status = outcome(changed || removed)
		return p.after(ctx, fx)
	})
	return status, err
}
