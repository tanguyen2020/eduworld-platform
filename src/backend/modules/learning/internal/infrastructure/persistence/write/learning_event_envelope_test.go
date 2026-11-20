package write

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

func answered(payload map[string]any) (write.LearningEvent, []byte) {
	cv := uuid.New()
	e := write.LearningEvent{ID: ids.New(), OccurredAt: time.Now(), Type: "question.answered", StudentID: uuid.New(), TenantID: uuid.New(),
		SchoolID: uuid.New(), ContentVersionID: &cv, ObjectiveIDs: []uuid.UUID{uuid.New()}, Context: "practice", ActorType: "student",
		IdempotencyKey: "qa:" + uuid.NewString() + ":1:1"}
	b, _ := json.Marshal(payload)
	return e, b
}

func validPayload() map[string]any {
	return map[string]any{"attempt_id": uuid.NewString(), "position": 1, "question_version_id": uuid.NewString(), "question_type": "single_choice",
		"is_correct": true, "is_scored": true, "score": map[string]any{"raw": 1, "max": 1, "scaled": 1}, "response_ms": 2500, "attempt_no": 1,
		"hints_used": 0, "rapid_guess": false}
}

// F031-AC-01, AC-05 (T008, T012): the envelope of a learning event is valid against learning-event.schema.json; a payload
// with a field outside the schema (a name, an email, a position in the world) or an envelope without its required
// fields is refused, and the error names JSON pointers only, never the values (R14).
func TestLearningEventEnvelopeValidation(t *testing.T) {
	e, p := answered(validPayload())
	doc, _ := json.Marshal(Envelope(e, p, time.Now()))
	if err := qbschema.ValidateLearningEvent(doc); err != nil {
		t.Fatalf("valid event refused: %v", err)
	}
	for field, value := range map[string]any{"nickname": "Bé Na", "email": "an@example.com", "world_position": map[string]any{"x": 1},
		"chat_text": "xin chao"} {
		bad := validPayload()
		bad[field] = value
		e, p := answered(bad)
		doc, _ := json.Marshal(Envelope(e, p, time.Now()))
		err := qbschema.ValidateLearningEvent(doc)
		if !errors.Is(err, qbschema.ErrLearningEventInvalid) {
			t.Fatalf("%s accepted: %v", field, err)
		}
		if s, _ := value.(string); s != "" && strings.Contains(err.Error(), s) {
			t.Fatalf("error leaks the value: %v", err)
		}
	}
	// Envelope: question.* needs content_version_id; actor is student for the student and the zero UUID for system.
	e, p = answered(validPayload())
	e.ContentVersionID = nil
	doc, _ = json.Marshal(Envelope(e, p, time.Now()))
	if err := qbschema.ValidateLearningEvent(doc); !errors.Is(err, qbschema.ErrLearningEventInvalid) {
		t.Fatalf("question.answered without content_version_id: %v", err)
	}
	e, p = answered(validPayload())
	env := Envelope(e, p, time.Now())
	if env["actor"].(map[string]any)["id"] != e.StudentID || env["subject"].(map[string]any)["type"] != "attempt_item" {
		t.Fatalf("envelope: %v", env)
	}
	e.ActorType = "system"
	if Envelope(e, p, time.Now())["actor"].(map[string]any)["id"] != uuid.Nil {
		t.Fatal("system actor id")
	}
	if got := uniqueIDs([]uuid.UUID{e.StudentID, e.StudentID}); len(got) != 1 {
		t.Fatalf("objective ids not deduplicated: %v", got)
	}
}
