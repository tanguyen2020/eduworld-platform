//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// assignResource assigns a platform resource to a tenant (TEST helper: the row the facade applyTenantAssignments
// of F020 writes).
func (c *curEnv) assignResource(ctx context.Context, t *testing.T, resourceType, id string, tenant uuid.UUID) {
	t.Helper()
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments
		(assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), $3, $4, $1, now())`, ids.New(), tenant, resourceType, id); err != nil {
		t.Fatal(err)
	}
}

// question creates a question of sub with the fixture of typ and extra fields; it returns content_id and the id of
// version 1.
func (c *curEnv) question(t *testing.T, sub string, qc qbCurriculum, extra map[string]any) (string, string) {
	t.Helper()
	r := must(t, c.createQ(t, sub, qc.body(qbFixture(t, "single_choice"), extra)), http.StatusCreated, "question")
	return r.str("content_id"), r.str("content_version_id")
}

// published creates and publishes a question; it returns its version id.
func (c *curEnv) published(ctx context.Context, t *testing.T, sub string, qc qbCurriculum, extra map[string]any) string {
	t.Helper()
	id, _ := c.question(t, sub, qc, extra)
	return c.publishQuestion(ctx, t, id, 1)
}

func setBody(qc qbCurriculum, def map[string]any) map[string]any {
	return map[string]any{"title": map[string]any{"vi": "Luyện bảng nhân 7"}, "subject_id": qc.subject, "grade_levels": []int{3},
		"curriculum_version_id": qc.version, "topic_ids": []string{qc.topic}, "objective_ids": []string{qc.lo}, "definition": def}
}

func staticDef(qvs ...string) map[string]any {
	items := make([]any, len(qvs))
	for i, q := range qvs {
		items[i] = map[string]any{"question_version_id": q}
	}
	return map[string]any{"kind": "static", "purpose": "exercise", "items": items}
}

// publishDoc marks a version published and sets the release pointer (TEST helper: the publish flow is F026).
func (c *curEnv) publishDoc(ctx context.Context, t *testing.T, itemTable, verTable, itemCol, id string, version int) string {
	t.Helper()
	var vid string
	// tenancyguard:ok TEST helper under learning_platform
	if err := c.platform.QueryRowContext(ctx, `UPDATE content.`+verTable+` SET status = 'published', published_at = now(), reviewer_id = author_id
		WHERE `+itemCol+` = $1 AND version_no = $2 RETURNING `+verTable[:len(verTable)-1]+`_id`, id, version).Scan(&vid); err != nil {
		t.Fatal(err)
	}
	// tenancyguard:ok TEST helper under learning_platform
	if _, err := c.platform.ExecContext(ctx, `UPDATE content.`+itemTable+` SET current_version_id = $2 WHERE `+itemCol+` = $1`, id, vid); err != nil {
		t.Fatal(err)
	}
	return vid
}

// runImports runs the queued import jobs (worker profile jobs).
func (c *curEnv) runImports(ctx context.Context, t *testing.T) {
	t.Helper()
	if _, err := (job.ImportWorker{Runner: c.cmd}).RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
}
