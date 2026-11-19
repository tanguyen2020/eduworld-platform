//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// learningEnvelope is a release event of module learning as the relay delivers it (TEST: envelope built by hand with
// the payload of specs/events/payloads/learning/content.*.v1).
func learningEnvelope(t *testing.T, typ string, payload map[string]any) outbox.Envelope {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return outbox.Envelope{EventID: uuid.New(), EventType: typ, SchemaVersion: 1, OccurredAt: time.Now().UTC(), Producer: "learning", Payload: b}
}

// F026-AC-23…AC-26 (US6): the release events rebuild the manifest of their target tenants (new manifest_version and
// ETag, 304 on the same ETag), activity entries carry activity_type and min_client_version, other tenants never see
// the item, the entry appears within publish_to_visible_max (30 s) of released_at.
func TestManifestOnLearningContent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	h := consumer.ManifestOnLearningContent{Manifests: command.Manifests{UoW: e.uow, Store: write.ManifestRepo{}, Events: write.Publisher{W: e.writer}, Metrics: observability.ManifestMetrics{}}}
	get := func(s student, inm string) resp {
		hd := map[string]string{"X-Tenant-Id": s.tenant.String()}
		if inm != "" {
			hd["If-None-Match"] = inm
		}
		return e.do(t, http.MethodGet, "/game/content-manifest", e.device(s).token, hd, nil)
	}
	item, act := uuid.New(), uuid.New()
	v1, v2 := uuid.New(), uuid.New()
	released := time.Now().UTC()
	if err := h.Handle(ctx, learningEnvelope(t, "content.published", map[string]any{"item_type": "question", "item_id": item, "to_version_id": v1,
		"to_version": 1, "release_id": uuid.New(), "tenant_ids": []uuid.UUID{e.tenantA}, "snapshot_sha256": "aa", "released_at": released})); err != nil {
		t.Fatal(err)
	}
	r := get(an, "")
	items, _ := r.body["items"].([]any)
	if r.status != http.StatusOK || r.body["manifest_version"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["version_id"] != v1.String() {
		t.Fatalf("manifest after publish: %d %s", r.status, r.raw)
	}
	gen, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(r.body["generated_at"]))
	if !gen.IsZero() && gen.Sub(released) > 30*time.Second {
		t.Fatalf("publish → manifest %v > publish_to_visible_max", gen.Sub(released))
	}
	etag := r.header.Get("ETag")
	if r2 := get(an, etag); r2.status != http.StatusNotModified {
		t.Fatalf("304: %d", r2.status)
	}
	// AC-26: tenant B never sees tenant A's item.
	if rb := get(dung, ""); rb.body["manifest_version"] != float64(0) {
		t.Fatalf("tenant B: %s", rb.raw)
	}
	// AC-25: an activity entry with its client version; AC-23: rollback replaces, unpublish removes.
	if err := h.Handle(ctx, learningEnvelope(t, "content.published", map[string]any{"item_type": "activity", "item_id": act, "to_version_id": uuid.New(),
		"to_version": 1, "release_id": uuid.New(), "tenant_ids": []uuid.UUID{e.tenantA, e.tenantB}, "activity_type": "quiz", "min_client_version": "0.1.0"})); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(ctx, learningEnvelope(t, "content.rolled_back", map[string]any{"item_type": "question", "item_id": item, "to_version_id": v2,
		"to_version": 2, "release_id": uuid.New(), "tenant_ids": []uuid.UUID{e.tenantA}})); err != nil {
		t.Fatal(err)
	}
	r = get(an, etag)
	if r.status != http.StatusOK || r.body["manifest_version"] != float64(3) || r.header.Get("ETag") == etag {
		t.Fatalf("after rollback: %d %s", r.status, r.raw)
	}
	byID := map[string]map[string]any{}
	for _, x := range r.body["items"].([]any) {
		m := x.(map[string]any)
		byID[m["item_id"].(string)] = m
	}
	if byID[item.String()]["version_id"] != v2.String() || byID[act.String()]["activity_type"] != "quiz" || byID[act.String()]["min_client_version"] != "0.1.0" {
		t.Fatalf("entries: %s", r.raw)
	}
	if rb := get(dung, ""); rb.body["manifest_version"] != float64(1) || len(rb.body["items"].([]any)) != 1 {
		t.Fatalf("tenant B assigned activity: %s", rb.raw)
	}
	if err := h.Handle(ctx, learningEnvelope(t, "content.unpublished", map[string]any{"item_type": "question", "item_id": item, "to_version_id": nil,
		"release_id": uuid.New(), "tenant_ids": []uuid.UUID{e.tenantA}})); err != nil {
		t.Fatal(err)
	}
	r = get(an, "")
	if len(r.body["items"].([]any)) != 1 {
		t.Fatalf("after unpublish: %s", r.raw)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'content.manifest_updated'`); n != 5 {
		t.Fatalf("%d content.manifest_updated", n)
	}
}
