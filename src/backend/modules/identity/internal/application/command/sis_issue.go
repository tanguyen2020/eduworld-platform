package command

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ResolveSisIssueInput is resolveSisReconciliationIssue.
type ResolveSisIssueInput struct {
	IssueID    uuid.UUID
	Resolution string // link | same_person | ignore | fixed_in_source
	Note       string
}

// ResolveSisIssueHandler resolves a reconciliation issue.
type ResolveSisIssueHandler interface {
	Handle(ctx context.Context, in ResolveSisIssueInput) (dto.SisIssue, error)
}

// ResolveSisIssue implements resolveSisReconciliationIssue (F011-AC-11, AC-17): tenant.sis.import on the
// school of the issue (outside the scope → 404) or platform.sis_connection.manage in the X-Tenant-Id
// tenant. same_person/link on duplicate_identifier links the existing account to the SIS source key
// instead of creating a second account; resolved_by/resolved_at and an audit record are kept.
type ResolveSisIssue struct {
	UoW    write.UnitOfWork
	Issues write.SisIssueRepository
	Events write.EventPublisher
	Audit  write.Auditor
	Clock  func() time.Time
}

var resolutions = map[string]bool{"link": true, "same_person": true, "ignore": true, "fixed_in_source": true}

// Handle resolves the issue.
func (h ResolveSisIssue) Handle(ctx context.Context, in ResolveSisIssueInput) (out dto.SisIssue, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ResolveSisReconciliationIssue")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !resolutions[in.Resolution] {
		return out, problem.Invalid("resolution", problem.SubInvalid, "must be link, same_person, ignore or fixed_in_source")
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > 1000 {
		return out, problem.Invalid("resolution_note", problem.SubMaxLength, "at most 1000 characters")
	}
	if p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "X-Tenant-Id is required")
	}
	perm := authz.PermTenantSisImport
	if !p.Has(perm) {
		perm = authz.PermPlatformSisConnectionManage
		if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
			return out, err
		}
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return out, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	ctx = tenancy.WithScope(ctx, sc)
	now := time.Now().UTC()
	if h.Clock != nil {
		now = h.Clock().UTC()
	}
	var row *write.IssueRow
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if row, err = h.Issues.LockIssue(ctx, in.IssueID); err != nil {
			return err
		}
		if perm == authz.PermTenantSisImport {
			if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: &row.TenantID, SchoolID: row.SchoolID}); err != nil {
				return problem.New(problem.NotFound, "") // outside the school scope (F011-AC-17)
			}
		}
		if row.Status != "open" {
			return problem.New(problem.Conflict, "the issue is no longer open")
		}
		if in.Resolution == "same_person" || in.Resolution == "link" {
			if err := h.linkExisting(ctx, row); err != nil {
				return err
			}
		}
		res, by := in.Resolution, p.UserID
		row.Status, row.Resolution, row.ResolvedBy, row.ResolvedAt = "resolved", &res, &by, &now
		if in.Resolution == "ignore" {
			row.Status = "ignored"
		}
		if note != "" {
			row.Note = &note
		}
		if err := h.Issues.SaveResolution(ctx, row); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: "sis.reconciliation_issue_resolved", TargetType: "reconciliation_issue",
			TargetID: row.ID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"issue_type": row.Type, "resolution": in.Resolution}})
	})
	if err != nil {
		return out, err
	}
	out = dto.SisIssue{IssueID: row.ID, TenantID: row.TenantID, SchoolID: row.SchoolID, ConnectionID: row.ConnectionID, IssueType: row.Type,
		EntityRefs: dto.SisIssueRefs(row.Refs), SuggestedAction: row.Suggested, Status: row.Status, Resolution: row.Resolution,
		ResolutionNote: row.Note, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, CreatedAt: row.CreatedAt}
	if !p.IsSystemAdmin {
		out.HideConflictingUser()
	}
	return out, nil
}

// linkExisting attaches the account holding the identifier to the SIS source key of the issue (F011-AC-11).
// A student of another tenant needs a transfer first (a student is a member of exactly one tenant,
// ACCOUNT_PROVISIONING §2.1) — OQ F011-1.
func (h ResolveSisIssue) linkExisting(ctx context.Context, row *write.IssueRow) error {
	if row.Type != sis.IssueDuplicateIdentifier {
		return problem.Invalid("resolution", problem.SubInvalid, "same_person and link apply to duplicate_identifier issues")
	}
	userID, err := uuid.Parse(refString(row.Refs, "user_id"))
	ext := refString(row.Refs, "student_external_id")
	if err != nil || ext == "" || row.ConnectionID == nil || row.SchoolID == nil {
		return problem.Invalid("resolution", problem.SubInvalid, "the issue has no SIS record to link")
	}
	found, linked, err := h.Issues.StudentOf(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return problem.New(problem.Conflict, "the account is not a student of this tenant; transfer it first")
	}
	if linked {
		return problem.New(problem.Conflict, "the student is already linked to a SIS record")
	}
	if err := h.Issues.LinkExisting(ctx, userID, row.TenantID, *row.SchoolID, *row.ConnectionID, ext); err != nil {
		return err
	}
	t := row.TenantID
	return h.Events.Publish(ctx, write.Event{Type: "student.sis_link_created", SubjectType: "user", SubjectID: userID.String(), TenantID: &t,
		Payload: map[string]any{"user_id": userID.String(), "tenant_id": t.String(), "school_id": row.SchoolID.String(),
			"sis_connection_id": row.ConnectionID.String(), "student_external_id": ext, "via": "provisioning"}})
}

func refString(refs map[string]any, k string) string {
	s, _ := refs[k].(string)
	return s
}
