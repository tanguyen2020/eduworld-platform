package command

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DocTypes are the composed content types of F024.
var DocTypes = []contentmodel.ContentType{contentmodel.TypeQuestionSet, contentmodel.TypeLesson, contentmodel.TypeLearningMaterial,
	contentmodel.TypeAssessmentBlueprint}

// BodyKeys are the request keys stored in the body of a version, by type (the other keys are metadata columns).
var BodyKeys = map[contentmodel.ContentType][]string{
	contentmodel.TypeQuestionSet:         {"definition", "default_feedback_timing"},
	contentmodel.TypeLesson:              {"blocks", "completion_rule", "estimated_minutes"},
	contentmodel.TypeLearningMaterial:    {"blocks", "material_kind"},
	contentmodel.TypeAssessmentBlueprint: {"definition", "exam_type", "feedback_timing", "result_release", "efficacy_instrument"},
}

// Docs holds the collaborators of the commands of the composed content types (F024 US1–US3, US6).
type Docs struct {
	UoW        write.UnitOfWork
	Store      write.DocStore
	Questions  write.QuestionStore
	Curriculum write.CurriculumStore
	Catalog    write.CatalogStore
	Schema     port.ContentSchema
	Media      write.MediaStore // nil: media references are only counted
	Read       read.Session     // platform reference checks (RESOURCE_NOT_ASSIGNED)
	Resources  *port.Registry
	Events     write.EventPublisher
	Audit      write.Auditor
	Clock      func() time.Time
	// ActivityEnabled: F030 registered the activity block validator (BLOCK_KIND_NOT_ENABLED otherwise).
	ActivityEnabled bool
	// MaxMeanPDiff is variant_balance_max_mean_p_diff (0 = blueprint.DefaultMaxMeanPDiff).
	MaxMeanPDiff float64
}

func (h Docs) trash() TrashDeps {
	return TrashDeps{UoW: h.UoW, Resources: h.Resources, Events: h.Events, Audit: h.Audit, Clock: h.Clock}
}

// DocFields are the fields of a create or merge-patch request; Present lists its keys (a present null clears).
type DocFields struct {
	Present             map[string]bool
	TenantID            *uuid.UUID
	OwnerScope          string
	Title               json.RawMessage
	Summary             json.RawMessage
	SubjectID           *uuid.UUID
	CurriculumVersionID *uuid.UUID
	GradeLevels         []int
	TopicIDs            []uuid.UUID
	ObjectiveIDs        []uuid.UUID
	ContentLanguage     string
	ChangeNote          *string
	AudioExemptReason   *string // F025: "" clears
	Body                map[string]json.RawMessage
}

func (f DocFields) has(k string) bool { return f.Present[k] }

// apply writes the present fields onto v.
func (f DocFields) apply(v *dto.DocVersion) error {
	if f.has("title") {
		v.Title = f.Title
	}
	if f.has("summary") {
		v.Summary = f.Summary
		if isNull(f.Summary) {
			v.Summary = nil
		}
	}
	if f.has("subject_id") {
		v.SubjectID = f.SubjectID
	}
	if f.has("curriculum_version_id") {
		v.CurriculumVersionID = f.CurriculumVersionID
	}
	if f.has("grade_levels") {
		v.GradeLevels = f.GradeLevels
	}
	if f.has("topic_ids") {
		v.TopicIDs = f.TopicIDs
	}
	if f.has("objective_ids") {
		v.ObjectiveIDs = f.ObjectiveIDs
	}
	if f.has("content_language") && f.ContentLanguage != "" {
		v.ContentLanguage = f.ContentLanguage
	}
	if f.has("change_note") {
		v.ChangeNote = f.ChangeNote
	}
	if f.has("audio_exempt_reason") {
		v.AudioExemptReason = nonEmpty(f.AudioExemptReason)
	}
	if len(f.Body) == 0 {
		return nil
	}
	body := map[string]json.RawMessage{}
	if !isNull(v.Body) {
		if err := json.Unmarshal(v.Body, &body); err != nil {
			return err
		}
	}
	for k, raw := range f.Body {
		if isNull(raw) {
			delete(body, k)
		} else {
			body[k] = raw
		}
	}
	b, err := json.Marshal(body)
	v.Body = b
	return err
}

// DocResult is an item with one version.
type DocResult struct {
	Doc     dto.Doc
	Version dto.DocVersion
}

// docOwner chooses the owner and scope of a new item (same rules as questions: CreateQuestion).
func docOwner(ctx context.Context, p authz.Principal, ownerScope string, target *uuid.UUID) (contentmodel.Owner, tenancy.Scope, error) {
	kind := contentmodel.OwnerPlatform
	if p.TenantID != nil || target != nil {
		kind = contentmodel.OwnerTenant
	}
	if ownerScope == string(contentmodel.OwnerPlatform) && p.TenantID != nil {
		return contentmodel.Owner{}, nil, denied(ctx, "platform_content_by_tenant")
	}
	sc, err := scope.ForWrite(p, kind, target)
	if err != nil {
		return contentmodel.Owner{}, nil, err
	}
	o := contentmodel.Owner{Scope: kind}
	if kind == contentmodel.OwnerTenant {
		o.TenantID = ptrClone(p.TenantID)
		if o.TenantID == nil {
			o.TenantID = ptrClone(target)
		}
	}
	return o, sc, nil
}

func (h Docs) record(ctx context.Context, t contentmodel.ContentType, o contentmodel.Owner, e audit.Event) error {
	e.TargetType = string(t)
	if e.Result == "" {
		e.Result = audit.ResultSuccess
	}
	return h.trash().recordFor(ctx, o.TenantID, e)
}

// Create creates an item with version 1 in draft (F024-AC-01, AC-06, AC-09, AC-10).
func (h Docs) Create(ctx context.Context, t contentmodel.ContentType, f DocFields) (out DocResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.Create."+string(t))
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return out, err
	}
	o, sc, err := docOwner(ctx, p, f.OwnerScope, f.TenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	d := dto.Doc{Type: t, ID: ids.New(), Owner: o, LatestVersionNo: 1}
	v := dto.DocVersion{Type: t, ID: ids.New(), ItemID: d.ID, VersionNo: 1, Status: contentmodel.StatusDraft, ContentLanguage: "vi", AuthorID: p.UserID,
		Owner: o, Body: json.RawMessage("{}")}
	if err := f.apply(&v); err != nil {
		return out, problem.Invalid("/", problem.SubInvalid, "invalid body")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.prepare(ctx, &v); err != nil {
			return err
		}
		v.TranslationStatus, v.TranslationMeta = translation(nil, docDoc(v), v.ContentLanguage, nil, now(h.Clock))
		if err := h.Store.InsertDoc(ctx, d); err != nil {
			return err
		}
		if err := h.Store.InsertDocVersion(ctx, v); err != nil {
			return err
		}
		if err := h.recordUsages(ctx, v); err != nil {
			return err
		}
		if err := h.record(ctx, t, o, audit.Event{Action: ActionContentVersionCreated, TargetID: d.ID.String(),
			Metadata: map[string]any{"content_version_id": v.ID.String(), "version": 1}}); err != nil {
			return err
		}
		if out, err = h.load(ctx, t, d.ID, 1); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, DocJSON(out, p.TenantID))
	})
	return out, err
}

func (h Docs) load(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, no int) (DocResult, error) {
	d, err := h.Store.Doc(ctx, t, id)
	if err != nil {
		return DocResult{}, err
	}
	v, err := h.Store.DocVersion(ctx, t, id, no)
	return DocResult{Doc: d, Version: v}, err
}

// UpdateDocDraftInput is a merge-patch of a draft.
type UpdateDocDraftInput struct {
	Type      contentmodel.ContentType
	ID        uuid.UUID
	VersionNo int
	IfMatch   int64
	Fields    DocFields
}

// UpdateDraft edits a draft (F024-AC-05): If-Match, other statuses 409 CONTENT_NOT_EDITABLE; the merged version is
// validated again.
func (h Docs) UpdateDraft(ctx context.Context, in UpdateDocDraftInput) (out DocResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateDraft."+string(in.Type))
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		cur, err := h.load(ctx, in.Type, in.ID, in.VersionNo)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermLearningContentWrite, cur.Doc.Owner); err != nil {
			return err
		}
		if cur.Version.Status != contentmodel.StatusDraft {
			return problem.New(problem.ContentNotEditable, "the version is not a draft; create a new version")
		}
		if err := checkRowVersion(cur.Version.RowVersion, in.IfMatch); err != nil {
			return err
		}
		if in.Fields.has("tenant_id") && in.Fields.TenantID != nil && (cur.Doc.Owner.TenantID == nil || *cur.Doc.Owner.TenantID != *in.Fields.TenantID) {
			return problem.Invalid("/tenant_id", problem.SubInvalid, "cannot change on a version")
		}
		v := cur.Version
		if err := in.Fields.apply(&v); err != nil {
			return problem.Invalid("/", problem.SubInvalid, "invalid body")
		}
		if err := h.prepare(ctx, &v); err != nil {
			return err
		}
		v.TranslationStatus, v.TranslationMeta = translation(docDoc(cur.Version), docDoc(v), v.ContentLanguage, cur.Version.TranslationMeta, now(h.Clock))
		if _, err := h.Store.UpdateDocVersion(ctx, v, in.IfMatch); err != nil {
			return err
		}
		if err := h.recordUsages(ctx, v); err != nil {
			return err
		}
		out, err = h.load(ctx, in.Type, in.ID, in.VersionNo)
		return err
	})
	return out, err
}

// NewDocVersionInput is NewVersionRequest.
type NewDocVersionInput struct {
	Type        contentmodel.ContentType
	ID          uuid.UUID
	FromVersion int
	Note        string
}

// NewVersion copies a version into a new draft (F024-AC-05): 409 CONFLICT while a draft or in_review version is open.
func (h Docs) NewVersion(ctx context.Context, in NewDocVersionInput) (out DocResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.NewVersion."+string(in.Type))
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return out, err
	}
	if utf8.RuneCountInString(in.Note) > 500 {
		return out, problem.Invalid("note", problem.SubMaxLength, "is longer than 500 characters")
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		d, err := h.Store.Doc(ctx, in.Type, in.ID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermLearningContentWrite, d.Owner); err != nil {
			return err
		}
		vs, err := h.Store.DocVersions(ctx, in.Type, d.ID)
		if err != nil {
			return err
		}
		var src *dto.DocVersion
		maxNo := d.LatestVersionNo
		for i, v := range vs {
			if v.Status == contentmodel.StatusDraft || v.Status == contentmodel.StatusInReview {
				return problem.New(problem.Conflict, "the item already has an open draft or in-review version")
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
		v.ID, v.VersionNo, v.Status, v.AuthorID, v.ReviewerID, v.PublishedAt, v.BasedOnID, v.ChangeNote = ids.New(), maxNo+1, contentmodel.StatusDraft,
			p.UserID, nil, nil, &srcID, nil
		if in.Note != "" {
			note := in.Note
			v.ChangeNote = &note
		}
		v.Owner = d.Owner
		if err := h.Store.InsertDocVersion(ctx, v); err != nil {
			return err
		}
		if err := h.recordUsages(ctx, v); err != nil {
			return err
		}
		d.LatestVersionNo = v.VersionNo
		if _, err := h.Store.UpdateDoc(ctx, d, d.RowVersion); err != nil {
			return err
		}
		if err := h.record(ctx, in.Type, d.Owner, audit.Event{Action: ActionContentVersionCreated, TargetID: d.ID.String(),
			Metadata: map[string]any{"content_version_id": v.ID.String(), "version": v.VersionNo, "based_on_version_id": srcID.String()}}); err != nil {
			return err
		}
		if out, err = h.load(ctx, in.Type, d.ID, v.VersionNo); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, DocJSON(out, p.TenantID))
	})
	return out, err
}

// DeleteDocInput is a delete request.
type DeleteDocInput struct {
	Type    contentmodel.ContentType
	ID      uuid.UUID
	IfMatch int64
	Reason  string
}

// Delete soft-deletes an item that is not released (F024-AC-23): Thùng rác, audit content.deleted, <type>.deleted.
func (h Docs) Delete(ctx context.Context, in DeleteDocInput) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.Delete."+string(in.Type))
	defer func() { done(err) }()
	p, err := authorize(ctx, authz.PermLearningContentWrite)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(in.Reason) > 500 {
		return problem.Invalid("reason", problem.SubMaxLength, "is longer than 500 characters")
	}
	rt, _ := contentmodel.ResourceOf(in.Type)
	handler, ok := h.Resources.Get(rt)
	if !ok {
		return problem.New(problem.Internal, "no resource handler for "+string(in.Type))
	}
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		d, err := h.Store.Doc(ctx, in.Type, in.ID)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, authz.PermLearningContentWrite, d.Owner); err != nil {
			return err
		}
		// The ETag of getCms<Type> is the row version of the newest version (no version endpoint): If-Match compares it.
		vs, err := h.Store.DocVersions(ctx, in.Type, d.ID)
		if err != nil {
			return err
		}
		if len(vs) > 0 {
			if err := checkRowVersion(vs[0].RowVersion, in.IfMatch); err != nil {
				return err
			}
		}
		if _, err := handler.SoftDelete(ctx, d.ID, in.Reason); err != nil {
			if errors.Is(err, contentmodel.ErrItemPublished) {
				return problem.New(problem.Conflict, "a released item cannot be deleted; unpublish it first")
			}
			return err
		}
		if err := h.record(ctx, in.Type, d.Owner, audit.Event{Action: ActionContentDeleted, TargetID: d.ID.String(), Reason: in.Reason,
			Metadata: map[string]any{"resource_type": string(rt)}}); err != nil {
			return err
		}
		by := p.UserID
		return h.trash().publishResource(ctx, rt, "deleted", ResourceEventPayload{ResourceType: string(rt), ResourceID: d.ID,
			TenantID: d.Owner.TenantID, DeletedBy: &by, Reason: in.Reason})
	})
}

func (h Docs) recordUsages(ctx context.Context, v dto.DocVersion) error {
	if h.Media == nil {
		return nil
	}
	return h.Media.ReplaceUsages(ctx, write.UsageSet{Owner: v.Owner, ItemType: v.Type, ItemID: v.ItemID, VersionID: v.ID, VersionNo: v.VersionNo,
		AssetIDs: v.MediaRefs})
}

// ---------- validation

func bodyOf(v dto.DocVersion) map[string]json.RawMessage {
	body := map[string]json.RawMessage{}
	_ = json.Unmarshal(v.Body, &body)
	return body
}

// prepare validates a version (shared metadata, then the rules of its type) and fills its derived columns.
func (h Docs) prepare(ctx context.Context, v *dto.DocVersion) error {
	var all contentmodel.Violations
	add := func(f, r string) { all = append(all, contentmodel.Violation{Field: f, Reason: r}) }
	if isNull(v.Title) {
		all = append(all, contentmodel.Violation{Field: "/title", Reason: "is required", Required: true})
	}
	if v.ContentLanguage != "vi" && v.ContentLanguage != "en" {
		add("/content_language", "must be vi or en")
	}
	body := bodyOf(*v)
	for k := range body {
		if !slices.Contains(BodyKeys[v.Type], k) {
			delete(body, k)
		}
	}
	switch v.Type {
	case contentmodel.TypeQuestionSet:
		all = append(all, h.prepareSet(ctx, v, body)...)
	case contentmodel.TypeLesson, contentmodel.TypeLearningMaterial:
		all = append(all, h.prepareLesson(v, body)...)
	case contentmodel.TypeAssessmentBlueprint:
		vs, err := h.prepareBlueprint(ctx, v, body)
		if err != nil {
			return err
		}
		all = append(all, vs...)
	}
	if len(all) > 0 {
		return problemOfViolations(all)
	}
	if v.Type == contentmodel.TypeQuestionSet {
		if err := h.checkItems(ctx, v); err != nil {
			return err
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	v.Body = b
	if err := h.checkMeta(ctx, *v); err != nil {
		return err
	}
	if h.Media != nil {
		return checkAssets(ctx, h.Media, v.MediaRefs)
	}
	if len(v.MediaRefs) > AssetsPerVersionMax {
		return problem.Invalid("/media_refs", problem.SubInvalid, "has more than "+strconv.Itoa(AssetsPerVersionMax)+" media (assets_per_version_max)")
	}
	return nil
}

// checkMeta: the subject exists, the topics and objectives belong to the curriculum version of the content
// (F024-AC-06, AC-10).
func (h Docs) checkMeta(ctx context.Context, v dto.DocVersion) error {
	if v.SubjectID != nil {
		if _, err := h.Catalog.Subject(ctx, *v.SubjectID); err != nil {
			return problem.Invalid("/subject_id", problem.SubInvalid, "is not a subject of the catalog")
		}
	}
	if (len(v.TopicIDs) > 0 || len(v.ObjectiveIDs) > 0) && v.CurriculumVersionID == nil {
		return problem.Invalid("/curriculum_version_id", problem.SubRequired, "topics and objectives need their curriculum version")
	}
	if v.CurriculumVersionID != nil {
		if _, err := h.Curriculum.VersionByID(ctx, *v.CurriculumVersionID); err != nil {
			return problem.Invalid("/curriculum_version_id", problem.SubInvalid, "is not a visible curriculum version")
		}
	}
	return h.checkCurriculumRefs(ctx, v.CurriculumVersionID, v.TopicIDs, v.ObjectiveIDs, "/topic_ids/", "/objective_ids/")
}

func (h Docs) checkCurriculumRefs(ctx context.Context, cv *uuid.UUID, topics, objectives []uuid.UUID, tptr, optr string) error {
	for i, id := range topics {
		t, err := h.Curriculum.Topic(ctx, id)
		if err != nil || cv == nil || t.VersionID != *cv {
			return problem.Invalid(tptr+strconv.Itoa(i), problem.SubInvalid, "must be a topic of the curriculum version")
		}
	}
	for i, id := range objectives {
		o, err := h.Curriculum.Objective(ctx, id)
		if err != nil || cv == nil || o.VersionID != *cv {
			return problem.Invalid(optr+strconv.Itoa(i), problem.SubInvalid, "must be an objective of the curriculum version")
		}
	}
	return nil
}

// prepareSet checks a question set (F024-AC-01, AC-02, AC-03) and fills its items.
func (h Docs) prepareSet(ctx context.Context, v *dto.DocVersion, body map[string]json.RawMessage) contentmodel.Violations {
	raw, ok := body["definition"]
	if !ok {
		return contentmodel.Violations{{Field: "/definition", Reason: "is required", Required: true}}
	}
	if vs := h.Schema.ValidateSetDefinition(raw); len(vs) > 0 {
		return vs
	}
	def, err := questionset.Parse(raw)
	if err != nil {
		return contentmodel.Violations{{Field: "/definition", Reason: "is invalid"}}
	}
	if vs := def.Validate(); len(vs) > 0 {
		return vs
	}
	v.Items = nil
	for i, it := range def.Items {
		pos := it.Position
		if pos == 0 {
			pos = i + 1
		}
		v.Items = append(v.Items, dto.SetItem{Position: pos, QuestionVersionID: it.QuestionVersionID, MaxScore: it.MaxScore})
	}
	return nil
}

// prepareLesson checks the blocks of a lesson or learning material (F024-AC-06, AC-07) and derives the worked
// example ids and the media of the blocks.
func (h Docs) prepareLesson(v *dto.DocVersion, body map[string]json.RawMessage) contentmodel.Violations {
	raw, ok := body["blocks"]
	if !ok {
		return contentmodel.Violations{{Field: "/blocks", Reason: "is required", Required: true}}
	}
	if v.Type == contentmodel.TypeLearningMaterial {
		if _, ok := body["material_kind"]; !ok {
			return contentmodel.Violations{{Field: "/material_kind", Reason: "is required", Required: true}}
		}
	}
	var blocks []lesson.Block
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return contentmodel.Violations{{Field: "/blocks", Reason: "must be a list of blocks"}}
	}
	res, vs := lesson.Validate(blocks, lesson.Options{ActivityEnabled: h.ActivityEnabled})
	if len(vs) > 0 {
		return vs
	}
	if vs := h.Schema.ValidateBlocks(raw); len(vs) > 0 {
		return vs
	}
	v.WorkedExampleIDs = res.WorkedExampleIDs()
	refs := MediaRefs(res.MediaRefs, raw)
	v.MediaRefs = refs
	// Objectives of worked examples are part of the version's objectives (same curriculum version).
	for _, w := range res.WorkedExamples {
		for _, lo := range w.ObjectiveIDs {
			if !slices.Contains(v.ObjectiveIDs, lo) {
				v.ObjectiveIDs = append(v.ObjectiveIDs, lo)
			}
		}
	}
	return nil
}

// prepareBlueprint checks a blueprint (F024-AC-10, AC-12): the configured balance default is stored.
func (h Docs) prepareBlueprint(ctx context.Context, v *dto.DocVersion, body map[string]json.RawMessage) (contentmodel.Violations, error) {
	raw, ok := body["definition"]
	if !ok {
		return contentmodel.Violations{{Field: "/definition", Reason: "is required", Required: true}}, nil
	}
	var def map[string]json.RawMessage
	if err := json.Unmarshal(raw, &def); err != nil {
		return contentmodel.Violations{{Field: "/definition", Reason: "must be an object"}}, nil
	}
	var ft, mode string
	_ = json.Unmarshal(body["feedback_timing"], &ft)
	_ = json.Unmarshal(body["result_release"], &mode)
	if _, ok := def["result_release"]; !ok && mode != "" {
		rr := map[string]string{"mode": mode}
		if ft != "" {
			rr["feedback_timing"] = ft
		}
		def["result_release"], _ = json.Marshal(rr)
	}
	merged, _ := json.Marshal(def)
	pdiff := h.MaxMeanPDiff
	if pdiff == 0 {
		pdiff = blueprint.DefaultMaxMeanPDiff
	}
	merged, err := blueprint.WithDefaults(merged, pdiff)
	if err != nil {
		return nil, err
	}
	if vs := h.Schema.ValidateBlueprintDefinition(merged); len(vs) > 0 {
		return vs, nil
	}
	d, err := blueprint.Parse(merged)
	if err != nil {
		return contentmodel.Violations{{Field: "/definition", Reason: "is invalid"}}, nil
	}
	if vs := d.Validate(ft); len(vs) > 0 {
		return vs, nil
	}
	var efficacy bool
	_ = json.Unmarshal(body["efficacy_instrument"], &efficacy)
	if efficacy && (!v.Owner.Platform() || d.Variants == nil || d.Variants.Strategy != blueprint.StrategyFixed) {
		return contentmodel.Violations{{Field: "/efficacy_instrument", Reason: "only platform blueprints with variants.strategy fixed"}}, nil
	}
	body["definition"] = merged
	objectives, topics := d.References()
	if len(objectives) > 0 || len(topics) > 0 {
		if v.CurriculumVersionID == nil {
			return contentmodel.Violations{{Field: "/curriculum_version_id", Reason: "the rules reference objectives or topics", Required: true}}, nil
		}
		if err := h.checkCurriculumRefs(ctx, v.CurriculumVersionID, topics, objectives, "/definition/topic_ids/", "/definition/objective_ids/"); err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) {
				return contentmodel.Violations{{Field: "/definition", Reason: "a rule references a topic or objective outside the curriculum version"}}, nil
			}
			return nil, err
		}
	}
	for i, q := range d.FixedQuestions() {
		if _, err := h.checkQuestion(ctx, v.Owner, q, "/definition/fixed_question_version_ids/"+strconv.Itoa(i)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// checkItems resolves the fixed items of a question set (F024-AC-02).
func (h Docs) checkItems(ctx context.Context, v *dto.DocVersion) error {
	for i := range v.Items {
		qv, err := h.checkQuestion(ctx, v.Owner, v.Items[i].QuestionVersionID, "/definition/items/"+strconv.Itoa(i)+"/question_version_id")
		if err != nil {
			return err
		}
		v.Items[i].QuestionID = qv.QuestionID
	}
	return nil
}

// checkQuestion resolves a referenced question version (F024-AC-02): visible in the scope (own, or platform assigned
// to the tenant); a platform item may not reference tenant questions (422); a platform question not assigned to the
// tenant is 403 RESOURCE_NOT_ASSIGNED; anything else 422 at the pointer.
func (h Docs) checkQuestion(ctx context.Context, o contentmodel.Owner, id uuid.UUID, ptr string) (dto.QuestionVersion, error) {
	qv, err := h.Questions.QuestionVersionByID(ctx, id)
	if err == nil {
		if o.Platform() && !qv.Owner.Platform() {
			return qv, problem.Invalid(ptr, problem.SubInvalid, "platform content cannot reference a tenant question")
		}
		return qv, nil
	}
	if pe, ok := problemOf(err); !ok || pe.Code != problem.NotFound {
		return qv, err
	}
	if !o.Platform() && h.Read != nil {
		ps, err := authz.SystemPlatformScope("learning-reference-check")
		if err != nil {
			return qv, err
		}
		exists := false
		if err := h.Read.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
			_, e := h.Questions.QuestionVersionByID(ctx, id)
			exists = e == nil
			return nil
		}); err != nil {
			return qv, err
		}
		if exists {
			return qv, problem.New(problem.ResourceNotAssigned, "the platform question is not assigned to the tenant")
		}
	}
	return qv, problem.Invalid(ptr, problem.SubInvalid, "is not a question version of the bank")
}

// DocJSON is the CMS view of an item with one version (ContentVersion + the input fields of its type).
func DocJSON(r DocResult, sessionTenant *uuid.UUID) map[string]any {
	d, v := r.Doc, r.Version
	out := map[string]any{"content_type": string(d.Type), "content_id": d.ID, "content_version_id": v.ID, "version": v.VersionNo,
		"status": string(v.Status), "owner_scope": string(d.Owner.Scope), "tenant_id": d.Owner.TenantID, "author_id": v.AuthorID,
		"updated_at": v.UpdatedAt, "content_language": v.ContentLanguage, "title": v.Title,
		"assigned": d.Owner.Platform() && sessionTenant != nil}
	if v.ReviewerID != nil {
		out["reviewer_id"] = *v.ReviewerID
	}
	if v.PublishedAt != nil {
		out["published_at"] = *v.PublishedAt
	}
	if v.TranslationStatus != nil {
		out["translation_status"] = *v.TranslationStatus
	}
	if of := i18ntext.OutdatedFields(docDoc(v), v.ContentLanguage, i18ntext.ParseMeta(v.TranslationMeta)); len(of) > 0 {
		out["outdated_fields"] = of
	}
	if v.AudioExemptReason != nil {
		out["audio_exempt_reason"] = *v.AudioExemptReason
	}
	if v.AudioExemptApprovedBy != nil {
		out["audio_exempt_approved_by"] = *v.AudioExemptApprovedBy
	}
	if d.ClonedFromID != nil {
		out["cloned_from_id"], out["cloned_from_tenant_id"] = *d.ClonedFromID, d.ClonedFromTenantID
	}
	summaryKey := "summary"
	if d.Type == contentmodel.TypeQuestionSet {
		summaryKey = "description"
	}
	if !isNull(v.Summary) {
		out[summaryKey] = v.Summary
	}
	if v.SubjectID != nil {
		out["subject_id"] = *v.SubjectID
	}
	if v.CurriculumVersionID != nil {
		out["curriculum_version_id"] = *v.CurriculumVersionID
	}
	switch d.Type {
	case contentmodel.TypeLesson, contentmodel.TypeLearningMaterial:
		if len(v.GradeLevels) > 0 {
			out["grade_level"] = v.GradeLevels[0]
		}
		if len(v.TopicIDs) > 0 {
			out["topic_id"] = v.TopicIDs[0]
		}
	default:
		out["grade_levels"], out["topic_ids"] = nonNil(v.GradeLevels), nonNil(v.TopicIDs)
	}
	out["objective_ids"] = nonNil(v.ObjectiveIDs)
	for k, raw := range bodyOf(v) {
		out[k] = raw
	}
	return out
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
