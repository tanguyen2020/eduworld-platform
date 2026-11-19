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
}

func schemas() map[string]string {
	c := ent.DefaultSchemaConfig
	return map[string]string{
		"content_releases": c.ContentRelease, "content_transitions": c.ContentTransition,
		"tenant_resource_assignments": c.TenantResourceAssignment,
		"curricula":                   c.Curriculum, "curriculum_versions": c.CurriculumVersion, "subjects": c.Subject, "subject_grades": c.SubjectGrade,
		"topics": c.Topic, "learning_objectives": c.LearningObjective, "competencies": c.Competency,
		"lo_version_mappings": c.LoVersionMapping, "textbook_series": c.TextbookSeries, "textbooks": c.Textbook,
		"textbook_sections": c.TextbookSection, "textbook_section_objectives": c.TextbookSectionObjective, "import_jobs": c.ImportJob,
		"questions": c.Question, "question_versions": c.QuestionVersion, "question_version_objectives": c.QuestionVersionObjective,
		"media_assets": c.MediaAsset, "media_usages": c.MediaUsage, "math_svgs": c.MathSvg,
		"question_sets": c.QuestionSet, "question_set_versions": c.QuestionSetVersion, "question_set_items": c.QuestionSetItem,
		"lessons": c.Lesson, "lesson_versions": c.LessonVersion, "learning_materials": c.LearningMaterial,
		"learning_material_versions": c.LearningMaterialVersion, "assessment_blueprints": c.AssessmentBlueprint,
		"assessment_blueprint_versions": c.AssessmentBlueprintVersion,
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
		want, err := entx.DDL(tablesOf(t, names), func(table string) string { return sc[table] })
		if err != nil {
			t.Fatal(err)
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
