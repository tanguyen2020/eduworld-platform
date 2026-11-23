//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// progressed is learning_objective.progressed of objective at level (owner F031).
func (e *env) progressed(s student, objective, level string) outbox.Envelope {
	return e.envelope("learning_objective.progressed", "learning", s.tenant, map[string]any{"student_id": s.id, "objective_id": objective,
		"from": 0.4, "to": 0.8, "level": level, "model_version": "bkt.v1", "source_event_id": uuid.New()})
}

func (e *env) mastery(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	c := consumer.CollectionOnMastery{Cards: e.eco().eco}
	if err := e.inTenant(ctx, t, *env.TenantID, func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
		t.Fatalf("%s: %v", env.EventType, err)
	}
}

// F075-AC-18, AC-19, AC-20 (TC-ECO-006): learning_objective.progressed at proficient grants the knowledge card once and a
// lower level later takes nothing back; every card added moves the collection progress (collection.item_added); the last
// one completes it once (collection.completed, completion rewards granted once); a hidden_until_first collection is shown
// from the first item, with the public "how to obtain" of each item (vi/en).
func TestKnowledgeCardsAndCollection(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.do(t, http.MethodGet, "/game/collections", e.gtoken(an), th(an), nil)
	if r.status != http.StatusOK || len(r.body["data"].([]any)) != 0 {
		t.Fatalf("hidden collection listed: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/game/collections/"+cardsColl, e.gtoken(an), th(an), nil); r.status != http.StatusNotFound {
		t.Fatalf("hidden collection readable: %d %s", r.status, r.raw)
	}
	e.mastery(ctx, t, e.progressed(an, cardObjective, "developing")) // below proficient: nothing
	if e.ownedOf(ctx, t, an, "card_times_table_7") != 0 {
		t.Fatal("card at developing")
	}
	e.mastery(ctx, t, e.progressed(an, cardObjective, "proficient"))
	e.mastery(ctx, t, e.progressed(an, cardObjective, "developing")) // decay: the card stays
	e.mastery(ctx, t, e.progressed(an, cardObjective, "mastered"))   // once per objective
	if e.ownedOf(ctx, t, an, "card_times_table_7") != 1 || e.balanceOf(ctx, t, an) != 0 {
		t.Fatalf("card: owned %d, Xu %d", e.ownedOf(ctx, t, an, "card_times_table_7"), e.balanceOf(ctx, t, an))
	}
	if n := e.events(ctx, t, "collection.item_added"); n != 1 {
		t.Fatalf("collection.item_added %d", n)
	}
	r = e.do(t, http.MethodGet, "/game/collections", e.gtoken(an), th(an), nil)
	data := r.body["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["owned_count"] != float64(1) || data[0].(map[string]any)["total_count"] != float64(2) ||
		data[0].(map[string]any)["type"] != "knowledge_card" {
		t.Fatalf("collections: %s", r.raw)
	}
	r = e.do(t, http.MethodGet, "/game/collections/"+cardsColl, e.gtoken(an), map[string]string{"X-Tenant-Id": an.tenant.String(), "Accept-Language": "en-US"}, nil)
	items := r.body["items"].([]any)
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if r.status != http.StatusOK || first["owned"] != true || second["owned"] != false || first["how_to_obtain"] != "Master the 7 times table" ||
		second["how_to_obtain"] == "" || len(r.body["completion_rewards"].([]any)) != 1 {
		t.Fatalf("detail: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/game/collections/"+cardsColl, e.gtoken(other), th(other), nil); r.status != http.StatusNotFound {
		t.Fatalf("hidden for the other student: %d", r.status)
	}
	e.mastery(ctx, t, e.progressed(an, cardObjective2, "proficient"))
	e.mastery(ctx, t, e.progressed(an, cardObjective2, "mastered"))
	if n := e.events(ctx, t, "collection.completed"); n != 1 {
		t.Fatalf("collection.completed %d", n)
	}
	if got := e.balanceOf(ctx, t, an); got != 50 {
		t.Fatalf("completion reward: %d", got)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1 AND trigger_kind = 'collection'`, an.id); n != 1 {
		t.Fatalf("completion grants %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM gamification.reward_grants WHERE student_id = $1 AND trigger_kind = 'knowledge_card'`, an.id); n != 2 {
		t.Fatalf("card grants %d", n)
	}
	r = e.do(t, http.MethodGet, "/game/collections", e.gtoken(an), th(an), nil)
	if c := r.body["data"].([]any)[0].(map[string]any); c["completed_at"] == nil || c["owned_count"] != float64(2) {
		t.Fatalf("completed: %+v", c)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["collection.item_added"] != 2 || seen["collection.completed"] != 1 {
		t.Fatalf("schemas %v", seen)
	}
}
