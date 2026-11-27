package command

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the curriculum workflow (plan.md "Bảo mật, tenant, idempotency, audit"; target_type curriculum).
const (
	ActionContentVersionCreated = "content.version_created"
	ActionContentSubmitted      = "content.submitted"
	ActionContentApproved       = "content.approved"
	ActionContentRejected       = "content.rejected"
	ActionContentRevised        = "content.revised"
	ActionContentPublished      = "content.published"
	ActionContentArchived       = "content.archived"
)

// EventCurriculumVersionPublished is the event of a publish (specs/events/payloads/learning/curriculum.version_published.v1).
const EventCurriculumVersionPublished = "curriculum.version_published"

// CurriculumMetrics counts publishes and import jobs (eduworld_curriculum_publish_total, eduworld_import_jobs_total,
// eduworld_import_rows_rejected_total); nil = not counted.
type CurriculumMetrics interface {
	Published(result string)
	ImportFinished(kind, status string, rejected int)
}

// Curriculum holds the collaborators of the curriculum commands (F021): the use cases are its methods, each one
// authorizing first (authz.Require) and running one UnitOfWork.
type Curriculum struct {
	UoW       write.UnitOfWork
	Store     write.CurriculumStore
	Catalog   write.CatalogStore
	Imports   write.ImportJobStore
	Resources *port.Registry
	Events    write.EventPublisher
	Audit     write.Auditor
	Assets    port.AssetReader
	Codec     port.CurriculumCodec
	Content   *ContentImport // question imports of F024 (nil: only curriculum kinds)
	Docx      *DocxImport    // Word import lessons_docx of F028 (nil: refused with IMPORT_KIND_NOT_SUPPORTED)
	Metrics   CurriculumMetrics
	Clock     func() time.Time
}

func (h Curriculum) now() time.Time { return now(h.Clock) }

func (h Curriculum) trash() TrashDeps {
	return TrashDeps{UoW: h.UoW, Resources: h.Resources, Events: h.Events, Audit: h.Audit, Clock: h.Clock}
}

// authorize returns the principal holding perm (tenant scope of the session for tenant users).
func authorize(ctx context.Context, perm authz.Permission) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	return p, scope.RequireContent(ctx, p, perm, nil)
}

// denied answers 403 PERMISSION_DENIED and records authz.denied (F021-AC-06). The audit is written outside the
// transaction of the use case (a fresh context with the principal and scope only): the denial rolls the transaction
// back, the audit must stay.
func denied(ctx context.Context, why string) error {
	rctx := context.Background()
	if p, ok := authz.PrincipalFrom(ctx); ok {
		rctx = authz.WithPrincipal(rctx, p)
	}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		rctx = tenancy.WithScope(rctx, s)
	}
	authz.RecordDenied(rctx, string(problem.PermissionDenied), map[string]any{"why": why}) //nolint:contextcheck // detached on purpose
	return problem.New(problem.PermissionDenied, "")
}

// platformOnly: the platform catalogs (subjects, textbooks) are written by platform staff only.
func platformOnly(ctx context.Context, p authz.Principal) error {
	if !p.IsPlatform() {
		return denied(ctx, "platform_catalog")
	}
	return nil
}

// writable applies the owner rule of a curriculum write (RequireWritable) and the permission on the owner tenant;
// a tenant user writing platform content is audited as denied (F021-AC-06).
func writable(ctx context.Context, p authz.Principal, perm authz.Permission, o contentmodel.Owner) error {
	if err := RequireWritable(p, o); err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.PermissionDenied {
			return denied(ctx, "platform_content_read_only")
		}
		return err
	}
	return scope.RequireContent(ctx, p, perm, o.TenantID)
}

// curriculumErr maps domain errors of the curriculum to problems.
func curriculumErr(err error) error {
	var ce curriculum.CycleError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, curriculum.ErrNotEditable):
		return problem.New(problem.ContentNotEditable, "the curriculum version is not a draft; create a new version")
	case errors.Is(err, curriculum.ErrOpenDraft):
		return problem.New(problem.Conflict, "the curriculum already has an open draft or in-review version")
	case errors.Is(err, contentmodel.ErrTransition), errors.Is(err, contentmodel.ErrReviewerIsAuthor), errors.Is(err, contentmodel.ErrCurrentVersion):
		return problem.New(problem.InvalidStateTransition, err.Error())
	case errors.Is(err, contentmodel.ErrReasonRequired):
		return problem.Invalid("note", problem.SubRequired, "a note is required to reject")
	case errors.As(err, &ce):
		return problem.New(problem.ValidationFailed, "prerequisite cycle")
	}
	return invalid(err)
}

// SubPrerequisiteCycle is errors[].code of a prerequisite cycle (F021-AC-12).
const SubPrerequisiteCycle = "PREREQUISITE_CYCLE"

// cycleProblem lists the LO codes of a prerequisite cycle (A → B → C → A).
func cycleProblem(ce curriculum.CycleError, codes map[uuid.UUID]string) error {
	path := make([]string, len(ce.Path))
	for i, id := range ce.Path {
		path[i] = codes[id]
	}
	pe := &problem.Error{Code: problem.ValidationFailed, Detail: "the prerequisites create a cycle"}
	pe.AddField("prerequisite_ids", SubPrerequisiteCycle, strings.Join(path, " → "))
	return pe.With("cycle", path)
}

// draftVersion loads a version of a live curriculum the principal may write, which must be draft.
func (h Curriculum) draftVersion(ctx context.Context, p authz.Principal, versionID uuid.UUID) (dto.CurriculumVersion, error) {
	v, err := h.Store.VersionByID(ctx, versionID)
	if err != nil {
		return v, err
	}
	if _, err := h.Store.Curriculum(ctx, v.CurriculumID); err != nil {
		return v, err
	}
	if err := writable(ctx, p, authz.PermCurriculumWrite, v.Owner); err != nil {
		return v, err
	}
	return v, curriculumErr(curriculum.EnsureDraft(v.Status))
}

// recordCurriculum audits e on the owner tenant of the curriculum (platform audit for platform content).
func (h Curriculum) recordCurriculum(ctx context.Context, o contentmodel.Owner, e audit.Event) error {
	e.TargetType = string(contentmodel.ResourceCurriculum)
	if e.Result == "" {
		e.Result = audit.ResultSuccess
	}
	return h.trash().recordFor(ctx, o.TenantID, e)
}

// refOf builds the short reference of a version.
func refOf(v dto.CurriculumVersion) *dto.VersionRef {
	return &dto.VersionRef{ID: v.ID, VersionNo: v.VersionNo, Label: v.Label, Status: v.Status}
}

// withRefs fills the current and latest version references of a curriculum.
func (h Curriculum) withRefs(ctx context.Context, c dto.Curriculum) (dto.Curriculum, error) {
	vs, err := h.Store.Versions(ctx, c.ID)
	if err != nil {
		return c, err
	}
	for _, v := range vs {
		if c.Latest == nil || v.VersionNo > c.Latest.VersionNo {
			c.Latest = refOf(v)
		}
		if c.CurrentVersionID != nil && v.ID == *c.CurrentVersionID {
			c.Current = refOf(v)
		}
	}
	return c, nil
}

// checkRowVersion compares the If-Match row version with the loaded row (412 VERSION_MISMATCH).
func checkRowVersion(loaded, ifMatch int64) error {
	if loaded != ifMatch {
		return problem.New(problem.VersionMismatch, "")
	}
	return nil
}

func ptrClone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
