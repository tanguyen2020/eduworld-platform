package command

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/lesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// WorkedExampleRef is question.worked_example_ref (QUESTION_BANK §2, CONTENT_MODEL §9.2).
type WorkedExampleRef struct {
	ContentItemID uuid.UUID `json:"content_item_id"`
	BlockID       string    `json:"block_id"`
}

// checkWorkedExample: the referenced worked example exists in a lesson or learning material visible in the scope
// (its current release or its newest draft) and shares an objective with the question (F024-AC-08).
func (h Questions) checkWorkedExample(ctx context.Context, v dto.QuestionVersion) error {
	if h.Docs == nil || isNull(v.WorkedExampleRef) {
		return nil
	}
	var ref WorkedExampleRef
	if err := json.Unmarshal(v.WorkedExampleRef, &ref); err != nil || ref.ContentItemID == uuid.Nil || ref.BlockID == "" {
		return problem.Invalid("/worked_example_ref", problem.SubInvalid, "must be {content_item_id, block_id}")
	}
	for _, t := range []contentmodel.ContentType{contentmodel.TypeLesson, contentmodel.TypeLearningMaterial} {
		d, err := h.Docs.Doc(ctx, t, ref.ContentItemID)
		if err != nil {
			if pe, ok := problemOf(err); ok && pe.Code == problem.NotFound {
				continue
			}
			return err
		}
		vs, err := h.Docs.DocVersions(ctx, t, d.ID)
		if err != nil {
			return err
		}
		for _, dv := range vs {
			current := d.CurrentVersionID != nil && *d.CurrentVersionID == dv.ID
			if !current && dv.Status != contentmodel.StatusDraft {
				continue
			}
			var body struct {
				Blocks []lesson.Block `json:"blocks"`
			}
			_ = json.Unmarshal(dv.Body, &body)
			we, ok := lesson.Find(body.Blocks, ref.BlockID)
			if !ok {
				continue
			}
			if !lesson.SharesObjective(we.ObjectiveIDs, v.ObjectiveIDs) {
				return problem.Invalid("/worked_example_ref", problem.SubInvalid, "the worked example shares no objective with the question")
			}
			return nil
		}
	}
	return problem.Invalid("/worked_example_ref/block_id", problem.SubInvalid, "is not a worked example of a lesson or learning material in scope")
}
