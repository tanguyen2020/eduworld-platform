package curriculum

import (
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// TopicNode is the position of a topic in the tree of its version.
type TopicNode struct {
	ID         uuid.UUID
	VersionID  uuid.UUID
	SubjectID  uuid.UUID
	GradeLevel int
	ParentID   *uuid.UUID
}

// CheckParent validates the parent of topic (F021-AC-05): the parent is a topic of the same version, subject and
// grade, and setting it does not create a loop. parents maps every topic of the version to its parent.
func CheckParent(topic TopicNode, parent TopicNode, parents map[uuid.UUID]*uuid.UUID) error {
	if parent.VersionID != topic.VersionID || parent.SubjectID != topic.SubjectID || parent.GradeLevel != topic.GradeLevel {
		return contentmodel.ErrInvalid{Field: "parent_topic_id", Reason: "must be a topic of the same version, subject and grade"}
	}
	if topic.ID != uuid.Nil && parent.ID == topic.ID {
		return contentmodel.ErrInvalid{Field: "parent_topic_id", Reason: "a topic cannot be its own parent"}
	}
	seen := map[uuid.UUID]bool{}
	for cur := &parent.ID; cur != nil; cur = parents[*cur] {
		if topic.ID != uuid.Nil && *cur == topic.ID {
			return contentmodel.ErrInvalid{Field: "parent_topic_id", Reason: "the parent is a descendant of the topic (loop)"}
		}
		if seen[*cur] {
			return contentmodel.ErrInvalid{Field: "parent_topic_id", Reason: "the topic tree has a loop"}
		}
		seen[*cur] = true
	}
	return nil
}
