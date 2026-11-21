package command

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// idempotencyRecordMax is the largest response pkg/idempotency stores (infra.idempotency_records).
const idempotencyRecordMax = 65536

// DefaultCommonStudentBatchMax is common_student_batch_max (CONFIG_DEFAULTS, scope code [Proposed]).
const DefaultCommonStudentBatchMax = 500

// CommonStudentInput is one row of CommonTenantStudentsCreate.
type CommonStudentInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	GradeLevel  int    `json:"grade_level"`
	ExternalRef string `json:"external_ref,omitempty"`
}

// CreateCommonStudentsInput is the request (TenantID from X-Tenant-Id, must be the common tenant).
type CreateCommonStudentsInput struct {
	TenantID uuid.UUID            `json:"-"`
	Students []CommonStudentInput `json:"students"`
}

// CommonStudentResult is one row of CommonTenantStudentsResult.
type CommonStudentResult struct {
	RowNo           int        `json:"row_no"`
	Status          string     `json:"status"`
	UserID          *uuid.UUID `json:"user_id,omitempty"`
	Username        string     `json:"username"`
	InitialPassword string     `json:"initial_password,omitempty"`
	ErrorCode       string     `json:"error_code,omitempty"`
}

// CommonStudentsResult is CommonTenantStudentsResult.
type CommonStudentsResult struct {
	Total            int                   `json:"total"`
	CreatedCount     int                   `json:"created_count"`
	FailedCount      int                   `json:"failed_count"`
	PasswordRedacted bool                  `json:"password_redacted"`
	Results          []CommonStudentResult `json:"results"`
}

// Redacted returns the result without passwords (what the idempotency record stores, F010-AC-29).
func (r CommonStudentsResult) Redacted() CommonStudentsResult {
	out := r
	out.PasswordRedacted = true
	out.Results = make([]CommonStudentResult, len(r.Results))
	for i, row := range r.Results {
		row.InitialPassword = ""
		out.Results[i] = row
	}
	return out
}

// CreateCommonStudentsHandler creates common-tenant students (createCommonTenantStudents, F010-AC-24,
// AC-25, AC-28, AC-29).
type CreateCommonStudentsHandler interface {
	Handle(ctx context.Context, in CreateCommonStudentsInput) (CommonStudentsResult, error)
}

// CreateCommonStudents implements CreateCommonStudentsHandler. Each row is independent: a failed row never
// rolls back another; passwords are generated, set in Keycloak, returned once and never stored or logged.
type CreateCommonStudents struct {
	UoW         write.UnitOfWork
	Tenants     write.TenantRepository
	Accounts    write.AccountRepository
	Students    write.StudentRepository // placement of the new student (student.placement_changed, ADR-0095)
	KC          port.KeycloakAdmin
	Events      write.EventPublisher
	Audit       write.Auditor
	LocalIssuer string // OIDC issuer of realm eduworld (first of EW_OIDC_ISSUERS)
	BatchMax    int
	Clock       func() time.Time
}

type validRow struct {
	username, display, ref string
	grade                  int
}

// Handle runs the command.
func (h CreateCommonStudents) Handle(ctx context.Context, in CreateCommonStudentsInput) (out CommonStudentsResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateCommonStudents")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err = authz.Require(ctx, p, authz.PermPlatformUserManage, authz.Resource{}); err != nil {
		return out, err
	}
	if in.TenantID != tenant.CommonTenantID {
		return out, problem.Invalid("X-Tenant-Id", problem.SubInvalid, "must be the common tenant")
	}
	rows, err := h.validate(in.Students)
	if err != nil {
		return out, err
	}
	sc, err := appscope.TargetTenant(p, tenant.CommonTenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	var orgID string
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Tenants.Get(ctx, tenant.CommonTenantID)
		if err != nil {
			return err
		}
		if t.OrgID == nil {
			return problem.New(problem.Unavailable, "the common tenant has no Keycloak Organization yet").Retry()
		}
		orgID = t.OrgID.String()
		return nil
	}); err != nil {
		return out, err
	}
	out.Total, out.Results = len(rows), make([]CommonStudentResult, 0, len(rows))
	for i, r := range rows {
		res := h.createOne(ctx, r, orgID)
		res.RowNo = i + 1
		if res.Status == "created" {
			out.CreatedCount++
		} else {
			out.FailedCount++
		}
		out.Results = append(out.Results, res)
	}
	// The idempotency record never holds passwords; a replay returns password_redacted = true. Rows are
	// already committed: a result above the record limit (64 KiB) is not stored rather than failing the
	// request — a replay then re-runs and every row reports USERNAME_TAKEN (no duplicate account).
	red := out.Redacted()
	if b, merr := json.Marshal(red); merr == nil && len(b) <= idempotencyRecordMax {
		err = h.UoW.Do(ctx, func(ctx context.Context) error { return idempotency.SetResult(ctx, red) })
	}
	return out, err
}

func (h CreateCommonStudents) validate(in []CommonStudentInput) ([]validRow, error) {
	max := h.BatchMax
	if max <= 0 {
		max = DefaultCommonStudentBatchMax
	}
	if len(in) == 0 || len(in) > max {
		return nil, problem.Invalid("students", problem.SubInvalid, "1 to common_student_batch_max rows")
	}
	pe := problem.New(problem.ValidationFailed, "")
	rows := make([]validRow, 0, len(in))
	seen := map[string]bool{}
	for i, s := range in {
		at := "students[" + itoa(i) + "]."
		u, err := user.NormalizeUsername(s.Username)
		if err != nil {
			pe.AddField(at+"username", problem.SubInvalid, "must match ^[A-Za-z0-9._-]{3,128}$")
		}
		d, err := user.CheckDisplayName(s.DisplayName)
		if err != nil {
			pe.AddField(at+"display_name", problem.SubInvalid, "must be 1–128 characters")
		}
		if user.CheckGrade(s.GradeLevel) != nil {
			pe.AddField(at+"grade_level", problem.SubInvalid, "must be 1–9") // F010-AC-25
		}
		ref, err := user.CheckExternalRef(s.ExternalRef)
		if err != nil {
			pe.AddField(at+"external_ref", problem.SubMaxLength, "at most 64 characters")
		}
		if u != "" && seen[u] {
			pe.AddField(at+"username", problem.SubInvalid, "duplicate username in the request")
		}
		seen[u] = true
		rows = append(rows, validRow{username: u, display: d, ref: ref, grade: s.GradeLevel})
	}
	if len(pe.Fields) > 0 {
		return nil, pe
	}
	return rows, nil
}

func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return digits[i : i+1]
	}
	return itoa(i/10) + digits[i%10:i%10+1]
}

func failed(r validRow, code problem.Code) CommonStudentResult {
	return CommonStudentResult{Status: "failed", Username: r.username, ErrorCode: string(code)}
}

// createOne provisions one row: duplicate checks, Keycloak user + password + role + membership, then the
// rows and events in one transaction; a database failure disables the Keycloak user it created.
func (h CreateCommonStudents) createOne(ctx context.Context, r validRow, orgID string) CommonStudentResult {
	var taken, refTaken bool
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if taken, err = h.Accounts.UsernameTaken(ctx, r.username); err != nil {
			return err
		}
		if r.ref != "" {
			refTaken, err = h.Accounts.ExternalRefTaken(ctx, user.ExternalRefIssuer, r.ref)
		}
		return err
	}); err != nil {
		return failed(r, problem.Unavailable)
	}
	if taken {
		return failed(r, problem.UsernameTaken) // no reconciliation issue: platform-created account
	}
	if refTaken {
		return failed(r, problem.IdentityConflict)
	}
	userID := ids.New()
	kcID, err := h.KC.CreateUser(ctx, port.KCNewUser{Username: r.username, FirstName: r.display, Enabled: true,
		Attributes: map[string][]string{"eduworld_user_id": {userID.String()}}})
	if errors.Is(err, port.ErrKCConflict) {
		return failed(r, problem.UsernameTaken)
	}
	if err != nil {
		return failed(r, problem.Unavailable)
	}
	password, err := user.GeneratePassword(r.username)
	if err == nil {
		err = h.KC.SetPassword(ctx, kcID, password, user.TemporaryPassword(r.grade))
	}
	if err == nil {
		err = h.KC.AddRealmRoles(ctx, kcID, "student")
	}
	if err == nil {
		err = h.KC.AddOrganizationMember(ctx, orgID, kcID)
	}
	if err != nil {
		_ = h.KC.DisableUser(context.WithoutCancel(ctx), kcID)
		return failed(r, problem.Unavailable)
	}
	acc := write.NewStudentAccount{UserID: userID, Username: r.username, DisplayName: r.display, TenantID: tenant.CommonTenantID,
		SchoolID: tenant.CommonSchoolID, Grade: r.grade, Issuer: h.LocalIssuer, Subject: kcID, ExternalRef: r.ref}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Accounts.CreateStudent(ctx, acc); err != nil {
			return err
		}
		return h.provisioned(ctx, acc)
	})
	if err != nil {
		_ = h.KC.DisableUser(context.WithoutCancel(ctx), kcID)
		var pe *problem.Error
		if errors.As(err, &pe) && (pe.Code == problem.UsernameTaken || pe.Code == problem.IdentityConflict) {
			return failed(r, pe.Code)
		}
		return failed(r, problem.Internal)
	}
	return CommonStudentResult{Status: "created", UserID: &userID, Username: r.username, InitialPassword: password}
}

func (h CreateCommonStudents) provisioned(ctx context.Context, a write.NewStudentAccount) error {
	t, uid := a.TenantID, a.UserID.String()
	school, grade := a.SchoolID.String(), a.Grade
	if err := h.Events.Publish(ctx, write.Event{Type: "user.provisioned", SubjectType: "user", SubjectID: uid, TenantID: &t,
		Payload: map[string]any{"user_id": uid, "user_type": "student", "source": "platform_common", "school_id": school, "grade_level": grade}}); err != nil {
		return err
	}
	// The student of the common tenant belongs to the virtual school with the declared grade (F010-AC-24, ADR-0095).
	placed, err := h.Students.Placement(ctx, a.UserID)
	if err != nil {
		return err
	}
	if placed == nil {
		return problem.New(problem.Internal, "the new student has no row")
	}
	if err := publishPlacement(ctx, h.Events, a.UserID, nil, *placed, PlacementChange{Source: placement.SourcePlatformCommon}); err != nil {
		return err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "tenant_membership.changed", SubjectType: "tenant_membership", SubjectID: uid, TenantID: &t,
		Payload: MembershipPayload{UserID: uid, TenantID: t.String(), Op: "added", Source: "platform_common", UserType: "student"}}); err != nil {
		return err
	}
	return h.Audit.Record(ctx, audit.Event{Action: "user.provisioned", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
		Metadata: map[string]any{"source": "platform_common", "grade_level": grade, "has_external_ref": strings.TrimSpace(a.ExternalRef) != ""}})
}
