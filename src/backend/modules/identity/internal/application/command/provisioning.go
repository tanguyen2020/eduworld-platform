package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ProvisionStudentAccountHandler creates the Keycloak account of a SIS student (consumer keycloak-provisioner).
type ProvisionStudentAccountHandler interface {
	Handle(ctx context.Context, studentID uuid.UUID) error
}

// ProvisionStudentAccount handles student.provisioning_requested (ACCOUNT_PROVISIONING §2, F011-AC-08,
// AC-09): Keycloak user without email with attributes eduworld_user_id/ew_tenant/ew_sis_connection/
// ew_external_id, realm role student, then external_identities and user.provisioned in one transaction.
// The Organization membership follows from tenant_membership.changed (consumer keycloak-membership, §2.1).
// No password is set: the school issues the login card through the password reset (§6d) [Proposed].
//
// Every step is idempotent so a redelivery after a partial failure completes the work: a 409 on create looks
// the user up by eduworld_user_id; another account holding the username raises duplicate_identifier.
type ProvisionStudentAccount struct {
	UoW         write.UnitOfWork
	KC          port.KeycloakAdmin
	Accounts    write.ProvisioningRepository
	Sync        write.SisSyncRepository
	Events      write.EventPublisher
	Audit       write.Auditor
	LocalIssuer string // OIDC issuer of realm eduworld
}

// Handle provisions one student.
func (h ProvisionStudentAccount) Handle(ctx context.Context, studentID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ProvisionStudentAccount")
	defer func() { done(err) }()
	var s *write.PendingStudent
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		s, err = h.Accounts.PendingStudent(ctx, studentID, h.LocalIssuer)
		return err
	}); err != nil {
		return err
	}
	if s == nil || s.Subject != nil || s.Status == "deactivated" || s.Status == "anonymized" {
		return nil // gone, already provisioned, or no longer allowed to sign in
	}
	uid := studentID.String()
	attrs := map[string][]string{"eduworld_user_id": {uid}, "ew_tenant": {s.TenantID.String()}}
	if s.ExternalID != "" {
		attrs["ew_sis_connection"] = []string{s.ConnectionID.String()}
		attrs["ew_external_id"] = []string{s.ExternalID}
	}
	kcID, err := h.KC.CreateUser(ctx, port.KCNewUser{Username: s.Username, FirstName: s.DisplayName, Enabled: true, Attributes: attrs})
	if errors.Is(err, port.ErrKCConflict) {
		u, ferr := h.KC.FindUserByAttribute(ctx, "eduworld_user_id", uid)
		if errors.Is(ferr, port.ErrKCNotFound) {
			return h.conflict(ctx, s) // the username belongs to another Keycloak account (F011-AC-09)
		}
		if ferr != nil {
			return ferr
		}
		kcID, err = u.ID, nil // created by an earlier delivery
	}
	if err != nil {
		return err
	}
	if err = h.KC.AddRealmRoles(ctx, kcID, "student"); err != nil {
		return err
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Accounts.LinkIdentity(ctx, studentID, h.LocalIssuer, kcID); err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.IdentityConflict {
				return nil // a concurrent delivery linked it
			}
			return err
		}
		t, school, conn := s.TenantID, s.SchoolID.String(), s.ConnectionID.String()
		if err := h.Events.Publish(ctx, write.Event{Type: "user.provisioned", SubjectType: "user", SubjectID: uid, TenantID: &t,
			Payload: map[string]any{"user_id": uid, "user_type": "student", "source": "sis", "school_id": school, "grade_level": s.Grade,
				"sis_connection_id": conn}}); err != nil {
			return err
		}
		if err := h.Events.Publish(ctx, write.Event{Type: "tenant_membership.changed", SubjectType: "tenant_membership", SubjectID: uid, TenantID: &t,
			Payload: MembershipPayload{UserID: uid, TenantID: t.String(), Op: "added", Source: "sis_sync", UserType: "student"}}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: "user.provisioned", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
			Metadata: map[string]any{"source": "sis", "sis_connection_id": conn, "actor": "system:keycloak-provisioner"}})
	})
}

// conflict raises duplicate_identifier for a username held by another Keycloak account.
func (h ProvisionStudentAccount) conflict(ctx context.Context, s *write.PendingStudent) error {
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		school := s.SchoolID
		id, err := h.Sync.RaiseIssue(ctx, write.Issue{SchoolID: &school, ConnectionID: s.ConnectionID, Type: sis.IssueDuplicateIdentifier,
			Refs: map[string]any{"user_id": s.UserID.String(), "student_external_id": s.ExternalID, "field": "username", "source": "keycloak"}})
		if err != nil {
			return err
		}
		t := s.TenantID
		return h.Events.Publish(ctx, write.Event{Type: "sis.reconciliation_issue_raised", SubjectType: "reconciliation_issue", SubjectID: id.String(),
			TenantID: &t, Payload: map[string]any{"issue_id": id.String(), "kind": sis.IssueDuplicateIdentifier, "school_id": school.String(),
				"sis_connection_id": s.ConnectionID.String()}})
	})
}
