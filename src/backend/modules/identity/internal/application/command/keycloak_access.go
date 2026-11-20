package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// KeycloakAccess mirrors the access of an account on Keycloak (F016-AC-03, AC-04, AC-26): a revoked access
// disables the Keycloak user and ends its sessions; a password change does neither (the game sessions are
// revoked by F013, the user stays enabled); a restored access enables the user again. Errors are returned so
// the consumer retries with backoff (the outbox consumer redelivers).
type KeycloakAccess struct {
	UoW     write.UnitOfWork
	Repo    write.LifecycleRepository
	KC      port.KeycloakAccessAdmin
	Anon    port.KeycloakAnonymizer // F016 US5 (nil = disable and logout only)
	Email   port.KeycloakEmailAdmin // OQ F012-1 (nil = email changes are not copied: user.email_changed is retried)
	Issuers []string                // the local realm issuer first (EW_OIDC_ISSUERS)
}

func (h KeycloakAccess) subject(ctx context.Context, userID uuid.UUID) (sub string, err error) {
	sc, err := authz.SystemPlatformScope("keycloak-access")
	if err != nil {
		return "", err
	}
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		sub, err = h.Repo.KeycloakSubject(ctx, userID, h.Issuers)
		return err
	})
	return sub, err
}

// Revoked handles user.access_revoked.
func (h KeycloakAccess) Revoked(ctx context.Context, userID uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.KeycloakAccessRevoked")
	defer func() { done(err) }()
	if reason == "password_changed" {
		return nil
	}
	sub, err := h.subject(ctx, userID)
	if err != nil || sub == "" {
		return err
	}
	if err := h.KC.DisableUser(ctx, sub); err != nil {
		return err
	}
	return h.KC.LogoutUser(ctx, sub)
}

// Restored handles user.access_restored.
func (h KeycloakAccess) Restored(ctx context.Context, userID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.KeycloakAccessRestored")
	defer func() { done(err) }()
	sub, err := h.subject(ctx, userID)
	if err != nil || sub == "" {
		return err
	}
	return h.KC.EnableUser(ctx, sub)
}

// CleanedUp handles student.cleaned_up (F016-AC-13): every Keycloak session ends but the user stays enabled
// so that a later login can restore the student (the Organization membership is removed by the membership
// worker on tenant_membership.changed).
func (h KeycloakAccess) CleanedUp(ctx context.Context, userID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.KeycloakAccessCleanedUp")
	defer func() { done(err) }()
	sub, err := h.subject(ctx, userID)
	if err != nil || sub == "" {
		return err
	}
	return h.KC.LogoutUser(ctx, sub)
}

// Anonymized handles user.anonymized (F016-AC-20): the Keycloak user is disabled, its sessions end and its
// profile is cleared.
func (h KeycloakAccess) Anonymized(ctx context.Context, userID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.KeycloakAccessAnonymized")
	defer func() { done(err) }()
	sub, err := h.subject(ctx, userID)
	if err != nil || sub == "" {
		return err
	}
	if err := h.KC.DisableUser(ctx, sub); err != nil {
		return err
	}
	if err := h.KC.LogoutUser(ctx, sub); err != nil {
		return err
	}
	if h.Anon == nil {
		return nil
	}
	return h.Anon.AnonymizeUser(ctx, sub)
}

// EmailChanged handles user.email_changed (OQ F012-1): the current email of the account (never carried by the
// event) is set on the Keycloak user with emailVerified=false. An account without a Keycloak identity has nothing
// to copy; a Keycloak conflict is returned as is (the consumer sends it to the DLQ for an operator).
func (h KeycloakAccess) EmailChanged(ctx context.Context, userID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.KeycloakAccessEmailChanged")
	defer func() { done(err) }()
	if h.Email == nil {
		return errors.New("keycloak access: email sync is not wired")
	}
	sub, err := h.subject(ctx, userID)
	if err != nil || sub == "" {
		return err
	}
	var email string
	sc, err := authz.SystemPlatformScope("keycloak-access")
	if err != nil {
		return err
	}
	if err := h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		email, err = h.Repo.UserEmail(ctx, userID)
		return err
	}); err != nil || email == "" {
		return err
	}
	return h.Email.SetEmail(ctx, sub, email)
}
