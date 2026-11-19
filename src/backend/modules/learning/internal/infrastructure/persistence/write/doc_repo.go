package write

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/google/uuid"

	entqsi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionsetitem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// docTable is the ent access to the table pair of one composed content type (doc_<type>.go, generated from
// doc_lesson.go by scripts in the F024 change: the four types share their columns).
type docTable interface {
	insertDoc(ctx context.Context, d dto.Doc) error
	doc(ctx context.Context, id uuid.UUID) (dto.Doc, error)
	updateDoc(ctx context.Context, d dto.Doc, ifRow int64) (int, error)
	softDelete(ctx context.Context, id uuid.UUID, reason string) (int, error)
	insertVersion(ctx context.Context, v dto.DocVersion) error
	version(ctx context.Context, itemID uuid.UUID, no int) (dto.DocVersion, error)
	versions(ctx context.Context, itemID uuid.UUID) ([]dto.DocVersion, error)
	updateVersion(ctx context.Context, v dto.DocVersion, ifRow int64) (int, error)
	versionExists(ctx context.Context, id uuid.UUID) (bool, error)
	docExists(ctx context.Context, id uuid.UUID) (bool, error)
	deleted(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error)
	restore(ctx context.Context, id uuid.UUID) (trash.Entry, error)
}

var docTables = map[contentmodel.ContentType]docTable{
	contentmodel.TypeQuestionSet:         questionSetTable{},
	contentmodel.TypeLesson:              lessonTable{},
	contentmodel.TypeLearningMaterial:    learningMaterialTable{},
	contentmodel.TypeAssessmentBlueprint: assessmentBlueprintTable{},
}

// DocRepo implements write.DocStore with ent (entx.HybridContent filter of the scope of ctx).
type DocRepo struct{}

var _ write.DocStore = DocRepo{}

func table(t contentmodel.ContentType) (docTable, error) {
	tb, ok := docTables[t]
	if !ok {
		return nil, problem.New(problem.Internal, "no table for content type "+string(t))
	}
	return tb, nil
}

func jsonOr(v json.RawMessage, def string) json.RawMessage {
	if len(v) == 0 || string(v) == "null" {
		return json.RawMessage(def)
	}
	return v
}

func ints(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func strs(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// InsertDoc implements write.DocStore.
func (DocRepo) InsertDoc(ctx context.Context, d dto.Doc) error {
	tb, err := table(d.Type)
	if err != nil {
		return err
	}
	return MapCurriculumErr(tb.insertDoc(ctx, d))
}

// Doc implements write.DocStore.
func (DocRepo) Doc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (dto.Doc, error) {
	tb, err := table(t)
	if err != nil {
		return dto.Doc{}, err
	}
	d, err := tb.doc(ctx, id)
	return d, MapCurriculumErr(err)
}

// UpdateDoc implements write.DocStore.
func (s DocRepo) UpdateDoc(ctx context.Context, d dto.Doc, ifRow int64) (dto.Doc, error) {
	tb, err := table(d.Type)
	if err != nil {
		return d, err
	}
	n, err := tb.updateDoc(ctx, d, ifRow)
	if err != nil {
		return d, MapCurriculumErr(err)
	}
	if n == 0 {
		return d, mismatchOrMissing(tb.docExists(ctx, d.ID))
	}
	return s.Doc(ctx, d.Type, d.ID)
}

// SoftDeleteDoc implements write.DocStore.
func (DocRepo) SoftDeleteDoc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, reason string) error {
	tb, err := table(t)
	if err != nil {
		return err
	}
	n, err := tb.softDelete(entx.WithDeleteReason(ctx, reason), id, reason)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.NotFound, "")
	}
	return nil
}

// InsertDocVersion implements write.DocStore.
func (s DocRepo) InsertDocVersion(ctx context.Context, v dto.DocVersion) error {
	tb, err := table(v.Type)
	if err != nil {
		return err
	}
	if err := tb.insertVersion(ctx, v); err != nil {
		return MapCurriculumErr(err)
	}
	return s.setItems(ctx, v)
}

// DocVersion implements write.DocStore.
func (s DocRepo) DocVersion(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, no int) (dto.DocVersion, error) {
	tb, err := table(t)
	if err != nil {
		return dto.DocVersion{}, err
	}
	v, err := tb.version(ctx, itemID, no)
	if err != nil {
		return v, MapCurriculumErr(err)
	}
	vs := []dto.DocVersion{v}
	err = s.items(ctx, vs)
	return vs[0], err
}

// DocVersions implements write.DocStore.
func (s DocRepo) DocVersions(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID) ([]dto.DocVersion, error) {
	tb, err := table(t)
	if err != nil {
		return nil, err
	}
	vs, err := tb.versions(ctx, itemID)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	return vs, s.items(ctx, vs)
}

// UpdateDocVersion implements write.DocStore.
func (s DocRepo) UpdateDocVersion(ctx context.Context, v dto.DocVersion, ifRow int64) (dto.DocVersion, error) {
	tb, err := table(v.Type)
	if err != nil {
		return v, err
	}
	n, err := tb.updateVersion(ctx, v, ifRow)
	if err != nil {
		return v, MapCurriculumErr(err)
	}
	if n == 0 {
		return v, mismatchOrMissing(tb.versionExists(ctx, v.ID))
	}
	if err := s.setItems(ctx, v); err != nil {
		return v, err
	}
	return s.DocVersion(ctx, v.Type, v.ItemID, v.VersionNo)
}

// DeletedDocs implements write.DocStore.
func (DocRepo) DeletedDocs(ctx context.Context, t contentmodel.ContentType, f port.TrashFilter) ([]trash.Entry, error) {
	tb, err := table(t)
	if err != nil {
		return nil, err
	}
	out, err := tb.deleted(ctx, f)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(out, func(a, b trash.Entry) int {
		if c := b.DeletedAt.Compare(a.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ResourceID.String(), a.ResourceID.String())
	})
	return out, nil
}

// RestoreDoc implements write.DocStore.
func (DocRepo) RestoreDoc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (trash.Entry, error) {
	tb, err := table(t)
	if err != nil {
		return trash.Entry{}, err
	}
	e, err := tb.restore(ctx, id)
	if err != nil && err != trash.ErrNotDeleted { //nolint:errorlint // sentinel returned as is
		return e, MapCurriculumErr(err)
	}
	return e, err
}

// setItems makes question_set_items of a question set version match v.Items (removed items are soft-deleted).
func (DocRepo) setItems(ctx context.Context, v dto.DocVersion) error {
	if v.Type != contentmodel.TypeQuestionSet {
		return nil
	}
	c := client(ctx)
	have, err := c.QuestionSetItem.Query().Where(entqsi.QuestionSetVersionID(v.ID)).All(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	want := map[uuid.UUID]dto.SetItem{}
	for _, it := range v.Items {
		want[it.QuestionVersionID] = it
	}
	kept := map[uuid.UUID]bool{}
	for _, h := range have {
		if it, ok := want[h.QuestionVersionID]; ok && int(h.Position) == it.Position && sameScore(h.MaxScore, it.MaxScore) {
			kept[h.QuestionVersionID] = true
			continue
		}
		if _, err := c.QuestionSetItem.Delete().Where(entqsi.ID(h.ID)).Exec(entx.WithDeleteReason(ctx, "item_removed")); err != nil {
			return MapCurriculumErr(err)
		}
	}
	for _, it := range v.Items {
		if kept[it.QuestionVersionID] {
			continue
		}
		b := c.QuestionSetItem.Create().SetID(ids.New()).SetQuestionSetID(v.ItemID).SetQuestionSetVersionID(v.ID).
			SetPosition(int16(it.Position)). //nolint:gosec // 1–200
			SetQuestionID(it.QuestionID).SetQuestionVersionID(it.QuestionVersionID).SetNillableMaxScore(it.MaxScore).SetOwnerScope(string(v.Owner.Scope))
		if v.Owner.TenantID != nil {
			b.SetTenantID(*v.Owner.TenantID)
		}
		if err := b.Exec(ctx); err != nil {
			return MapCurriculumErr(err)
		}
	}
	return nil
}

func sameScore(a, b *float64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// items loads the fixed items of question set versions.
func (DocRepo) items(ctx context.Context, vs []dto.DocVersion) error {
	if len(vs) == 0 || vs[0].Type != contentmodel.TypeQuestionSet {
		return nil
	}
	vids := make([]uuid.UUID, len(vs))
	for i, v := range vs {
		vids[i] = v.ID
	}
	rows, err := client(ctx).QuestionSetItem.Query().Where(entqsi.QuestionSetVersionIDIn(vids...)).Order(entqsi.ByPosition()).All(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	by := map[uuid.UUID][]dto.SetItem{}
	for _, r := range rows {
		by[r.QuestionSetVersionID] = append(by[r.QuestionSetVersionID], dto.SetItem{Position: int(r.Position), QuestionID: r.QuestionID,
			QuestionVersionID: r.QuestionVersionID, MaxScore: r.MaxScore})
	}
	for i := range vs {
		vs[i].Items = by[vs[i].ID]
	}
	return nil
}
