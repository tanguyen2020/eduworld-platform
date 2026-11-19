package facade

import (
	"context"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
)

// Receiver-side facade test of cleanupStudentData / restoreStudentData (F016 T021): module learning keeps no
// student-owned row yet, so both answer nothing_to_do; only module identity may call them.
func TestPlatformStudentsFacade(t *testing.T) {
	f := Facade{}
	id := uuid.New()
	c, err := f.CleanupStudentData(context.Background(), learningapi.CleanupStudentDataRequestObject{StudentId: id,
		Body: &learningapi.CleanupStudentDataJSONRequestBody{CleanupRunId: uuid.New(), Reason: "inactive_cleanup"}})
	if v, ok := c.(learningapi.CleanupStudentData200JSONResponse); err != nil || !ok || v.StudentId != id || v.Status != "nothing_to_do" || v.BlockedReason != nil {
		t.Fatalf("cleanup: %+v %v", c, err)
	}
	r, err := f.RestoreStudentData(context.Background(), learningapi.RestoreStudentDataRequestObject{StudentId: id,
		Body: &learningapi.RestoreStudentDataJSONRequestBody{Reason: "admin_restore"}})
	if v, ok := r.(learningapi.RestoreStudentData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" {
		t.Fatalf("restore: %+v %v", r, err)
	}
	a, err := f.AnonymizeUserData(context.Background(), learningapi.AnonymizeUserDataRequestObject{UserId: id, Body: &learningapi.AnonymizeUserDataJSONRequestBody{ErasureRequestId: uuid.New(), Reason: "legal_order"}})
	if v, ok := a.(learningapi.AnonymizeUserData200JSONResponse); err != nil || !ok || v.Status != "nothing_to_do" {
		t.Fatalf("anonymize: %+v %v", a, err)
	}
	for _, o := range []string{"cleanupStudentData", "restoreStudentData", "anonymizeUserData"} {
		if c := learningapi.AllowedCallers[o]; len(c) != 1 || c[0] != "identity" {
			t.Fatalf("x-allowed-callers %s: %v", o, c)
		}
	}
}
