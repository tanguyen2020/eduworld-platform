package command

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// contentStore answers the content reads of an assignment (the other methods are not used by Assignments.content).
type contentStore struct {
	write.SchoolAssignmentStore
	typ string
}

func (s contentStore) CurrentVersion(_ context.Context, _ string, item uuid.UUID) (*write.VersionAccess, uuid.UUID, error) {
	v := uuid.New()
	return &write.VersionAccess{ItemID: item, VersionID: v, Status: "published", Visible: true, ObjectiveIDs: []uuid.UUID{uuid.New()}}, v, nil
}

func (s contentStore) ActivityType(context.Context, uuid.UUID) (string, error) { return s.typ, nil }

// F041-AC-03: an activity whose type is outside the release (written_response, speaking, project) is refused with 422
// ACTIVITY_TYPE_NOT_SUPPORTED; a quiz is pinned with its objectives.
func TestAssignmentContentActivityType(t *testing.T) {
	for _, typ := range []string{"written_response", "speaking", "project"} {
		h := Assignments{Store: contentStore{typ: typ}}
		a := write.AssignmentRow{ContentType: "activity", ContentID: uuid.New()}
		err := h.content(context.Background(), &a, nil)
		if pe, ok := problemOf(err); !ok || pe.Code != problem.ActivityTypeNotSupported {
			t.Fatalf("%s: %v", typ, err)
		}
	}
	h := Assignments{Store: contentStore{typ: "quiz"}}
	a := write.AssignmentRow{ContentType: "activity", ContentID: uuid.New()}
	if err := h.content(context.Background(), &a, nil); err != nil || a.PinnedVersionID == nil || len(a.ObjectiveIDs) != 1 || *a.ActivityType != "quiz" {
		t.Fatalf("quiz: %v %+v", err, a)
	}
	a = write.AssignmentRow{ContentType: "activity", ContentID: uuid.New(), FollowLatest: true}
	if err := h.content(context.Background(), &a, nil); err != nil || a.PinnedVersionID != nil {
		t.Fatalf("follow_latest pins nothing: %v %+v", err, a)
	}
	a = write.AssignmentRow{ContentType: "assessment_blueprint", Mode: "graded"}
	if err := h.content(context.Background(), &a, nil); err == nil {
		t.Fatal("blueprint without exam sessions")
	}
}
