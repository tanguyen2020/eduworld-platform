package write

import (
	"context"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	enttj "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/translationjob"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// TranslationJobRepo implements write.TranslationJobStore with ent (entx.TenantOptional filter of the scope of ctx).
type TranslationJobRepo struct{}

var _ write.TranslationJobStore = TranslationJobRepo{}

func translationJobOf(r *ent.TranslationJob) write.TranslationJob {
	return write.TranslationJob{ID: r.ID, Owner: owner(r.OwnerScope, r.TenantID), Kind: r.Kind, Status: r.Status, Format: r.Format,
		SourceLang: r.SourceLanguage, TargetLang: r.TargetLanguage, Request: r.Request, InputAssetID: r.InputAssetID,
		ResultKey: r.ResultStorageKey, Report: r.Report, RequestedBy: r.RequestedBy, CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt}
}

// InsertTranslationJob implements write.TranslationJobStore.
func (TranslationJobRepo) InsertTranslationJob(ctx context.Context, j write.TranslationJob) error {
	q := client(ctx).TranslationJob.Create().SetID(j.ID).SetOwnerScope(string(j.Owner.Scope)).SetKind(j.Kind).SetStatus(j.Status).
		SetFormat(j.Format).SetSourceLanguage(j.SourceLang).SetTargetLanguage(j.TargetLang).SetRequest(jsonOr(j.Request, "{}")).
		SetNillableInputAssetID(j.InputAssetID).SetRequestedBy(j.RequestedBy)
	if j.Owner.TenantID != nil {
		q.SetTenantID(*j.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// TranslationJob implements write.TranslationJobStore.
func (TranslationJobRepo) TranslationJob(ctx context.Context, id uuid.UUID) (write.TranslationJob, error) {
	r, err := client(ctx).TranslationJob.Query().Where(enttj.ID(id)).Only(ctx)
	if err != nil {
		return write.TranslationJob{}, MapCurriculumErr(err)
	}
	return translationJobOf(r), nil
}

// UpdateTranslationJob implements write.TranslationJobStore.
func (TranslationJobRepo) UpdateTranslationJob(ctx context.Context, j write.TranslationJob) error {
	u := client(ctx).TranslationJob.Update().Where(enttj.ID(j.ID)).SetStatus(j.Status).SetNillableResultStorageKey(j.ResultKey).
		SetNillableFinishedAt(j.FinishedAt).AddVersion(1)
	if present(j.Report) {
		u.SetReport(j.Report)
	}
	return MapCurriculumErr(u.Exec(ctx))
}

// QueuedTranslationJobs implements write.TranslationJobStore.
func (TranslationJobRepo) QueuedTranslationJobs(ctx context.Context, limit int) ([]write.TranslationJob, error) {
	rows, err := client(ctx).TranslationJob.Query().Where(enttj.Status("queued")).
		Order(enttj.ByCreatedAt(entsql.OrderAsc()), enttj.ByID(entsql.OrderAsc())).Limit(limit).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]write.TranslationJob, len(rows))
	for i, r := range rows {
		out[i] = translationJobOf(r)
	}
	return out, nil
}
