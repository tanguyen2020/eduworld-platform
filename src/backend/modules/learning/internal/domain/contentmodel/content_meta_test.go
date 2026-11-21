package contentmodel

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

// F020-AC-06: only LearningContentType (+ curriculum) and the resource types of module learning are accepted.
func TestContentAndResourceTypes(t *testing.T) {
	for _, s := range []string{"curriculum", "question", "question_set", "assessment_blueprint", "lesson", "learning_material",
		"activity"} {
		if _, err := ParseContentType(s); err != nil {
			t.Errorf("%s refused: %v", s, err)
		}
	}
	for _, s := range []string{"", "quest", "media", "written_response", "QUESTION", "learning_path"} {
		var e ErrInvalid
		if _, err := ParseContentType(s); !errors.As(err, &e) || e.Field != "content_type" {
			t.Errorf("%q accepted or wrong error: %v", s, err)
		}
	}
	for _, s := range []string{"curriculum", "lesson", "question", "question_set", "assessment_blueprint", "media", "assignment",
		"activity"} {
		if _, err := ParseResourceType(s); err != nil {
			t.Errorf("resource %s refused: %v", s, err)
		}
	}
	for _, s := range []string{"quest", "item", "tournament_template", "class_template", "course", "learning_path"} {
		if _, err := ParseResourceType(s); err == nil {
			t.Errorf("resource %s of another module accepted", s)
		}
	}
	if r, ok := ResourceOf(TypeQuestionSet); !ok || r != ResourceQuestionSet {
		t.Fatalf("ResourceOf(question_set) = %v %v", r, ok)
	}
	// OQ F030 owner 2026-11-20: activity is assigned, cloned and trashed like a question set.
	if r, ok := ResourceOf(TypeActivity); !ok || r != ResourceActivity {
		t.Fatalf("ResourceOf(activity) = %v %v", r, ok)
	}
	if c, ok := ContentOf(ResourceCurriculum); !ok || c != TypeCurriculum {
		t.Fatalf("ContentOf(curriculum) = %v %v", c, ok)
	}
	if _, ok := ContentOf(ResourceMedia); ok {
		t.Fatal("media has no content versions")
	}
}

func validMeta() ContentMeta {
	topic, cv := uuid.New(), uuid.New()
	tenant := uuid.New()
	return ContentMeta{GradeLevels: []int{3}, SubjectID: uuid.New(), TopicID: &topic, ObjectiveIDs: []uuid.UUID{uuid.New()},
		Difficulty: ptr(2), VersionNo: 1, Status: StatusDraft, AuthorID: uuid.New(),
		Owner: Owner{Scope: OwnerTenant, TenantID: &tenant}, ContentLanguage: LangVI, CurriculumVersionID: &cv}
}

func fields(err error) []string {
	var v Violations
	if !errors.As(err, &v) {
		return nil
	}
	out := make([]string, len(v))
	for i, x := range v {
		out[i] = x.Field
	}
	return out
}

// F020-AC-07: missing mandatory metadata is reported field by field, by content type and status.
func TestContentMetaValidate(t *testing.T) {
	if err := validMeta().Validate(TypeQuestion); err != nil {
		t.Fatalf("valid meta: %v", err)
	}
	empty := ContentMeta{}
	want := []string{"grade_levels", "subject_id", "topic_id", "objective_ids", "difficulty", "version", "status", "author_id",
		"owner_scope", "content_language", "curriculum_version_id"}
	if got := fields(empty.Validate(TypeQuestion)); !slices.Equal(got, want) {
		t.Fatalf("empty question: %v, want %v", got, want)
	}
	// lesson: no objectives/difficulty required.
	m := validMeta()
	m.ObjectiveIDs, m.Difficulty = nil, nil
	if err := m.Validate(TypeLesson); err != nil {
		t.Fatalf("lesson without objectives: %v", err)
	}
	if got := fields(m.Validate(TypeQuestionSet)); !slices.Equal(got, []string{"objective_ids"}) {
		t.Fatalf("question set without objectives: %v", got)
	}
	// approved needs a reviewer; published also published_at.
	m = validMeta()
	m.Status = StatusApproved
	if got := fields(m.Validate(TypeQuestion)); !slices.Equal(got, []string{"reviewer_id"}) {
		t.Fatalf("approved: %v", got)
	}
	m.Status, m.ReviewerID = StatusPublished, ptr(uuid.New())
	if got := fields(m.Validate(TypeQuestion)); !slices.Equal(got, []string{"published_at"}) {
		t.Fatalf("published: %v", got)
	}
	m.PublishedAt = ptr(time.Now())
	if err := m.Validate(TypeQuestion); err != nil {
		t.Fatalf("published complete: %v", err)
	}
	// invalid values.
	m = validMeta()
	m.GradeLevels, m.Difficulty, m.ContentLanguage = []int{0, 10}, ptr(6), "fr"
	m.Tags = []string{""}
	if got := fields(m.Validate(TypeQuestion)); !slices.Equal(got, []string{"grade_levels", "difficulty", "content_language", "tags"}) {
		t.Fatalf("invalid values: %v", got)
	}
	var v Violations
	if err := empty.Validate("quest"); !errors.As(err, &v) || v[0].Field != "content_type" {
		t.Fatalf("unknown type: %v", err)
	}
}

// F020-AC-08: (owner_scope = 'platform') = (tenant_id IS NULL).
func TestOwnerScopeTenantInvariant(t *testing.T) {
	tenant := uuid.New()
	cases := []struct {
		scope  OwnerScope
		tenant *uuid.UUID
		ok     bool
	}{
		{OwnerPlatform, nil, true}, {OwnerTenant, &tenant, true},
		{OwnerPlatform, &tenant, false}, {OwnerTenant, nil, false}, {OwnerTenant, ptr(uuid.Nil), false}, {"school", &tenant, false},
	}
	for _, c := range cases {
		_, err := NewOwner(c.scope, c.tenant)
		if (err == nil) != c.ok {
			t.Errorf("NewOwner(%s, %v) = %v, want ok=%v", c.scope, c.tenant, err, c.ok)
		}
	}
	o, _ := NewOwner(OwnerTenant, &tenant)
	if !o.WritableBy(tenant) || o.WritableBy(uuid.New()) {
		t.Fatal("tenant content writable only by its tenant")
	}
	p, _ := NewOwner(OwnerPlatform, nil)
	if p.WritableBy(tenant) {
		t.Fatal("platform content is read-only for tenants (F020-AC-18)")
	}
}

// Workflow of CONTENT_MODEL §6 (data-model §2) and deletion rules (ADR-0028, CONTENT_MODEL §5 rule 10).
func TestLifecycle(t *testing.T) {
	author, reviewer := uuid.New(), uuid.New()
	steps := []struct {
		from   Status
		action Action
		in     TransitionInput
		to     Status
		err    error
	}{
		{StatusDraft, ActionSubmitReview, TransitionInput{}, StatusInReview, nil},
		{StatusInReview, ActionApprove, TransitionInput{Actor: author, Author: author}, StatusInReview, ErrReviewerIsAuthor},
		{StatusInReview, ActionApprove, TransitionInput{Actor: reviewer, Author: author}, StatusApproved, nil},
		{StatusInReview, ActionReject, TransitionInput{}, StatusInReview, ErrReasonRequired},
		{StatusInReview, ActionReject, TransitionInput{Note: "sai đáp án"}, StatusRejected, nil},
		{StatusRejected, ActionRevise, TransitionInput{}, StatusDraft, nil},
		{StatusApproved, ActionRevise, TransitionInput{}, StatusDraft, nil},
		{StatusApproved, ActionPublish, TransitionInput{}, StatusPublished, nil},
		{StatusPublished, ActionArchive, TransitionInput{IsCurrent: true}, StatusPublished, ErrCurrentVersion},
		{StatusPublished, ActionArchive, TransitionInput{}, StatusArchived, nil},
		{StatusDraft, ActionPublish, TransitionInput{}, StatusDraft, ErrTransition},
		{StatusArchived, ActionRevise, TransitionInput{}, StatusArchived, ErrTransition},
	}
	for _, s := range steps {
		to, err := Transition(s.from, s.action, s.in)
		if to != s.to || !errors.Is(err, s.err) {
			t.Errorf("%s --%s--> %s %v, want %s %v", s.from, s.action, to, err, s.to, s.err)
		}
	}
	for _, s := range []Status{StatusDraft, StatusRejected} {
		if CanDeleteVersion(s) != nil {
			t.Errorf("%s version must be deletable", s)
		}
	}
	for _, s := range []Status{StatusInReview, StatusApproved, StatusPublished, StatusArchived} {
		if CanDeleteVersion(s) == nil {
			t.Errorf("%s version must not be deletable", s)
		}
	}
	if CanDeleteItem(nil) != nil || CanDeleteItem(ptr(uuid.New())) == nil {
		t.Fatal("an item is deletable only without a current release")
	}
	if !StatusPublished.Immutable() || StatusDraft.Immutable() {
		t.Fatal("Immutable")
	}
}
