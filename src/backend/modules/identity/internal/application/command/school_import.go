package command

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
)

// SchoolImportRow is one parsed row of a school import (CSV columns of ADMIN_PORTAL §3.16).
type SchoolImportRow struct {
	Code, NameVi, NameEn, ProvinceCode, MoetCode, SchoolLevel, Address string
	GradeMin, GradeMax                                                 int
}

// SchoolImportMessage is an error or warning of one row.
type SchoolImportMessage struct {
	Row     int    `json:"row"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// SchoolImportResult summarizes an import (SchoolImportJob counters).
type SchoolImportResult struct {
	TotalRows   int                   `json:"total_rows"`
	CreatedRows int                   `json:"created_rows"`
	ErrorRows   int                   `json:"error_rows"`
	Errors      []SchoolImportMessage `json:"errors"`
	Warnings    []SchoolImportMessage `json:"warnings"`
	DryRun      bool                  `json:"dry_run"`
}

// ImportSchoolsHandler imports schools into one target tenant (F010-AC-08). The asynchronous job of
// importSchools (202 + getSchoolImport, file storage, IDENTITY_SIS_JOBS) runs this use case per batch; the
// job store comes with F011 (sis_imports).
type ImportSchoolsHandler interface {
	Handle(ctx context.Context, tenantID uuid.UUID, rows []SchoolImportRow, dryRun bool) (SchoolImportResult, error)
}

// ImportSchools implements ImportSchoolsHandler: a former province maps to its successor (warning
// province_outdated), a missing province puts the school in pending_review, an unknown province or a
// duplicate MOET/code is a row error (school_duplicate is never merged automatically). A dry run rolls back.
type ImportSchools struct {
	SchoolDeps
	Savepoints write.Savepoints
}

var errDryRun = errors.New("dry run")

// Handle runs the import in one transaction.
func (h ImportSchools) Handle(ctx context.Context, tenantID uuid.UUID, rows []SchoolImportRow, dryRun bool) (out SchoolImportResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ImportSchools")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err = authz.Require(ctx, p, authz.PermPlatformCatalogWrite, authz.Resource{}); err != nil {
		return out, err
	}
	sc, err := appscope.TargetTenant(p, tenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	out = SchoolImportResult{TotalRows: len(rows), DryRun: dryRun, Errors: []SchoolImportMessage{}, Warnings: []SchoolImportMessage{}}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Tenants.Get(ctx, tenantID)
		if err != nil {
			return err
		}
		existing, err := h.Schools.CountInTenant(ctx, tenantID)
		if err != nil {
			return err
		}
		for i, r := range rows {
			n := i + 1
			if err := school.CanAdd(t.Type, existing); err != nil {
				out.Errors = append(out.Errors, SchoolImportMessage{Row: n, Code: "VALIDATION_FAILED", Message: "tenant shape: " + err.Error()})
				continue
			}
			s, warn, code := h.importRow(ctx, tenantID, r)
			if code != "" {
				out.Errors = append(out.Errors, SchoolImportMessage{Row: n, Code: code})
				continue
			}
			if warn != "" {
				out.Warnings = append(out.Warnings, SchoolImportMessage{Row: n, Code: warn})
			}
			// Each row in its own savepoint: a duplicate aborts only that row (PostgreSQL aborts the whole
			// transaction on an error otherwise).
			err := h.Savepoints.Savepoint(ctx, func(ctx context.Context) error {
				if err := h.Schools.Create(ctx, s); err != nil {
					return err
				}
				return h.emit(ctx, s, "school.imported", "", nil)
			})
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.Conflict {
				out.Errors = append(out.Errors, SchoolImportMessage{Row: n, Code: "school_duplicate"})
				continue
			}
			if err != nil {
				return err
			}
			existing++
			out.CreatedRows++
		}
		out.ErrorRows = len(out.Errors)
		if dryRun {
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		err = nil
	}
	return out, err
}

// importRow builds the school of one row; it returns a warning code or an error code.
func (h ImportSchools) importRow(ctx context.Context, tenantID uuid.UUID, r SchoolImportRow) (*school.School, string, string) {
	var prov *catalog.Province
	warn := ""
	if code := strings.TrimSpace(r.ProvinceCode); code != "" {
		p, err := h.Catalog.ProvinceByCode(ctx, code)
		if err != nil {
			return nil, "", "province_unknown"
		}
		var succ *catalog.Province
		if !p.Active() {
			if succ, err = h.Catalog.Successor(ctx, p.ID); err != nil {
				return nil, "", "province_unknown"
			}
		}
		res, err := catalog.ResolveForImport(p, succ)
		if err != nil {
			return nil, "", "province_unknown"
		}
		if res.Outdated {
			warn = "province_outdated"
		}
		prov = &res.Province
	}
	s, err := school.NewImported(ids.New(), tenantID, r.Code, school.Name{Vi: r.NameVi, En: r.NameEn}, "VN", prov,
		school.Level(r.SchoolLevel), r.GradeMin, r.GradeMax, optional(r.MoetCode), optional(r.Address))
	if err != nil {
		return nil, "", "VALIDATION_FAILED"
	}
	if prov == nil {
		warn = "province_missing"
	}
	return s, warn, ""
}

func optional(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
