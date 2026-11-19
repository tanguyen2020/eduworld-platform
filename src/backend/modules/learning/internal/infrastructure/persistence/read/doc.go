package read

import (
	"context"
	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	entqsi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionsetitem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// docModel is the per-type access of DocModel (doc_<type>.go).
type docModel interface {
	doc(ctx context.Context, id uuid.UUID) (dto.Doc, error)
	version(ctx context.Context, itemID uuid.UUID, no int) (dto.DocVersion, error)
	search(ctx context.Context, f read.DocFilter) ([]read.DocHit, error)
}

var docModels = map[contentmodel.ContentType]docModel{
	contentmodel.TypeQuestionSet:         questionSetModel{},
	contentmodel.TypeLesson:              lessonModel{},
	contentmodel.TypeLearningMaterial:    learningMaterialModel{},
	contentmodel.TypeAssessmentBlueprint: assessmentBlueprintModel{},
	contentmodel.TypeActivity:            activityModel{},
}

// DocModel implements read.DocReadModel with ent (entx.HybridContent filter of the scope of ctx). Full-text search
// uses the generated column search_tsv (unaccent + simple, GIN, F024 research R1).
type DocModel struct{}

var _ read.DocReadModel = DocModel{}

func model(t contentmodel.ContentType) (docModel, error) {
	m, ok := docModels[t]
	if !ok {
		return nil, problem.New(problem.Internal, "no read model for content type "+string(t))
	}
	return m, nil
}

// Doc implements read.DocReadModel.
func (DocModel) Doc(ctx context.Context, t contentmodel.ContentType, id uuid.UUID) (dto.Doc, error) {
	m, err := model(t)
	if err != nil {
		return dto.Doc{}, err
	}
	return m.doc(ctx, id)
}

// Version implements read.DocReadModel.
func (s DocModel) Version(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, no int) (dto.DocVersion, error) {
	m, err := model(t)
	if err != nil {
		return dto.DocVersion{}, err
	}
	v, err := m.version(ctx, itemID, no)
	if err != nil {
		return v, err
	}
	return v, s.withItems(ctx, &v)
}

// Latest implements read.DocReadModel.
func (s DocModel) Latest(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID) (dto.DocVersion, error) {
	return s.Version(ctx, t, itemID, 0)
}

// Search implements read.DocReadModel.
func (s DocModel) Search(ctx context.Context, t contentmodel.ContentType, f read.DocFilter) ([]read.DocHit, error) {
	m, err := model(t)
	if err != nil {
		return nil, err
	}
	hits, err := m.search(ctx, f)
	if err != nil || t != contentmodel.TypeQuestionSet {
		return hits, err
	}
	vids := make([]uuid.UUID, len(hits))
	for i, h := range hits {
		vids[i] = h.Version.ID
	}
	items, err := s.Items(ctx, vids)
	for i := range hits {
		hits[i].Version.Items = items[hits[i].Version.ID]
	}
	return hits, err
}

func (s DocModel) withItems(ctx context.Context, v *dto.DocVersion) error {
	if v.Type != contentmodel.TypeQuestionSet {
		return nil
	}
	items, err := s.Items(ctx, []uuid.UUID{v.ID})
	v.Items = items[v.ID]
	return err
}

// Items implements read.DocReadModel.
func (DocModel) Items(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID][]dto.SetItem, error) {
	out := map[uuid.UUID][]dto.SetItem{}
	if len(versionIDs) == 0 {
		return out, nil
	}
	rows, err := client(ctx).QuestionSetItem.Query().Where(entqsi.QuestionSetVersionIDIn(versionIDs...)).Order(entqsi.ByPosition()).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.QuestionSetVersionID] = append(out[r.QuestionSetVersionID], dto.SetItem{Position: int(r.Position), QuestionID: r.QuestionID,
			QuestionVersionID: r.QuestionVersionID, MaxScore: r.MaxScore})
	}
	return out, nil
}

// latestVersion restricts a version query to the newest version of each item (latest_version_no of the item row).
func latestVersion(s *entsql.Selector, versionNo, itemColumn, itemTable, itemID string) *entsql.Predicate {
	return entsql.P(func(b *entsql.Builder) {
		// tenancyguard:ok scalar subquery on the same item as the outer row, which the entx filter of the scope already restricts (tenant + deleted_at)
		b.WriteString(s.C(versionNo) + " = (SELECT i.latest_version_no FROM content." + itemTable + " i WHERE i." + itemID + " = " + s.C(itemColumn) + ")")
	})
}

// docFilters are the metadata and full-text predicates shared by the version tables (jsonb containment on the
// grade, topic and objective arrays; websearch query on search_tsv with unaccent).
func docFilters[P ~func(*entsql.Selector)](f read.DocFilter) []P {
	var out []P
	contains := func(col string, v any) {
		b, _ := json.Marshal([]any{v})
		out = append(out, func(s *entsql.Selector) {
			s.Where(entsql.P(func(bd *entsql.Builder) {
				bd.WriteString(s.C(col) + " @> ").Arg(string(b)).WriteString("::jsonb")
			}))
		})
	}
	if f.GradeLevel != nil {
		contains("grade_levels", *f.GradeLevel)
	}
	if f.TopicID != nil {
		contains("topic_ids", f.TopicID.String())
	}
	if f.ObjectiveID != nil {
		contains("objective_ids", f.ObjectiveID.String())
	}
	if f.ActivityType != "" {
		at := f.ActivityType
		out = append(out, func(s *entsql.Selector) { s.Where(entsql.EQ(s.C("activity_type"), at)) })
	}
	if f.Q != "" {
		q := f.Q
		out = append(out, func(s *entsql.Selector) {
			s.Where(entsql.P(func(b *entsql.Builder) {
				b.WriteString(s.C("search_tsv") + " @@ websearch_to_tsquery('simple', content.f_unaccent(lower(").Arg(q).WriteString(")))")
			}))
		})
	}
	return out
}
