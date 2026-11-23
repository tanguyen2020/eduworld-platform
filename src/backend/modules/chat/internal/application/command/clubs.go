package command

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Events and audit actions of the clubs (EVENT_DESIGN §4.9; ids only, no name or description).
const (
	EventClubCreated      = "club.created"
	EventClubApproved     = "club.approved"
	EventClubArchived     = "club.archived"
	EventClubMemberJoined = "club.member_joined"
	EventClubMemberLeft   = "club.member_left"
	EventClubGoalComplete = "club.goal_completed"
	AuditClubCreated      = "club.created"
	AuditClubApproved     = "club.approved"
	AuditClubArchived     = "club.archived"
	AuditClubMemberRemove = "club.member_removed"
)

// ClubInput is the body of createClub / updateClub (ClubCreate / ClubUpdate).
type ClubInput struct {
	Type        club.Type
	Name        *string
	Description *string
	IconID      *string
	SubjectID   *uuid.UUID
}

// GoalInput is ClubGoalCreate; EventInput is ClubEventCreate.
type GoalInput struct {
	Title     *string
	Metric    string
	SubjectID *uuid.UUID
	Target    int
}

// EventInput is ClubEventCreate.
type EventInput struct {
	Title                string
	Description          *string
	Kind                 string
	TournamentTemplateID *uuid.UUID
	StartsAt             time.Time
	EndsAt               *time.Time
}

// ClubView is a club with the role of the caller (Club of the contract).
type ClubView struct {
	Club   club.Club
	MyRole string
	Staff  bool // rendered for staff: school_id included
}

// Clubs are the write use cases of the clubs (F120 US4, SOCIAL_SYSTEM §6): every change runs in one UnitOfWork with its
// outbox events and audit; the membership of the fixed team channel of the club follows in the same transaction
// (F120-AC-20). Students act through the game client (game_player); staff of the school approve and archive (portal,
// social.club.approve). Names, descriptions and event titles go through the moderation pipeline (F120-AC-17).
type Clubs struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Store       write.ClubStore
	Social      write.SocialRepository
	Directory   write.Directory
	Channels    write.ChannelRepository
	Status      write.StatusRepository
	Names       port.DisplayTextChecker
	Events      write.EventPublisher
	Audit       write.Auditor
	Facts       FactsInvalidator    // nil: no cache
	Push        port.MembershipPush // nil: no push
	Limits      club.Limits
	Now         func() time.Time
}

func (h Clubs) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// clubActor is the caller of a club use case: a student of the game client, or staff of the portal.
type clubActor struct {
	p       authz.Principal
	tenant  uuid.UUID
	student *social.Student // nil for staff
}

func (h Clubs) actor(ctx context.Context) (clubActor, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() || p.TenantID == nil {
		return clubActor{}, problem.New(problem.Unauthenticated, "")
	}
	a := clubActor{p: p, tenant: *p.TenantID}
	switch {
	case p.UserType == authz.UserTypeStudent && p.Channel == authz.ChannelGame:
		return a, nil
	case p.UserType == authz.UserTypeStaff && p.Channel == authz.ChannelPortal:
		return a, nil
	case p.UserType == authz.UserTypeStudent:
		return clubActor{}, problem.New(problem.GameAccessStudentOnly, "")
	}
	return clubActor{}, problem.New(problem.PortalAccessStaffOnly, "")
}

func (a clubActor) isStudent() bool { return a.p.UserType == authz.UserTypeStudent }

// loadStudent resolves the calling student inside the transaction (503 retryable until projected).
func (h Clubs) loadStudent(ctx context.Context, a *clubActor) error {
	if !a.isStudent() {
		return nil
	}
	s, err := h.Directory.Student(ctx, a.tenant, a.p.UserID)
	if err != nil {
		return err
	}
	if s == nil {
		return problem.New(problem.Unavailable, "student not projected yet").Retry()
	}
	if !s.Active() {
		return problem.New(problem.GameAccessStudentOnly, "")
	}
	a.student = s
	return nil
}

func clubProblem(err error) error {
	switch {
	case errors.Is(err, club.ErrType), errors.Is(err, club.ErrName):
		field := "club_type"
		if errors.Is(err, club.ErrName) {
			field = "name"
		}
		return problem.Invalid(field, problem.SubInvalid, err.Error())
	case errors.Is(err, club.ErrDisabled), errors.Is(err, club.ErrStudentType):
		return problem.New(problem.PermissionDenied, "")
	case errors.Is(err, club.ErrNotApproved):
		return problem.New(problem.ClubNotApproved, "")
	case errors.Is(err, club.ErrNotActive):
		return problem.New(problem.Conflict, "club is not active")
	case errors.Is(err, club.ErrFull):
		return problem.New(problem.ClubFull, "")
	case errors.Is(err, club.ErrLimitReached):
		return problem.New(problem.ClubLimitReached, "")
	case errors.Is(err, club.ErrOwnerLeaves):
		return problem.New(problem.Conflict, "the owner archives the club instead of leaving")
	}
	return err
}

func scopeViolation() error { return problem.New(problem.SocialScopeViolation, "") }

// clean runs a text typed by a student through the moderation pipeline: blocked → 451 CHAT_CONTENT_BLOCKED (no echo).
func (h Clubs) clean(ctx context.Context, tenant uuid.UUID, texts ...*string) error {
	for _, t := range texts {
		if t == nil || strings.TrimSpace(*t) == "" {
			continue
		}
		if h.Names == nil {
			return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
		}
		ok, err := h.Names.CheckDisplayText(ctx, tenant, *t)
		if err != nil {
			slog.WarnContext(ctx, "chat: club text not checked", "error", err)
			return problem.New(problem.ChatTemporarilyUnavailable, "").Retry()
		}
		if !ok {
			return problem.New(problem.ChatContentBlocked, "")
		}
	}
	return nil
}

func (h Clubs) publish(ctx context.Context, typ string, c club.Club, at time.Time, payload map[string]any) error {
	t := c.TenantID
	payload["club_id"], payload["tenant_id"], payload["occurred_at"] = c.ID, c.TenantID, at
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "club", SubjectID: c.ID.String(), PartitionKey: c.ID.String(),
		TenantID: &t, OccurredAt: at, Payload: payload})
}

func (h Clubs) record(ctx context.Context, action string, c club.Club, meta map[string]any) error {
	e := audit.Event{Action: action, TargetType: "club", TargetID: c.ID.String(), Result: audit.ResultSuccess, Metadata: meta}
	if c.SchoolID != nil {
		e.SchoolID = c.SchoolID.String()
	}
	return h.Audit.Record(ctx, e)
}

// afterCommit drops the cached facts of the club channel and pushes chat.channel.membership_changed.
func (h Clubs) afterCommit(ctx context.Context, c club.Club, op, status string, users ...uuid.UUID) error {
	if h.AfterCommit == nil || c.ChannelID == nil {
		return nil
	}
	ch := *c.ChannelID
	return h.AfterCommit(ctx, func(ctx context.Context) {
		if h.Facts != nil {
			_ = h.Facts.InvalidateChannel(ctx, c.TenantID, ch)
		}
		if h.Push == nil {
			return
		}
		for _, u := range users {
			if err := h.Push.MembershipChanged(ctx, u, ch, string(channel.Team), op, status); err != nil {
				slog.WarnContext(ctx, "chat: membership change not pushed", "error", err)
			}
		}
	})
}

// openChannel creates the fixed team channel of an active club (scope_ref {club_id}, is_gameplay false, never purged).
func (h Clubs) openChannel(ctx context.Context, c *club.Club, now time.Time) error {
	if c.ChannelID != nil {
		return nil
	}
	id := channel.ClubID(c.TenantID, c.ID)
	ch, err := channel.New(id, c.TenantID, c.SchoolID, channel.Team, channel.ClubScope(c.ID), now)
	if err != nil {
		return err
	}
	if _, err := h.Channels.CreateChannel(ctx, ch); err != nil {
		return err
	}
	c.ChannelID = &id
	return nil
}

// joinChannel adds a member to the club channel (students only — staff is never a member, F120-AC-04).
func (h Clubs) joinChannel(ctx context.Context, c club.Club, m write.ClubMember, source uuid.UUID) error {
	if c.ChannelID == nil {
		return nil
	}
	if _, err := h.Channels.JoinMember(ctx, channel.Member{ChannelID: *c.ChannelID, UserID: m.UserID, TenantID: c.TenantID,
		SchoolID: m.SchoolID, Role: channel.RoleMember, JoinedAt: m.JoinedAt}); err != nil {
		return err
	}
	return h.Status.SetSource(ctx, c.TenantID, *c.ChannelID, m.UserID, source)
}

// join adds a student to an active club: clubs_per_student_max and club_max_members checked in the transaction (club row
// locked, user lock), membership row, member_count, channel member, club.member_joined (F120-AC-19, AC-20).
func (h Clubs) join(ctx context.Context, c *club.Club, s social.Student, role, source string, sourceID uuid.UUID, now time.Time) error {
	if err := c.CanAdd(); err != nil {
		return clubProblem(err)
	}
	if existing, err := h.Store.Member(ctx, c.TenantID, c.ID, s.ID); err != nil || existing != nil {
		if existing != nil {
			return problem.New(problem.Conflict, "already a member")
		}
		return err
	}
	if err := h.Social.LockUsers(ctx, s.ID); err != nil {
		return err
	}
	n, err := h.Store.CountClubs(ctx, c.TenantID, s.ID)
	if err != nil {
		return err
	}
	if h.Limits.PerStudent > 0 && n >= h.Limits.PerStudent {
		return clubProblem(club.ErrLimitReached)
	}
	sc := s.SchoolID
	m := write.ClubMember{ClubID: c.ID, UserID: s.ID, SchoolID: &sc, Role: role, JoinedAt: now}
	if err := h.Store.AddMember(ctx, c.TenantID, m, sourceID); err != nil {
		return err
	}
	c.MemberCount++
	if err := h.joinChannel(ctx, *c, m, sourceID); err != nil {
		return err
	}
	payload := map[string]any{"user_id": s.ID, "role": role, "source": source}
	if c.ChannelID != nil {
		payload["chat_channel_id"] = *c.ChannelID
	}
	return h.publish(ctx, EventClubMemberJoined, *c, now, payload)
}

// leave ends the membership of user (left/removed): member_count, channel left_at, club.member_left (F120-AC-20).
func (h Clubs) leave(ctx context.Context, c *club.Club, m write.ClubMember, reason string, actor uuid.UUID, now time.Time) error {
	ok, err := h.Store.EndMember(ctx, c.TenantID, c.ID, m.UserID, now, reason)
	if err != nil || !ok {
		return err
	}
	if c.MemberCount > 0 {
		c.MemberCount--
	}
	payload := map[string]any{"user_id": m.UserID, "role": m.Role, "reason": reason, "actor_id": actor}
	if c.ChannelID != nil {
		if _, err := h.Channels.LeaveMember(ctx, c.TenantID, *c.ChannelID, m.UserID, now); err != nil {
			return err
		}
		payload["chat_channel_id"] = *c.ChannelID
	}
	return h.publish(ctx, EventClubMemberLeft, *c, now, payload)
}

// inScope reports whether student s may see/join club c: same school, or another school of a tenant unit with
// cross-school social enabled (ADR-0054, FR-SOC-002).
func (h Clubs) inScope(ctx context.Context, c club.Club, s social.Student) (bool, error) {
	if c.SchoolID == nil || *c.SchoolID == s.SchoolID {
		return true, nil
	}
	t, err := h.Directory.Tenant(ctx, c.TenantID)
	if err != nil || t == nil {
		return false, err
	}
	return t.Type == social.TenantUnit && t.CrossSchool, nil
}

// club loads a live club locked for update for a caller allowed to see it (404 otherwise: never telling it exists).
func (h Clubs) club(ctx context.Context, a clubActor, id uuid.UUID) (*club.Club, *write.ClubMember, error) {
	c, err := h.Store.Club(ctx, a.tenant, id, true)
	if err != nil {
		return nil, nil, err
	}
	if c == nil {
		return nil, nil, notFound()
	}
	if !a.isStudent() {
		var school *uuid.UUID
		if c.SchoolID != nil {
			s := *c.SchoolID
			school = &s
		}
		t := a.tenant
		if err := authz.Require(ctx, a.p, authz.PermSocialClubApprove, authz.Resource{TenantID: &t, SchoolID: school}); err != nil {
			return nil, nil, err
		}
		return c, nil, nil
	}
	m, err := h.Store.Member(ctx, a.tenant, c.ID, a.p.UserID)
	if err != nil {
		return nil, nil, err
	}
	if m != nil {
		return c, m, nil
	}
	if c.Status != club.StatusActive && c.Status != club.StatusPending {
		return nil, nil, notFound()
	}
	ok, err := h.inScope(ctx, *c, *a.student)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, notFound()
	}
	return c, nil, nil
}

func view(c club.Club, role string, staff bool) ClubView {
	return ClubView{Club: c, MyRole: role, Staff: staff}
}

// ClubBody renders a Club of the contract (no school for a student: R16).
func ClubBody(v ClubView) map[string]any {
	c := v.Club
	m := map[string]any{"club_id": c.ID, "club_type": string(c.Type), "name": c.Name, "status": c.Status, "member_count": c.MemberCount,
		"max_members": c.MaxMembers, "owner_id": c.OwnerID, "version": c.Version, "created_at": c.CreatedAt.UTC()}
	if c.Description != nil {
		m["description"] = *c.Description
	}
	if c.IconID != nil {
		m["icon_id"] = *c.IconID
	}
	if c.SubjectID != nil {
		m["subject_id"] = *c.SubjectID
	}
	if c.ChannelID != nil {
		m["chat_channel_id"] = *c.ChannelID
	}
	if v.MyRole != "" {
		m["my_role"] = v.MyRole
	}
	if v.Staff && c.SchoolID != nil {
		m["school_id"] = *c.SchoolID
	}
	return m
}

// Create implements createClub (F120-AC-17): a student creates a competition_team or interest_group of its school when
// student_club_creation allows it (disabled → 403 PERMISSION_DENIED; requires_approval → pending_approval; auto → active
// with its team channel); name and description go through the pipeline (451, no echo); the creator is the owner.
func (h Clubs) Create(ctx context.Context, in ClubInput) (out ClubView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	if !in.Type.Valid() {
		return out, clubProblem(club.ErrType)
	}
	if in.Name == nil {
		return out, clubProblem(club.ErrName)
	}
	name, err := club.CheckName(*in.Name)
	if err != nil {
		return out, clubProblem(err)
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 500 {
		return out, problem.Invalid("description", problem.SubInvalid, "at most 500 characters")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		policy, err := h.Store.SchoolCreation(ctx, a.tenant, a.student.SchoolID)
		if err != nil {
			return err
		}
		status, err := club.InitialStatus(false, in.Type, policy)
		if err != nil {
			return clubProblem(err)
		}
		if err := h.clean(ctx, a.tenant, &name, in.Description); err != nil {
			return err
		}
		if err := h.Social.LockUsers(ctx, a.p.UserID); err != nil {
			return err
		}
		if n, err := h.Store.CountClubs(ctx, a.tenant, a.p.UserID); err != nil {
			return err
		} else if h.Limits.PerStudent > 0 && n >= h.Limits.PerStudent {
			return clubProblem(club.ErrLimitReached)
		}
		school := a.student.SchoolID
		c := club.Club{ID: ids.New(), TenantID: a.tenant, SchoolID: &school, Type: in.Type, Name: name, Description: in.Description,
			IconID: in.IconID, SubjectID: in.SubjectID, OwnerID: a.p.UserID, CreatedByKind: "student", Status: status,
			MaxMembers: h.Limits.MaxMembers, MemberCount: 1, Version: 1, CreatedAt: now}
		if status == club.StatusActive {
			if err := h.openChannel(ctx, &c, now); err != nil {
				return err
			}
		}
		if err := h.Store.CreateClub(ctx, c); err != nil {
			return err
		}
		if err := h.record(ctx, AuditClubCreated, c, map[string]any{"status": status, "club_type": string(c.Type)}); err != nil {
			return err
		}
		if err := h.publish(ctx, EventClubCreated, c, now, map[string]any{"club_type": string(c.Type), "owner_id": c.OwnerID,
			"school_id": school, "status": status, "created_by_kind": "student"}); err != nil {
			return err
		}
		owner := write.ClubMember{ClubID: c.ID, UserID: a.p.UserID, SchoolID: &school, Role: club.RoleOwner, JoinedAt: now}
		if err := h.Store.AddMember(ctx, a.tenant, owner, c.ID); err != nil {
			return err
		}
		if status == club.StatusActive {
			if err := h.activated(ctx, c, owner, nil, now); err != nil {
				return err
			}
		}
		out = view(c, club.RoleOwner, false)
		return idempotency.SetResult(ctx, ClubBody(out))
	})
	return out, err
}

// activated joins the owner to the new team channel and publishes club.approved (approve, or auto creation).
func (h Clubs) activated(ctx context.Context, c club.Club, owner write.ClubMember, approver *uuid.UUID, now time.Time) error {
	if err := h.joinChannel(ctx, c, owner, c.ID); err != nil {
		return err
	}
	payload := map[string]any{"club_type": string(c.Type), "owner_id": c.OwnerID, "chat_channel_id": *c.ChannelID}
	if c.SchoolID != nil {
		payload["school_id"] = *c.SchoolID
	}
	if approver != nil {
		payload["approver_id"] = *approver
	}
	if err := h.publish(ctx, EventClubApproved, c, now, payload); err != nil {
		return err
	}
	if err := h.publish(ctx, EventClubMemberJoined, c, now, map[string]any{"user_id": owner.UserID, "role": club.RoleOwner,
		"chat_channel_id": *c.ChannelID, "source": "created"}); err != nil {
		return err
	}
	return h.afterCommit(ctx, c, MembershipAdded, channel.StatusOpen, owner.UserID)
}

// deniedOutside records authz.denied after a refused transaction (the denial audited inside it was rolled back).
func deniedOutside(ctx context.Context, err error, op string) error {
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.PermissionDenied {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"why": "outside_grant_scope", "operation": op})
	}
	return err
}

// Approve implements approveClub (F120-AC-18): staff holding social.club.approve in the school of the club; pending →
// active, the fixed team channel is created with the owner, club.approved. Another state → 409 CONFLICT.
func (h Clubs) Approve(ctx context.Context, id uuid.UUID) (out ClubView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ApproveClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if a.isStudent() {
		return out, problem.New(problem.PortalAccessStaffOnly, "")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, _, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if c.Status != club.StatusPending {
			return problem.New(problem.Conflict, "club is not pending approval")
		}
		owner, err := h.Store.Member(ctx, a.tenant, c.ID, c.OwnerID)
		if err != nil {
			return err
		}
		if owner == nil {
			return problem.New(problem.Conflict, "club has no owner")
		}
		c.Status = club.StatusActive
		if err := h.openChannel(ctx, c, now); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		c.Version++
		if err := h.record(ctx, AuditClubApproved, *c, map[string]any{"chat_channel_id": c.ChannelID.String()}); err != nil {
			return err
		}
		approver := a.p.UserID
		if err := h.activated(ctx, *c, *owner, &approver, now); err != nil {
			return err
		}
		out = view(*c, "", true)
		return idempotency.SetResult(ctx, ClubBody(out))
	})
	return out, deniedOutside(ctx, err, "approveClub")
}

// Update implements updateClub (owner/officer; If-Match version; name and description through the pipeline).
func (h Clubs) Update(ctx context.Context, id uuid.UUID, ifMatch int64, in ClubInput) (out ClubView, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil || !club.Manager(m.Role) {
			return problem.New(problem.PermissionDenied, "")
		}
		if c.Version != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if in.Name != nil {
			name, err := club.CheckName(*in.Name)
			if err != nil {
				return clubProblem(err)
			}
			c.Name = name
		}
		if in.Description != nil {
			if utf8.RuneCountInString(*in.Description) > 500 {
				return problem.Invalid("description", problem.SubInvalid, "at most 500 characters")
			}
			c.Description = in.Description
		}
		if in.IconID != nil {
			c.IconID = in.IconID
		}
		if err := h.clean(ctx, a.tenant, in.Name, in.Description); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		c.Version++
		out = view(*c, m.Role, false)
		return nil
	})
	return out, err
}

// Leave implements leaveClub: the membership and the channel membership end in the same transaction (history 404 after,
// F120-AC-04, AC-20). The owner archives instead.
func (h Clubs) Leave(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.LeaveClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return err
	}
	if !a.isStudent() {
		return problem.New(problem.GameAccessStudentOnly, "")
	}
	now := h.now()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil {
			return notFound()
		}
		if m.Role == club.RoleOwner {
			return clubProblem(club.ErrOwnerLeaves)
		}
		if err := h.leave(ctx, c, *m, "left", a.p.UserID, now); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		return h.afterCommit(ctx, *c, MembershipRemoved, channel.StatusOpen, m.UserID)
	})
}

// Archive implements archiveClub (owner, or staff with social.club.approve): archived, the team channel read_only
// (history kept by retention), club.archived (F120-AC-20).
func (h Clubs) Archive(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ArchiveClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return err
	}
	now := h.now()
	return deniedOutside(ctx, h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if a.isStudent() && (m == nil || m.Role != club.RoleOwner) {
			return problem.New(problem.PermissionDenied, "")
		}
		if c.Status == club.StatusArchived {
			return idempotency.SetResult(ctx, nil)
		}
		c.Status = club.StatusArchived
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		if c.ChannelID != nil {
			if _, err := h.Status.SetStatus(ctx, a.tenant, *c.ChannelID, []string{channel.StatusOpen}, channel.StatusReadOnly); err != nil {
				return err
			}
		}
		if err := h.record(ctx, AuditClubArchived, *c, nil); err != nil {
			return err
		}
		payload := map[string]any{"club_type": string(c.Type), "owner_id": c.OwnerID, "actor_id": a.p.UserID}
		if c.SchoolID != nil {
			payload["school_id"] = *c.SchoolID
		}
		if c.ChannelID != nil {
			payload["chat_channel_id"] = *c.ChannelID
		}
		if err := h.publish(ctx, EventClubArchived, *c, now, payload); err != nil {
			return err
		}
		if err := idempotency.SetResult(ctx, nil); err != nil {
			return err
		}
		return h.afterCommit(ctx, *c, MembershipStatusChanged, channel.StatusReadOnly) // cached facts: read_only at once
	}), "archiveClub")
}

// SetRole implements updateClubMemberRole (the owner names officers; the owner role is never given away here).
func (h Clubs) SetRole(ctx context.Context, id, user uuid.UUID, role string) (out write.ClubMember, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.UpdateClubMemberRole")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	if role != club.RoleOfficer && role != club.RoleMember {
		return out, problem.Invalid("role", problem.SubInvalid, "officer or member")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil || m.Role != club.RoleOwner {
			return problem.New(problem.PermissionDenied, "")
		}
		target, err := h.Store.Member(ctx, a.tenant, c.ID, user)
		if err != nil {
			return err
		}
		if target == nil || target.Role == club.RoleOwner {
			return notFound()
		}
		if _, err := h.Store.SetRole(ctx, a.tenant, c.ID, user, role); err != nil {
			return err
		}
		target.Role = role
		out = *target
		return nil
	})
	return out, err
}

// Remove implements removeClubMember (owner: anyone but the owner; officer: members only), F120-AC-20.
func (h Clubs) Remove(ctx context.Context, id, user uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RemoveClubMember")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return err
	}
	if !a.isStudent() {
		return problem.New(problem.GameAccessStudentOnly, "")
	}
	now := h.now()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil || !club.Manager(m.Role) {
			return problem.New(problem.PermissionDenied, "")
		}
		target, err := h.Store.Member(ctx, a.tenant, c.ID, user)
		if err != nil {
			return err
		}
		if target == nil || target.Role == club.RoleOwner || target.UserID == a.p.UserID {
			return notFound()
		}
		if m.Role == club.RoleOfficer && target.Role != club.RoleMember {
			return problem.New(problem.PermissionDenied, "")
		}
		if err := h.leave(ctx, c, *target, "removed", a.p.UserID, now); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		if err := h.record(ctx, AuditClubMemberRemove, *c, map[string]any{"user_id": user.String()}); err != nil {
			return err
		}
		return h.afterCommit(ctx, *c, MembershipRemoved, channel.StatusOpen, user)
	})
}

// StaffRemoveReasonMax is maxLength of the reason of removeSchoolClubMember.
const StaffRemoveReasonMax = 500

// staffSchools is the club scope of staff from its UserRole grants holding social.club.approve: nil = the whole tenant;
// no grant at all → 403 PERMISSION_DENIED (audited by authz.Require).
func staffSchools(ctx context.Context, p authz.Principal, tenant uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	held := false
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, authz.PermSocialClubApprove) {
			continue
		}
		held = true
		if g.SchoolID == nil {
			return nil, nil
		}
		out = append(out, *g.SchoolID)
	}
	if !held {
		t := tenant
		return nil, authz.Require(ctx, p, authz.PermSocialClubApprove, authz.Resource{TenantID: &t})
	}
	return out, nil
}

// RemoveByStaff implements removeSchoolClubMember (F122 T061, F122-AC-23): staff holding social.club.approve for the
// school of the club (another school or tenant → 404, never telling it exists) removes a member — never the owner (404)
// — with a reason (1–500). In one transaction, as removeClubMember (F120-AC-20): membership ended (removed), member_count,
// the member leaves the fixed team channel, club.member_left (removed); audit club.member_removed with the reason.
func (h Clubs) RemoveByStaff(ctx context.Context, id, user uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RemoveSchoolClubMember")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return err
	}
	if a.isStudent() {
		return problem.New(problem.PortalAccessStaffOnly, "")
	}
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return problem.Invalid("reason", problem.SubRequired, "a reason is required")
	case utf8.RuneCountInString(reason) > StaffRemoveReasonMax:
		return problem.Invalid("reason", problem.SubMaxLength, "at most 500 characters")
	}
	schools, err := staffSchools(ctx, a.p, a.tenant)
	if err != nil {
		return err
	}
	now := h.now()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Club(ctx, a.tenant, id, true)
		if err != nil {
			return err
		}
		if c == nil || (schools != nil && (c.SchoolID == nil || !slices.Contains(schools, *c.SchoolID))) {
			return notFound()
		}
		target, err := h.Store.Member(ctx, a.tenant, c.ID, user)
		if err != nil {
			return err
		}
		if target == nil || target.Role == club.RoleOwner {
			return notFound()
		}
		if err := h.leave(ctx, c, *target, "removed", a.p.UserID, now); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		e := audit.Event{Action: AuditClubMemberRemove, TargetType: "club", TargetID: c.ID.String(), Result: audit.ResultSuccess,
			Reason: reason, Metadata: map[string]any{"user_id": user.String(), "by": "staff"}}
		if c.SchoolID != nil {
			e.SchoolID = c.SchoolID.String()
		}
		if err := h.Audit.Record(ctx, e); err != nil {
			return err
		}
		if err := idempotency.SetResult(ctx, nil); err != nil {
			return err
		}
		return h.afterCommit(ctx, *c, MembershipRemoved, channel.StatusOpen, user)
	})
}

// RequestJoin implements requestJoinClub: an active club in scope, not blocked by the owner (SOCIAL_SCOPE_VIOLATION);
// pending club → 409 CLUB_NOT_APPROVED; a second request answers the pending one.
func (h Clubs) RequestJoin(ctx context.Context, id uuid.UUID) (out write.ClubRequest, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RequestJoinClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m != nil {
			return problem.New(problem.Conflict, "already a member")
		}
		if err := c.Usable(); err != nil {
			return clubProblem(err)
		}
		blocked, err := h.Social.Blocked(ctx, a.tenant, a.p.UserID, c.OwnerID)
		if err != nil {
			return err
		}
		if blocked {
			return scopeViolation()
		}
		r := write.ClubRequest{ID: ids.New(), TenantID: a.tenant, ClubID: c.ID, RequesterID: a.p.UserID, Status: "pending", CreatedAt: now}
		created, err := h.Store.CreateRequest(ctx, r)
		if err != nil {
			return err
		}
		if !created {
			p, err := h.Store.PendingRequest(ctx, a.tenant, c.ID, a.p.UserID)
			if err != nil || p == nil {
				return err
			}
			r = *p
		}
		out = r
		return idempotency.SetResult(ctx, RequestBody(r))
	})
	return out, err
}

// RequestBody renders a ClubJoinRequest.
func RequestBody(r write.ClubRequest) map[string]any {
	return map[string]any{"request_id": r.ID, "club_id": r.ClubID, "requester_id": r.RequesterID, "status": r.Status,
		"created_at": r.CreatedAt.UTC()}
}

// MemberBody renders a ClubMember (nickname filled by the presentation from the directory when known).
func MemberBody(m write.ClubMember, nickname string) map[string]any {
	return map[string]any{"club_id": m.ClubID, "user_id": m.UserID, "nickname": nickname, "role": m.Role, "joined_at": m.JoinedAt.UTC()}
}

// DecideRequest implements acceptClubJoinRequest (accept true: joins with the limits of F120-AC-19) and
// rejectClubJoinRequest; owner/officer of the club only.
func (h Clubs) DecideRequest(ctx context.Context, requestID uuid.UUID, accept bool) (out write.ClubMember, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DecideClubJoinRequest")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		r, err := h.Store.Request(ctx, a.tenant, requestID)
		if err != nil {
			return err
		}
		if r == nil {
			return notFound()
		}
		c, m, err := h.club(ctx, a, r.ClubID)
		if err != nil {
			return err
		}
		if m == nil || !club.Manager(m.Role) {
			return notFound()
		}
		if r.Status != "pending" {
			return problem.New(problem.Conflict, "request is not pending")
		}
		if !accept {
			_, err := h.Store.DecideRequest(ctx, a.tenant, r.ID, "rejected", a.p.UserID, now)
			return err
		}
		requester, err := h.Directory.Student(ctx, a.tenant, r.RequesterID)
		if err != nil {
			return err
		}
		if requester == nil || !requester.Active() {
			return notFound()
		}
		if ok, err := h.inScope(ctx, *c, *requester); err != nil {
			return err
		} else if !ok {
			return scopeViolation()
		}
		if err := h.join(ctx, c, *requester, club.RoleMember, "join_request", r.ID, now); err != nil {
			return err
		}
		if _, err := h.Store.DecideRequest(ctx, a.tenant, r.ID, "accepted", a.p.UserID, now); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		sc := requester.SchoolID
		out = write.ClubMember{ClubID: c.ID, UserID: requester.ID, SchoolID: &sc, Role: club.RoleMember, JoinedAt: now}
		if err := idempotency.SetResult(ctx, MemberBody(out, "")); err != nil {
			return err
		}
		return h.afterCommit(ctx, *c, MembershipAdded, channel.StatusOpen, requester.ID)
	})
	return out, err
}

// Invite implements inviteToClub (owner/officer): pending club → 409 CLUB_NOT_APPROVED; an invitee of another tenant,
// out of scope or in a block with the inviter → 403 SOCIAL_SCOPE_VIOLATION (F120-AC-19); members are skipped.
func (h Clubs) Invite(ctx context.Context, id uuid.UUID, invitees []uuid.UUID) (out []write.ClubInvitation, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.InviteToClub")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return nil, err
	}
	if !a.isStudent() {
		return nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	if len(invitees) == 0 || len(invitees) > 50 {
		return nil, problem.Invalid("invitee_ids", problem.SubInvalid, "1..50 students")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil || !club.Manager(m.Role) {
			return problem.New(problem.PermissionDenied, "")
		}
		if err := c.Usable(); err != nil {
			return clubProblem(err)
		}
		for _, u := range invitees {
			if u == a.p.UserID {
				return self("invitee_ids")
			}
			s, err := h.Directory.Student(ctx, a.tenant, u)
			if err != nil {
				return err
			}
			if s == nil || !s.Active() {
				return scopeViolation() // another tenant or unknown: never telling which
			}
			if ok, err := h.inScope(ctx, *c, *s); err != nil {
				return err
			} else if !ok {
				return scopeViolation()
			}
			blocked, err := h.Social.Blocked(ctx, a.tenant, a.p.UserID, u)
			if err != nil {
				return err
			}
			if blocked {
				return scopeViolation()
			}
			if existing, err := h.Store.Member(ctx, a.tenant, c.ID, u); err != nil {
				return err
			} else if existing != nil {
				continue
			}
			inv, _, err := h.Store.CreateInvitation(ctx, write.ClubInvitation{ID: ids.New(), TenantID: a.tenant, ClubID: c.ID,
				InviterID: a.p.UserID, InviteeID: u, ExpiresAt: now.Add(h.inviteTTL())})
			if err != nil {
				return err
			}
			out = append(out, *inv)
		}
		data := make([]map[string]any, len(out))
		for i, v := range out {
			data[i] = InvitationBody(v)
		}
		return idempotency.SetResult(ctx, map[string]any{"data": data})
	})
	return out, err
}

func (h Clubs) inviteTTL() time.Duration {
	if h.Limits.InviteTTL > 0 {
		return h.Limits.InviteTTL
	}
	return 7 * 24 * time.Hour
}

// InvitationBody renders a ClubInvitation.
func InvitationBody(v write.ClubInvitation) map[string]any {
	return map[string]any{"invitation_id": v.ID, "club_id": v.ClubID, "inviter_id": v.InviterID, "invitee_id": v.InviteeID,
		"status": v.Status, "expires_at": v.ExpiresAt.UTC()}
}

// DecideInvitation implements acceptClubInvitation (accept true: joins with the limits of F120-AC-19; expired → 410) and
// declineClubInvitation; the invitee only (404 for anyone else).
func (h Clubs) DecideInvitation(ctx context.Context, invitationID uuid.UUID, accept bool) (out write.ClubMember, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.DecideClubInvitation")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		inv, err := h.Store.Invitation(ctx, a.tenant, invitationID)
		if err != nil {
			return err
		}
		if inv == nil || inv.InviteeID != a.p.UserID {
			return notFound()
		}
		if inv.Status != "pending" {
			return problem.New(problem.Conflict, "invitation is not pending")
		}
		if !now.Before(inv.ExpiresAt) {
			if _, err := h.Store.DecideInvitation(ctx, a.tenant, inv.ID, "expired", now); err != nil {
				return err
			}
			return problem.New(problem.PartyInvitationExpired, "invitation expired")
		}
		if !accept {
			_, err := h.Store.DecideInvitation(ctx, a.tenant, inv.ID, "declined", now)
			return err
		}
		c, err := h.Store.Club(ctx, a.tenant, inv.ClubID, true)
		if err != nil {
			return err
		}
		if c == nil {
			return notFound()
		}
		if blocked, err := h.Social.Blocked(ctx, a.tenant, a.p.UserID, inv.InviterID); err != nil {
			return err
		} else if blocked {
			return scopeViolation()
		}
		if err := h.join(ctx, c, *a.student, club.RoleMember, "invitation", inv.ID, now); err != nil {
			return err
		}
		if _, err := h.Store.DecideInvitation(ctx, a.tenant, inv.ID, "accepted", now); err != nil {
			return err
		}
		if err := h.Store.SaveClub(ctx, *c, a.p.UserID, now); err != nil {
			return err
		}
		sc := a.student.SchoolID
		out = write.ClubMember{ClubID: c.ID, UserID: a.p.UserID, SchoolID: &sc, Role: club.RoleMember, JoinedAt: now}
		if err := idempotency.SetResult(ctx, MemberBody(out, "")); err != nil {
			return err
		}
		return h.afterCommit(ctx, *c, MembershipAdded, channel.StatusOpen, a.p.UserID)
	})
	return out, err
}

// countedMetrics are the goal metrics fed by a verified event today (F120-AC-21): minutes_learning waits for
// activity.graded.duration_seconds (owner F031) and boss_defeated for boss.encounter_completed (owner F103) [Proposed].
var countedMetrics = map[string]bool{club.MetricActivities: true, club.MetricQuests: true, club.MetricDiscoveries: true}

// CreateGoal implements createClubGoal (owner/officer): a goal of the current week; a metric without a verified source
// event yet → 422 VALIDATION_FAILED.
func (h Clubs) CreateGoal(ctx context.Context, id uuid.UUID, in GoalInput, loc *time.Location) (out write.ClubGoal, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateClubGoal")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	if !countedMetrics[in.Metric] {
		return out, problem.Invalid("metric", problem.SubInvalid, "metric not counted from a verified event yet")
	}
	if in.Target < 1 || in.Target > 100000 {
		return out, problem.Invalid("target", problem.SubInvalid, "1..100000")
	}
	now := h.now()
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil || !club.Manager(m.Role) {
			return problem.New(problem.PermissionDenied, "")
		}
		if err := c.Usable(); err != nil {
			return clubProblem(err)
		}
		if err := h.clean(ctx, a.tenant, in.Title); err != nil {
			return err
		}
		out = write.ClubGoal{ID: ids.New(), TenantID: a.tenant, ClubID: c.ID, Title: in.Title, Metric: in.Metric, SubjectID: in.SubjectID,
			Target: in.Target, WeekStart: club.WeekStart(now, loc), Status: "active"}
		if err := h.Store.CreateGoal(ctx, out, a.p.UserID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, GoalBody(out, 0))
	})
	return out, err
}

// GoalBody renders a ClubGoal.
func GoalBody(g write.ClubGoal, mine int) map[string]any {
	m := map[string]any{"goal_id": g.ID, "club_id": g.ClubID, "metric": g.Metric, "target": g.Target, "progress": g.Progress,
		"my_contribution": mine, "week_start": g.WeekStart.Format("2006-01-02"), "status": g.Status}
	if g.Title != nil {
		m["title"] = *g.Title
	}
	if g.SubjectID != nil {
		m["subject_id"] = *g.SubjectID
	}
	return m
}

// CreateEvent implements createClubEvent (owner/officer; title and description through the pipeline).
func (h Clubs) CreateEvent(ctx context.Context, id uuid.UUID, in EventInput) (out write.ClubEvent, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.CreateClubEvent")
	defer func() { done(err) }()
	a, err := h.actor(ctx)
	if err != nil {
		return out, err
	}
	if !a.isStudent() {
		return out, problem.New(problem.GameAccessStudentOnly, "")
	}
	if t := strings.TrimSpace(in.Title); t == "" || utf8.RuneCountInString(t) > 128 {
		return out, problem.Invalid("title", problem.SubInvalid, "1..128 characters")
	}
	if in.EndsAt != nil && !in.EndsAt.After(in.StartsAt) {
		return out, problem.Invalid("ends_at", problem.SubInvalid, "after starts_at")
	}
	if in.Kind == "" {
		in.Kind = "other"
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.loadStudent(ctx, &a); err != nil {
			return err
		}
		c, m, err := h.club(ctx, a, id)
		if err != nil {
			return err
		}
		if m == nil || !club.Manager(m.Role) {
			return problem.New(problem.PermissionDenied, "")
		}
		if err := c.Usable(); err != nil {
			return clubProblem(err)
		}
		title := strings.TrimSpace(in.Title)
		if err := h.clean(ctx, a.tenant, &title, in.Description); err != nil {
			return err
		}
		out = write.ClubEvent{ID: ids.New(), TenantID: a.tenant, ClubID: c.ID, Title: title, Description: in.Description, Kind: in.Kind,
			TournamentTemplateID: in.TournamentTemplateID, StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt}
		if err := h.Store.CreateEvent(ctx, out, a.p.UserID); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, EventBody(out))
	})
	return out, err
}

// EventBody renders a ClubEvent.
func EventBody(e write.ClubEvent) map[string]any {
	m := map[string]any{"event_id": e.ID, "club_id": e.ClubID, "title": e.Title, "event_kind": e.Kind, "starts_at": e.StartsAt.UTC()}
	if e.Description != nil {
		m["description"] = *e.Description
	}
	if e.EndsAt != nil {
		m["ends_at"] = e.EndsAt.UTC()
	}
	return m
}

// RecordProgress adds the contribution of one verified event (activity.graded, quest.completed, world.discovery_found) to
// the active goals of metric of the clubs the student is an active member of (F120-AC-21): one event counts once per
// goal (event id), never a score or mastery value; reaching the target publishes club.goal_completed. Event scope.
func (h Clubs) RecordProgress(ctx context.Context, tenant, student uuid.UUID, metric string, delta int, event uuid.UUID, at time.Time,
	loc *time.Location) (completed int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.RecordClubGoalProgress")
	defer func() { done(err) }()
	if delta <= 0 || !countedMetrics[metric] {
		return 0, nil
	}
	err = h.UoW.InTenantTx(ctx, mustScope(tenant), func(ctx context.Context) error {
		goals, err := h.Store.GoalsOf(ctx, tenant, student, metric, club.WeekStart(at, loc))
		if err != nil {
			return err
		}
		for _, g := range goals {
			added, err := h.Store.Contribute(ctx, tenant, g.ID, student, delta, event)
			if err != nil {
				return err
			}
			if !added {
				continue
			}
			after, err := h.Store.AdvanceGoal(ctx, tenant, g.ID, delta, at)
			if err != nil {
				return err
			}
			if after.Status == "completed" {
				completed++
				c := club.Club{ID: g.ClubID, TenantID: tenant}
				if err := h.publish(ctx, EventClubGoalComplete, c, at, map[string]any{"goal_id": g.ID, "metric": g.Metric,
					"target": g.Target, "week_start": g.WeekStart.Format("2006-01-02")}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return completed, err
}
