package command

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ---------- topics

// TopicInput is the request of createTopic.
type TopicInput struct {
	VersionID  uuid.UUID
	Code       string
	Name       string
	SubjectID  uuid.UUID
	GradeLevel int
	ParentID   *uuid.UUID
	SortOrder  int // 0 = after its siblings
}

func parentsOf(ts []dto.Topic) map[uuid.UUID]*uuid.UUID {
	out := make(map[uuid.UUID]*uuid.UUID, len(ts))
	for _, t := range ts {
		out[t.ID] = t.ParentID
	}
	return out
}

func node(t dto.Topic) curriculum.TopicNode {
	return curriculum.TopicNode{ID: t.ID, VersionID: t.VersionID, SubjectID: t.SubjectID, GradeLevel: t.GradeLevel, ParentID: t.ParentID}
}

// checkTopicParent validates parent of t against the topics of its version (F021-AC-05).
func checkTopicParent(t dto.Topic, all []dto.Topic) error {
	if t.ParentID == nil {
		return nil
	}
	i := slices.IndexFunc(all, func(x dto.Topic) bool { return x.ID == *t.ParentID })
	if i < 0 {
		return problem.Invalid("parent_topic_id", problem.SubInvalid, "must be a topic of the same version, subject and grade")
	}
	return invalid(curriculum.CheckParent(node(t), node(all[i]), parentsOf(all)))
}

// CreateTopic adds a topic to a draft version (F021-AC-05, AC-07); the subject × grade pair is added when missing.
func (h Curriculum) CreateTopic(ctx context.Context, in TopicInput) (out dto.Topic, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateTopic")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := curriculum.CheckTopicCode(in.Code); err != nil {
		return out, invalid(err)
	}
	if err := curriculum.CheckName("name", in.Name); err != nil {
		return out, invalid(err)
	}
	if in.GradeLevel < 1 || in.GradeLevel > 9 {
		return out, problem.Invalid("grade_level", problem.SubInvalid, "must be 1–9")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		v, err := h.draftVersion(ctx, p, in.VersionID)
		if err != nil {
			return err
		}
		out, err = h.addTopic(ctx, v, in)
		if err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.TopicJSON(out))
	})
	return out, err
}

// addTopic inserts a topic into the draft version v (shared with the import and the deep copy).
func (h Curriculum) addTopic(ctx context.Context, v dto.CurriculumVersion, in TopicInput) (dto.Topic, error) {
	if _, err := h.Catalog.Subject(ctx, in.SubjectID); err != nil {
		return dto.Topic{}, problem.Invalid("subject_id", problem.SubInvalid, "is not a subject of the catalog")
	}
	all, err := h.Store.Topics(ctx, v.ID)
	if err != nil {
		return dto.Topic{}, err
	}
	t := dto.Topic{ID: ids.New(), CurriculumID: v.CurriculumID, VersionID: v.ID, SubjectID: in.SubjectID, GradeLevel: in.GradeLevel,
		ParentID: in.ParentID, Code: in.Code, Name: in.Name, SortOrder: in.SortOrder, Owner: v.Owner}
	if err := checkTopicParent(t, all); err != nil {
		return t, err
	}
	if t.SortOrder < 1 {
		t.SortOrder = 1
		for _, x := range all {
			if x.SubjectID == t.SubjectID && x.GradeLevel == t.GradeLevel && sameParent(x.ParentID, t.ParentID) && x.SortOrder >= t.SortOrder {
				t.SortOrder = x.SortOrder + 1
			}
		}
	}
	if err := h.ensureSubjectGrade(ctx, v, t.SubjectID, t.GradeLevel); err != nil {
		return t, err
	}
	if err := h.Store.InsertTopic(ctx, t); err != nil {
		return t, err
	}
	return h.Store.Topic(ctx, t.ID)
}

func sameParent(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// UpdateTopicInput is the merge-patch of updateTopic.
type UpdateTopicInput struct {
	ID        uuid.UUID
	IfMatch   int64
	Code      *string
	Name      *string
	Parent    Patch[uuid.UUID]
	SortOrder *int
}

// UpdateTopic edits a topic of a draft version (If-Match, F021-AC-05, AC-07, AC-09).
func (h Curriculum) UpdateTopic(ctx context.Context, in UpdateTopicInput) (out dto.Topic, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateTopic")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Store.Topic(ctx, in.ID)
		if err != nil {
			return err
		}
		if _, err := h.draftVersion(ctx, p, t.VersionID); err != nil {
			return err
		}
		if in.Code != nil {
			if err := curriculum.CheckTopicCode(*in.Code); err != nil {
				return invalid(err)
			}
			t.Code = *in.Code
		}
		if in.Name != nil {
			if err := curriculum.CheckName("name", *in.Name); err != nil {
				return invalid(err)
			}
			t.Name = *in.Name
		}
		if in.SortOrder != nil {
			t.SortOrder = *in.SortOrder
		}
		if in.Parent.Set {
			t.ParentID = ptrClone(in.Parent.Value)
			all, err := h.Store.Topics(ctx, t.VersionID)
			if err != nil {
				return err
			}
			if err := checkTopicParent(t, all); err != nil {
				return err
			}
		}
		out, err = h.Store.UpdateTopic(ctx, t, in.IfMatch)
		return err
	})
	return out, err
}

// DeleteInput is the request of deleteTopic / deleteObjective.
type DeleteInput struct {
	ID      uuid.UUID
	IfMatch int64
	Reason  string
}

// DeleteTopic soft-deletes a topic of a draft version without sub-topics or LOs (F021-AC-10).
func (h Curriculum) DeleteTopic(ctx context.Context, in DeleteInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DeleteTopic")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(in.Reason) > 500 {
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 500 characters")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Store.Topic(ctx, in.ID)
		if err != nil {
			return err
		}
		v, err := h.draftVersion(ctx, p, t.VersionID)
		if err != nil {
			return err
		}
		all, err := h.Store.Topics(ctx, v.ID)
		if err != nil {
			return err
		}
		for _, x := range all {
			if x.ParentID != nil && *x.ParentID == t.ID {
				return problem.Invalid("topic_id", problem.SubInvalid, "the topic has sub-topics ("+x.Code+")")
			}
		}
		los, err := h.Store.Objectives(ctx, v.ID)
		if err != nil {
			return err
		}
		var held []string
		for _, lo := range los {
			if lo.TopicID == t.ID {
				held = append(held, lo.Code)
			}
		}
		if len(held) > 0 {
			return problem.Invalid("topic_id", problem.SubInvalid, "the topic holds objectives: "+strings.Join(held, ", "))
		}
		if err := h.Store.DeleteTopic(ctx, t.ID, in.IfMatch, in.Reason); err != nil {
			return err
		}
		return h.recordCurriculum(ctx, v.Owner, audit.Event{Action: ActionContentDeleted, TargetID: v.CurriculumID.String(), Reason: in.Reason,
			Metadata: map[string]any{"entity": "topic", "topic_id": t.ID.String(), "curriculum_version_id": v.ID.String()}})
	})
}

// ---------- objectives

// ObjectiveInput is the request of createObjective.
type ObjectiveInput struct {
	TopicID         uuid.UUID
	Code            string
	Description     string
	SortOrder       int
	BloomLevel      *string
	YccdRef         *string
	CompetencyIDs   []uuid.UUID
	PrerequisiteIDs []uuid.UUID
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// checkObjective validates o in its draft version: fields (F021-AC-03, AC-04), competencies of the version
// (AC-04), prerequisites of the version and the DAG (AC-11, AC-12).
func (h Curriculum) checkObjective(ctx context.Context, o dto.Objective, topic dto.Topic) error {
	if err := (curriculum.Objective{ID: o.ID, Code: o.Code, Description: o.Description, BloomLevel: deref(o.BloomLevel), YccdRef: deref(o.YccdRef),
		CompetencyIDs: o.CompetencyIDs, PrerequisiteIDs: o.PrerequisiteIDs, TopicGrade: topic.GradeLevel}).Validate(); err != nil {
		return invalid(err)
	}
	if len(o.CompetencyIDs) > 0 {
		comps, err := h.Store.Competencies(ctx, o.VersionID)
		if err != nil {
			return err
		}
		for _, id := range o.CompetencyIDs {
			if !slices.ContainsFunc(comps, func(c dto.Competency) bool { return c.ID == id }) {
				return problem.Invalid("competency_ids", problem.SubInvalid, "must be competencies of the same curriculum version")
			}
		}
	}
	los, err := h.Store.Objectives(ctx, o.VersionID)
	if err != nil {
		return err
	}
	graph := curriculum.PrerequisiteDAG{}
	codes := map[uuid.UUID]string{o.ID: o.Code}
	for _, lo := range los {
		if lo.ID == o.ID {
			continue
		}
		graph[lo.ID] = lo.PrerequisiteIDs
		codes[lo.ID] = lo.Code
	}
	for _, id := range o.PrerequisiteIDs {
		if _, ok := graph[id]; !ok {
			return problem.Invalid("prerequisite_ids", problem.SubInvalid, "must be objectives of the same curriculum version")
		}
	}
	var ce curriculum.CycleError
	if err := graph.With(o.ID, o.PrerequisiteIDs).DetectCycle(); errors.As(err, &ce) {
		return cycleProblem(ce, codes)
	}
	return nil
}

// CreateObjective adds a learning objective to a draft version (F021-AC-03, AC-04, AC-11, AC-12); stable_key = code.
func (h Curriculum) CreateObjective(ctx context.Context, in ObjectiveInput) (out dto.Objective, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateObjective")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		t, err := h.Store.Topic(ctx, in.TopicID)
		if err != nil {
			return err
		}
		if _, err := h.draftVersion(ctx, p, t.VersionID); err != nil {
			return err
		}
		if out, err = h.addObjective(ctx, t, in); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.ObjectiveJSON(out))
	})
	return out, err
}

// addObjective validates and inserts an objective under topic t (shared with the import).
func (h Curriculum) addObjective(ctx context.Context, t dto.Topic, in ObjectiveInput) (dto.Objective, error) {
	o := dto.Objective{ID: ids.New(), CurriculumID: t.CurriculumID, VersionID: t.VersionID, TopicID: t.ID, Code: in.Code, StableKey: in.Code,
		Description: in.Description, BloomLevel: in.BloomLevel, YccdRef: in.YccdRef, CompetencyIDs: in.CompetencyIDs,
		PrerequisiteIDs: in.PrerequisiteIDs, SortOrder: in.SortOrder, Owner: t.Owner}
	if o.SortOrder < 1 {
		o.SortOrder = 1
	}
	if err := h.checkObjective(ctx, o, t); err != nil {
		return o, err
	}
	if err := h.Store.InsertObjective(ctx, o); err != nil {
		return o, err
	}
	return h.Store.Objective(ctx, o.ID)
}

// UpdateObjectiveInput is the merge-patch of updateObjective.
type UpdateObjectiveInput struct {
	ID              uuid.UUID
	IfMatch         int64
	Code            *string
	Description     *string
	TopicID         *uuid.UUID
	SortOrder       *int
	BloomLevel      *string
	YccdRef         Patch[string]
	CompetencyIDs   *[]uuid.UUID
	PrerequisiteIDs *[]uuid.UUID
}

// UpdateObjective edits an objective of a draft version (If-Match); a prerequisite cycle is 422 PREREQUISITE_CYCLE
// and nothing is saved (F021-AC-12).
func (h Curriculum) UpdateObjective(ctx context.Context, in UpdateObjectiveInput) (out dto.Objective, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateObjective")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		o, err := h.Store.Objective(ctx, in.ID)
		if err != nil {
			return err
		}
		if _, err := h.draftVersion(ctx, p, o.VersionID); err != nil {
			return err
		}
		if err := checkRowVersion(o.RowVersion, in.IfMatch); err != nil {
			return err
		}
		if in.Code != nil {
			o.Code = *in.Code
		}
		if in.Description != nil {
			o.Description = *in.Description
		}
		if in.TopicID != nil {
			o.TopicID = *in.TopicID
		}
		if in.SortOrder != nil {
			o.SortOrder = *in.SortOrder
		}
		if in.BloomLevel != nil {
			o.BloomLevel = in.BloomLevel
		}
		o.YccdRef = in.YccdRef.apply(o.YccdRef)
		if in.CompetencyIDs != nil {
			o.CompetencyIDs = *in.CompetencyIDs
		}
		if in.PrerequisiteIDs != nil {
			o.PrerequisiteIDs = *in.PrerequisiteIDs
		}
		t, err := h.Store.Topic(ctx, o.TopicID)
		if err != nil {
			if in.TopicID != nil {
				return problem.Invalid("topic_id", problem.SubInvalid, "is not a topic of the same curriculum version")
			}
			return err
		}
		if t.VersionID != o.VersionID {
			return problem.Invalid("topic_id", problem.SubInvalid, "is not a topic of the same curriculum version")
		}
		if err := h.checkObjective(ctx, o, t); err != nil {
			return err
		}
		out, err = h.Store.UpdateObjective(ctx, o, in.IfMatch)
		return err
	})
	return out, err
}

// DeleteObjective soft-deletes an objective of a draft version that is no other objective's prerequisite (spec
// edge case: 422 listing the dependent LOs).
func (h Curriculum) DeleteObjective(ctx context.Context, in DeleteInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DeleteObjective")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(in.Reason) > 500 {
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 500 characters")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		o, err := h.Store.Objective(ctx, in.ID)
		if err != nil {
			return err
		}
		v, err := h.draftVersion(ctx, p, o.VersionID)
		if err != nil {
			return err
		}
		los, err := h.Store.Objectives(ctx, v.ID)
		if err != nil {
			return err
		}
		var dependents []string
		for _, lo := range los {
			if lo.ID != o.ID && slices.Contains(lo.PrerequisiteIDs, o.ID) {
				dependents = append(dependents, lo.Code)
			}
		}
		if len(dependents) > 0 {
			slices.Sort(dependents)
			return problem.Invalid("objective_id", problem.SubInvalid, "is a prerequisite of "+strings.Join(dependents, ", ")).
				With("dependents", dependents)
		}
		if err := h.Store.DeleteObjective(ctx, o.ID, in.IfMatch, in.Reason); err != nil {
			return err
		}
		return h.recordCurriculum(ctx, v.Owner, audit.Event{Action: ActionContentDeleted, TargetID: v.CurriculumID.String(), Reason: in.Reason,
			Metadata: map[string]any{"entity": "learning_objective", "objective_id": o.ID.String(), "curriculum_version_id": v.ID.String()}})
	})
}

// ---------- competencies

// CompetencyInput is the request of createCompetency.
type CompetencyInput struct {
	VersionID uuid.UUID
	Code      string
	Name      map[string]string
	Kind      string
	SubjectID *uuid.UUID
}

// CreateCompetency adds a competency to a draft version (CURRICULUM_MODEL §3.4).
func (h Curriculum) CreateCompetency(ctx context.Context, in CompetencyInput) (out dto.Competency, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateCompetency")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		v, err := h.draftVersion(ctx, p, in.VersionID)
		if err != nil {
			return err
		}
		if out, err = h.addCompetency(ctx, v, in); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, dto.CompetencyJSON(out))
	})
	return out, err
}

func (h Curriculum) addCompetency(ctx context.Context, v dto.CurriculumVersion, in CompetencyInput) (dto.Competency, error) {
	if err := curriculum.CheckCurriculumCode("code", in.Code); err != nil {
		return dto.Competency{}, invalid(err)
	}
	if err := curriculum.CheckLearningName("name", in.Name); err != nil {
		return dto.Competency{}, invalid(err)
	}
	if err := curriculum.CheckCompetencyKind(in.Kind, in.SubjectID != nil); err != nil {
		return dto.Competency{}, invalid(err)
	}
	if in.SubjectID != nil {
		if _, err := h.Catalog.Subject(ctx, *in.SubjectID); err != nil {
			return dto.Competency{}, problem.Invalid("subject_id", problem.SubInvalid, "is not a subject of the catalog")
		}
	}
	c := dto.Competency{ID: ids.New(), CurriculumID: v.CurriculumID, VersionID: v.ID, Code: in.Code, Name: in.Name, Kind: in.Kind,
		SubjectID: in.SubjectID, Owner: v.Owner}
	if err := h.Store.InsertCompetency(ctx, c); err != nil {
		return c, err
	}
	return h.Store.Competency(ctx, c.ID)
}

// UpdateCompetencyInput is the merge-patch of updateCompetency.
type UpdateCompetencyInput struct {
	ID      uuid.UUID
	IfMatch int64
	Name    map[string]string
}

// UpdateCompetency renames a competency of a draft version.
func (h Curriculum) UpdateCompetency(ctx context.Context, in UpdateCompetencyInput) (out dto.Competency, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateCompetency")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermCurriculumWrite)
	if err != nil {
		return out, err
	}
	if err := curriculum.CheckLearningName("name", in.Name); err != nil {
		return out, invalid(err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Store.Competency(ctx, in.ID)
		if err != nil {
			return err
		}
		if _, err := h.draftVersion(ctx, p, c.VersionID); err != nil {
			return err
		}
		c.Name = in.Name
		out, err = h.Store.UpdateCompetency(ctx, c, in.IfMatch)
		return err
	})
	return out, err
}
