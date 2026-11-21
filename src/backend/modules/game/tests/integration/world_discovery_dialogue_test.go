//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/world"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F071-AC-24, AC-25 (T041, TC-GAM-010, TC-WLD-003): in learning_zone.math a valid discovery is recorded once with the
// percentage, world.discovery_found, and world.area_completed at 100 %; the same Idempotency-Key replays the result; a
// new report of a found anchor is 409 DISCOVERY_ALREADY_FOUND; from another area, an unknown or non-discovery id, or a
// lost presence → 409 STEP_PRECONDITION_FAILED; above discovery_report_rate → 429; when the hub reports the student far
// from the anchor → 409 STEP_PRECONDITION_FAILED (status of API_DESIGN §4.1).
func TestReportDiscovery(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d, session := e.session(t, an)
	if r := e.enter(t, an, d, session, "learning_zone.math", ""); r.status != http.StatusOK {
		t.Fatalf("enter: %d %s", r.status, r.raw)
	}
	key := newKey()
	r := e.discover(t, an, d, "learning_zone.math", "discovery.math.old_well", key)
	if r.status != http.StatusOK || r.body["area_completion_pct"] != 33.33 || r.body["kind"] != "discovery" {
		t.Fatalf("discover: %d %s", r.status, r.raw)
	}
	if again := e.discover(t, an, d, "learning_zone.math", "discovery.math.old_well", key); again.status != http.StatusOK ||
		again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v", again.status, again.header)
	}
	if dup := e.discover(t, an, d, "learning_zone.math", "discovery.math.old_well", newKey()); dup.status != http.StatusConflict ||
		dup.code() != string(problem.DiscoveryAlreadyFound) {
		t.Fatalf("found twice: %d %s", dup.status, dup.raw)
	}
	for _, id := range []string{"discovery.nowhere", "npc.mr_numbers"} {
		if bad := e.discover(t, an, d, "learning_zone.math", id, newKey()); bad.code() != string(problem.StepPreconditionFailed) {
			t.Fatalf("%s: %d %s", id, bad.status, bad.raw)
		}
	}
	if other := e.discover(t, an, d, "main_hub", "discovery.hub.spark_tree", newKey()); other.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("from another area: %d %s", other.status, other.raw)
	}
	_ = e.discover(t, an, d, "learning_zone.math", "discovery.math.number_bridge", newKey())
	done := e.discover(t, an, d, "learning_zone.math", "collectible.math.shell_abacus", newKey())
	if done.status != http.StatusOK || done.body["area_completion_pct"] != float64(100) || done.body["kind"] != "collectible" {
		t.Fatalf("100 %%: %d %s", done.status, done.raw)
	}
	if e.events(ctx, t, "world.discovery_found") != 3 || e.events(ctx, t, "world.area_completed") != 1 {
		t.Fatalf("events: %d %d", e.events(ctx, t, "world.discovery_found"), e.events(ctx, t, "world.area_completed"))
	}
	x := e.exploration(t, an)
	if !strings.Contains(x.raw, `"area_id":"learning_zone.math"`) || !strings.Contains(x.raw, `"completion_pct":100`) {
		t.Fatalf("exploration: %s", x.raw)
	}
	// Valkey lost the presence: 422 until the client enters the area again.
	e.presence.lose(true)
	if r := e.discover(t, an, d, "learning_zone.math", "discovery.math.old_well", newKey()); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("lost presence: %d %s", r.status, r.raw)
	}
	e.presence.lose(false)
	// The hub (F091) reports the student 20 m away from the old well of main_hub → too far.
	if r := e.enter(t, an, d, session, "main_hub", ""); r.status != http.StatusOK {
		t.Fatalf("enter hub: %d", r.status)
	}
	e.hub = &memHub{pos: map[uuid.UUID]*port.HubPosition{an.id: {AreaID: "main_hub", Position: world.Vec3{20, 0, 20}}}}
	if r := e.discover(t, an, d, "main_hub", "discovery.hub.spark_tree", newKey()); r.code() != string(problem.StepPreconditionFailed) {
		t.Fatalf("too far: %d %s", r.status, r.raw)
	}
	e.hub = &memHub{pos: map[uuid.UUID]*port.HubPosition{an.id: {AreaID: "main_hub", Position: world.Vec3{1, 0, -5}}}}
	if r := e.discover(t, an, d, "main_hub", "discovery.hub.spark_tree", newKey()); r.status != http.StatusOK {
		t.Fatalf("within reach: %d %s", r.status, r.raw)
	}
	e.hub = nil
	// discovery_report_rate: 30 per minute.
	var last resp
	for i := 0; i < 30; i++ {
		last = e.discover(t, an, d, "main_hub", "discovery.nowhere", newKey())
	}
	if last.status != http.StatusTooManyRequests || last.header.Get("Retry-After") == "" {
		t.Fatalf("rate limit: %d %s", last.status, last.raw)
	}
	if got := e.checkEventSchemas(ctx, t); got["world.discovery_found"] != 4 || got["world.area_completed"] != 2 { // learning_zone.math, then main_hub (one discovery)
		t.Fatalf("schemas: %v", got)
	}
}

func (e *env) dialogue(t *testing.T, s student, npc, inm string) resp {
	t.Helper()
	h := th(s)
	if inm != "" {
		h["If-None-Match"] = inm
	}
	return e.do(t, http.MethodGet, "/game/world/npcs/"+npc+"/dialogue", e.gtoken(s), h, nil)
}

// F071-AC-17 (T033): the published dialogue of the NPC for the grade of the student (grade 3 and grade 7 get different
// graphs of Mr. Numbers), vi/en texts, ETag by version and 304 on If-None-Match; an unknown NPC → 404.
func TestNpcDialogue(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active") // grade 3
	dung := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, dung, 7)
	r3, r7 := e.dialogue(t, an, "npc.mr_numbers", ""), e.dialogue(t, dung, "npc.mr_numbers", "")
	if r3.status != http.StatusOK || r7.status != http.StatusOK || r3.body["dialogue_id"] == r7.body["dialogue_id"] || r3.body["status"] != "published" ||
		r3.body["version"] != float64(1) || r3.body["npc_name"].(map[string]any)["en"] != "Mr. Numbers" {
		t.Fatalf("dialogues: %s / %s", r3.raw, r7.raw)
	}
	if !strings.Contains(r7.raw, "open_minigame") || !strings.Contains(r3.raw, "give_quest") || !strings.Contains(r3.raw, `"en":`) {
		t.Fatalf("graphs by grade: %s", r7.raw)
	}
	etag := r3.header.Get("ETag")
	if nm := e.dialogue(t, an, "npc.mr_numbers", etag); nm.status != http.StatusNotModified {
		t.Fatalf("304: %d", nm.status)
	}
	if r := e.dialogue(t, an, "npc.nobody", ""); r.status != http.StatusNotFound {
		t.Fatalf("unknown npc: %d %s", r.status, r.raw)
	}
	// A new version changes the ETag.
	v2 := seedItem(t, write.ItemNPCDialogue, "dialogue_id", r3.body["dialogue_id"].(string), func(m map[string]any) {
		m["nodes"].([]any)[1].(map[string]any)["text"] = map[string]any{"vi": "Cảm ơn cháu nhiều!", "en": "Thank you so much!"}
	})
	if res, err := e.publish(ctx, t, []command.ContentItem{v2}, "dialogue v2"); err != nil || res.Published != 1 {
		t.Fatalf("v2: %+v %v", res, err)
	}
	if r := e.dialogue(t, an, "npc.mr_numbers", etag); r.status != http.StatusOK || r.body["version"] != float64(2) {
		t.Fatalf("v2 served: %d %s", r.status, r.raw)
	}
}

// F071-AC-18 (T034): a dialogue without en on a node or a choice → 422 TRANSLATION_INCOMPLETE; a quest or an NPC that
// does not exist → 422 ITEM_REF_INVALID; a second dialogue of the NPC for the same grade → 422; nothing is published.
func TestNpcDialoguePublishValidation(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	cloudy := func(edit func(m map[string]any)) command.ContentItem {
		return seedItem(t, write.ItemNPCDialogue, "npc_id", "npc.cloudy", edit)
	}
	cases := []struct {
		name string
		item command.ContentItem
		code problem.Code
	}{
		{"missing en on a node", cloudy(func(m map[string]any) {
			m["nodes"].([]any)[0].(map[string]any)["text"] = map[string]any{"vi": "Chào!"}
		}), problem.TranslationIncomplete},
		{"missing en on a choice", cloudy(func(m map[string]any) {
			m["nodes"].([]any)[0].(map[string]any)["choices"].([]any)[0].(map[string]any)["text"] = map[string]any{"vi": "Được"}
		}), problem.TranslationIncomplete},
		{"unknown quest", cloudy(func(m map[string]any) {
			m["nodes"].([]any)[0].(map[string]any)["choices"].([]any)[0].(map[string]any)["actions"] =
				[]any{map[string]any{"type": "give_quest", "quest_id": "0193a5f4-6c1e-7b2a-9d1f-00000000dead"}}
		}), problem.ItemRefInvalid},
		{"unknown npc", cloudy(func(m map[string]any) {
			m["npc_id"], m["dialogue_id"] = "npc.ghost", "0193a5f4-6c1e-7b2a-9d1f-00000000beef"
		}), problem.ItemRefInvalid},
		{"same grade twice", cloudy(func(m map[string]any) {
			m["dialogue_id"] = "0193a5f4-6c1e-7b2a-9d1f-00000000cafe"
		}), problem.ValidationFailed},
	}
	for _, c := range cases {
		if _, err := e.publish(ctx, t, []command.ContentItem{c.item}, c.name); codeOf(err) != c.code {
			t.Fatalf("%s: %v", c.name, err)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.npc_dialogue_versions`); n != 14 {
		t.Fatalf("dialogue versions: %d", n)
	}
}
