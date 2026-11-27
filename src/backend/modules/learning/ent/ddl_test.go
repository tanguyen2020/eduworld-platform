package ent_test

import (
	"flag"
	"os"
	"testing"

	"entgo.io/ent/dialect/sql/schema"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// The generated block of each migration equals the DDL of the ent tables it introduces (AGENTS R27).
// go test ./modules/learning/ent -run TestMigrationMatchesEnt -args -print=<migration file> prints the block.
var printDDL = flag.String("print", "", "print the entddl block of this migration file")

// migrations lists, per migration file, the ent tables created in its entddl block (in order). The ref.*
// projections are hand-written (no entx mixin: DATABASE_DESIGN §2.17 minimal columns, F020 clarification).
var migrations = map[string][]string{
	"20261122000001_f020_foundation.sql": {"content_releases", "content_transitions", "tenant_resource_assignments"},
	"20261124000001_f021_curriculum.sql": {"curricula", "curriculum_versions", "subjects", "subject_grades", "topics",
		"learning_objectives", "competencies", "lo_version_mappings", "textbook_series", "textbooks", "textbook_sections",
		"textbook_section_objectives", "import_jobs"},
	"20261125000001_f022_question_bank.sql": {"questions", "question_versions", "question_version_objectives"},
	"20261126000001_f027_media.sql":         {"media_assets", "media_usages", "math_svgs"},
	"20261127000001_f024_content_types.sql": {"question_sets", "question_set_versions", "question_set_items", "lessons", "lesson_versions",
		"learning_materials", "learning_material_versions", "assessment_blueprints", "assessment_blueprint_versions"},
	"20261128000001_f025_workflow.sql":         {"review_tasks", "translation_jobs"},
	"20261129000001_f026_release.sql":          {"scheduled_publications"},
	"20261130000001_f030_activity_attempt.sql": {"activities", "activity_versions", "attempts", "attempt_items", "evaluations"},
	"20261208000001_f025_field_comments.sql":   {},
	"20261222000001_f042_exam_engine.sql":      {},
	"20261229000003_f105_question_plans.sql":   {"question_plans"},
	"20270101000073_f073_minigame.sql":         {},
	"20270202000028_f028_import_items.sql":     {"import_items"},
}

// added lists, per migration file, the columns and indexes it adds to ent tables created by an earlier migration
// (ALTER TABLE … ADD COLUMN in its entddl block, after its CREATE TABLEs); earlier blocks are checked without them.
type addition struct {
	table            string
	columns, indexes []string
}

var workflowColumns = []string{"translation_status", "translation_meta", "audio_exempt_reason", "audio_exempt_approved_by"}

var added = map[string][]addition{
	"20261128000001_f025_workflow.sql": {
		{"content_transitions", []string{"content_id", "action", "idempotency_key"}, []string{"content_transitions_idem_uq"}},
		{"question_versions", workflowColumns, nil},
		{"question_set_versions", workflowColumns, nil},
		{"lesson_versions", workflowColumns, nil},
		{"learning_material_versions", workflowColumns, nil},
		{"assessment_blueprint_versions", workflowColumns, nil},
	},
	"20261208000001_f025_field_comments.sql": {{"content_transitions", []string{"field_comments"}, nil}},
	"20261129000001_f026_release.sql": {
		{"question_versions", []string{"published_by"}, nil},
		{"question_set_versions", []string{"published_by"}, nil},
		{"lesson_versions", []string{"published_by"}, nil},
		{"learning_material_versions", []string{"published_by"}, nil},
		{"assessment_blueprint_versions", []string{"published_by"}, nil},
	}, // F042: the exam columns of the attempts.
	"20261222000001_f042_exam_engine.sql": {{"attempts", []string{"exam_paper_id", "active_device_token", "client_platform", "game_session_id"}, nil}},
	// F073: the game score, streak and pinned mini-game configuration version of the attempts.
	"20270101000073_f073_minigame.sql": {{"attempts", []string{"game_score", "game_streak", "minigame_config_version_id"}, nil}},
}

// withoutAdded drops from tables the columns/indexes added by migrations other than file.
func withoutAdded(file string, tbs []*schema.Table) []*schema.Table {
	out := make([]*schema.Table, 0, len(tbs))
	for _, tb := range tbs {
		for f, as := range added {
			for _, a := range as {
				if f != file && a.table == tb.Name {
					tb = entx.Without(tb, a.columns, a.indexes)
				}
			}
		}
		out = append(out, tb)
	}
	return out
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{
		"content_releases": c.ContentRelease, "content_transitions": c.ContentTransition,
		"tenant_resource_assignments": c.TenantResourceAssignment,
		"curricula":                   c.Curriculum, "curriculum_versions": c.CurriculumVersion, "subjects": c.Subject, "subject_grades": c.SubjectGrade,
		"topics": c.Topic, "learning_objectives": c.LearningObjective, "competencies": c.Competency,
		"lo_version_mappings": c.LoVersionMapping, "textbook_series": c.TextbookSeries, "textbooks": c.Textbook,
		"textbook_sections": c.TextbookSection, "textbook_section_objectives": c.TextbookSectionObjective, "import_jobs": c.ImportJob, "import_items": c.ImportItem,
		"questions": c.Question, "question_versions": c.QuestionVersion, "question_version_objectives": c.QuestionVersionObjective,
		"media_assets": c.MediaAsset, "media_usages": c.MediaUsage, "math_svgs": c.MathSvg,
		"question_sets": c.QuestionSet, "question_set_versions": c.QuestionSetVersion, "question_set_items": c.QuestionSetItem,
		"lessons": c.Lesson, "lesson_versions": c.LessonVersion, "learning_materials": c.LearningMaterial,
		"learning_material_versions": c.LearningMaterialVersion, "assessment_blueprints": c.AssessmentBlueprint,
		"assessment_blueprint_versions": c.AssessmentBlueprintVersion, "review_tasks": c.ReviewTask, "translation_jobs": c.TranslationJob, "scheduled_publications": c.ScheduledPublication,
		"activities": c.Activity, "activity_versions": c.ActivityVersion, "attempts": c.Attempt, "attempt_items": c.AttemptItem, "evaluations": c.Evaluation,
		"question_plans": c.QuestionPlan,
	}
}

func tablesOf(t *testing.T, names []string) []*schema.Table {
	t.Helper()
	byName := map[string]*schema.Table{}
	for _, tb := range migrate.Tables {
		byName[tb.Name] = tb
	}
	out := make([]*schema.Table, 0, len(names))
	for _, n := range names {
		tb, ok := byName[n]
		if !ok {
			t.Fatalf("no ent table %s", n)
		}
		out = append(out, tb)
	}
	return out
}

func TestMigrationMatchesEnt(t *testing.T) {
	sc := schemas()
	listed := map[string]bool{}
	for _, tbs := range migrations {
		for _, n := range tbs {
			listed[n] = true
		}
	}
	for _, tb := range migrate.Tables {
		if sc[tb.Name] == "" {
			t.Fatalf("table %s has no schema in the test map", tb.Name)
		}
		if !listed[tb.Name] {
			t.Errorf("ent table %s is created by no migration", tb.Name)
		}
	}
	for file, names := range migrations {
		want, err := entx.DDL(withoutAdded(file, tablesOf(t, names)), func(table string) string { return sc[table] })
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range added[file] {
			alter, err := entx.AddedDDL(tablesOf(t, []string{a.table})[0], a.columns, a.indexes, func(table string) string { return sc[table] })
			if err != nil {
				t.Fatal(err)
			}
			want += alter
		}
		if *printDDL == file {
			_, _ = os.Stdout.WriteString(want)
			continue
		}
		if *printDDL != "" {
			continue
		}
		b, err := os.ReadFile("../migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if ok, got := entx.CheckDDLBlock(string(b), want); !ok {
			t.Fatalf("entddl block of %s differs from the ent schema; regenerate with -print=%s.\n--- want\n%s\n--- got\n%s", file, file, want, got)
		}
	}
}
