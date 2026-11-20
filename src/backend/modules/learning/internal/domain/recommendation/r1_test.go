package recommendation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// F041-AC-31: an open assignment not submitted with a personal due date less than 48 hours ahead is recommended with
// priority 1; later, past or submitted assignments are not.
func TestR1AssignmentDue(t *testing.T) {
	now := time.Date(2026, 11, 23, 2, 0, 0, 0, time.UTC)
	soon, later, past := uuid.New(), uuid.New(), uuid.New()
	s := Snapshot{Now: now, Assignments: []PendingAssignment{
		{ID: soon, Title: "Bảng nhân 3", SubjectID: uuid.New(), DueAt: now.Add(47 * time.Hour)},
		{ID: later, SubjectID: uuid.New(), DueAt: now.Add(49 * time.Hour)},
		{ID: past, SubjectID: uuid.New(), DueAt: now.Add(-time.Minute)},
	}}
	cs := R1{}.Evaluate(s)
	if len(cs) != 1 || cs[0].TargetID != soon || cs[0].Priority != 1 || cs[0].Kind != KindAssignment || cs[0].TargetType != "assignment" ||
		cs[0].Reason != "assignment_soon" {
		t.Fatalf("R1: %+v", cs)
	}
}
