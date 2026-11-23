package read

import (
	"context"
	"encoding/json"
	entactv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/activityversion"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entabv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/assessmentblueprintversion"
	entcr "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/contentrelease"
	entct "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/contenttransition"
	entlmv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/learningmaterialversion"
	entlv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lessonversion"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/predicate"
	entqsi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionsetitem"
	entqsv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionsetversion"
	entqv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionversion"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReleaseModel implements read.ReleaseReadModel with ent (entx filters of the scope of ctx: own and assigned items).
type ReleaseModel struct{}

var _ read.ReleaseReadModel = ReleaseModel{}

func summary(no int, id uuid.UUID, status string, note *string, author uuid.UUID, reviewer, publishedBy *uuid.UUID, pubAt, at *time.Time) dto.VersionSummary {
	return dto.VersionSummary{Version: no, ContentVersionID: id, Status: status, ChangeNote: note, AuthorID: author, ReviewerID: reviewer,
		PublishedBy: publishedBy, PublishedAt: pubAt, PublishAt: at}
}

// titleOfJSON is the text of an i18n title in lang (or the other language), cut to 255 characters.
func titleOfJSON(raw []byte, lang string) string {
	var m map[string]string
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	s := m[lang]
	if s == "" {
		for _, v := range m {
			s = v
		}
	}
	if r := []rune(s); len(r) > 255 {
		s = string(r[:255])
	}
	return s
}

// Versions implements read.ReleaseReadModel: one keyset page of the versions of an item, newest first (version_no DESC,
// id DESC), with the status and id filters in SQL (API_DESIGN §5).
func (ReleaseModel) Versions(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, f read.VersionFilter) ([]dto.VersionSummary, error) {
	if f.IDs != nil && len(f.IDs) == 0 {
		return nil, nil
	}
	cl := client(ctx)
	n := limitOf(f.Seek)
	var out []dto.VersionSummary
	switch t {
	case contentmodel.TypeQuestion:
		q := cl.QuestionVersion.Query().Where(entqv.QuestionID(itemID)).Where(versionPreds[predicate.QuestionVersion](f, entqv.FieldID)...)
		if n > 0 {
			q = q.Limit(n)
		}
		rows, err := q.Order(ent.Desc(entqv.FieldVersionNo), ent.Desc(entqv.FieldID)).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, summary(r.VersionNo, r.ID, r.Status, r.ChangeNote, r.AuthorID, r.ReviewerID, r.PublishedBy, r.PublishedAt, r.PublishAt))
		}
	case contentmodel.TypeQuestionSet:
		q := cl.QuestionSetVersion.Query().Where(entqsv.QuestionSetID(itemID)).Where(versionPreds[predicate.QuestionSetVersion](f, entqsv.FieldID)...)
		if n > 0 {
			q = q.Limit(n)
		}
		rows, err := q.Order(ent.Desc(entqsv.FieldVersionNo), ent.Desc(entqsv.FieldID)).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, summary(r.VersionNo, r.ID, r.Status, r.ChangeNote, r.AuthorID, r.ReviewerID, r.PublishedBy, r.PublishedAt, r.PublishAt))
		}
	case contentmodel.TypeLesson:
		q := cl.LessonVersion.Query().Where(entlv.LessonID(itemID)).Where(versionPreds[predicate.LessonVersion](f, entlv.FieldID)...)
		if n > 0 {
			q = q.Limit(n)
		}
		rows, err := q.Order(ent.Desc(entlv.FieldVersionNo), ent.Desc(entlv.FieldID)).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, summary(r.VersionNo, r.ID, r.Status, r.ChangeNote, r.AuthorID, r.ReviewerID, r.PublishedBy, r.PublishedAt, r.PublishAt))
		}
	case contentmodel.TypeLearningMaterial:
		q := cl.LearningMaterialVersion.Query().Where(entlmv.LearningMaterialID(itemID)).
			Where(versionPreds[predicate.LearningMaterialVersion](f, entlmv.FieldID)...)
		if n > 0 {
			q = q.Limit(n)
		}
		rows, err := q.Order(ent.Desc(entlmv.FieldVersionNo), ent.Desc(entlmv.FieldID)).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, summary(r.VersionNo, r.ID, r.Status, r.ChangeNote, r.AuthorID, r.ReviewerID, r.PublishedBy, r.PublishedAt, r.PublishAt))
		}
	case contentmodel.TypeAssessmentBlueprint:
		q := cl.AssessmentBlueprintVersion.Query().Where(entabv.AssessmentBlueprintID(itemID)).
			Where(versionPreds[predicate.AssessmentBlueprintVersion](f, entabv.FieldID)...)
		if n > 0 {
			q = q.Limit(n)
		}
		rows, err := q.Order(ent.Desc(entabv.FieldVersionNo), ent.Desc(entabv.FieldID)).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, summary(r.VersionNo, r.ID, r.Status, r.ChangeNote, r.AuthorID, r.ReviewerID, r.PublishedBy, r.PublishedAt, r.PublishAt))
		}
	case contentmodel.TypeActivity:
		q := cl.ActivityVersion.Query().Where(entactv.ActivityID(itemID)).Where(versionPreds[predicate.ActivityVersion](f, entactv.FieldID)...)
		if n > 0 {
			q = q.Limit(n)
		}
		rows, err := q.Order(ent.Desc(entactv.FieldVersionNo), ent.Desc(entactv.FieldID)).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, summary(r.VersionNo, r.ID, r.Status, r.ChangeNote, r.AuthorID, r.ReviewerID, r.PublishedBy, r.PublishedAt, r.PublishAt))
		}
	default:
		return nil, problem.New(problem.Internal, "no version table for "+string(t))
	}
	return out, withFieldComments(ctx, out)
}

// versionPreds are the SQL filters of a version list: status, ids and the keyset position (version_no DESC, id DESC).
func versionPreds[P ~func(*entsql.Selector)](f read.VersionFilter, idCol string) []P {
	var out []P
	if f.Status != "" {
		st := f.Status
		out = append(out, func(s *entsql.Selector) { s.Where(entsql.EQ(s.C("status"), st)) })
	}
	if len(f.IDs) > 0 {
		ids := make([]any, len(f.IDs))
		for i, id := range f.IDs {
			ids[i] = id
		}
		out = append(out, func(s *entsql.Selector) { s.Where(entsql.In(s.C(idCol), ids...)) })
	}
	if p := seekAfter(f.Seek.After, true, idCol, "version_no"); p != nil {
		out = append(out, p)
	}
	return out
}

// withFieldComments adds the per-field comments of the latest reject of each version (F025 T046, OQ F054-1).
func withFieldComments(ctx context.Context, out []dto.VersionSummary) error {
	if len(out) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(out))
	for i, v := range out {
		ids[i] = v.ContentVersionID
	}
	rows, err := client(ctx).ContentTransition.Query().Where(entct.ContentVersionIDIn(ids...), entct.Action(string(contentmodel.ActionReject)),
		entct.FieldCommentsNotNil()).Order(ent.Desc(entct.FieldAt)).All(ctx)
	if err != nil {
		return err
	}
	latest := map[uuid.UUID]json.RawMessage{}
	for _, r := range rows {
		if _, seen := latest[r.ContentVersionID]; !seen {
			latest[r.ContentVersionID] = r.FieldComments
		}
	}
	for i := range out {
		out[i].FieldComments = latest[out[i].ContentVersionID]
	}
	return nil
}

// Releases implements read.ReleaseReadModel (newest first, keyset after a release id).
func (ReleaseModel) Releases(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, after *uuid.UUID, limit int) ([]dto.ReleaseRecord, error) {
	q := client(ctx).ContentRelease.Query().Where(entcr.ItemType(string(t)), entcr.ItemID(itemID))
	if after != nil {
		q = q.Where(entcr.IDLT(*after))
	}
	rows, err := q.Order(entcr.ByID(entsql.OrderDesc())).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ReleaseRecord, len(rows))
	for i, r := range rows {
		out[i] = dto.ReleaseRecord{ID: r.ID, ItemType: r.ItemType, ItemID: r.ItemID, Action: r.Action, FromVersionID: r.FromVersionID,
			ToVersionID: r.ToVersionID, Reason: r.Reason, ScheduledFor: r.ScheduledFor, ReleasedBy: r.ReleasedBy, ReleasedAt: r.ReleasedAt.UTC()}
	}
	return out, nil
}

// QuestionUsages implements read.ReleaseReadModel: the question set versions holding a version of the question.
func (ReleaseModel) QuestionUsages(ctx context.Context, questionID uuid.UUID, after *uuid.UUID, limit int) ([]dto.ContentUsageView, *uuid.UUID, bool, error) {
	cl := client(ctx)
	q := cl.QuestionSetItem.Query().Where(entqsi.QuestionID(questionID))
	if after != nil {
		q = q.Where(entqsi.IDGT(*after))
	}
	items, err := q.Order(ent.Asc(entqsi.FieldID)).Limit(limit + 1).All(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	var last *uuid.UUID
	if len(items) > 0 {
		id := items[len(items)-1].ID
		last = &id
	}
	out := []dto.ContentUsageView{}
	for _, it := range items {
		sv, err := cl.QuestionSetVersion.Query().Where(entqsv.ID(it.QuestionSetVersionID)).Only(ctx)
		if err != nil {
			continue // not visible in the scope
		}
		qv, err := cl.QuestionVersion.Query().Where(entqv.ID(it.QuestionVersionID)).Only(ctx)
		used := 0
		if err == nil {
			used = qv.VersionNo
		}
		id := sv.ID
		u := dto.ContentUsageView{RefType: string(contentmodel.TypeQuestionSet), RefID: sv.QuestionSetID, RefVersion: sv.VersionNo, RefVersionID: &id,
			UsedVersion: used}
		u.Title = titleOfJSON(sv.Title, sv.ContentLanguage)
		out = append(out, u)
	}
	return out, last, more, nil
}
