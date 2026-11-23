//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// F074-AC-17, F074-AC-18: getProgression returns the progression of the student itself; listXpHistory pages the postings
// newest first with a friendly label and never a technical id; listProgressionLevels and getXpStatus answer the curve
// and today's cap.
func TestProgressionAPI(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(an, "main"))
	e.clock.Add(1e9)
	e.mustDeliver(ctx, t, uc, e.lessonEvent(an, uuid.New(), []int{3}, "practice"))
	for _, id := range []string{"discovery.hub.da", "discovery.hub.db", "discovery.hub.dc"} {
		e.clock.Add(1e9)
		e.mustDeliver(ctx, t, uc, e.envelope("world.discovery_found", "game", an.tenant, map[string]any{"student_id": an.id, "area_id": "main_hub",
			"interactable_id": id, "kind": "discovery", "completion_pct": 10, "found_at": e.clock.Now()}))
	}
	e.mustDeliver(ctx, t, uc, e.questEvent(other, "main"))
	tok := e.gtoken(an)
	r := e.do(t, http.MethodGet, "/game/progression", tok, th(an), nil)
	b := r.body
	if r.status != http.StatusOK || b["level"] != float64(2) || b["xp_total"] != float64(200) || b["xp_in_level"] != float64(80) ||
		b["xp_to_next"] != float64(215) || b["daily_xp_remaining"] != float64(800) || b["curve_version"] != float64(1) || b["max_level"] != float64(60) {
		t.Fatalf("getProgression: %d %s", r.status, r.raw)
	}
	title, _ := b["title"].(map[string]any)
	label, _ := title["label"].(map[string]any)
	if title["code"] != "beginner" || label["vi"] != "Tân binh" || label["en"] != "Beginner" {
		t.Fatalf("title %+v", title)
	}
	if _, ok := b["next_level_rewards"].([]any); !ok {
		t.Fatalf("next_level_rewards %+v", b["next_level_rewards"])
	}
	r = e.do(t, http.MethodGet, "/game/progression/xp-history?limit=2", tok, map[string]string{"X-Tenant-Id": an.tenant.String(),
		"Accept-Language": "en-US"}, nil)
	data, _ := r.body["data"].([]any)
	pg, _ := r.body["page"].(map[string]any)
	if r.status != http.StatusOK || len(data) != 2 || pg["has_more"] != true || pg["next_cursor"] == nil {
		t.Fatalf("history page 1: %d %s", r.status, r.raw)
	}
	first, _ := data[0].(map[string]any)
	if first["source_code"] != "exploration.discovery" || first["source_label"] != "New discovery" || first["amount"] != float64(10) {
		t.Fatalf("newest first: %+v", first)
	}
	for _, banned := range []string{"source_id", "rule_id", "idempotency_key", "xp_entry_id", "student_id", "rule_version"} {
		if strings.Contains(r.raw, banned) {
			t.Fatalf("history exposes %s: %s", banned, r.raw)
		}
	}
	var codes []string
	for _, d := range data {
		codes = append(codes, d.(map[string]any)["source_code"].(string))
	}
	cursor, _ := pg["next_cursor"].(string)
	for cursor != "" {
		r = e.do(t, http.MethodGet, "/game/progression/xp-history?limit=2&cursor="+cursor, tok, th(an), nil)
		if r.status != http.StatusOK {
			t.Fatalf("next page: %d %s", r.status, r.raw)
		}
		for _, d := range r.body["data"].([]any) {
			codes = append(codes, d.(map[string]any)["source_code"].(string))
		}
		cursor = ""
		if nc, ok := r.body["page"].(map[string]any)["next_cursor"].(string); ok {
			cursor = nc
		}
	}
	if len(codes) != 5 || codes[3] != "lesson.completed" || codes[4] != "quest.main_completed" {
		t.Fatalf("all pages: %v", codes)
	}
	r = e.do(t, http.MethodGet, "/game/progression/levels", tok, th(an), nil)
	levels, _ := r.body["levels"].([]any)
	if r.status != http.StatusOK || len(levels) != 60 || r.body["curve_version"] != float64(1) {
		t.Fatalf("levels: %d %s", r.status, r.raw)
	}
	l1 := levels[0].(map[string]any)
	l10 := levels[9].(map[string]any)
	if l1["xp_required"] != float64(120) || l1["xp_cumulative"] != float64(0) || l10["title_code"] != "explorer" || len(l10["rewards"].([]any)) != 1 {
		t.Fatalf("level rows %+v %+v", l1, l10)
	}
	r = e.do(t, http.MethodGet, "/game/xp", tok, th(an), nil)
	caps, _ := r.body["daily_caps"].([]any)
	if r.status != http.StatusOK || r.body["level"] != float64(2) || r.body["xp_total"] != float64(200) || r.body["xp_to_next_level"] != float64(215) ||
		len(caps) != 1 || caps[0].(map[string]any)["granted"] != float64(200) || caps[0].(map[string]any)["cap"] != float64(1000) {
		t.Fatalf("getXpStatus: %d %s", r.status, r.raw)
	}
	// A student without XP yet: level 1, a title, the full daily cap.
	fresh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r = e.do(t, http.MethodGet, "/game/progression", e.gtoken(fresh), th(fresh), nil); r.status != http.StatusOK || r.body["level"] != float64(1) ||
		r.body["xp_total"] != float64(0) || r.body["daily_xp_remaining"] != float64(1000) {
		t.Fatalf("fresh student: %d %s", r.status, r.raw)
	}
}
