package read

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entitem "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/importitem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/predicate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

func decodeJSON(v any, dst any) {
	b, _ := json.Marshal(v)
	_ = json.Unmarshal(b, dst)
}

// ImportItems implements read.CurriculumReadModel (F028): the items of a job by item_no, keyset after (item_no, id);
// tenant and deleted_at are filtered by the entx interceptor of the session.
func (CurriculumModel) ImportItems(ctx context.Context, importID uuid.UUID, status string, seek read.Seek) ([]dto.ImportItem, error) {
	q := client(ctx).ImportItem.Query().Where(entitem.ImportID(importID))
	if status != "" {
		q = q.Where(entitem.Status(status))
	}
	if p := seekAfter(seek.After, false, entitem.FieldID, entitem.FieldItemNo); p != nil {
		q = q.Where(predicate.ImportItem(p))
	}
	if n := limitOf(seek); n > 0 {
		q = q.Limit(n)
	}
	rows, err := q.Order(ent.Asc(entitem.FieldItemNo), ent.Asc(entitem.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ImportItem, len(rows))
	for i, r := range rows {
		scope := "tenant"
		if r.TenantID == nil {
			scope = "platform"
		}
		it := dto.ImportItem{ID: r.ID, ImportID: r.ImportID, Owner: ownerOf(scope, r.TenantID), No: int(r.ItemNo), Label: r.SourceLabel,
			Status: r.Status, QuestionType: r.QuestionType, StemPreview: r.StemPreview, QuestionID: r.QuestionID,
			QuestionVersionID: r.QuestionVersionID, UpdatedAt: r.UpdatedAt}
		decodeJSON(r.Issues, &it.Issues)
		decodeJSON(r.Formulas, &it.Formulas)
		decodeJSON(r.Images, &it.Images)
		decodeJSON(r.Payload, &it.Payload)
		out[i] = it
	}
	return out, nil
}
