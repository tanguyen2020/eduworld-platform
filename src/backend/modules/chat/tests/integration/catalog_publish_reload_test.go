//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	chatmod "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
)

const (
	phPubWord = "zqpobterm"
	phPubCrit = "zqpubcrit"
)

// instance is one more instance of module chat on the same database and Valkey (its own compiled catalogue store).
func (c *chatEnv) instance() *chatmod.Pipeline {
	c.t.Helper()
	p, err := chatmod.Real(chatmod.Deps{Read: c.read, Cache: c.facts.Cache, Valkey: c.vk, UoW: c.uow,
		Events: persistence.Publisher{W: c.writer}, Audit: persistence.Auditor{R: c.rec}, Now: c.clock.Now})
	if err != nil {
		c.t.Fatal(err)
	}
	if err := p.Catalogs.Warm(context.Background()); err != nil {
		c.t.Fatal(err)
	}
	return p
}

func view(t *testing.T, p *chatmod.Pipeline, tenant [16]byte) service.CatalogView {
	t.Helper()
	v, err := p.Catalogs.Get(context.Background(), tenant)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// decide evaluates text against a catalogue view with every step on (what a message in flight with that view gets).
func decide(p *chatmod.Pipeline, v service.CatalogView, text string) string {
	e := p.Validator.Engine
	return e.Evaluate(moderation.Input{View: e.View(text), Strictness: moderation.StrictnessStandard,
		Switches: moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}, Categories: v.Categories, Catalog: v.Parts}).Decision
}

// T020 / F116-AC-14, F116-AC-15 (TC-CHAT-012): publishSensitiveCatalog runs the regression suite on the candidate — a
// failing filter_quality_gate is 422 SENSITIVE_CATALOG_REGRESSION_FAILED without a version; a passing one writes the
// next version with its sha256 checksum and regression numbers, marks the terms published and raises
// sensitive_catalog.published. Two instances listening to ew:chat:sensitive:version apply it well within 60 s, a third
// one without the notification catches up on reconcile, a message in flight keeps the view it got, and a student of two
// tenants is then blocked on a written variant of the new term.
func TestCatalogPublishReload(t *testing.T) {
	c := setupChat(t, chatOpts{real: true})
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	second, quiet := c.instance(), c.instance()
	go func() { _ = c.pipe.Run(ctx) }()
	go func() { _ = second.Run(ctx) }()
	time.Sleep(300 * time.Millisecond) // subscriptions registered

	inflightA, inflightB, quietA := view(t, c.pipe, c.tenantA), view(t, second, c.tenantB), view(t, quiet, c.tenantA)
	if inflightA.PlatformVersion != 1 || inflightB.PlatformVersion != 1 || quietA.PlatformVersion != 1 {
		t.Fatalf("seed version: %d %d %d", inflightA.PlatformVersion, inflightB.PlatformVersion, quietA.PlatformVersion)
	}
	versions := func() int {
		return c.count(`SELECT count(*) FROM chat.sensitive_catalog_versions WHERE tenant_id IS NULL`)
	}
	sa := c.platformStaff("system_admin")
	for _, in := range []map[string]any{termInput(catInsult, phPubWord, "word"), termInput(catSelfHarm, phPubCrit, "word")} {
		if r := c.admin(sa, http.MethodPost, "/admin/chat/sensitive-terms", in); r.status != http.StatusCreated {
			t.Fatalf("create term: %d %s", r.status, r.raw)
		}
	}

	// An allowlist entry hiding the critical term: recall under 0.95 → 422, no version, terms still drafts.
	r := c.admin(sa, http.MethodPost, "/admin/chat/sensitive-allowlist", map[string]any{"phrase": phPubCrit, "locale": "vi",
		"reason": "ngoại lệ sai để thử"})
	entry := idOf(r, "entry_id")
	r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-catalog:publish", map[string]any{"change_summary": "thêm term thử nghiệm"})
	if r.status != http.StatusUnprocessableEntity || r.code() != "SENSITIVE_CATALOG_REGRESSION_FAILED" {
		t.Fatalf("regression gate: %d %s", r.status, r.raw)
	}
	if reg, _ := r.body["regression"].(map[string]any); reg == nil || reg["recall"].(float64) >= moderation.RegressionMinRecall {
		t.Fatalf("regression numbers: %s", r.raw)
	}
	if strings.Contains(r.raw, "zq") || versions() != 1 || c.count(`SELECT count(*) FROM chat.sensitive_terms WHERE status = 'published'`) != 0 {
		t.Fatalf("failed publish left a trace: %s", r.raw)
	}
	if len(c.events("sensitive_catalog.published")) != 0 {
		t.Fatal("failed publish raised sensitive_catalog.published")
	}

	// Gate passed: version 2, checksum of the snapshot, regression numbers, terms published.
	if r = c.admin(sa, http.MethodDelete, "/admin/chat/sensitive-allowlist/"+entry, nil); r.status != http.StatusNoContent {
		t.Fatalf("remove allowlist entry: %s", r.raw)
	}
	r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-catalog:publish", map[string]any{"change_summary": "thêm term thử nghiệm"})
	published := time.Now()
	reg, _ := r.body["regression"].(map[string]any)
	if r.status != http.StatusCreated || r.body["catalog_version"] != float64(2) || r.body["scope"] != "platform" ||
		r.body["term_count"] != float64(2) || reg["recall"] != float64(1) || reg["false_positive_rate"] != float64(0) {
		t.Fatalf("publish: %d %s", r.status, r.raw)
	}
	var raw []byte
	var sum string
	var recall, fp float64
	if err := c.su.QueryRowContext(ctx, `SELECT snapshot::text, checksum, regression_recall, regression_fp_rate
		FROM chat.sensitive_catalog_versions WHERE tenant_id IS NULL AND catalog_version = 2`).Scan(&raw, &sum, &recall, &fp); err != nil {
		t.Fatal(err)
	}
	snap, err := catalog.ParseSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, want, _ := catalog.Encode(snap); want != sum || len(sum) != 64 || recall != 1 || fp != 0 || snap.Version != 2 || len(snap.Terms) != 2 {
		t.Fatalf("version row: checksum %s (want %s) recall %v fp %v version %d terms %d", sum, want, recall, fp, snap.Version, len(snap.Terms))
	}
	if c.count(`SELECT count(*) FROM chat.sensitive_terms WHERE status = 'published' AND published_in_version = 2`) != 2 {
		t.Fatal("terms not marked published")
	}
	if seen := c.checkEvents("sensitive_catalog.published"); seen["sensitive_catalog.published"] != 1 {
		t.Fatalf("sensitive_catalog.published: %v", seen)
	}
	if !slices.Contains(c.audits(), "chat.sensitive_catalog.published") {
		t.Fatal("publish not audited")
	}
	// A second publish without change is the next version (monotonic per scope).
	if r = c.admin(sa, http.MethodPost, "/admin/chat/sensitive-catalog:publish", map[string]any{"change_summary": "phát hành lại"}); r.body["catalog_version"] != float64(3) {
		t.Fatalf("next version: %s", r.raw)
	}

	// Every listening instance applies the new version within moderation_config_propagation_max (pub/sub).
	deadline := published.Add(moderation.PropagationMax)
	for {
		if view(t, c.pipe, c.tenantA).PlatformVersion == 3 && view(t, second, c.tenantB).PlatformVersion == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("instances did not reload within %s", moderation.PropagationMax)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if took := time.Since(published); took > moderation.PropagationMax {
		t.Fatalf("propagation took %s", took)
	}
	// The views taken before the publish (messages in flight) keep the old catalogue; new views use the new one.
	if inflightA.PlatformVersion != 1 || decide(c.pipe, inflightA, "nói "+phPubWord) != moderation.DecisionAllow {
		t.Fatal("a message in flight changed catalogue")
	}
	if decide(c.pipe, view(t, c.pipe, c.tenantA), "nói "+phPubWord) != moderation.DecisionBlock {
		t.Fatal("new view does not block the published term")
	}
	// An instance that missed the notification catches up on its periodic reconcile.
	if view(t, quiet, c.tenantA).PlatformVersion != 1 {
		t.Fatal("the quiet instance got the notification")
	}
	quiet.Catalogs.Reconcile(ctx)
	if view(t, quiet, c.tenantA).PlatformVersion != 3 {
		t.Fatal("reconcile did not load the new version")
	}

	// Students of two tenants: written variants (letters separated by dots, upper case) of the new term are blocked.
	for i, tn := range []struct {
		tenant, school [16]byte
		text           string
	}{{c.tenantA, c.schoolA, "z.q.p.o.b.t.e.r.m"}, {c.tenantB, c.schoolB, "ZQPOBTERM"}} {
		s1, s2 := c.student(tn.tenant, tn.school), c.student(tn.tenant, tn.school)
		ch := c.open(channel.Party, s1, s2)
		reply := c.sendWS(s1, "conn-pub-"+string(rune('a'+i)), ch, tn.text)
		if reply.OK || replyCode(reply) != "CHAT_CONTENT_BLOCKED" || reply.Problem.Details["category_code"] != "insult" {
			t.Fatalf("tenant %d variant: %+v", i, reply.Problem)
		}
	}
	c.outboxLeaks(phPubWord, phPubCrit)
}
