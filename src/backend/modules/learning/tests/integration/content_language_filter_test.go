//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
)

// F024 T041 (OQ F054-2, owner 2026-11-20): searchQuestions and the lists of the four F024 types filter the newest
// version on content_language and translation_status, within the scope of the caller (tenant isolation).
func TestContentLanguageTranslationFilters(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	// Two items of tenant A per type; the newest version of the second is set to en/complete (TEST fixture of the
	// columns the draft save computes, F025).
	retag := func(table, itemCol, id string) {
		// tenancyguard:ok TEST fixture under learning_platform: the language columns of the version rows of one item
		if _, err := c.platform.ExecContext(ctx, `UPDATE content.`+table+` SET content_language = 'en', translation_status = 'complete'
			WHERE `+itemCol+` = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	q1, _ := c.question(t, qlcmA, qc, nil)
	q2, _ := c.question(t, qlcmA, qc, nil)
	qv1 := c.publishQuestion(ctx, t, q1, 1) // a static set needs a published question
	retag("question_versions", "question_id", q2)
	mb := lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("x")})
	delete(mb, "completion_rule")
	mb["material_kind"] = "rich_text"
	types := []struct {
		path, table, itemCol string
		body                 func() map[string]any
	}{
		{setsPath, "question_set_versions", "question_set_id", func() map[string]any { return setBody(qc, staticDef(qv1)) }},
		{lessonsPath, "lesson_versions", "lesson_id", func() map[string]any {
			return lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("x")})
		}},
		{materialsPath, "learning_material_versions", "learning_material_id", func() map[string]any { return mb }},
		{blueprintsPath, "assessment_blueprint_versions", "assessment_blueprint_id", func() map[string]any {
			return blueprintBody(qc, 1, []any{map[string]any{"objective_ids": []string{qc.lo}, "count": 1, "points_each": 1}}, nil)
		}},
	}
	ids := map[string][2]string{}
	for _, ty := range types {
		a := must(t, c.do(t, http.MethodPost, ty.path, qlcmA, idem(), ty.body()), http.StatusCreated, "create "+ty.path).str("content_id")
		b := must(t, c.do(t, http.MethodPost, ty.path, qlcmA, idem(), ty.body()), http.StatusCreated, "create "+ty.path).str("content_id")
		retag(ty.table, ty.itemCol, b)
		ids[ty.path] = [2]string{a, b}
	}
	idsOf := func(r resp, key string) []string {
		var out []string
		for _, x := range r.items("data") {
			out = append(out, x[key].(string))
		}
		return out
	}
	check := func(path, sub, query, key string, want ...string) {
		t.Helper()
		got := idsOf(must(t, c.do(t, http.MethodGet, path+"?"+query, sub, nil, nil), http.StatusOK, path+"?"+query), key)
		if len(got) != len(want) {
			t.Fatalf("%s?%s as %s = %v, want %v", path, query, sub, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s?%s as %s = %v, want %v", path, query, sub, got, want)
			}
		}
	}
	const questions = "/cms/learning/questions"
	check(questions, qlcmA, "content_language=en", "content_id", q2)
	check(questions, qlcmA, "translation_status=complete", "content_id", q2)
	check(questions, qlcmA, "content_language=vi", "content_id", q1)
	check(questions, qlcmB, "content_language=en", "content_id")
	for _, ty := range types {
		a, b := ids[ty.path][0], ids[ty.path][1]
		check(ty.path, qlcmA, "content_language=en", "content_id", b)
		check(ty.path, qlcmA, "translation_status=complete&content_language=en", "content_id", b)
		check(ty.path, qlcmA, "content_language=vi", "content_id", a)
		check(ty.path, qlcmB, "translation_status=complete", "content_id")
	}
	if r := c.do(t, http.MethodGet, setsPath+"?content_language=fr", qlcmA, nil, nil); r.status != http.StatusUnprocessableEntity &&
		r.status != http.StatusBadRequest {
		t.Fatalf("unknown language: %d %s", r.status, r.raw)
	}
}
