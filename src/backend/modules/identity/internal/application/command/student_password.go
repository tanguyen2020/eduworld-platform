package command

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// StudentPasswordResetResult is StudentPasswordResetResult; InitialPassword is only in the first answer.
type StudentPasswordResetResult struct {
	StudentID         uuid.UUID `json:"student_id"`
	Username          string    `json:"username"`
	InitialPassword   string    `json:"initial_password,omitempty"`
	PasswordTemporary bool      `json:"password_temporary"`
	PasswordRedacted  bool      `json:"password_redacted"`
	ResetAt           time.Time `json:"reset_at"`
}

// ResetStudentPassword implements resetStudentPassword (F016-AC-25..AC-27, ACCOUNT_PROVISIONING §6d): School
// Admin of the student's school, Tenant Admin, or System Admin with the target tenant; a random password is
// set on Keycloak (temporary for grades 6–9), returned once, never stored or logged; the game sessions are
// revoked (password_changed) and user.access_revoked (password_changed) is published — the user stays enabled.
type ResetStudentPassword struct {
	UoW      write.UnitOfWork
	Repo     write.LifecycleRepository
	KC       port.KeycloakPasswordAdmin
	Sessions GameAuthSessionRevoker
	Events   write.EventPublisher
	Audit    write.Auditor
	Issuers  []string
	Clock    func() time.Time
}

// Handle resets the password of studentID.
func (h ResetStudentPassword) Handle(ctx context.Context, studentID uuid.UUID, reason string) (out StudentPasswordResetResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ResetStudentPassword")
	defer func() { done(err) }()
	if n := len(strings.TrimSpace(reason)); n < 3 || n > 500 {
		return out, problem.Invalid("reason", problem.SubInvalid, "3..500 characters")
	}
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.TenantID == nil || p.UserType != authz.UserTypeStaff {
		return out, problem.New(problem.PermissionDenied, "")
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	ctx = tenancy.WithScope(ctx, sc)
	var target *write.LifecycleTarget
	var sub string
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if target, err = h.Repo.Target(ctx, studentID); err != nil {
			return err
		}
		if target == nil || target.UserType != string(authz.UserTypeStudent) || target.StudentTenantID == nil || *target.StudentTenantID != *p.TenantID {
			return problem.New(problem.NotFound, "")
		}
		if !p.IsSystemAdmin {
			if err := authz.Require(ctx, p, authz.PermLmsStudentPasswordReset, authz.Resource{TenantID: p.TenantID, SchoolID: target.StudentSchoolID}); err != nil {
				return err
			}
		}
		sub, err = h.Repo.KeycloakSubject(ctx, studentID, h.Issuers)
		return err
	}); err != nil {
		return out, err
	}
	if sub == "" {
		return out, problem.New(problem.Conflict, "the student has no login account yet")
	}
	password, err := user.GeneratePassword(target.Username)
	if err != nil {
		return out, unavailable(err)
	}
	temporary := user.TemporaryPassword(target.GradeLevel)
	if err := h.KC.SetPassword(ctx, sub, password, temporary); err != nil {
		return out, unavailable(err)
	}
	if h.Sessions != nil {
		if _, err := h.Sessions.RevokeSessions(ctx, write.SessionFilter{UserID: studentID}, gamelogin.RevokePasswordChanged); err != nil {
			return out, unavailable(err)
		}
	}
	now := time.Now().UTC()
	if h.Clock != nil {
		now = h.Clock().UTC()
	}
	out = StudentPasswordResetResult{StudentID: studentID, Username: target.Username, InitialPassword: password,
		PasswordTemporary: temporary, ResetAt: now}
	tenant := *p.TenantID
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Events.Publish(ctx, write.Event{Type: "user.access_revoked", SubjectType: "user", SubjectID: studentID.String(),
			TenantID: &tenant, Payload: map[string]any{"user_id": studentID, "reason": "password_changed", "tenant_id": tenant}}); err != nil {
			return err
		}
		ev := audit.Event{Action: "lms.student.password_reset", TargetType: "user", TargetID: studentID.String(), Result: audit.ResultSuccess,
			Reason: reason, Metadata: map[string]any{"password_temporary": temporary}}
		if target.StudentSchoolID != nil {
			ev.SchoolID = target.StudentSchoolID.String()
		}
		if err := h.Audit.Record(ctx, ev); err != nil {
			return err
		}
		red := out
		red.InitialPassword, red.PasswordRedacted = "", true
		return idempotency.SetResult(ctx, red) // a replay never carries the password
	})
	if err != nil {
		return StudentPasswordResetResult{}, err
	}
	return out, nil
}
