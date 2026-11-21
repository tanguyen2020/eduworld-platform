package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CreateInput is the PartyCreate of the contract.
type CreateInput struct {
	Name     string
	Type     string
	Settings party.Settings // visibility, invitation_policy, max_members (nil: default)
}

// moderation of a typed name (F100-AC-05, research R2): blocked → 422 without the matched words; module chat silent →
// the generated default name (useDefault).
func (h Parties) moderate(ctx context.Context, tenant, me uuid.UUID, name string) (useDefault bool, err error) {
	if h.Moderator == nil {
		return true, nil
	}
	ok, err := h.Moderator.Check(ctx, tenant, me, name)
	if err != nil {
		// Module chat silent or failing: never store an unchecked name (research R2).
		if !errors.Is(err, port.ErrModerationUnavailable) {
			slog.WarnContext(ctx, "competition: party name moderation failed", "error", err)
		}
		return true, nil //nolint:nilerr // the fallback is the contract (F100-AC-05), not an error
	}
	switch {
	case !ok:
		return false, problem.Invalid("name", problem.SubInvalid, "content not allowed")
	}
	return false, nil
}

// nickname returns the nickname of student for the default name ("" when no profile yet).
func (h Parties) nickname(ctx context.Context, tenant, student uuid.UUID) (string, error) {
	cards, err := h.Directory.Cards(ctx, tenant, []uuid.UUID{student})
	if err != nil {
		return "", err
	}
	if c, ok := cards[student]; ok && c.Nickname != nil {
		return *c.Nickname, nil
	}
	return "", nil
}

// CreateParty implements createParty (F100-AC-01…05): the caller becomes the leader of an open party; tenant, school
// and grade band come from the projected profile of the session, never from the body.
func (h Parties) CreateParty(ctx context.Context, in CreateInput) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CreateParty")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	name, err := party.CleanName(in.Name)
	if err != nil {
		return res, h.finish(ctx, CmdCreate, err)
	}
	if !party.ValidType(in.Type) {
		return res, h.finish(ctx, CmdCreate, party.ValidationError{Field: "party_type", Reason: problem.SubInvalid, Message: "unknown party type"})
	}
	useDefault, err := h.moderate(ctx, tenant, me, name)
	if err != nil {
		return res, h.finish(ctx, CmdCreate, err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		sub, err := h.caller(ctx, tenant, me, now)
		if err != nil {
			return err
		}
		if _, err := h.scope(ctx, tenant); err != nil {
			return err
		}
		if sub.Band == "" { // no grade projected: no grade band to place the party in
			return h.policyDenied(party.PolicyError{Rule: party.RuleGradeBand}, nil, me, CmdCreate)
		}
		rules, err := h.rules(ctx, tenant, sub.Band, sub.SchoolID)
		if err != nil {
			return err
		}
		maxMembers := min(h.Config.DefaultMaxMembers, rules.MaxMembersCap)
		if in.Settings.MaxMembers != nil {
			maxMembers = *in.Settings.MaxMembers
		}
		if err := party.CheckCreate(rules, sub, in.Type, maxMembers); err != nil {
			return h.policyDenied(err, nil, me, CmdCreate)
		}
		if err := h.Repo.LockStudents(ctx, me); err != nil {
			return err
		}
		if err := h.createRate(ctx, tenant, me, now); err != nil {
			return err
		}
		if active, err := h.Repo.ActivePartyOf(ctx, tenant, me); err != nil {
			return err
		} else if active != uuid.Nil {
			return write.ErrAlreadyInParty
		}
		if useDefault {
			nick, err := h.nickname(ctx, tenant, me)
			if err != nil {
				return err
			}
			name = party.DefaultName(nick)
		}
		s := in.Settings
		s.MaxMembers = &maxMembers
		p, err := party.New(newID(), tenant, sub.SchoolID, me, in.Type, sub.Band, name, s, maxMembers, now)
		if err != nil {
			return err
		}
		if err := h.Repo.Create(ctx, p); err != nil {
			return err
		}
		// Being in a party ends the invitations the creator was waiting on (F100-AC-14).
		if err := h.endPending(ctx, fx, tenant, write.PendingFilter{InviteeID: &me}, party.InvitationExpired, party.EndJoinedOtherParty,
			nil, now); err != nil {
			return err
		}
		if err := h.history(ctx, p, histCreated, &me, now, map[string]any{"party_type": p.Type, "max_members": p.MaxMembers}); err != nil {
			return err
		}
		if err := h.publish(ctx, "party.created", p, now, map[string]any{"party_id": p.ID, "tenant_id": tenant, "school_id": p.SchoolID,
			"leader_id": me, "party_type": p.Type, "visibility": p.Visibility, "invitation_policy": p.InvitationPolicy,
			"max_members": p.MaxMembers, "grade_band": p.GradeBand, "state_version": p.StateVersion, "created_at": p.CreatedAt}); err != nil {
			return err
		}
		if err := h.record(ctx, AuditPartyCreated, p, me, "", map[string]any{"party_type": p.Type}); err != nil {
			return err
		}
		if res, err = h.view(ctx, p); err != nil {
			return err
		}
		if err := idempotency.SetResult(ctx, res.Body()); err != nil {
			return err
		}
		fx.state(p)
		return h.after(ctx, fx)
	})
	return res, h.finish(ctx, CmdCreate, err)
}

// createRate is party_create_rate (10 creations or disbands per hour): 429 RATE_LIMITED with Retry-After.
func (h Parties) createRate(ctx context.Context, tenant, me uuid.UUID, now time.Time) error {
	if h.Config.CreatePerHour <= 0 {
		return nil
	}
	times, err := h.Repo.HistorySince(ctx, tenant, me, []string{histCreated, histDisbanded}, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	if len(times) < h.Config.CreatePerHour {
		return nil
	}
	after := retryAt(times[len(times)-h.Config.CreatePerHour], time.Hour, now)
	return denied{err: RetryAfter{Err: problem.New(problem.RateLimited, "").Retry(), After: after}, action: AuditRateLimited,
		target: me, reason: "party_create_rate"}
}

// UpdatePartySettings implements updatePartySettings (F100-AC-21): leader only; max_members between the current
// members and the cap of the policies; a typed name goes through moderation like at creation.
func (h Parties) UpdatePartySettings(ctx context.Context, partyID uuid.UUID, s party.Settings) (res view.Party, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.UpdatePartySettings")
	defer func() { done(err) }()
	tenant, me, err := student(ctx)
	if err != nil {
		return res, err
	}
	useDefault := false
	if s.Name != nil {
		name, err := party.CleanName(*s.Name)
		if err != nil {
			return res, h.finish(ctx, CmdSettings, err)
		}
		s.Name = &name
		if useDefault, err = h.moderate(ctx, tenant, me, name); err != nil {
			return res, h.finish(ctx, CmdSettings, err)
		}
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		now := h.now()
		fx := &effects{}
		if _, err := h.caller(ctx, tenant, me, now); err != nil {
			return err
		}
		p, err := h.load(ctx, tenant, partyID)
		if err != nil {
			return err
		}
		if err := p.RequireLeader(me); err != nil {
			return err
		}
		if useDefault {
			nick, err := h.nickname(ctx, tenant, me)
			if err != nil {
				return err
			}
			s.Name = ptr(party.DefaultName(nick))
		}
		rules, err := h.rules(ctx, tenant, p.GradeBand, p.SchoolID)
		if err != nil {
			return err
		}
		changed, err := p.Update(s, rules.MaxMembersCap, now)
		if err != nil {
			return err
		}
		if len(changed) > 0 {
			if err := h.Repo.Save(ctx, p); err != nil {
				return err
			}
			if err := h.history(ctx, p, histSettings, &me, now, map[string]any{"changed_fields": changed}); err != nil {
				return err
			}
			if err := h.publish(ctx, "party.settings_updated", p, now, map[string]any{"party_id": p.ID, "tenant_id": tenant,
				"actor_id": me, "changed_fields": changed, "state_version": p.StateVersion}); err != nil {
				return err
			}
		}
		if res, err = h.view(ctx, p); err != nil {
			return err
		}
		if len(changed) > 0 {
			fx.send(p.MemberIDs(), rtSnapshot, res.Snapshot())
			fx.state(p)
		}
		return h.after(ctx, fx)
	})
	return res, h.finish(ctx, CmdSettings, err)
}
