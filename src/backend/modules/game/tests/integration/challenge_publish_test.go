//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func problemOf(err error) (problem.Code, string) {
	var pe *problem.Error
	if !errors.As(err, &pe) {
		return "", ""
	}
	field := ""
	if len(pe.Fields) > 0 {
		field = pe.Fields[0].Field
	}
	return pe.Code, field
}

// F076-AC-20: a challenge version is refused at publish with the field: outside challenge.schema.json, min > max, a
// level above g1_3_challenge_rules for grades 1–3, a group type played solo, a time challenge without a time limit, a
// question_selection source (createAttempt opens learning activities only), an unknown quest of the unlock rule;
// TRANSLATION_INCOMPLETE without en. F077 (waited for F076): an event naming an unknown challenge is 422
// ITEM_REF_INVALID, a published one is accepted.
func TestChallengePublish(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	id := "0193a5f4-6c1e-7b2a-9d1f-000000076199"
	cases := []struct {
		name  string
		edit  func(m map[string]any)
		code  problem.Code
		field string
	}{
		{"schema", func(m map[string]any) { m["lives"] = 3 }, problem.ValidationFailed, "payload"},
		{"min > max", func(m map[string]any) {
			m["difficulties"].([]any)[0].(map[string]any)["question_difficulty_min"] = 3
		}, problem.ValidationFailed, "difficulties/0/question_difficulty_min"},
		{"expert for grade 3", func(m map[string]any) {
			m["grade_levels"] = []int{3, 4}
			delete(m["difficulties"].([]any)[3].(map[string]any), "grade_levels")
		}, problem.ValidationFailed, "difficulties/3/grade_levels"},
		{"boss solo", func(m map[string]any) {
			m["challenge_type"], m["boss_id"] = "boss", "0193a5f4-6c1e-7b2a-9d1f-000000076198"
		}, problem.ValidationFailed, "mode"},
		{"time limit", func(m map[string]any) { delete(m, "time_limit_seconds") }, problem.ValidationFailed, "payload"},
		{"question set", func(m map[string]any) {
			m["activity_source"] = map[string]any{"kind": "question_selection", "question_set_id": chlSubject}
		}, problem.ValidationFailed, "activity_source/kind"},
		{"unlock quest", func(m map[string]any) {
			m["unlock_rule"] = map[string]any{"completed_quest_ids": []string{uuid.NewString()}}
		}, problem.ItemRefInvalid, ""},
		{"translation", func(m map[string]any) { m["name"] = map[string]any{"vi": "Thử thách", "en": ""} }, problem.TranslationIncomplete, ""},
	}
	for _, c := range cases {
		_, err := e.publish(ctx, t, []command.ContentItem{challengeItem(id, "chl.math.bad", "time", []int{4, 5, 6, 7}, c.edit)}, "bad")
		code, field := problemOf(err)
		if code != c.code || (c.field != "" && field != c.field) {
			t.Errorf("%s: %v (field %q)", c.name, err, field)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.challenges`); n != 0 {
		t.Fatalf("a refused challenge was written: %d", n)
	}
	// A valid challenge publishes once (identical payload unchanged), with its release row.
	ok := challengeItem(id, "chl.math.ok", "time", []int{4, 5, 6, 7}, nil)
	res, err := e.publish(ctx, t, []command.ContentItem{ok}, "ok")
	if err != nil || res.Published != 1 {
		t.Fatalf("publish: %+v %v", res, err)
	}
	if res, err := e.publish(ctx, t, []command.ContentItem{ok}, "again"); err != nil || res.Unchanged != 1 {
		t.Fatalf("unchanged: %+v %v", res, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.content_releases WHERE item_type = 'challenge' AND item_id = $1`, id); n != 1 {
		t.Fatalf("release rows: %d", n)
	}
	// F077: events reference challenges.
	unknown := e.festival(uuid.New(), func(m map[string]any) {
		m["components"].(map[string]any)["challenge_ids"] = []string{uuid.NewString()}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{unknown}, "event"); !codeIs(err, problem.ItemRefInvalid) {
		t.Fatalf("event with an unknown challenge: %v", err)
	}
	known := e.festival(uuid.New(), func(m map[string]any) {
		m["components"].(map[string]any)["challenge_ids"] = []string{id}
		// Xu rewards only: this environment publishes no item catalog (the badge of the festival is an F075 item).
		m["milestone_rewards"] = []any{map[string]any{"milestone_id": "m1", "event_points": 50, "rewards": []any{xuGrant(20)}}}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{known}, "event"); err != nil {
		t.Fatalf("event with a published challenge: %v", err)
	}
}

// T011: RLS of game.challenge_attempts: game_app with tenant A never reads tenant B, nothing without app.tenant_id;
// no DELETE for game_app and game_platform (soft-delete only, R21); game_readonly reads the _live view only.
func TestChallengeRLS(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	e.publishChallenges(ctx, t, challengeSet(), e.tenantA, e.tenantB)
	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	bao := e.studentOfGrade(ctx, t, e.tenantB, e.schoolB, 3)
	for _, s := range []student{an, bao} {
		if r := e.startChallenge(t, s, chlTime3, "easy", ""); r.status != 201 {
			t.Fatalf("start: %s", r.raw)
		}
	}
	read := func(tenant *uuid.UUID) (own, other int) {
		tx, err := e.app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if tenant != nil {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant.String()); err != nil {
				t.Fatal(err)
			}
		}
		// tenancyguard:ok RLS test: counts the rows of each tenant visible to game_app on purpose
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE tenant_id = $1), count(*) FILTER (WHERE tenant_id <> $1)
			FROM game.challenge_attempts`, e.tenantA).Scan(&own, &other); err != nil {
			t.Fatal(err)
		}
		return own, other
	}
	if own, other := read(&e.tenantA); own != 1 || other != 0 {
		t.Fatalf("tenant A: own %d other %d", own, other)
	}
	if own, other := read(nil); own+other != 0 {
		t.Fatalf("without tenant: %d", own+other)
	}
	for name, db := range map[string]interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}{"app": e.app, "platform": e.platform} {
		if _, err := db.ExecContext(ctx, deleteAll("game.challenge_attempts")); err == nil {
			t.Fatalf("game_%s may delete challenge attempts", name)
		}
	}
	ro := open(t, e.cl.ModuleDSN("game", "readonly"))
	tx, err := ro.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String()); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, tx, `SELECT count(*) FROM game.challenge_attempts_live`); n != 1 {
		t.Fatalf("readonly live view (tenant A): %d", n)
	}
	if _, err := ro.ExecContext(ctx, `SELECT 1 FROM game.challenge_attempts LIMIT 1`); err == nil {
		t.Fatal("game_readonly reads the table")
	}
}
