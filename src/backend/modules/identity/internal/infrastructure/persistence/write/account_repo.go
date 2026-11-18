package write

import (
	"context"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entext "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/externalidentity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/enrollment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AccountRepo implements write.AccountRepository.
type AccountRepo struct{}

var _ write.AccountRepository = AccountRepo{}

// UsernameTaken checks every row, soft-deleted included: a username is freed only by anonymization
// (ADR-0067, ADR-0069), so the ent soft-delete filter must not apply here.
func (AccountRepo) UsernameTaken(ctx context.Context, username string) (bool, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return false, entx.ErrNoTransaction
	}
	var taken bool
	// tenancyguard:ok identity.users is global (no tenant_id); soft-deleted rows still hold their username
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM identity.users WHERE lower(username) = lower($1))`, username).Scan(&taken)
	return taken, err
}

// ExternalRefTaken reports whether a live identity (issuer, subject) exists.
func (AccountRepo) ExternalRefTaken(ctx context.Context, issuer, subject string) (bool, error) {
	return client(ctx).ExternalIdentity.Query().Where(entext.Issuer(issuer), entext.Subject(subject)).Exist(ctx)
}

// CreateStudent inserts user, student, identities and the active enrollment.
func (AccountRepo) CreateStudent(ctx context.Context, a write.NewStudentAccount) error {
	c := client(ctx)
	if _, err := c.User.Create().SetID(a.UserID).SetUserType("student").SetStatus("active").SetUsername(a.Username).
		SetDisplayName(a.DisplayName).SetPrimaryTenantID(a.TenantID).SetPrimarySchoolID(a.SchoolID).Save(ctx); err != nil {
		return mapAccountErr(err)
	}
	if _, err := c.Student.Create().SetID(a.UserID).SetTenantID(a.TenantID).SetSchoolID(a.SchoolID).SetGradeLevel(int16(a.Grade)).
		SetFullName(a.DisplayName).Save(ctx); err != nil {
		return mapAccountErr(err)
	}
	if _, err := c.ExternalIdentity.Create().SetIssuer(a.Issuer).SetSubject(a.Subject).SetUserID(a.UserID).Save(ctx); err != nil {
		return mapAccountErr(err)
	}
	if a.ExternalRef != "" {
		if _, err := c.ExternalIdentity.Create().SetIssuer(user.ExternalRefIssuer).SetSubject(a.ExternalRef).SetUserID(a.UserID).Save(ctx); err != nil {
			return mapAccountErr(err)
		}
	}
	e, err := enrollment.Open(a.EnrollmentID, a.UserID, enrollment.Placement{TenantID: a.TenantID, SchoolID: a.SchoolID, ClassID: a.ClassID},
		entxNow(), enrollment.SourcePlatformCommon)
	if err != nil {
		return err
	}
	return EnrollmentRepo{}.Open(ctx, e)
}

func mapAccountErr(err error) error {
	if !ent.IsConstraintError(err) {
		return err
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "users_username_uq"):
		return problem.New(problem.UsernameTaken, "").Wrap(err)
	case strings.Contains(msg, "external_identities"):
		return problem.New(problem.IdentityConflict, "").Wrap(err)
	}
	return problem.New(problem.Conflict, "").Wrap(err)
}

func entxNow() time.Time { return time.Now().UTC() }
