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
	// Authoring fields the Activity Builder edits again (F058): description, instructions, topic, timer, reward rule,
	// stored formulas, author and the assigned flag of platform content.
	if len(v.Summary) > 0 && string(v.Summary) != "null" {
		out["description"] = v.Summary
	}
	if len(a.Instructions) > 0 && string(a.Instructions) != "null" {
		out["instructions"] = a.Instructions
	}
	if len(v.TopicIDs) > 0 {
		out["topic_id"] = v.TopicIDs[0]
	}
	if a.TimeLimitSeconds != nil {
		out["time_limit_seconds"] = *a.TimeLimitSeconds
	}
	if a.RewardRuleID != nil {
		out["reward_rule_id"] = *a.RewardRuleID
	}
	if len(a.MathAssets) > 0 && string(a.MathAssets) != "null" {
		out["math_assets"] = a.MathAssets
	}
	if v.AuthorID != uuid.Nil {
		out["author_id"] = v.AuthorID
	}
	out["assigned"] = false
	return out
}

// ActivityJSONFor is ActivityJSON with the assigned flag of the session: platform content seen by a tenant session is
// assigned (read only "Từ EduWorld", F058-AC-27).
func ActivityJSONFor(doc dto.Doc, v dto.DocVersion, current *int, sessionTenant *uuid.UUID) map[string]any {
	out := ActivityJSON(doc, v, current)
	out["assigned"] = sessionTenant != nil && doc.Owner.TenantID == nil
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
