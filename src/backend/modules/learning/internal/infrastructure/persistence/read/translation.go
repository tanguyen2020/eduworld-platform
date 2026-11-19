package read

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	enttj "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/translationjob"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// TranslationJobModel implements read.TranslationJobReadModel with ent (entx.TenantOptional filter of the scope of ctx).
type TranslationJobModel struct{}

var _ read.TranslationJobReadModel = TranslationJobModel{}

// Job implements read.TranslationJobReadModel.
func (TranslationJobModel) Job(ctx context.Context, id uuid.UUID) (dto.TranslationJobView, error) {
	r, err := client(ctx).TranslationJob.Query().Where(enttj.ID(id)).Only(ctx)
	if err != nil {
		return dto.TranslationJobView{}, notFound(err)
	}
	v := dto.TranslationJobView{JobID: r.ID, Kind: r.Kind, Status: r.Status, ResultKey: r.ResultStorageKey, TenantID: r.TenantID}
	if len(r.Report) > 0 && string(r.Report) != "null" {
		var rep dto.TranslationReport
		if json.Unmarshal(r.Report, &rep) == nil {
			v.Report = &rep
		}
	}
	return v, nil
}
