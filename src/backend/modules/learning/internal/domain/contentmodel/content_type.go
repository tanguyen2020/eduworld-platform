// Package contentmodel is the shared model of learning content (F020, CONTENT_MODEL §2–§6): content and
// resource types, owner scope, the mandatory metadata of a version, the workflow state machine and the
// deletion rules. Every content feature (F021–F027) builds on it; it does no I/O.
package contentmodel

import "slices"

// ContentType is a learning content type with versions and releases: LearningContentType of
// specs/schemas/common/enums.schema.json plus curriculum (its unit of version is CurriculumVersion,
// CONTENT_MODEL §2).
type ContentType string

// Content types (CONTENT_MODEL §2, enums.schema.json#/$defs/LearningContentType).
const (
	TypeCurriculum          ContentType = "curriculum"
	TypeQuestion            ContentType = "question"
	TypeQuestionSet         ContentType = "question_set"
	TypeAssessmentBlueprint ContentType = "assessment_blueprint"
	TypeLesson              ContentType = "lesson"
	TypeLearningMaterial    ContentType = "learning_material"
	TypeLearningPath        ContentType = "learning_path"
	TypeActivity            ContentType = "activity"
)

var contentTypes = []ContentType{TypeCurriculum, TypeQuestion, TypeQuestionSet, TypeAssessmentBlueprint, TypeLesson,
	TypeLearningMaterial, TypeLearningPath, TypeActivity}

// ContentTypes returns the accepted content types.
func ContentTypes() []ContentType { return slices.Clone(contentTypes) }

// ParseContentType accepts only the registered content types (F020-AC-06).
func ParseContentType(s string) (ContentType, error) {
	if slices.Contains(contentTypes, ContentType(s)) {
		return ContentType(s), nil
	}
	return "", ErrInvalid{Field: "content_type", Reason: "is not a learning content type"}
}

// ResourceType is the ResourceType of the trash, assignment and clone operations
// (specs/openapi/modules/common/internal-common.v1.yaml) restricted to the resources owned by module
// learning (DATABASE_DESIGN §2.17: curriculum, lesson, question, question set, blueprint, plus media and
// the LMS resources).
type ResourceType string

// Resource types owned by module learning.
const (
	ResourceCurriculum          ResourceType = "curriculum"
	ResourceLesson              ResourceType = "lesson"
	ResourceQuestion            ResourceType = "question"
	ResourceQuestionSet         ResourceType = "question_set"
	ResourceAssessmentBlueprint ResourceType = "assessment_blueprint"
	ResourceMedia               ResourceType = "media"
	ResourceAssignment          ResourceType = "assignment"
	ResourceCourse              ResourceType = "course"
	ResourceLearningPath        ResourceType = "learning_path"
)

var resourceTypes = []ResourceType{ResourceCurriculum, ResourceLesson, ResourceQuestion, ResourceQuestionSet,
	ResourceAssessmentBlueprint, ResourceMedia, ResourceAssignment, ResourceCourse, ResourceLearningPath}

// ParseResourceType accepts only the resource types owned by module learning (F020-AC-06).
func ParseResourceType(s string) (ResourceType, error) {
	if slices.Contains(resourceTypes, ResourceType(s)) {
		return ResourceType(s), nil
	}
	return "", ErrInvalid{Field: "resource_type", Reason: "is not a resource type of module learning"}
}

// ResourceOf maps a content type to its ResourceType; ok is false for content types that are not
// trash/assignment/clone resources (learning_material, activity).
func ResourceOf(t ContentType) (ResourceType, bool) {
	switch t {
	case TypeCurriculum, TypeQuestion, TypeQuestionSet, TypeAssessmentBlueprint, TypeLesson, TypeLearningPath:
		return ResourceType(t), true
	}
	return "", false
}

// ContentOf maps a resource type to its content type; ok is false for resources without versions
// (media, assignment, course).
func ContentOf(r ResourceType) (ContentType, bool) {
	t := ContentType(r)
	if slices.Contains(contentTypes, t) {
		return t, true
	}
	return "", false
}

// ErrInvalid is a rule violation on one field (mapped to 422 VALIDATION_FAILED).
type ErrInvalid struct{ Field, Reason string }

func (e ErrInvalid) Error() string { return "contentmodel: " + e.Field + " " + e.Reason }
