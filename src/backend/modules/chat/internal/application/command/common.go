// Package command holds the write use cases of module chat (ADR-0087): each loads its facts through a write port,
// asks the domain, saves in one entx.UnitOfWork transaction with its outbox events and audit, and returns ids.
package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SocialConfig is the configuration of the social use cases (CONFIG_DEFAULTS friend_limits, friend_request_ttl,
// friend_code_ttl); read by infrastructure/config, never hard-coded.
type SocialConfig struct {
	Limits     social.Limits
	RequestTTL time.Duration
	CodeTTL    time.Duration
}

// Audit actions of the social use cases (no PII: ids only).
const (
	AuditFriendRequestSent = "social.friend_request_sent"
	AuditFriendAdded       = "social.friend_added"
	AuditFriendRemoved     = "social.friend_removed"
	AuditUserBlocked       = "social.user_blocked"
	AuditUserUnblocked     = "social.user_unblocked"
	AuditUserMuted         = "social.user_muted"
	AuditUserUnmuted       = "social.user_unmuted"
)

// HoldChecker reports an active safety hold of a student (case_hold or block_and_alert_hold, MODERATION §6.3).
type HoldChecker interface {
	Held(ctx context.Context, tenant, user uuid.UUID) (bool, error)
}

// RetryAfter wraps a 429 problem with the delay of the Retry-After header.
type RetryAfter struct {
	Err   error
	After time.Duration
}

func (e RetryAfter) Error() string { return e.Err.Error() }
func (e RetryAfter) Unwrap() error { return e.Err }

// Social are the write use cases of friends, blocks and mutes (F095 US1–US3). Every use case runs for the calling
// student only, in one UnitOfWork with its outbox events and audit.
type Social struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Repo        write.SocialRepository
	Directory   write.Directory
	Events      write.EventPublisher
	Audit       write.Auditor
	Codes       port.CodeGenerator
	Friends     port.FriendCache // nil: no cache
	Metrics     port.Metrics     // nil: no metrics
	// Holds reports a temporary safety hold of the sender (F121-AC-04: no new friend request while held); nil: none.
	Holds  HoldChecker
	Config SocialConfig
	Now    func() time.Time
}

func (h Social) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Social) metric(result string) {
	if h.Metrics != nil {
		h.Metrics.FriendRequest(result)
	}
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

// self is the 422 of an operation targeting the caller (F095-AC-04, AC-18).
func self(field string) error {
	return problem.Invalid(field, problem.SubInvalid, "must be another student")
}

func notFound() error { return problem.New(problem.NotFound, "") }

// scopeProblem maps the domain errors of the scope and limit rules to the contract.
func (h Social) scopeProblem(err error, now time.Time) error {
	switch {
	case errors.Is(err, social.ErrFriendsDisabled), errors.Is(err, social.ErrBlocked), errors.Is(err, social.ErrOtherSchool):
		h.metric(port.ResultScope)
		return problem.New(problem.SocialScopeViolation, "")
	case errors.Is(err, social.ErrLimitReached):
		h.metric(port.ResultLimit)
		return problem.New(problem.FriendLimitReached, "")
	case errors.Is(err, social.ErrDailyLimit):
		h.metric(port.ResultRate)
		return RetryAfter{Err: problem.New(problem.RateLimited, "").Retry(), After: h.Config.Limits.RetryAfter(now)}
	}
	return err
}

// tenant returns the projected tenant of the session (503 retryable until identity projected it).
func (h Social) tenant(ctx context.Context, id uuid.UUID) (social.Tenant, error) {
	t, err := h.Directory.Tenant(ctx, id)
	if err != nil {
		return social.Tenant{}, err
	}
	if t == nil {
		return social.Tenant{}, problem.New(problem.Unavailable, "tenant not projected yet").Retry()
	}
	return *t, nil
}

// caller returns the projected calling student (503 retryable until projected; refused when not active).
func (h Social) caller(ctx context.Context, tenant, me uuid.UUID) (social.Student, error) {
	s, err := h.Directory.Student(ctx, tenant, me)
	if err != nil {
		return social.Student{}, err
	}
	if s == nil {
		return social.Student{}, problem.New(problem.Unavailable, "student not projected yet").Retry()
	}
	if !s.Active() {
		return social.Student{}, problem.New(problem.GameAccessStudentOnly, "")
	}
	return *s, nil
}

// target returns another student of the tenant: 404 when unknown in the session tenant, cleaned up or (active
// required) not active (F095-AC-02, AC-18). Never tells which.
func (h Social) target(ctx context.Context, tenant, id uuid.UUID, active bool) (social.Student, error) {
	s, err := h.Directory.Student(ctx, tenant, id)
	if err != nil {
		return social.Student{}, err
	}
	if s == nil || s.Deleted || (active && !s.Active()) {
		return social.Student{}, notFound()
	}
	return *s, nil
}

func (h Social) record(ctx context.Context, action string, target uuid.UUID, meta map[string]any) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "student", TargetID: target.String(),
		Result: audit.ResultSuccess, Metadata: meta})
}

func (h Social) publish(ctx context.Context, typ string, subject uuid.UUID, tenant uuid.UUID, at time.Time, payload map[string]any) error {
	t := tenant
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "student", SubjectID: subject.String(),
		PartitionKey: subject.String(), TenantID: &t, OccurredAt: at, Payload: payload})
}

// invalidate drops the cached friend ids of users after the commit (best effort: the cache also expires).
func (h Social) invalidate(ctx context.Context, users ...uuid.UUID) error {
	if h.Friends == nil || h.AfterCommit == nil {
		return nil
	}
	return h.AfterCommit(ctx, func(ctx context.Context) { _ = h.Friends.Invalidate(ctx, users...) })
}

// version is the source_version of the relation events (projections keep the newest, DATABASE_DESIGN §2.17).
func version(at time.Time) int64 { return at.UnixMicro() }
