package write

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entitem "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/importitem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
)

// ImportItemRepo implements write.ImportItemStore (content.import_items, T*: tenant rows of tenant jobs, NULL for
// platform jobs written under PlatformScope — F028).
type ImportItemRepo struct{}

var _ write.ImportItemStore = ImportItemRepo{}

// jsonList converts a typed slice to the []map of an ent JSON column (and back with fromJSON).
func jsonList[T any](v []T) []map[string]any {
	out := []map[string]any{}
	if len(v) == 0 {
		return out // an empty JSON array, never null (CHECK jsonb_typeof = array)
	}
	b, _ := json.Marshal(v)
	_ = json.Unmarshal(b, &out)
	return out
}

func fromJSON[T any](v any) T {
	var out T
	b, _ := json.Marshal(v)
	_ = json.Unmarshal(b, &out)
	return out
}

// ImportItemOf maps an ent import item.
func ImportItemOf(r *ent.ImportItem) dto.ImportItem {
	scope := "tenant"
	if r.TenantID == nil {
		scope = "platform"
	}
	return dto.ImportItem{ID: r.ID, ImportID: r.ImportID, Owner: owner(scope, r.TenantID), No: int(r.ItemNo), Label: r.SourceLabel, Status: r.Status,
		QuestionType: r.QuestionType, StemPreview: r.StemPreview, Issues: fromJSON[[]docximport.Issue](r.Issues), Formulas: fromJSON[[]docximport.Formula](r.Formulas),
		Images: fromJSON[[]docximport.ImageRef](r.Images), Payload: fromJSON[dto.ImportPayload](r.Payload), QuestionID: r.QuestionID,
		QuestionVersionID: r.QuestionVersionID, UpdatedAt: r.UpdatedAt}
}

// InsertImportItems implements write.ImportItemStore.
func (ImportItemRepo) InsertImportItems(ctx context.Context, items []dto.ImportItem) error {
	builders := make([]*ent.ImportItemCreate, len(items))
	for i, it := range items {
		c := client(ctx).ImportItem.Create().SetID(it.ID).SetImportID(it.ImportID).SetItemNo(int32(it.No)). //nolint:gosec // ≤ docx_import_max_questions
															SetSourceLabel(it.Label).SetStatus(it.Status).SetNillableQuestionType(it.QuestionType).SetStemPreview(it.StemPreview).
															SetIssues(jsonList(it.Issues)).SetFormulas(jsonList(it.Formulas)).SetImages(jsonList(it.Images)).
															SetPayload(fromJSON[map[string]any](it.Payload))
		if it.Owner.TenantID != nil {
			c.SetTenantID(*it.Owner.TenantID)
		}
		builders[i] = c
	}
	return MapCurriculumErr(client(ctx).ImportItem.CreateBulk(builders...).Exec(ctx))
}

// ImportItem implements write.ImportItemStore.
func (ImportItemRepo) ImportItem(ctx context.Context, importID uuid.UUID, no int) (dto.ImportItem, error) {
	r, err := client(ctx).ImportItem.Query().Where(entitem.ImportID(importID), entitem.ItemNo(int32(no))).Only(ctx) //nolint:gosec // path ≤ 10000
	if err != nil {
		return dto.ImportItem{}, MapCurriculumErr(err)
	}
	return ImportItemOf(r), nil
}

// UpdateImportItem implements write.ImportItemStore.
func (ImportItemRepo) UpdateImportItem(ctx context.Context, it dto.ImportItem) error {
	u := client(ctx).ImportItem.Update().Where(entitem.ID(it.ID)).SetStatus(it.Status).SetIssues(jsonList(it.Issues)).AddVersion(1)
	if it.QuestionID != nil {
		u.SetQuestionID(*it.QuestionID)
	}
	if it.QuestionVersionID != nil {
		u.SetQuestionVersionID(*it.QuestionVersionID)
	}
	n, err := u.Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return mismatchOrMissing(false, nil)
	}
	return nil
}
