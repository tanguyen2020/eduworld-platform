package command

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Limits of CSV imports (CONFIG_DEFAULTS) [Proposed].
const (
	CsvImportMax       = 10 << 20         // csv_import_max: 10 MB
	SisImportUploadTTL = 15 * time.Minute // presigned upload TTL of createSisImport
)

var importKinds = map[string]bool{"schools": true, "students": true, "classes": true, "enrollments": true}

// CreateSisImportInput is createSisImport (school path) or createPlatformSisImport (school in the body).
type CreateSisImportInput struct {
	SchoolID    uuid.UUID
	FileName    string
	Size        int64
	SHA256      string
	EntityKinds []string
	DryRun      *bool // default true (F011-AC-01)
	Platform    bool  // createPlatformSisImport (platform.sis_connection.manage in the X-Tenant-Id tenant)
}

// CreateSisImportHandler creates an import and returns the presigned upload URL.
type CreateSisImportHandler interface {
	Handle(ctx context.Context, in CreateSisImportInput) (dto.SisImport, error)
}

// CreateSisImport implements createSisImport (F011-AC-06): tenant.sis.import on the school (School Admin),
// one CSV of one entity kind up to csv_import_max, a school served by an active csv_import connection.
type CreateSisImport struct {
	UoW     write.UnitOfWork
	Imports write.SisImportRepository
	Storage port.ObjectStorage
	Audit   write.Auditor
	Clock   func() time.Time
}

// Handle creates the import.
func (h CreateSisImport) Handle(ctx context.Context, in CreateSisImportInput) (out dto.SisImport, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateSisImport")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if in.Platform {
		if err := authz.Require(ctx, p, authz.PermPlatformSisConnectionManage, authz.Resource{}); err != nil {
			return out, err
		}
		if p.TenantID == nil {
			return out, problem.New(problem.TenantHeaderRequired, "")
		}
		sc, err := appscope.TargetTenant(p, *p.TenantID)
		if err != nil {
			return out, err
		}
		ctx = tenancy.WithScope(ctx, sc)
	} else {
		if err := authz.Require(ctx, p, authz.PermTenantSisImport, authz.Resource{TenantID: p.TenantID, SchoolID: &in.SchoolID}); err != nil {
			return out, err
		}
		sc, err := authz.ScopeFor(p)
		if err != nil {
			return out, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		ctx = tenancy.WithScope(ctx, sc)
	}
	pe := problem.New(problem.ValidationFailed, "")
	name := strings.TrimSpace(in.FileName)
	if name == "" || len(name) > 200 || !strings.HasSuffix(strings.ToLower(name), ".csv") {
		pe.AddField("file_name", problem.SubInvalid, "a .csv file name of at most 200 characters")
	}
	if in.Size < 1 || in.Size > CsvImportMax {
		pe.AddField("size_bytes", problem.SubInvalid, "1 byte to 10 MB (csv_import_max)")
	}
	if len(in.EntityKinds) != 1 || !importKinds[in.EntityKinds[0]] {
		pe.AddField("entity_kinds", problem.SubInvalid, "exactly one of schools, students, classes, enrollments")
	}
	if len(in.SHA256) > 128 {
		pe.AddField("sha256", problem.SubMaxLength, "at most 128 characters")
	}
	if len(pe.Fields) > 0 {
		return out, pe
	}
	dry := true
	if in.DryRun != nil {
		dry = *in.DryRun
	}
	now := time.Now().UTC()
	if h.Clock != nil {
		now = h.Clock().UTC()
	}
	row := &write.SisImportRow{ID: ids.New(), SchoolID: in.SchoolID, RequestedBy: p.UserID, Size: in.Size, Kinds: in.EntityKinds,
		Status: "awaiting_upload", DryRun: dry}
	if in.SHA256 != "" {
		sha := in.SHA256
		row.SHA256 = &sha
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		sc, _ := tenancy.ScopeFrom(ctx)
		tenants := sc.TenantIDs()
		if len(tenants) != 1 {
			return problem.New(problem.PermissionDenied, "")
		}
		row.TenantID = tenants[0]
		if _, err := h.Imports.CsvConnectionOf(ctx, in.SchoolID); err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.NotFound {
				return problem.Invalid("school_id", problem.SubInvalid, "no active csv_import connection serves this school")
			}
			return err
		}
		row.StorageKey = "sis-imports/" + row.TenantID.String() + "/" + row.ID.String() + ".csv"
		if err := h.Imports.CreateImport(ctx, row); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: "sis_import.created", TargetType: "sis_import", TargetID: row.ID.String(), Result: audit.ResultSuccess,
			Metadata: map[string]any{"school_id": in.SchoolID.String(), "entity_kind": in.EntityKinds[0], "size_bytes": in.Size, "dry_run": dry}})
	})
	if err != nil {
		return out, err
	}
	url, err := h.Storage.PresignPut(ctx, row.StorageKey, in.Size, SisImportUploadTTL)
	if err != nil {
		return out, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	exp := now.Add(SisImportUploadTTL)
	out = ImportView(row)
	out.UploadURL, out.UploadExpiresAt = &url, &exp
	return out, nil
}

// ImportView maps an import row to the API.
func ImportView(r *write.SisImportRow) dto.SisImport {
	return dto.NewSisImport(r.ID, r.SchoolID, r.Status, r.DryRun, r.Kinds, r.RunID, r.Stats, r.CreatedAt)
}

// ProcessSisImportHandler processes an uploaded import file (sis-jobs worker, job csv_import).
type ProcessSisImportHandler interface {
	Handle(ctx context.Context, tenantID, importID uuid.UUID) error
}

// ProcessSisImport scans the uploaded file, reads it against the contract and runs the pipeline as the
// system actor (dry run or apply as requested). The caller sets nothing: the scope comes from tenantID.
type ProcessSisImport struct {
	UoW     write.UnitOfWork
	Imports write.SisImportRepository
	Storage port.ObjectStorage
	Scanner port.MalwareScanner
	Parser  port.SisFileParser
	Run     SisRun
	Audit   write.Auditor
	Clock   func() time.Time
}

// Handle processes the import once (a processed import is a no-op).
func (h ProcessSisImport) Handle(ctx context.Context, tenantID, importID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ProcessSisImport")
	defer func() { done(err) }()
	sc, err := authz.EventScope(tenantID)
	if err != nil {
		return err
	}
	ctx = tenancy.WithScope(ctx, sc)
	now := time.Now().UTC()
	if h.Clock != nil {
		now = h.Clock().UTC()
	}
	var row *write.SisImportRow
	var conn uuid.UUID
	if err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if row, err = h.Imports.GetImport(ctx, importID); err != nil {
			return err
		}
		if row.Status != "awaiting_upload" && row.Status != "queued" {
			return nil
		}
		conn, err = h.Imports.CsvConnectionOf(ctx, row.SchoolID)
		return err
	}); err != nil {
		return err
	}
	if row.Status != "awaiting_upload" && row.Status != "queued" {
		return nil
	}
	size, err := h.Storage.Head(ctx, row.StorageKey)
	if errors.Is(err, port.ErrObjectNotFound) {
		if now.Sub(row.CreatedAt) > SisImportUploadTTL {
			return h.finish(ctx, row, "expired", nil, "upload_expired")
		}
		return nil // not uploaded yet
	}
	if err != nil {
		return err
	}
	if size != row.Size || size > CsvImportMax {
		return h.finish(ctx, row, "failed", nil, "size_mismatch")
	}
	if err := h.setStatus(ctx, row, "validating"); err != nil {
		return err
	}
	clean, sig, err := h.scan(ctx, row.StorageKey)
	if err != nil {
		return err // redelivered: the scanner or the storage is unavailable
	}
	if !clean {
		return h.finish(ctx, row, "failed", nil, "malware:"+sig)
	}
	file, err := h.parse(ctx, row, now)
	if err != nil {
		return h.finish(ctx, row, "failed", nil, "unreadable_csv")
	}
	if !row.DryRun {
		if err := h.setStatus(ctx, row, "applying"); err != nil {
			return err
		}
	}
	res, err := h.Run.RunImportSystem(ctx, tenantID, RunImportInput{ConnectionID: conn, SchoolID: row.SchoolID, Files: []SisFile{file}, DryRun: row.DryRun})
	if err != nil {
		return err
	}
	return h.finish(ctx, row, importStatus(res), &res, "")
}

func importStatus(res RunImportResult) string {
	switch {
	case res.Status == "failed":
		return "failed"
	case res.Status == "partially_failed" || res.Issues > 0:
		return "completed_with_issues"
	}
	return "completed"
}

func (h ProcessSisImport) scan(ctx context.Context, key string) (bool, string, error) {
	rc, err := h.Storage.Open(ctx, key)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = rc.Close() }()
	return h.Scanner.Scan(ctx, io.LimitReader(rc, CsvImportMax+1))
}

func (h ProcessSisImport) parse(ctx context.Context, row *write.SisImportRow, now time.Time) (SisFile, error) {
	rc, err := h.Storage.Open(ctx, row.StorageKey)
	if err != nil {
		return SisFile{}, err
	}
	defer func() { _ = rc.Close() }()
	pf, err := h.Parser.Parse(row.Kinds[0], io.LimitReader(rc, CsvImportMax+1), now)
	if err != nil {
		return SisFile{}, err
	}
	f := SisFile{Kind: pf.Kind}
	for _, r := range pf.Records {
		f.Records = append(f.Records, SisRecord{Row: r.Row, Data: r.Data})
	}
	for _, v := range pf.Violations {
		f.Errors = append(f.Errors, write.RecordError{Kind: pf.Kind, Row: v.Row, Column: v.Column, Code: v.Code, Message: v.Message})
	}
	return f, nil
}

func (h ProcessSisImport) setStatus(ctx context.Context, row *write.SisImportRow, status string) error {
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		row.Status = status
		return h.Imports.SaveImport(ctx, row)
	})
}

// finish stores the outcome and audits it (no PII: counts and codes only).
func (h ProcessSisImport) finish(ctx context.Context, row *write.SisImportRow, status string, res *RunImportResult, failure string) error {
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		row.Status = status
		if res != nil {
			id := res.RunID
			row.RunID = &id
			total, rejected := 0, len(res.Errors)
			for _, actions := range res.Diff {
				for a, n := range actions {
					row.Stats["diff_"+a] = n
					total += n
				}
			}
			row.Stats["rows_total"] = total + rejected
			row.Stats["rows_rejected"] = rejected
			if !res.DryRun {
				row.Stats["rows_applied"] = total
			}
			row.Stats["issues"] = res.Issues
		}
		if err := h.Imports.SaveImport(ctx, row); err != nil {
			return err
		}
		meta := map[string]any{"status": status, "actor": "system:sis-sync"}
		if failure != "" {
			meta["failure"] = failure
		}
		return h.Audit.Record(ctx, audit.Event{Action: "sis_import.processed", TargetType: "sis_import", TargetID: row.ID.String(),
			Result: audit.ResultSuccess, Metadata: meta})
	})
}

// WatchSisImports detects uploaded files (no bucket notifications on every S3-compatible storage): imports
// awaiting their upload are checked with HEAD; an uploaded file queues one csv_import job (deduplicated by
// Nats-Msg-Id import:<id>); an import past its upload TTL expires. Runs in the jobs worker (platform_job).
type WatchSisImports struct {
	UoW     write.UnitOfWork
	Imports write.SisImportRepository
	Storage port.ObjectStorage
	Jobs    port.SisImportJobQueue
	Clock   func() time.Time
	Batch   int
}

// RunOnce checks one batch and returns the number of imports queued.
func (h WatchSisImports) RunOnce(ctx context.Context) (int, error) {
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	batch := h.Batch
	if batch <= 0 {
		batch = 100
	}
	var rows []write.SisImportRow
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		rows, err = h.Imports.AwaitingUploads(ctx, batch)
		return err
	}); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	if h.Clock != nil {
		now = h.Clock().UTC()
	}
	queued := 0
	for _, r := range rows {
		_, err := h.Storage.Head(ctx, r.StorageKey)
		switch {
		case errors.Is(err, port.ErrObjectNotFound):
			if now.Sub(r.CreatedAt) > SisImportUploadTTL {
				if err := h.Jobs.EnqueueCsvImport(ctx, port.CsvImportJob{TenantID: r.TenantID, ImportID: r.ID}); err != nil {
					return queued, err // the job expires it
				}
			}
		case err != nil:
			return queued, err
		default:
			if err := h.Jobs.EnqueueCsvImport(ctx, port.CsvImportJob{TenantID: r.TenantID, ImportID: r.ID}); err != nil {
				return queued, err
			}
			queued++
		}
	}
	return queued, nil
}
