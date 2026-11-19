package contract

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/readiness"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F025-AC-17 (T027): PublishReadinessGuard is one function (command.Workflow.Readiness) for submit_review and the
// publish of F026, with the problem codes of the contract (ProblemCode: VALIDATION_FAILED, MEDIA_NOT_READY with
// asset_ids, AUDIO_REQUIRED, TRANSLATION_INCOMPLETE — each 422 with errors[].field as JSON pointers).
func TestReadinessGuardProblems(t *testing.T) {
	asset := uuid.New()
	cases := []struct {
		s     command.Subject
		code  problem.Code
		field string
	}{
		{command.Subject{Type: contentmodel.TypeLesson, ContentLanguage: "vi", Doc: json.RawMessage(`{"title":{"vi":"x"}}`),
			Meta: contentmodel.Violations{{Field: "/grade_levels", Required: true, Reason: "is required"}}}, problem.ValidationFailed, "/grade_levels"},
		{command.Subject{Type: contentmodel.TypeQuestion, GradeLevels: []int{2}, ContentLanguage: "vi",
			Doc: json.RawMessage(`{"stem":{"blocks":[{"type":"paragraph","text":"x"}]}}`)}, problem.AudioRequired, "/stem/audio_ref"},
		{command.Subject{Type: contentmodel.TypeQuestion, GradeLevels: []int{5}, ContentLanguage: "vi",
			Doc: json.RawMessage(`{"stem":{"blocks":[{"type":"paragraph","text":{"en":"x"}}]}}`)}, problem.TranslationIncomplete, "/stem/blocks/0/text"},
	}
	wf := command.Workflow{Guard: readiness.Default()}
	for _, c := range cases {
		_, err := wf.Readiness(context.Background(), c.s)
		var pe *problem.Error
		if !errors.As(err, &pe) || pe.Code != c.code || problem.Status(pe.Code) != 422 || len(pe.Fields) == 0 || pe.Fields[0].Field != c.field {
			t.Fatalf("%s: %v", c.code, err)
		}
	}
	err := command.ReadinessProblem(readiness.Result{Code: readiness.CodeMediaNotReady,
		Violations: []readiness.Violation{{Code: readiness.CodeMediaNotReady, Pointer: "/media_refs", AssetID: asset, Reason: "media is processing"}}})
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.MediaNotReady || problem.Status(pe.Code) != 422 {
		t.Fatalf("media: %v", err)
	}
	ids, _ := pe.Ext["asset_ids"].([]string)
	if len(ids) != 1 || ids[0] != asset.String() {
		t.Fatalf("asset_ids %v", pe.Ext)
	}
	if _, err := wf.Readiness(context.Background(), command.Subject{Type: contentmodel.TypeQuestion, GradeLevels: []int{5}, ContentLanguage: "vi",
		Doc: json.RawMessage(`{"stem":{"blocks":[{"type":"paragraph","text":"x"}]}}`)}); err != nil {
		t.Fatalf("ready version: %v", err)
	}
}
