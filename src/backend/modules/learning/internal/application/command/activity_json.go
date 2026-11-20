package command

import (
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// ActivityJSON is ActivityDefinition: the activity fields of the version body plus its metadata columns (the response
// of the CMS activity endpoints and the replay body of their Idempotency-Key).
func ActivityJSON(doc dto.Doc, v dto.DocVersion, current *int) map[string]any {
	a := attemptview.ParseBody(v.Body)
	out := map[string]any{"activity_id": doc.ID, "activity_version_id": v.ID, "version": v.VersionNo, "activity_type": a.ActivityType,
		"title": v.Title, "status": string(v.Status), "owner_scope": string(doc.Owner.Scope), "grade_levels": nonNil(v.GradeLevels),
		"objective_ids": nonNil(v.ObjectiveIDs), "current_published_version": current, "updated_at": v.UpdatedAt,
		"content_language": v.ContentLanguage}
	if doc.Owner.TenantID != nil {
		out["tenant_id"] = *doc.Owner.TenantID
	}
	if v.SubjectID != nil {
		out["subject_id"] = *v.SubjectID
	}
	if a.Difficulty != nil {
		out["difficulty"] = *a.Difficulty
	}
	if len(a.Config) > 0 {
		out["config"] = a.Config
	}
	if v.TranslationStatus != nil {
		out["translation_status"] = *v.TranslationStatus
	}
	return out
}

// docResultJSON is the replay body of a create or new-version request: ActivityDefinition for an activity, the
// authoring view of the composed types otherwise.
func docResultJSON(out DocResult, tenant *uuid.UUID, current *int) any {
	if out.Doc.Type == contentmodel.TypeActivity {
		return ActivityJSON(out.Doc, out.Version, current)
	}
	return DocJSON(out, tenant)
}
