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
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PartyConfig is the configuration of the party use cases (CONFIG_DEFAULTS party_max_members, party_invite_ttl,
// party_invite_rate, party_invite_repeat_rules, party_pending_invites_max, party_create_rate, party_kick_rate,
// party_kick_reinvite_block); read by infrastructure/config, never hard-coded.
type PartyConfig struct {
	DefaultMaxMembers int
	InviteTTL         time.Duration
	InvitePerMinute   int
	InvitePerDay      int
	RepeatWait        time.Duration
	RejectsPerDay     int
	PendingMax        int
	CreatePerHour     int
	KickPerHour       int
	KickReinviteBlock time.Duration
}

// Audit actions of the party use cases (ids only, never names or chat content — PARTY_SYSTEM §11).
const (
	AuditPartyCreated        = "party.created"
	AuditPartyDisbanded      = "party.disbanded"
	AuditMemberKicked        = "party.member_kicked"
	AuditLeaderTransferred   = "party.leader_transferred"
	AuditPolicyDenied        = "party.invitation_policy_denied"
	AuditInviteRateLimited   = "party.invite_rate_limited"
	AuditMemberRemovedPolicy = "party.member_removed_by_policy"
	AuditRateLimited         = "party.rate_limited"
)

// Commands of eduworld_party_command_total.
const (
	CmdCreate   = "create"
	CmdSettings = "update_settings"
	CmdInvite   = "invite"
	CmdAccept   = "accept"
	CmdReject   = "reject"
	CmdCancel   = "cancel"
	CmdJoin     = "join"
	CmdLeave    = "leave"
	CmdKick     = "kick"
	CmdTransfer = "transfer_leader"
	CmdDisband  = "disband"
)

// History event types (multiplayer.party_histories.event_type).
const (
	histCreated     = "created"
	histJoined      = "member_joined"
	histLeft        = "member_left"
	histKicked      = "member_kicked"
	histTransferred = "leader_transferred"
	histSettings    = "settings_updated"
	histDisbanded   = "disbanded"
)

// Realtime message types (AsyncAPI, EVENT_DESIGN §5).
const (
	rtInvitationReceived = "party.invitation.received"
	rtInvitationUpdated  = "party.invitation.updated"
	rtMemberJoined       = "party.member.joined"
	rtMemberLeft         = "party.member.left"
	rtLeaderTransferred  = "party.leader.transferred"
	rtSnapshot           = "party.state.snapshot"
	rtDisbanded          = "party.party.disbanded"
)

// RetryAfter wraps a 429 problem with the delay of the Retry-After header.
type RetryAfter struct {
	Err   error
	After time.Duration
}

func (e RetryAfter) Error() string { return e.Err.Error() }
func (e RetryAfter) Unwrap() error { return e.Err }

// Parties are the write use cases of the party (F100 US1–US4). Every command runs for the calling student in one
// UnitOfWork with its outbox events, party history and audit; realtime pushes and the Valkey state follow the commit.
type Parties struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.PartyRepository
	Directory   write.Directory
	Events      write.EventPublisher
	Audit       write.Auditor
	Moderator   port.NameModerator   // nil: module chat offers no moderation → default name (F100-AC-05)
	Presence    port.PartyPresence   // nil: nobody online
	Realtime    port.PartyRealtime   // nil: no push
	Cache       port.PartyStateCache // nil: no Valkey state
	Metrics     port.PartyMetrics    // nil: no metrics
	Config      PartyConfig
	Now         func() time.Time
}

func (h Parties) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Parties) metric(cmd string, err error) {
	if h.Metrics == nil {
		return
	}
	result := "ok"
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) {
			result = string(pe.Code)
		} else {
			result = "error"
		}
	}
	h.Metrics.Command(cmd, result)
}

// student returns the tenant and id of the calling student. The game_player guard already refused anyone else; the
// check is repeated so a use case is never reachable without it (R23).
func student(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return uuid.Nil, uuid.Nil, problem.New(problem.Unauthenticated, "")
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return uuid.Nil, uuid.Nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *p.TenantID, p.UserID, nil
}

// caller returns the policy view of the calling student: 503 retryable until projected (edge case), refused when not
// active.
func (h Parties) caller(ctx context.Context, tenant, me uuid.UUID, now time.Time) (party.Subject, error) {
	s, err := h.Directory.Subject(ctx, tenant, me, now)
	if err != nil {
		return party.Subject{}, err
	}
	if s == nil {
		return party.Subject{}, problem.New(problem.Unavailable, "student not projected yet").Retry()
	}
	if !s.Active {
		return party.Subject{}, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *s, nil
}

// subject returns the policy view of another student (inactive when unknown in the tenant).
func (h Parties) subject(ctx context.Context, tenant, id uuid.UUID, now time.Time) (party.Subject, error) {
	s, err := h.Directory.Subject(ctx, tenant, id, now)
	if err != nil {
		return party.Subject{}, err
	}
	if s == nil {
		return party.Subject{ID: id}, nil
	}
	return *s, nil
}

// scope returns the tenant side of the policies (503 retryable until identity projected the tenant).
func (h Parties) scope(ctx context.Context, tenant uuid.UUID) (party.Scope, error) {
	t, err := h.Directory.Tenant(ctx, tenant)
	if err != nil {
		return party.Scope{}, err
	}
	if t == nil {
		return party.Scope{}, problem.New(problem.Unavailable, "tenant not projected yet").Retry()
	}
	return party.Scope{CrossSchool: t.Type == "unit" && t.CrossSchool}, nil
}

// rules are the effective policies of a grade band in a school, tightened by the class policies of the students.
func (h Parties) rules(ctx context.Context, tenant uuid.UUID, band string, school uuid.UUID, subjects ...party.Subject) (party.Rules, error) {
	var classes []uuid.UUID
	for _, s := range subjects {
		if s.ClassID != nil {
			classes = append(classes, *s.ClassID)
		}
	}
	r := party.PlatformRules(band, party.MaxMembers)
	os, err := h.Directory.Overrides(ctx, tenant, school, classes)
	if err != nil {
		return r, err
	}
	for _, o := range os {
		r = r.Tighten(o)
	}
	return r, nil
}

// load returns an active-or-disbanded party of the tenant locked for the command; 404 when unknown.
func (h Parties) load(ctx context.Context, tenant, id uuid.UUID) (*party.Party, error) {
	p, err := h.Repo.Load(ctx, tenant, id, true)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, problem.New(problem.PartyNotFound, "")
	}
	return p, nil
}

func (h Parties) online(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]bool {
	if h.Presence == nil || len(ids) == 0 {
		return map[uuid.UUID]bool{}
	}
	m, err := h.Presence.Online(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "competition: party presence unavailable", "error", err) // ephemeral: nobody online
		return map[uuid.UUID]bool{}
	}
	return m
}

// view builds the Party of the contract with the cards and presence of the members.
func (h Parties) view(ctx context.Context, p *party.Party) (view.Party, error) {
	cards, err := h.Directory.Cards(ctx, p.TenantID, p.MemberIDs())
	if err != nil {
		return view.Party{}, err
	}
	return view.NewParty(*p, cards, h.online(ctx, p.MemberIDs())), nil
}

func ptr[T any](v T) *T { return &v }

func (h Parties) publish(ctx context.Context, typ string, p *party.Party, at time.Time, payload map[string]any) error {
	t := p.TenantID
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "party", SubjectID: p.ID.String(), PartitionKey: p.ID.String(),
		TenantID: &t, OccurredAt: at, Payload: payload})
}

func (h Parties) history(ctx context.Context, p *party.Party, typ string, actor *uuid.UUID, at time.Time, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	data["state_version"] = p.StateVersion
	return h.Repo.AppendHistory(ctx, write.History{TenantID: p.TenantID, SchoolID: p.SchoolID, PartyID: p.ID, EventType: typ,
		ActorID: actor, Data: data, At: at})
}

func (h Parties) record(ctx context.Context, action string, p *party.Party, target uuid.UUID, reason string, meta map[string]any) error {
	return h.recordResult(ctx, action, audit.ResultSuccess, p, target, reason, meta)
}

func (h Parties) recordResult(ctx context.Context, action string, result audit.Result, p *party.Party, target uuid.UUID, reason string,
	meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	school := ""
	if p != nil {
		meta["party_id"] = p.ID.String()
		school = p.SchoolID.String()
	}
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "student", TargetID: target.String(), Result: result,
		Reason: reason, SchoolID: school, Metadata: meta})
}

// effects are what follows the commit: realtime pushes and the Valkey state (best effort, ADR-0004).
type effects struct {
	pushes []push
	sync   *port.PartySnapshot
	drop   *dropState
}

type push struct {
	to      []uuid.UUID
	typ     string
	payload any
}

type dropState struct {
	party    uuid.UUID
	students []uuid.UUID
}

func (fx *effects) send(to []uuid.UUID, typ string, payload any) {
	if len(to) == 0 {
		return
	}
	fx.pushes = append(fx.pushes, push{to: append([]uuid.UUID(nil), to...), typ: typ, payload: payload})
}

func snapshot(p *party.Party) *port.PartySnapshot {
	roles := make(map[uuid.UUID]string, len(p.Members))
	for _, m := range p.Members {
		roles[m.StudentID] = m.Role
	}
	return &port.PartySnapshot{PartyID: p.ID, LeaderID: p.LeaderID, Type: p.Type, MaxMembers: p.MaxMembers, Status: p.Status,
		StateVersion: p.StateVersion, Members: roles}
}

// state records the Valkey state of p after the commit: synced while active, dropped (with the students who left)
// when disbanded.
func (fx *effects) state(p *party.Party, left ...uuid.UUID) {
	if p.Active() {
		fx.sync = snapshot(p)
		if len(left) > 0 {
			fx.drop = &dropState{students: left}
		}
		return
	}
	fx.sync = nil
	fx.drop = &dropState{party: p.ID, students: left}
}

// after registers the effects to run after the commit of the transaction of ctx.
func (h Parties) after(ctx context.Context, fx *effects) error {
	if h.AfterCommit == nil || fx == nil {
		return nil
	}
	return h.AfterCommit(ctx, func(ctx context.Context) {
		if h.Realtime != nil {
			for _, p := range fx.pushes {
				if err := h.Realtime.Send(ctx, p.to, p.typ, p.payload); err != nil {
					slog.WarnContext(ctx, "competition: party push failed", "type", p.typ, "error", err) // client resyncs with getMyParty
				}
			}
		}
		if h.Cache == nil {
			return
		}
		if fx.drop != nil {
			if err := h.Cache.Drop(ctx, fx.drop.party, fx.drop.students); err != nil {
				slog.WarnContext(ctx, "competition: party state not dropped", "error", err)
			}
		}
		if fx.sync != nil {
			if err := h.Cache.Sync(ctx, *fx.sync); err != nil {
				slog.WarnContext(ctx, "competition: party state not synced", "error", err) // rebuilt from PostgreSQL
			}
		}
	})
}

// denied is a refusal that is audited in its own transaction (the command transaction rolls back).
type denied struct {
	err    error
	action string
	party  *party.Party
	target uuid.UUID
	reason string
	meta   map[string]any
}

func (d denied) Error() string { return d.err.Error() }
func (d denied) Unwrap() error { return d.err }

// finish maps the error of a command to the contract and records the audit of a refusal after the rollback
// (PARTY_SYSTEM §11: invitations refused by policy, rate limits).
func (h Parties) finish(ctx context.Context, cmd string, err error) error {
	if err == nil {
		h.metric(cmd, nil)
		return nil
	}
	var d denied
	if errors.As(err, &d) {
		rec := h.UoW.Do(ctx, func(ctx context.Context) error {
			return h.recordResult(ctx, d.action, audit.ResultDenied, d.party, d.target, d.reason, d.meta)
		})
		if rec != nil {
			slog.WarnContext(ctx, "competition: party refusal not audited", "action", d.action, "error", rec)
		}
		err = d.err
	}
	err = mapError(err)
	h.metric(cmd, err)
	return err
}

// mapError maps the domain errors to the problem codes of the contract.
func mapError(err error) error {
	var ve party.ValidationError
	var pe party.PolicyError
	switch {
	case errors.As(err, &ve):
		return problem.Invalid(ve.Field, ve.Reason, ve.Message)
	case errors.As(err, &pe):
		return problem.New(problem.PartyPolicyViolation, "")
	case errors.Is(err, party.ErrNotMember), errors.Is(err, party.ErrDisbanded):
		return problem.New(problem.PartyNotFound, "")
	case errors.Is(err, party.ErrNotLeader):
		return problem.New(problem.PartyNotLeader, "")
	case errors.Is(err, party.ErrFull):
		return problem.New(problem.PartyFull, "")
	case errors.Is(err, party.ErrInviteNotAllowed):
		return problem.New(problem.PartyInviteNotAllowed, "")
	case errors.Is(err, party.ErrExpired):
		return problem.New(problem.PartyInvitationExpired, "")
	case errors.Is(err, party.ErrNotInvitee), errors.Is(err, party.ErrCannotCancel):
		return problem.New(problem.PermissionDenied, "")
	case errors.Is(err, party.ErrNotPending):
		return problem.New(problem.NotFound, "")
	case errors.Is(err, write.ErrAlreadyInParty):
		return problem.New(problem.PartyAlreadyInParty, "")
	case errors.Is(err, write.ErrPendingExists):
		return problem.New(problem.Conflict, "a pending invitation exists")
	}
	return err
}

// policyDenied wraps a policy refusal of an invitation or join with its audit and metric (F100-AC-09).
func (h Parties) policyDenied(err error, p *party.Party, target uuid.UUID, op string) error {
	var pe party.PolicyError
	if errors.As(err, &pe) {
		if h.Metrics != nil {
			h.Metrics.PolicyDenied(pe.Rule)
		}
		return denied{err: err, action: AuditPolicyDenied, party: p, target: target, reason: pe.Rule, meta: map[string]any{"operation": op}}
	}
	if errors.Is(err, party.ErrInviteNotAllowed) {
		return denied{err: err, action: AuditPolicyDenied, party: p, target: target, reason: "invite_not_allowed",
			meta: map[string]any{"operation": op}}
	}
	return err
}

// endPending ends the pending invitations selected by f (expired or cancelled with reason): events, pushes to both
// sides. by is the student who cancelled (nil: the system).
func (h Parties) endPending(ctx context.Context, fx *effects, tenant uuid.UUID, f write.PendingFilter, status, reason string,
	by *uuid.UUID, now time.Time) error {
	invs, err := h.Repo.Pending(ctx, tenant, f)
	if err != nil {
		return err
	}
	for i := range invs {
		inv := &invs[i]
		if status == party.InvitationExpired {
			inv.Expire(reason, now)
		} else {
			inv.Cancel(reason, now)
		}
		if err := h.Repo.SaveInvitation(ctx, inv); err != nil {
			return err
		}
		if err := h.invitationEvent(ctx, inv, by, now); err != nil {
			return err
		}
		fx.send([]uuid.UUID{inv.InviteeID, inv.InviterID}, rtInvitationUpdated, view.InvitationUpdated(*inv))
		if h.Metrics != nil {
			h.Metrics.Invitation(inv.Status)
		}
	}
	return nil
}

// invitationEvent publishes the event of an invitation that changed status.
func (h Parties) invitationEvent(ctx context.Context, inv *party.Invitation, by *uuid.UUID, now time.Time) error {
	payload := map[string]any{"invitation_id": inv.ID, "party_id": inv.PartyID, "tenant_id": inv.TenantID, "inviter_id": inv.InviterID,
		"invitee_id": inv.InviteeID}
	typ := "party.invitation_" + inv.Status
	switch inv.Status {
	case party.InvitationExpired, party.InvitationCancelled:
		payload["reason"] = inv.EndReason
		if by != nil {
			payload["cancelled_by"] = *by
		}
	case party.InvitationPending:
		typ = "party.invitation_sent"
		payload["expires_at"] = inv.ExpiresAt
	}
	t := inv.TenantID
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "party_invitation", SubjectID: inv.ID.String(),
		PartitionKey: inv.PartyID.String(), TenantID: &t, OccurredAt: now, Payload: payload})
}

// newMember is a membership of student starting now.
func newMember(s party.Subject, inv *uuid.UUID, now time.Time) party.Member {
	return party.Member{StudentID: s.ID, SchoolID: s.SchoolID, Role: party.RoleMember, JoinedAt: now, InvitationID: inv}
}

// retryAt is the Retry-After of a sliding window: when the oldest counted time leaves it.
func retryAt(oldest time.Time, window time.Duration, now time.Time) time.Duration {
	d := oldest.Add(window).Sub(now)
	if d < time.Second {
		d = time.Second
	}
	return d
}

func newID() uuid.UUID { return ids.New() }

type portCard = port.Card
