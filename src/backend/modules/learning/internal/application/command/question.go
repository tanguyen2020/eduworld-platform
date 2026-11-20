package command

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// AssetsPerVersionMax is assets_per_version_max (CONFIG_DEFAULTS: 50, owner CONTENT_MODEL) until platform settings are
// read by module learning (F015 projection).
const AssetsPerVersionMax = 50

// Questions holds the collaborators of the question bank commands (F022).
type Questions struct {
	UoW        write.UnitOfWork
	Store      write.QuestionStore
	Curriculum write.CurriculumStore
	Catalog    write.CatalogStore
	Schema     port.QuestionSchema
	Registry   *question.Registry
	Resources  *port.Registry
	Events     write.EventPublisher
	Audit      write.Auditor
	Clock      func() time.Time
	// Math validates and stores the formulas of a version (F027 US3); nil keeps the F022 check only.
	Math *MathAssets
	// Media checks media references and records media_usages (F027-AC-11); nil skips it.
	Media write.MediaStore
	// Docs resolves worked_example_ref in lessons and learning materials (F024-AC-08); nil skips the check.
	Docs write.DocStore
}

func (h Questions) trash() TrashDeps {
	return TrashDeps{UoW: h.UoW, Resources: h.Resources, Events: h.Events, Audit: h.Audit, Clock: h.Clock}
}

// QuestionFields are the fields of QuestionAuthoringInput; Present lists the keys of the request (merge-patch: a
// present null clears a nullable field).
type QuestionFields struct {
	Present             map[string]bool
	TenantID            *uuid.UUID
	OwnerScope          string
	Type                string
	Stem                json.RawMessage
	RenderPayload       json.RawMessage
	AnswerKey           json.RawMessage
	ScoringRule         json.RawMessage
	Explanation         json.RawMessage
	Hints               json.RawMessage
	WorkedExampleRef    json.RawMessage
	MathAssets          json.RawMessage
	MinResponseMS       *int
	MaxScore            *float64
	Tags                []string
	Difficulty          *int
	GradeLevels         []int
	SubjectID           *uuid.UUID
	TopicID             *uuid.UUID
	CurriculumVersionID *uuid.UUID
	ObjectiveIDs        []uuid.UUID
	MediaRefs           []uuid.UUID
	ContentLanguage     string
	ChangeNote          *string
	ExamOnly            *bool
	EfficacyInstrument  *bool
	AudioExemptReason   *string // F025: "" clears
}

func (f QuestionFields) has(k string) bool { return f.Present[k] }

func isNull(r json.RawMessage) bool { return len(r) == 0 || string(r) == "null" }

// apply writes the present fields onto v; authRender is the authoring render payload (with option feedback).
func (f QuestionFields) apply(v *dto.QuestionVersion, authRender *json.RawMessage) {
	set := func(k string, dst *json.RawMessage, val json.RawMessage) {
		if f.has(k) {
			if isNull(val) {
				*dst = nil
			} else {
				*dst = val
			}
		}
	}
	if f.has("type") {
		v.Type = f.Type
	}
	set("stem", &v.Stem, f.Stem)
	set("render_payload", authRender, f.RenderPayload)
	set("answer_key", &v.AnswerKey, f.AnswerKey)
	set("scoring_rule", &v.ScoringRule, f.ScoringRule)
	set("explanation", &v.Explanation, f.Explanation)
	set("hints", &v.Hints, f.Hints)
	set("worked_example_ref", &v.WorkedExampleRef, f.WorkedExampleRef)
	set("math_assets", &v.MathAssets, f.MathAssets)
	if f.has("min_response_ms") {
		v.MinResponseMS = f.MinResponseMS
	}
	if f.has("max_score") && f.MaxScore != nil {
		v.MaxScore = *f.MaxScore
	}
	if f.has("tags") {
		v.Tags = f.Tags
	}
	if f.has("difficulty") && f.Difficulty != nil {
		v.Difficulty = *f.Difficulty
	}
	if f.has("grade_levels") {
		v.GradeLevels = f.GradeLevels
	}
	if f.has("subject_id") && f.SubjectID != nil {
		v.SubjectID = *f.SubjectID
	}
	if f.has("topic_id") && f.TopicID != nil {
		v.TopicID = *f.TopicID
	}
	if f.has("curriculum_version_id") && f.CurriculumVersionID != nil {
		v.CurriculumVersionID = *f.CurriculumVersionID
	}
	if f.has("objective_ids") {
		v.ObjectiveIDs = f.ObjectiveIDs
	}
	if f.has("media_refs") {
		v.MediaRefs = f.MediaRefs
	}
	if f.has("content_language") && f.ContentLanguage != "" {
		v.ContentLanguage = f.ContentLanguage
	}
	if f.has("change_note") {
		v.ChangeNote = f.ChangeNote
	}
	if f.has("exam_only") && f.ExamOnly != nil {
		v.ExamOnly = *f.ExamOnly
	}
	if f.has("efficacy_instrument") && f.EfficacyInstrument != nil {
		v.EfficacyInstrument = *f.EfficacyInstrument
	}
	if f.has("audio_exempt_reason") {
		v.AudioExemptReason = nonEmpty(f.AudioExemptReason)
	}
}

// problemOfViolations turns domain violations (JSON pointers) into 422 VALIDATION_FAILED.
func problemOfViolations(vs contentmodel.Violations) error {
	pe := &problem.Error{Code: problem.ValidationFailed, Detail: "request validation failed"}
	for _, v := range vs {
		sub := problem.SubInvalid
		if v.Required {
			sub = problem.SubRequired
		}
		msg := v.Reason
		if utf8.RuneCountInString(msg) > 500 {
			msg = string([]rune(msg)[:500])
		}
		pe.AddField(v.Field, sub, msg)
	}
	return pe
}

// prepare validates a version in its authoring form and turns it into the stored form (F022-AC-01…06, AC-12): NFC
// text, JSON Schema then semantic validation of the type, mandatory metadata, references (topic, LOs, subject in
// the curriculum version), option feedback split off the render payload, content hash.
func (h Questions) prepare(ctx context.Context, v *dto.QuestionVersion, authRender json.RawMessage) error {
	v.Stem, authRender, v.AnswerKey = question.NFC(v.Stem), question.NFC(authRender), question.NFC(v.AnswerKey)
	v.Explanation, v.Hints = question.NFC(v.Explanation), question.NFC(v.Hints)
	var hints []json.RawMessage
	if !isNull(v.Hints) {
		if err := json.Unmarshal(v.Hints, &hints); err != nil {
			return problem.Invalid("/hints", problem.SubInvalid, "must be a list of rich contents")
		}
	}
	typ := question.Type(v.Type)
	if !typ.Known() {
		return problem.Invalid("/type", problem.SubInvalid, "is not a question type")
	}
	if _, err := h.Registry.Handler(typ); errors.Is(err, question.ErrTypeNotEnabled) {
		return problem.Invalid("/type", question.SubTypeNotEnabled, v.Type+" is not enabled yet (F023)")
	}
	if vs := h.Schema.ValidateAuthoring(port.QuestionDoc{Type: v.Type, Stem: v.Stem, RenderPayload: authRender, AnswerKey: v.AnswerKey,
		ScoringRule: v.ScoringRule, Explanation: v.Explanation, Hints: hints}); len(vs) > 0 {
		return problemOfViolations(vs)
	}
	var rule question.ScoringRule
	_ = json.Unmarshal(v.ScoringRule, &rule)
	var assets []question.MathAsset
	if h.Math != nil {
		stored, resolved, err := h.Math.Resolve(ctx, v.MathAssets)
		if err != nil {
			return err
		}
		v.MathAssets, assets = stored, resolved
	} else if !isNull(v.MathAssets) {
		if err := json.Unmarshal(v.MathAssets, &assets); err != nil {
			return problem.Invalid("/math_assets", problem.SubInvalid, "must be a list of rendered formulas")
		}
	}
	a := question.Authoring{Type: typ, Stem: v.Stem, RenderPayload: authRender, AnswerKey: v.AnswerKey, Scoring: rule, MaxScore: v.MaxScore,
		Explanation: v.Explanation, Hints: hints, MathAssets: assets, MinResponseMS: v.MinResponseMS, Language: v.ContentLanguage, MediaRefs: v.MediaRefs}
	var all contentmodel.Violations
	if err := h.Registry.Validate(a, question.Limits{AssetsPerVersion: AssetsPerVersionMax}); err != nil {
		var vs contentmodel.Violations
		if !errors.As(err, &vs) {
			return err
		}
		all = append(all, vs...)
	}
	topic, cv := v.TopicID, v.CurriculumVersionID
	difficulty := v.Difficulty
	meta := contentmodel.ContentMeta{GradeLevels: v.GradeLevels, SubjectID: v.SubjectID, TopicID: &topic, ObjectiveIDs: v.ObjectiveIDs,
		Difficulty: &difficulty, VersionNo: v.VersionNo, Status: v.Status, AuthorID: v.AuthorID, Owner: v.Owner,
		ContentLanguage: contentmodel.Language(v.ContentLanguage), CurriculumVersionID: &cv, Tags: v.Tags}
	if topic == uuid.Nil {
		meta.TopicID = nil
	}
	if cv == uuid.Nil {
		meta.CurriculumVersionID = nil
	}
	if difficulty == 0 {
		meta.Difficulty = nil
	}
	if err := meta.Validate(contentmodel.TypeQuestion); err != nil {
		var vs contentmodel.Violations
		if errors.As(err, &vs) {
			for _, x := range vs {
				x.Field = "/" + x.Field
				all = append(all, x)
			}
		}
	}
	if v.EfficacyInstrument {
		// F035-AC-01: platform, exam_only and never parameterized.
		all = append(all, question.EfficacyQuestion(v.Owner.TenantID == nil, v.ExamOnly, authRender)...)
	}
	if len(all) > 0 {
		return problemOfViolations(all)
	}
	if err := h.checkReferences(ctx, *v); err != nil {
		return err
	}
	if err := h.checkFuzzy(ctx, *v); err != nil {
		return err
	}
	if err := h.checkMedia(ctx, *v, authRender); err != nil {
		return err
	}
	if err := h.checkWorkedExample(ctx, *v); err != nil {
		return err
	}
	render, feedback, err := question.SplitFeedback(authRender)
	if err != nil {
		return problem.Invalid("/render_payload", problem.SubInvalid, "is not an object")
	}
	v.RenderPayload = render
	v.OptionFeedback = nil
	if len(feedback) > 0 {
		b, _ := json.Marshal(feedback)
		v.OptionFeedback = b
	}
	v.ContentHash = question.ContentHash(json.RawMessage(v.Type), v.Stem, v.RenderPayload, v.AnswerKey, v.ScoringRule)
	return nil
}

// checkReferences: the topic and LOs belong to the curriculum version, the topic to the subject, all visible to the
// scope (a platform curriculum assigned to the tenant included).
func (h Questions) checkReferences(ctx context.Context, v dto.QuestionVersion) error {
	if _, err := h.Catalog.Subject(ctx, v.SubjectID); err != nil {
		return problem.Invalid("/subject_id", problem.SubInvalid, "is not a subject of the catalog")
	}
	t, err := h.Curriculum.Topic(ctx, v.TopicID)
	if err != nil || t.VersionID != v.CurriculumVersionID || t.SubjectID != v.SubjectID {
		return problem.Invalid("/topic_id", problem.SubInvalid, "must be a topic of the subject in the curriculum version")
	}
	for i, id := range v.ObjectiveIDs {
		o, err := h.Curriculum.Objective(ctx, id)
		if err != nil || o.VersionID != v.CurriculumVersionID {
			return problem.Invalid("/objective_ids/"+strconv.Itoa(i), problem.SubInvalid, "must be an objective of the curriculum version")
		}
	}
	return nil
}

// QuestionResult is a question with one version and the authoring render payload (feedback merged back).
type QuestionResult struct {
	Question dto.Question
	Version  dto.QuestionVersion
}

// View returns the CMS view of r for the session tenant.
func (r QuestionResult) View(sessionTenant *uuid.UUID) dto.QuestionAuthoringView {
	var fb map[string]question.OptionFeedback
	if !isNull(r.Version.OptionFeedback) {
		_ = json.Unmarshal(r.Version.OptionFeedback, &fb)
	}
	render, err := question.MergeFeedback(r.Version.RenderPayload, fb)
	if err != nil {
		render = r.Version.RenderPayload
	}
	return dto.QuestionJSON(r.Question, r.Version, render, sessionTenant)
}

// CreateQuestion creates a question with version 1 in draft (F022-AC-01): a tenant user for its session tenant,
// platform staff a platform question, System Admin a tenant one with tenant_id (scope.ForWrite).
func (h Questions) CreateQuestion(ctx context.Context, f QuestionFields) (out QuestionResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateQuestion")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return out, err
	}
	ownerScope := contentmodel.OwnerPlatform
	if p.TenantID != nil || f.TenantID != nil || f.OwnerScope == string(contentmodel.OwnerTenant) {
		ownerScope = contentmodel.OwnerTenant
	}
	if f.OwnerScope == string(contentmodel.OwnerPlatform) && p.TenantID != nil {
		return out, denied(ctx, "platform_content_by_tenant")
	}
	sc, err := scope.ForWrite(p, ownerScope, f.TenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	o := contentmodel.Owner{Scope: ownerScope}
	if ownerScope == contentmodel.OwnerTenant {
		o.TenantID = ptrClone(p.TenantID)
		if o.TenantID == nil {
			o.TenantID = ptrClone(f.TenantID)
		}
	}
	q := dto.Question{ID: ids.New(), Owner: o, LatestVersionNo: 1}
	v := dto.QuestionVersion{ID: ids.New(), QuestionID: q.ID, VersionNo: 1, Status: contentmodel.StatusDraft, MaxScore: 1, ContentLanguage: "vi",
		AuthorID: p.UserID, Owner: o}
	var authRender json.RawMessage
	f.apply(&v, &authRender)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.prepare(ctx, &v, authRender); err != nil {
			return err
		}
		next, _ := questionDoc(v)
		v.TranslationStatus, v.TranslationMeta = translation(nil, next, v.ContentLanguage, nil, now(h.Clock))
		if err := h.Store.InsertQuestion(ctx, q); err != nil {
			return err
		}
		if err := h.Store.InsertQuestionVersion(ctx, v); err != nil {
			return err
		}
		if err := h.recordUsages(ctx, v); err != nil {
			return err
		}
		if err := h.recordQuestion(ctx, o, audit.Event{Action: ActionContentVersionCreated, TargetID: q.ID.String(),
			Metadata: map[string]any{"question_version_id": v.ID.String(), "version": 1, "type": v.Type}}); err != nil {
			return err
		}
		if out, err = h.load(ctx, q.ID, 1); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out.View(p.TenantID))
	})
	return out, err
}

func (h Questions) load(ctx context.Context, id uuid.UUID, versionNo int) (QuestionResult, error) {
	q, err := h.Store.Question(ctx, id)
	if err != nil {
		return QuestionResult{}, err
	}
	v, err := h.Store.QuestionVersion(ctx, id, versionNo)
	return QuestionResult{Question: q, Version: v}, err
}

func (h Questions) recordQuestion(ctx context.Context, o contentmodel.Owner, e audit.Event) error {
	e.TargetType = string(contentmodel.ResourceQuestion)
	if e.Result == "" {
		e.Result = audit.ResultSuccess
	}
	return h.trash().recordFor(ctx, o.TenantID, e)
}

// UpdateQuestionDraftInput is the merge-patch of updateQuestionDraft.
type UpdateQuestionDraftInput struct {
	QuestionID uuid.UUID
	VersionNo  int
	IfMatch    int64
	Fields     QuestionFields
}

// UpdateQuestionDraft edits a draft version (If-Match, F022-AC-07, AC-08): other statuses → 409
// CONTENT_NOT_EDITABLE; the whole version is validated again.
func (h Questions) UpdateQuestionDraft(ctx context.Context, in UpdateQuestionDraftInput) (out QuestionResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateQuestionDraft")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.load(ctx, in.QuestionID, in.VersionNo)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermLearningContentWrite, cur.Question.Owner); err != nil {
			return err
		}
		if cur.Version.Status != contentmodel.StatusDraft {
			return problem.New(problem.ContentNotEditable, "the question version is not a draft; create a new version")
		}
		if err := checkRowVersion(cur.Version.RowVersion, in.IfMatch); err != nil {
			return err
		}
		if err := sameOwner(in.Fields, cur.Question.Owner); err != nil {
			return err
		}
		v := cur.Version
		authRender, err := mergedRender(v)
		if err != nil {
			return err
		}
		in.Fields.apply(&v, &authRender)
		if err := h.prepare(ctx, &v, authRender); err != nil {
			return err
		}
		prev, _ := questionDoc(cur.Version)
		next, _ := questionDoc(v)
		v.TranslationStatus, v.TranslationMeta = translation(prev, next, v.ContentLanguage, cur.Version.TranslationMeta, now(h.Clock))
		if _, err := h.Store.UpdateQuestionVersion(ctx, v, in.IfMatch); err != nil {
			return err
		}
		if err := h.recordUsages(ctx, v); err != nil {
			return err
		}
		out, err = h.load(ctx, in.QuestionID, in.VersionNo)
		return err
	})
	return out, err
}

func mergedRender(v dto.QuestionVersion) (json.RawMessage, error) {
	var fb map[string]question.OptionFeedback
	if !isNull(v.OptionFeedback) {
		if err := json.Unmarshal(v.OptionFeedback, &fb); err != nil {
			return nil, err
		}
	}
	return question.MergeFeedback(v.RenderPayload, fb)
}

// NewQuestionVersionInput is the request of createQuestionVersion.
type NewQuestionVersionInput struct {
	QuestionID  uuid.UUID
	FromVersion int
	Note        string
}

// CreateQuestionVersion copies a version into a new draft (F022-AC-07): based_on_version_id = the source; refused with
// 409 CONFLICT while the question has a draft or in_review version.
func (h Questions) CreateQuestionVersion(ctx context.Context, in NewQuestionVersionInput) (out QuestionResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateQuestionVersion")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return out, err
	}
	if utf8.RuneCountInString(in.Note) > 500 {
		return out, problem.Invalid("note", problem.SubMaxLength, "is longer than 500 characters")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		q, err := h.Store.Question(ctx, in.QuestionID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermLearningContentWrite, q.Owner); err != nil {
			return err
		}
		vs, err := h.Store.QuestionVersions(ctx, q.ID)
		if err != nil {
			return err
		}
		var src *dto.QuestionVersion
		maxNo := q.LatestVersionNo
		for i, v := range vs {
			if v.Status == contentmodel.StatusDraft || v.Status == contentmodel.StatusInReview {
				return problem.New(problem.Conflict, "the question already has an open draft or in-review version")
			}
			if v.VersionNo == in.FromVersion {
				src = &vs[i]
			}
			maxNo = max(maxNo, v.VersionNo)
		}
		if src == nil {
			return problem.New(problem.NotFound, "the source version does not exist")
		}
		v := *src
		srcID := src.ID
		v.ID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID = ids.New(), maxNo+1, contentmodel.StatusDraft, p.UserID, nil, nil, &srcID
		v.AudioExemptApprovedBy = nil
		v.ChangeNote = nil
		if in.Note != "" {
			note := in.Note
			v.ChangeNote = &note
		}
		v.Owner = q.Owner
		if err := h.Store.InsertQuestionVersion(ctx, v); err != nil {
			return err
		}
		if err := h.recordUsages(ctx, v); err != nil {
			return err
		}
		q.LatestVersionNo = v.VersionNo
		if _, err := h.Store.UpdateQuestion(ctx, q, q.RowVersion); err != nil {
			return err
		}
		if err := h.recordQuestion(ctx, q.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: q.ID.String(),
			Metadata: map[string]any{"question_version_id": v.ID.String(), "version": v.VersionNo, "based_on_version_id": srcID.String()}}); err != nil {
			return err
		}
		if out, err = h.load(ctx, q.ID, v.VersionNo); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out.View(p.TenantID))
	})
	return out, err
}

// DeleteQuestionInput is the request of deleteQuestion.
type DeleteQuestionInput struct {
	ID      uuid.UUID
	IfMatch int64
	Reason  string
}

// DeleteQuestion soft-deletes a question that is not released (F022-AC-11): Thùng rác, audit content.deleted,
// question.deleted; its versions stay, so attempts made on them can still be reviewed.
func (h Questions) DeleteQuestion(ctx context.Context, in DeleteQuestionInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.DeleteQuestion")
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(in.Reason) > 500 {
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 500 characters")
	}
	handler, ok := h.Resources.Get(contentmodel.ResourceQuestion)
	if !ok {
		return problem.New(problem.Internal, "no question resource handler")
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		q, err := h.Store.Question(ctx, in.ID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermLearningContentWrite, q.Owner); err != nil {
			return err
		}
		if err := checkRowVersion(q.RowVersion, in.IfMatch); err != nil {
			return err
		}
		if _, err := handler.SoftDelete(ctx, q.ID, in.Reason); err != nil {
			if errors.Is(err, contentmodel.ErrItemPublished) {
				return problem.New(problem.Conflict, "a released question cannot be deleted; unpublish it first")
			}
			return err
		}
		if err := h.recordQuestion(ctx, q.Owner, audit.Event{Action: ActionContentDeleted, TargetID: q.ID.String(), Reason: in.Reason,
			Metadata: map[string]any{"resource_type": string(contentmodel.ResourceQuestion)}}); err != nil {
			return err
		}
		by := p.UserID
		return h.trash().publishResource(ctx, contentmodel.ResourceQuestion, "deleted", ResourceEventPayload{
			ResourceType: string(contentmodel.ResourceQuestion), ResourceID: q.ID, TenantID: q.Owner.TenantID, DeletedBy: &by, Reason: in.Reason})
	})
}

// QuestionResource is the ResourceHandler of resource_type question (F022 T007): trash, restore, owners and clone
// (question + latest version as a draft of the target tenant; never attempts, R22).
type QuestionResource struct {
	Store write.QuestionStore
}

var _ port.ResourceHandler = QuestionResource{}

// ResourceType implements port.ResourceHandler.
func (QuestionResource) ResourceType() contentmodel.ResourceType {
	return contentmodel.ResourceQuestion
}

// WritePermission implements port.ResourceHandler.
func (QuestionResource) WritePermission() authz.Permission { return authz.PermLearningContentWrite }

// Owners implements port.ResourceHandler.
func (r QuestionResource) Owners(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]contentmodel.Owner, error) {
	out := map[uuid.UUID]contentmodel.Owner{}
	for _, id := range idList {
		q, err := r.Store.Question(ctx, id)
		if err == nil {
			out[id] = q.Owner
			continue
		}
		if pe, ok := problemOf(err); !ok || pe.Code != problem.NotFound {
			return nil, err
		}
	}
	return out, nil
}

// SoftDelete implements port.ResourceHandler.
func (r QuestionResource) SoftDelete(ctx context.Context, id uuid.UUID, reason string) (port.Deleted, error) {
	q, err := r.Store.Question(ctx, id)
	if err != nil {
		return port.Deleted{}, err
	}
	if err := contentmodel.CanDeleteItem(q.CurrentVersionID); err != nil {
		return port.Deleted{}, err
	}
	if err := r.Store.SoftDeleteQuestion(ctx, id, reason); err != nil {
		return port.Deleted{}, err
	}
	return port.Deleted{Owner: q.Owner, Title: "question " + id.String()}, nil
}

// ListDeleted implements port.ResourceHandler.
func (r QuestionResource) ListDeleted(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	return r.Store.DeletedQuestions(ctx, f)
}

// Restore implements port.ResourceHandler.
func (r QuestionResource) Restore(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	return r.Store.RestoreQuestion(ctx, id)
}

// Clone implements port.ResourceHandler (ctx carries tenancy.Bypass(tenant_clone)).
func (r QuestionResource) Clone(ctx context.Context, in port.CloneInput) (uuid.UUID, error) {
	src, err := r.Store.Question(ctx, in.SourceID)
	if err != nil || src.Owner.TenantID == nil || *src.Owner.TenantID != in.SourceTenant {
		return uuid.Nil, port.ErrNotInSourceTenant
	}
	vs, err := r.Store.QuestionVersions(ctx, src.ID)
	if err != nil {
		return uuid.Nil, err
	}
	target, srcTenant, at, by := in.TargetTenant, in.SourceTenant, in.At, in.Actor
	o := contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &target}
	q := dto.Question{ID: ids.New(), Owner: o, ClonedFromID: &src.ID, ClonedFromTenantID: &srcTenant, ClonedAt: &at, ClonedBy: &by}
	if len(vs) > 0 {
		q.LatestVersionNo = 1
	}
	if err := r.Store.InsertQuestion(ctx, q); err != nil {
		return uuid.Nil, err
	}
	if len(vs) == 0 {
		return q.ID, nil
	}
	v := vs[0]
	latest := v.ID
	v.ID, v.QuestionID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID, v.Owner =
		ids.New(), q.ID, 1, contentmodel.StatusDraft, in.Actor, nil, nil, &latest, o
	return q.ID, r.Store.InsertQuestionVersion(ctx, v)
}

// sameOwner refuses a draft edit that would move the question to another owner. The OpenAPI validator fills the
// default owner_scope "tenant" into every body, so only an explicit "platform" on a tenant question and a different
// tenant_id are a change request.
func sameOwner(f QuestionFields, o contentmodel.Owner) error {
	if f.has("owner_scope") && f.OwnerScope == string(contentmodel.OwnerPlatform) && o.Scope != contentmodel.OwnerPlatform {
		return problem.Invalid("/owner_scope", problem.SubInvalid, "cannot change on a version")
	}
	if f.has("tenant_id") && f.TenantID != nil && (o.TenantID == nil || *o.TenantID != *f.TenantID) {
		return problem.Invalid("/tenant_id", problem.SubInvalid, "cannot change on a version")
	}
	return nil
}

// SubFuzzyNotAllowed is errors[].code of a fuzzy match on a spelling subject (F023-AC-03).
const SubFuzzyNotAllowed = "FUZZY_NOT_ALLOWED_FOR_SUBJECT"

// fuzzyBlockedSubjects are the subject codes whose spelling is the learning goal (QUESTION_BANK §4 step 7) [Proposed].
var fuzzyBlockedSubjects = []string{"vietnamese", "english"}

// checkFuzzy refuses fuzzy matching on questions of Vietnamese or English (F023-AC-03).
func (h Questions) checkFuzzy(ctx context.Context, v dto.QuestionVersion) error {
	if !question.FuzzyAnywhere(v.AnswerKey) {
		return nil
	}
	s, err := h.Catalog.Subject(ctx, v.SubjectID)
	if err != nil {
		return nil //nolint:nilerr // the subject is checked by checkReferences
	}
	if slices.Contains(fuzzyBlockedSubjects, s.Code) {
		return problem.Invalid("/answer_key", SubFuzzyNotAllowed, "fuzzy matching is not allowed for "+s.Code)
	}
	return nil
}

// nonEmpty returns nil for a nil or blank string (a present "" clears an optional text).
func nonEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}

// EfficacyInstrument implements port.EfficacyFlagged: a version of the question is an efficacy instrument (F035-AC-04).
func (r QuestionResource) EfficacyInstrument(ctx context.Context, id uuid.UUID) (bool, error) {
	vs, err := r.Store.QuestionVersions(ctx, id)
	if err != nil {
		if pe, ok := problemOf(err); ok && pe.Code == problem.NotFound {
			return false, nil
		}
		return false, err
	}
	for _, v := range vs {
		if v.EfficacyInstrument {
			return true, nil
		}
	}
	return false, nil
}
