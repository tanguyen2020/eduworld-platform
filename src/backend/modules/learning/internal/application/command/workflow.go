package command

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/readiness"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/workflow"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Events of the review workflow (specs/events/payloads/learning/content.version_*.v1, owner F025).
const (
	EventContentVersionSubmitted = "content.version_submitted"
	EventContentVersionApproved  = "content.version_approved"
	EventContentVersionRejected  = "content.version_rejected"
)

// ContentReviewSLA is content_review_sla_days (CONFIG_DEFAULTS, owner ADMIN_PORTAL §4: 3 days [Proposed]) until module
// learning reads platform settings (F015 projection): due_at of a new review task.
const ContentReviewSLA = 3 * 24 * time.Hour

// WorkflowMetrics counts transitions (eduworld_content_transitions_total{content_type,action,result}); nil = not counted.
type WorkflowMetrics interface {
	Transition(contentType, action, result string)
}

// Workflow holds the collaborators of the review workflow of every learning content type (F025 US1, US3).
type Workflow struct {
	UoW       write.UnitOfWork
	Status    write.VersionStatusStore
	Reviews   write.ReviewStore
	Questions write.QuestionStore
	Docs      write.DocStore
	// Schema re-validates the JSON Schema of a question on submit; nil skips it (drafts are validated on save).
	Schema port.QuestionSchema
	// Media reads the status of the referenced assets (MEDIA_NOT_READY); nil skips the media rule (no media pipeline).
	Media   write.MediaStore
	Guard   readiness.Guard
	Events  write.EventPublisher
	Audit   write.Auditor
	Clock   func() time.Time
	Metrics WorkflowMetrics
	SLA     time.Duration
}

func (h Workflow) now() time.Time { return now(h.Clock) }

func (h Workflow) trash() TrashDeps {
	return TrashDeps{UoW: h.UoW, Events: h.Events, Audit: h.Audit, Clock: h.Clock}
}

// FieldComment is a reviewer comment on one field of a rejected version (workflow.FieldComment, F025 T046).
type FieldComment = workflow.FieldComment

// TransitionContentInput is the request of transitionContentVersion.
type TransitionContentInput struct {
	Type                       string
	ContentID                  uuid.UUID
	VersionNo                  int
	IfMatch                    int64
	Action                     string
	Note                       string
	AudioExemptionAcknowledged bool
	// FieldComments are the per-field comments of a reject (F025 T046, OQ F054-1).
	FieldComments []workflow.FieldComment
}

// WorkflowTypes are the content types with a workflow in this release (activity: F030; no learning_path, ADR-0095).
var WorkflowTypes = []contentmodel.ContentType{contentmodel.TypeQuestion, contentmodel.TypeQuestionSet, contentmodel.TypeLesson,
	contentmodel.TypeLearningMaterial, contentmodel.TypeAssessmentBlueprint, contentmodel.TypeActivity}

// ActionPermission is the permission of a workflow action on a content type (plan.md "Module và tầng").
func ActionPermission(t contentmodel.ContentType, a contentmodel.Action) authz.Permission {
	review := a == contentmodel.ActionApprove || a == contentmodel.ActionReject
	switch {
	case t == contentmodel.TypeActivity && review:
		return authz.PermLearningContentActivityReview
	case t == contentmodel.TypeActivity:
		return authz.PermLearningContentActivityWrite
	case review:
		return authz.PermLearningContentReview
	case a == contentmodel.ActionPublish || a == contentmodel.ActionArchive:
		return authz.PermLearningContentPublish
	}
	return authz.PermLearningContentWrite
}

// Subject is a content version under workflow: its row, the item owner and release pointer, and the document checked
// by the readiness guard.
type Subject struct {
	Type              contentmodel.ContentType
	ItemID            uuid.UUID
	VersionID         uuid.UUID
	VersionNo         int
	Status            contentmodel.Status
	AuthorID          uuid.UUID
	ReviewerID        *uuid.UUID
	Owner             contentmodel.Owner
	RowVersion        int64
	CurrentVersionID  *uuid.UUID
	GradeLevels       []int
	ContentLanguage   string
	Doc               json.RawMessage
	Meta              contentmodel.Violations
	MediaRefs         []uuid.UUID
	Title             string
	AudioExemptReason string
	ExemptApprovedBy  *uuid.UUID
	TranslationStatus *string
	OutdatedFields    []string
	PublishedAt       *time.Time
	UpdatedAt         time.Time
	// F026: release data of the item and the version.
	ItemRowVersion int64
	SnapshotDoc    json.RawMessage
	SnapshotSHA    *string
	QuestionType   string
	ExamOnly       bool
	SubjectID      *uuid.UUID
	ObjectiveIDs   []uuid.UUID
	ActivityType   string
	// Children are the question versions a question set or blueprint references (publish checks of F026).
	Children []uuid.UUID
}

// IsCurrent reports the release pointer of the item.
func (s Subject) IsCurrent() bool {
	return s.CurrentVersionID != nil && *s.CurrentVersionID == s.VersionID
}

// Load returns a version of an item of type t in the scope of ctx (404 when invisible).
func (h Workflow) Load(ctx context.Context, t contentmodel.ContentType, itemID uuid.UUID, versionNo int) (Subject, error) {
	if t == contentmodel.TypeQuestion {
		q, err := h.Questions.Question(ctx, itemID)
		if err != nil {
			return Subject{}, err
		}
		v, err := h.Questions.QuestionVersion(ctx, itemID, versionNo)
		if err != nil {
			return Subject{}, err
		}
		return h.questionSubject(q, v), nil
	}
	d, err := h.Docs.Doc(ctx, t, itemID)
	if err != nil {
		return Subject{}, err
	}
	v, err := h.Docs.DocVersion(ctx, t, itemID, versionNo)
	if err != nil {
		return Subject{}, err
	}
	return docSubject(d, v), nil
}

// questionDoc is the document of a question version (texts, no answers) and its authoring render payload.
func questionDoc(v dto.QuestionVersion) (json.RawMessage, json.RawMessage) {
	return dto.QuestionDoc(v, false), dto.QuestionRender(v)
}

// docDoc is the document of a composed content version.
func docDoc(v dto.DocVersion) json.RawMessage { return dto.DocDoc(v) }

// translation computes translation_status and translation_meta of a draft save from the previous document (F025-AC-19).
func translation(prev, next json.RawMessage, lang string, prevMeta json.RawMessage, at time.Time) (*string, json.RawMessage) {
	meta := i18ntext.Track(prev, next, lang, i18ntext.ParseMeta(prevMeta), at)
	st := string(i18ntext.ComputeStatus(next, lang, meta))
	return &st, meta.JSON()
}

func (h Workflow) questionSubject(q dto.Question, v dto.QuestionVersion) Subject {
	b, render := questionDoc(v)
	s := Subject{Type: contentmodel.TypeQuestion, ItemID: q.ID, VersionID: v.ID, VersionNo: v.VersionNo, Status: v.Status, AuthorID: v.AuthorID,
		ReviewerID: v.ReviewerID, Owner: q.Owner, RowVersion: v.RowVersion, CurrentVersionID: q.CurrentVersionID, GradeLevels: v.GradeLevels,
		ContentLanguage: v.ContentLanguage, Doc: b, Title: titleOf(v.Stem, v.ContentLanguage), AudioExemptReason: deref(v.AudioExemptReason),
		ExemptApprovedBy: v.AudioExemptApprovedBy, TranslationStatus: v.TranslationStatus, PublishedAt: v.PublishedAt, UpdatedAt: v.UpdatedAt,
		ItemRowVersion: q.RowVersion, SnapshotDoc: dto.QuestionSnapshotDoc(v), SnapshotSHA: v.SnapshotSHA256, QuestionType: v.Type, ExamOnly: v.ExamOnly,
		ObjectiveIDs: v.ObjectiveIDs}
	subject := v.SubjectID
	s.SubjectID = &subject
	s.OutdatedFields = i18ntext.OutdatedFields(b, v.ContentLanguage, i18ntext.ParseMeta(v.TranslationMeta))
	docs := append([]json.RawMessage{v.Stem, v.RenderPayload, v.OptionFeedback, v.Explanation}, hintsOf(v.Hints)...)
	s.MediaRefs = MediaRefs(v.MediaRefs, docs...)
	topic, cv, difficulty := v.TopicID, v.CurriculumVersionID, v.Difficulty
	meta := contentmodel.ContentMeta{GradeLevels: v.GradeLevels, SubjectID: v.SubjectID, TopicID: &topic, ObjectiveIDs: v.ObjectiveIDs,
		Difficulty: &difficulty, VersionNo: v.VersionNo, Status: contentmodel.StatusDraft, AuthorID: v.AuthorID, Owner: v.Owner,
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
				s.Meta = append(s.Meta, x)
			}
		}
	}
	if h.Schema != nil {
		var hints []json.RawMessage
		_ = json.Unmarshal(v.Hints, &hints)
		s.Meta = append(s.Meta, h.Schema.ValidateAuthoring(port.QuestionDoc{Type: v.Type, Stem: v.Stem, RenderPayload: render,
			AnswerKey: v.AnswerKey, ScoringRule: v.ScoringRule, Explanation: v.Explanation, Hints: hints})...)
	}
	return s
}

// docSubject: the metadata required to leave draft (CONTENT_MODEL §3) — grade levels and subject for every composed
// type, learning objectives for a question set; a draft of F024 may still lack them (F025-AC-02).
func docSubject(d dto.Doc, v dto.DocVersion) Subject {
	b := docDoc(v)
	s := Subject{Type: v.Type, ItemID: d.ID, VersionID: v.ID, VersionNo: v.VersionNo, Status: v.Status, AuthorID: v.AuthorID,
		ReviewerID: v.ReviewerID, Owner: d.Owner, RowVersion: v.RowVersion, CurrentVersionID: d.CurrentVersionID, GradeLevels: v.GradeLevels,
		ContentLanguage: v.ContentLanguage, Doc: b, MediaRefs: v.MediaRefs, Title: titleOf(v.Title, v.ContentLanguage),
		AudioExemptReason: deref(v.AudioExemptReason), ExemptApprovedBy: v.AudioExemptApprovedBy, TranslationStatus: v.TranslationStatus,
		PublishedAt: v.PublishedAt, UpdatedAt: v.UpdatedAt, ItemRowVersion: d.RowVersion, SnapshotDoc: b, SnapshotSHA: v.SnapshotSHA256,
		SubjectID: v.SubjectID, ObjectiveIDs: v.ObjectiveIDs}
	if v.Type == contentmodel.TypeActivity {
		s.ActivityType = bodyString(v, "activity_type")
	}
	s.OutdatedFields = i18ntext.OutdatedFields(b, v.ContentLanguage, i18ntext.ParseMeta(v.TranslationMeta))
	if len(v.GradeLevels) == 0 {
		s.Meta = append(s.Meta, contentmodel.Violation{Field: "/grade_levels", Required: true, Reason: "is required before review"})
	}
	if v.SubjectID == nil {
		s.Meta = append(s.Meta, contentmodel.Violation{Field: "/subject_id", Required: true, Reason: "is required before review"})
	}
	if v.Type == contentmodel.TypeQuestionSet && len(v.ObjectiveIDs) == 0 {
		s.Meta = append(s.Meta, contentmodel.Violation{Field: "/objective_ids", Required: true, Reason: "is required for question_set"})
	}
	for _, it := range v.Items {
		s.Children = append(s.Children, it.QuestionVersionID)
	}
	return s
}

// titleOf is the first text of a rich content or the text of an i18n title in lang, cut to 255 characters (queue view).
func titleOf(raw json.RawMessage, lang string) string {
	for _, t := range i18ntext.Texts(raw, lang) {
		s := t.Values[lang]
		if s == "" {
			s = t.Values[i18ntext.Other(lang)]
		}
		if s != "" {
			return truncate(strings.TrimSpace(s), 255)
		}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return truncate(s, 255)
	}
	var m map[string]string
	if json.Unmarshal(raw, &m) == nil {
		if m[lang] != "" {
			return truncate(m[lang], 255)
		}
		return truncate(m[i18ntext.Other(lang)], 255)
	}
	return ""
}

// Readiness runs PublishReadinessGuard on s (F025-AC-17): the same checks and problem codes for submit_review and the
// publish of F026. It returns the non-blocking warnings.
func (h Workflow) Readiness(ctx context.Context, s Subject) ([]readiness.Violation, error) {
	sub := readiness.Subject{Type: s.Type, GradeLevels: s.GradeLevels, ContentLanguage: s.ContentLanguage, Doc: s.Doc, Meta: s.Meta,
		AudioExemptReason: s.AudioExemptReason}
	if h.Media != nil && len(s.MediaRefs) > 0 {
		found, err := h.Media.Assets(ctx, s.MediaRefs)
		if err != nil {
			return nil, err
		}
		sub.Media = map[uuid.UUID]string{}
		for _, id := range s.MediaRefs {
			sub.Media[id] = ""
			if a, ok := found[id]; ok {
				sub.Media[id] = string(a.Status)
			}
		}
	}
	guard := h.Guard
	if len(guard.Rules) == 0 {
		guard = readiness.Default()
	}
	res := guard.Check(sub)
	if res.OK() {
		return res.Warnings, nil
	}
	return res.Warnings, ReadinessProblem(res)
}

// ReadinessProblem maps a failed guard to its problem: errors[] with the JSON pointers (VALIDATION_FAILED, AUDIO_REQUIRED,
// TRANSLATION_INCOMPLETE) or asset_ids (MEDIA_NOT_READY).
func ReadinessProblem(res readiness.Result) error {
	pe := &problem.Error{Code: problem.Code(res.Code)}
	switch res.Code {
	case readiness.CodeMediaNotReady:
		pe.Detail = "referenced media are not ready"
		assets := make([]string, 0, len(res.Violations))
		for _, v := range res.Violations {
			assets = append(assets, v.AssetID.String())
			pe.AddField("/media_refs", problem.SubInvalid, v.AssetID.String()+" "+v.Reason)
		}
		return pe.With("asset_ids", assets)
	case readiness.CodeAudioRequired:
		pe.Detail = "audio is required for grades 1–3"
	case readiness.CodeTranslationIncomplete:
		pe.Detail = "the content_language text is missing"
	default:
		pe.Detail = "request validation failed"
	}
	for _, v := range res.Violations {
		sub := problem.SubInvalid
		if v.Required {
			sub = problem.SubRequired
		}
		pe.AddField(v.Pointer, sub, truncate(v.Reason, 500))
	}
	return pe
}

// workflowErr maps the domain errors of a review action.
func workflowErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, workflow.ErrSelfReview):
		return denied(ctx, "self_review")
	case errors.Is(err, workflow.ErrNoteRequired):
		return problem.Invalid("note", problem.SubRequired, "reject needs a note")
	case errors.Is(err, workflow.ErrNoteTooLong):
		return problem.Invalid("note", problem.SubMaxLength, "is longer than 2000 characters")
	case errors.Is(err, workflow.ErrExemptionNotAcknowledged):
		return problem.Invalid("audio_exemption_acknowledged", problem.SubRequired, "the audio exemption must be acknowledged")
	case errors.Is(err, workflow.ErrInvalidTransition):
		return problem.New(problem.InvalidStateTransition, "the action is not allowed from the current status")
	}
	return err
}

// VersionEventPayload is the payload of content.version_submitted/approved/rejected (v1).
type VersionEventPayload struct {
	ContentType                string     `json:"content_type"`
	ContentID                  uuid.UUID  `json:"content_id"`
	ContentVersionID           uuid.UUID  `json:"content_version_id"`
	Version                    int        `json:"version"`
	TenantID                   *uuid.UUID `json:"tenant_id"`
	ReviewLane                 string     `json:"review_lane"`
	TaskID                     uuid.UUID  `json:"task_id"`
	AuthorID                   uuid.UUID  `json:"author_id"`
	ReviewerID                 *uuid.UUID `json:"reviewer_id,omitempty"`
	AudioExemptionAcknowledged *bool      `json:"audio_exemption_acknowledged,omitempty"`
	Note                       *string    `json:"note,omitempty"`
}

var transitionEvents = map[contentmodel.Action]string{
	contentmodel.ActionSubmitReview: EventContentVersionSubmitted,
	contentmodel.ActionApprove:      EventContentVersionApproved,
	contentmodel.ActionReject:       EventContentVersionRejected,
}

// schoolOf is the school of the author's write grant when every such grant is limited to one school (review tasks of a
// school are reviewed by the reviewers of that school, F025-AC-12) [Proposed].
func schoolOf(p authz.Principal, perm authz.Permission) *uuid.UUID {
	var school *uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, perm) {
			continue
		}
		if g.SchoolID == nil || (school != nil && *school != *g.SchoolID) {
			return nil
		}
		school = ptrClone(g.SchoolID)
	}
	return school
}

// Transition applies a review action to a version of any learning content type (F025-AC-01…11, AC-15, AC-16): the
// permission of the action, the owner rule (assigned platform content is read-only for a tenant, another tenant's
// content does not exist), If-Match, the readiness guard on submit_review, the review task of the lane, one
// content_transitions row, the audit record and the outbox event in one transaction. Attempts, evaluations and learning
// events are never touched (FR-CMS-007).
func (h Workflow) Transition(ctx context.Context, in TransitionContentInput) (out dto.ContentVersionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.TransitionContentVersion")
	action := contentmodel.Action(in.Action)
	defer func() {
		h.count(in.Type, in.Action, err)
		done(err)
	}()
	t, err := contentmodel.ParseContentType(in.Type)
	if err != nil || !slices.Contains(WorkflowTypes, t) {
		return out, problem.Invalid("content_type", problem.SubInvalid, "has no workflow in this release")
	}
	if !workflow.Supported(action) {
		return out, problem.New(problem.InvalidStateTransition, "the action is not a workflow action; publish has its own endpoint")
	}
	if utf8.RuneCountInString(in.Note) > workflow.MaxNoteLen {
		return out, problem.Invalid("note", problem.SubMaxLength, "is longer than 2000 characters")
	}
	if vs := workflow.CheckFieldComments(action, in.FieldComments); len(vs) > 0 {
		return out, problemOfViolations(vs)
	}
	var comments json.RawMessage
	if len(in.FieldComments) > 0 {
		if comments, err = json.Marshal(in.FieldComments); err != nil {
			return out, err
		}
	}
	perm := ActionPermission(t, action)
	p, err := authorize(ctx, perm)
	if err != nil {
		return out, err
	}
	key := ""
	if pend, ok := idempotency.PendingFrom(ctx); ok {
		key = pend.Key.Key.String()
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Load(ctx, t, in.ContentID, in.VersionNo)
		if err != nil {
			return err
		}
		if err := writable(ctx, p, perm, s.Owner); err != nil {
			return err
		}
		task, hasTask, err := h.Reviews.OpenTask(ctx, s.VersionID)
		if err != nil {
			return err
		}
		// A decision needs a grant covering the task's school; a tenant-wide task (no school) needs a grant without a
		// school limit — a school reviewer only decides on its schools' tasks (F025 [Proposed]), even though a school
		// grant covers the content itself (scope.RequireContent).
		if hasTask && (action == contentmodel.ActionApprove || action == contentmodel.ActionReject) {
			if err := authz.Require(ctx, p, perm, authz.Resource{TenantID: s.Owner.TenantID, SchoolID: task.SchoolID}); err != nil {
				return err
			}
		}
		if err := checkRowVersion(s.RowVersion, in.IfMatch); err != nil {
			return err
		}
		res, err := workflow.Apply(workflow.Version{Status: s.Status, Author: s.AuthorID, AudioExemptReason: s.AudioExemptReason, IsCurrent: s.IsCurrent()},
			workflow.Request{Action: action, Actor: p.UserID, Note: in.Note, AudioExemptionAcknowledged: in.AudioExemptionAcknowledged})
		if err != nil {
			return workflowErr(ctx, err)
		}
		var warnings []readiness.Violation
		if action == contentmodel.ActionSubmitReview {
			if warnings, err = h.Readiness(ctx, s); err != nil {
				return err
			}
		}
		at := h.now()
		change := write.StatusChange{Type: t, VersionID: s.VersionID, IfRow: in.IfMatch, Status: res.To, ClearReview: action == contentmodel.ActionRevise}
		if res.Reviewer {
			change.ReviewerID = &p.UserID
		}
		if res.ExemptionApproved {
			change.ExemptApprovedBy = &p.UserID
		}
		if _, err := h.Status.SetVersionStatus(ctx, change); err != nil {
			return err
		}
		switch res.Task {
		case workflow.TaskOpen:
			sla := h.SLA
			if sla == 0 {
				sla = ContentReviewSLA
			}
			due := at.Add(sla)
			task = write.ReviewTask{ID: ids.New(), Owner: s.Owner, Lane: workflow.LaneOf(s.Owner), ContentType: t, ContentID: s.ItemID,
				ContentVersionID: s.VersionID, VersionNo: s.VersionNo, Title: s.Title, SchoolID: schoolOf(p, perm), AuthorID: s.AuthorID,
				Status: workflow.TaskOpen, SubmittedAt: at, DueAt: &due}
			if err := h.Reviews.InsertTask(ctx, task); err != nil {
				return err
			}
		case workflow.TaskApproved, workflow.TaskRejected:
			if hasTask {
				task.Status, task.ResolvedAt, task.ResolvedBy, task.Comments = res.Task, &at, &p.UserID, comments
				if err := h.Reviews.ResolveTask(ctx, task); err != nil {
					return err
				}
			}
		}
		if err := h.Reviews.InsertTransition(ctx, write.Transition{Owner: s.Owner, ContentType: t, ContentID: s.ItemID, ContentVersionID: s.VersionID,
			Action: action, From: res.From, To: res.To, ActorID: p.UserID, Note: in.Note, FieldComments: comments, IdempotencyKey: key, At: at}); err != nil {
			return err
		}
		meta := map[string]any{"content_version_id": s.VersionID.String(), "version": s.VersionNo, "from_status": string(res.From),
			"to_status": string(res.To), "review_lane": string(workflow.LaneOf(s.Owner))}
		if res.ExemptionApproved {
			meta["audio_exemption_acknowledged"] = true
		}
		if err := h.trash().recordFor(ctx, s.Owner.TenantID, audit.Event{Action: transitionAudit[action], TargetType: string(t),
			TargetID: s.ItemID.String(), Result: audit.ResultSuccess, Reason: truncate(in.Note, 500), Metadata: meta}); err != nil {
			return err
		}
		if action == contentmodel.ActionArchive {
			pl := map[string]any{"item_type": string(t), "item_id": s.ItemID, "version_id": s.VersionID, "version": s.VersionNo,
				"owner_scope": string(s.Owner.Scope), "tenant_id": s.Owner.TenantID, "archived_by": p.UserID}
			if in.Note != "" {
				pl["note"] = in.Note
			}
			e := write.Event{Type: EventContentVersionArchived, SubjectType: string(t), SubjectID: s.VersionID.String(), Payload: pl}
			if s.Owner.TenantID == nil {
				e.Platform = true
			} else {
				e.TenantID = ptrClone(s.Owner.TenantID)
			}
			if err := h.Events.Publish(ctx, e); err != nil {
				return err
			}
		}
		if typ, ok := transitionEvents[action]; ok {
			if err := h.publish(ctx, typ, s, task, res, p.UserID, in.Note); err != nil {
				return err
			}
		}
		if s, err = h.Load(ctx, t, in.ContentID, in.VersionNo); err != nil {
			return err
		}
		out = ContentVersionViewOf(s, p.TenantID, warnings)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func (h Workflow) publish(ctx context.Context, typ string, s Subject, task write.ReviewTask, res workflow.Result, actor uuid.UUID, note string) error {
	pl := VersionEventPayload{ContentType: string(s.Type), ContentID: s.ItemID, ContentVersionID: s.VersionID, Version: s.VersionNo,
		TenantID: ptrClone(s.Owner.TenantID), ReviewLane: string(workflow.LaneOf(s.Owner)), TaskID: task.ID, AuthorID: s.AuthorID}
	if typ != EventContentVersionSubmitted {
		pl.ReviewerID = &actor
	}
	if res.ExemptionApproved {
		ack := true
		pl.AudioExemptionAcknowledged = &ack
	}
	if typ == EventContentVersionRejected {
		n := note
		pl.Note = &n
	}
	e := write.Event{Type: typ, SubjectType: string(s.Type), SubjectID: s.VersionID.String(), Payload: pl}
	if s.Owner.TenantID == nil {
		e.Platform = true
	} else {
		e.TenantID = ptrClone(s.Owner.TenantID)
	}
	return h.Events.Publish(ctx, e)
}

func (h Workflow) count(contentType, action string, err error) {
	if h.Metrics == nil {
		return
	}
	result := "success"
	var pe *problem.Error
	if err != nil {
		result = "error"
		if errors.As(err, &pe) && problem.Status(pe.Code) < 500 {
			result = "rejected"
		}
	}
	h.Metrics.Transition(contentType, action, result)
}

// ContentVersionViewOf is schema ContentVersion for the session tenant (warnings of submit_review included).
func ContentVersionViewOf(s Subject, sessionTenant *uuid.UUID, warnings []readiness.Violation) dto.ContentVersionView {
	v := dto.ContentVersionView{ContentType: string(s.Type), ContentID: s.ItemID, ContentVersionID: s.VersionID, Version: s.VersionNo,
		Status: string(s.Status), OwnerScope: string(s.Owner.Scope), TenantID: s.Owner.TenantID, Assigned: s.Owner.TenantID == nil && sessionTenant != nil,
		AuthorID: s.AuthorID, ReviewerID: s.ReviewerID, ContentLanguage: s.ContentLanguage, TranslationStatus: s.TranslationStatus,
		AudioExemptApprovedBy: s.ExemptApprovedBy, UpdatedAt: s.UpdatedAt.UTC(), IsCurrent: s.IsCurrent(), RowVersion: s.RowVersion,
		OutdatedFields: s.OutdatedFields}
	if s.AudioExemptReason != "" {
		r := s.AudioExemptReason
		v.AudioExemptReason = &r
	}
	if s.PublishedAt != nil {
		at := s.PublishedAt.UTC()
		v.PublishedAt = &at
	}
	for _, w := range warnings {
		v.Warnings = append(v.Warnings, dto.WarningView{Field: w.Pointer, Code: string(w.Code), Message: w.Reason})
	}
	return v
}
