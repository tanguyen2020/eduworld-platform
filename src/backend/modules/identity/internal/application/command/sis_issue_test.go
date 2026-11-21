package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// fakeIssueRepo is a TEST DOUBLE of the reconciliation issue repository (one issue).
type fakeIssueRepo struct {
	write.SisIssueRepository
	row   write.IssueRow
	saved int
}

func (f *fakeIssueRepo) LockIssue(_ context.Context, id uuid.UUID) (*write.IssueRow, error) {
	if id != f.row.ID {
		return nil, problem.New(problem.NotFound, "")
	}
	r := f.row
	return &r, nil
}

func (f *fakeIssueRepo) SaveResolution(_ context.Context, r *write.IssueRow) error {
	f.saved++
	f.row = *r
	return nil
}

// F052-1: resolveSchoolReconciliationIssue reuses the F011 handler with the school of the path — fixed_in_source
// or ignore with a 3–500 character reason, tenant.sis.import on that school (403 / 404), the issue of that
// school only, and the audit sis.reconciliation_issue.resolved with the school.
func TestResolveSchoolSisIssue(t *testing.T) {
	ctx := context.Background()
	tenant, school, other := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeIssueRepo{row: write.IssueRow{ID: uuid.New(), TenantID: tenant, SchoolID: &school, Type: "duplicate_identifier",
		Refs: map[string]any{"user_id": uuid.New().String(), "field": "username"}, Status: "open"}}
	aud := &fakeAudit{}
	h := command.ResolveSisIssue{UoW: &fakeUoW{}, Issues: repo, Audit: aud}
	admin := authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school)))
	in := func(res, reason string, s uuid.UUID) command.ResolveSisIssueInput {
		return command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: res, Note: reason, SchoolID: &s}
	}
	for name, c := range map[string]command.ResolveSisIssueInput{
		"same_person":  in("same_person", "same child", school),
		"link":         in("link", "same child", school),
		"short reason": in("ignore", " ab ", school),
		"long reason":  in("ignore", string(make([]rune, 501)), school),
	} {
		if _, err := h.Handle(admin, c); codeOf(err) != problem.ValidationFailed {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := h.Handle(authztest.Ctx(ctx, authztest.NoRole(tenant)), in("ignore", "not ours", school)); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("no permission: %v", err)
	}
	if _, err := h.Handle(authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(other))), in("ignore", "not ours", school)); codeOf(err) != problem.NotFound {
		t.Fatalf("another school's admin: %v", err)
	}
	if _, err := h.Handle(authztest.Ctx(ctx, authztest.Staff(tenant, "tenant_admin")), in("ignore", "not ours", other)); codeOf(err) != problem.NotFound {
		t.Fatalf("issue under another school's path: %v", err)
	}
	if repo.saved != 0 {
		t.Fatal("a refused request saved the issue")
	}
	out, err := h.Handle(admin, in("fixed_in_source", "  fixed in SIS  ", school))
	if err != nil || out.Status != "resolved" || out.ResolutionNote == nil || *out.ResolutionNote != "fixed in SIS" || out.ResolvedBy == nil {
		t.Fatalf("resolve = %+v %v", out, err)
	}
	if _, ok := out.EntityRefs["user_id"]; ok {
		t.Fatal("the conflicting account is returned on the school route")
	}
	if len(aud.events) != 1 || aud.events[0].Action != command.AuditSisIssueResolved || aud.events[0].SchoolID != school.String() ||
		aud.events[0].Metadata["route"] != "school" {
		t.Fatalf("audit = %+v", aud.events)
	}
	if _, err := h.Handle(admin, in("ignore", "again", school)); codeOf(err) != problem.Conflict {
		t.Fatalf("resolve twice: %v", err)
	}
}

func codeOf(err error) problem.Code {
	var p *problem.Error
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

// F011 T041 (OQ F045-7): School Admin proposes a merge on a duplicate issue of their school — resolution
// merge_requested with a reason, the issue stays open (System Admin runs mergeUsers), audited; refused on other issue
// types, on the platform route and when already requested.
func TestRequestMergeOnSchoolSisIssue(t *testing.T) {
	ctx := context.Background()
	tenant, school := uuid.New(), uuid.New()
	issue := func(typ string) *fakeIssueRepo {
		return &fakeIssueRepo{row: write.IssueRow{ID: uuid.New(), TenantID: tenant, SchoolID: &school, Type: typ, Refs: map[string]any{}, Status: "open"}}
	}
	admin := authztest.Ctx(ctx, authztest.Staff(tenant, "school_admin", authztest.AtSchool(school)))
	for _, typ := range []string{"duplicate_identifier", "possible_duplicate"} {
		repo, aud := issue(typ), &fakeAudit{}
		h := command.ResolveSisIssue{UoW: &fakeUoW{}, Issues: repo, Audit: aud}
		out, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: "merge_requested", Note: " cùng một học sinh ", SchoolID: &school})
		if err != nil || out.Status != "open" || out.Resolution == nil || *out.Resolution != "merge_requested" || out.ResolvedBy != nil ||
			out.ResolvedAt != nil || *out.ResolutionNote != "cùng một học sinh" {
			t.Fatalf("%s: %+v %v", typ, out, err)
		}
		if len(aud.events) != 1 || aud.events[0].Metadata["resolution"] != "merge_requested" || aud.events[0].SchoolID != school.String() {
			t.Fatalf("%s audit = %+v", typ, aud.events)
		}
		// Already requested: 409; the school may still close it otherwise (fixed_in_source).
		if _, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: "merge_requested", Note: "again", SchoolID: &school}); codeOf(err) != problem.Conflict {
			t.Fatalf("%s twice: %v", typ, err)
		}
		if out, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: "fixed_in_source", Note: "sửa ở SIS", SchoolID: &school}); err != nil || out.Status != "resolved" {
			t.Fatalf("%s close after request: %+v %v", typ, out, err)
		}
	}
	repo := issue("missing_class")
	h := command.ResolveSisIssue{UoW: &fakeUoW{}, Issues: repo, Audit: &fakeAudit{}}
	if _, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: "merge_requested", Note: "không phải trùng", SchoolID: &school}); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("missing_class: %v", err)
	}
	if _, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: "merge_requested", Note: "platform route"}); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("platform route: %v", err)
	}
	if _, err := h.Handle(admin, command.ResolveSisIssueInput{IssueID: repo.row.ID, Resolution: "merge_requested", Note: "ab", SchoolID: &school}); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("short reason: %v", err)
	}
	if repo.saved != 0 {
		t.Fatal("a refused request saved the issue")
	}
}
