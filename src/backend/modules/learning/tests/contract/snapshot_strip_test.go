package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/snapshot"
)

// F026-AC-12 (T024, SC-002): the snapshot of every QUESTION_BANK §3 example and of the composed types holds no answer
// data, even when the source document carries the answer key, feedback and hints.
func TestSnapshotStrip(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "domain", "question", "testdata", "qb_examples")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) < 8 {
		t.Fatalf("examples: %v %d", err, len(files))
	}
	for _, f := range files {
		raw, err := os.ReadFile(f) //nolint:gosec // test fixture of the repository
		if err != nil {
			t.Fatal(err)
		}
		b, _, err := dto.BuildSnapshot(contentmodel.TypeQuestion, uuid.New(), uuid.New(), 1, "vi", []int{3}, "x", raw)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if leaks := snapshot.Leaks(b); len(leaks) != 0 {
			t.Fatalf("%s leaks %v", filepath.Base(f), leaks)
		}
	}
	lesson := json.RawMessage(`{"title":{"vi":"Bài"},"blocks":[{"kind":"worked_example","steps":[{"content":{},"feedback":"x"}]}]}`)
	for _, typ := range []contentmodel.ContentType{contentmodel.TypeLesson, contentmodel.TypeLearningMaterial, contentmodel.TypeQuestionSet,
		contentmodel.TypeAssessmentBlueprint} {
		b, sha, err := dto.BuildSnapshot(typ, uuid.New(), uuid.New(), 1, "vi", nil, "", lesson)
		if err != nil || len(snapshot.Leaks(b)) != 0 || len(sha) != 64 {
			t.Fatalf("%s: %v %s", typ, err, b)
		}
	}
}
