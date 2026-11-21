//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

const adr0095Rollback = "scripts/db/rollback/adr-0095-learning.sql"

// preADR0095Schema recreates, inside tx (rolled back by the caller), the structures dropped by the contract migration
// 20261230000001 (ADR-0095 mục 8) with its rollback script, so a test can re-run a block of an earlier migration on
// legacy rows (class targets, class grades, class cohorts).
func preADR0095Schema(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()
	if _, err := tx.ExecContext(ctx, testkit.RepoFile(t, adr0095Rollback)); err != nil {
		t.Fatalf("rollback script %s: %v", adr0095Rollback, err)
	}
}

// ADR-0095 mục 8 (WP-R): the rollback script recreates every dropped table, view and column of module learning, and the
// contract migration drops them again (backup plan step 4).
func TestADR0095RollbackRoundTrip(t *testing.T) {
	testkit.RollbackRoundTrip(context.Background(), t, "learning",
		"src/backend/modules/learning/migrations/20261230000001_adr0095_drop_class_course.sql", adr0095Rollback,
		[]string{"ref.classes", "ref.enrollments", "lms.courses", "lms.courses_live", "lms.learning_paths", "lms.learning_paths_live",
			"lms.path_nodes", "lms.path_nodes_live", "lms.path_node_unlocks", "lms.path_node_unlocks_live", "lms.schedule_items",
			"lms.schedule_items_live", "lms.study_groups", "lms.study_groups_live", "lms.study_group_members",
			"lms.study_group_members_live", "learning.rollup_class_daily", "learning.rollup_class_daily_default",
			"learning.rollup_class_weekly", "learning.rollup_class_weekly_default", "learning.rollup_class_monthly",
			"learning.rollup_class_objective_weekly", "learning.rollup_class_objective_weekly_default",
			"learning.rm_class_objective_stats", "learning.rm_class_subject_summary", "learning.efficacy_study_classes",
			"learning.efficacy_study_classes_live", "learning.efficacy_wave_sessions", "learning.progress_daily_class_idx",
			"lms.assignments_course_idx", "lms.assignment_targets_ref_idx", "learning.efficacy_participants_class_idx",
			"learning.rm_reference_levels_school_idx"},
		[]string{"learning.learning_events.class_id", "learning.progress_daily.class_id", "learning.lesson_sessions.learning_path_id",
			"lms.assignments.course_id", "lms.assignment_targets.target_ref_id", "lms.assignment_target_students.class_id_at_assign",
			"lms.assignment_submissions.class_id", "assessment.exam_sessions.course_id", "learning.rm_student_attention.class_id",
			"learning.rm_student_subject_reference_levels.class_id", "learning.efficacy_participants.class_id",
			"learning.efficacy_withdrawals.class_id", "learning.rollup_school_daily.active_classes",
			"learning.rollup_school_weekly.active_classes", "learning.rollup_school_monthly.active_classes",
			"learning.rollup_tenant_daily.active_classes", "learning.rollup_tenant_weekly.active_classes",
			"learning.rollup_tenant_monthly.active_classes"},
		map[string][]string{"content_releases_item_type_ck": {"learning_path"}, "content_transitions_type_ck": {"learning_path"},
			"media_usages_item_type_ck": {"learning_path"}, "review_tasks_type_ck": {"learning_path"}, "scheduled_publications_type_ck": {"learning_path"},
			"tra_resource_type_ck": {"course", "learning_path"}, "attempts_context_type_ck": {"learning_path"},
			"report_jobs_type_ck": {"course_results", "class_progress"}, "efficacy_withdrawals_scope_ck": {"class"}, "experiments_unit_ck": {"class"}})
}
