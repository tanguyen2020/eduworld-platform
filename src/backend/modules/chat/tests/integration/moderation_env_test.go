//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
)

// Seeded platform categories (seed/moderation/categories.platform.json).
var (
	catProfanity = uuid.MustParse("01936a00-0000-7000-8000-000000000101")
	catInsult    = uuid.MustParse("01936a00-0000-7000-8000-000000000102")
	catSexual    = uuid.MustParse("01936a00-0000-7000-8000-000000000104")
	catSelfHarm  = uuid.MustParse("01936a00-0000-7000-8000-000000000106")
	catPolitics  = uuid.MustParse("01936a00-0000-7000-8000-000000000108")
)

// placeholder returns a term of the test catalogue: placeholders only (R15).
func placeholder(cat uuid.UUID, term, locale string) catalog.Term {
	return catalog.Term{ID: uuid.New(), CategoryID: cat, Term: term, MatchType: catalog.MatchWord, Locale: locale}
}

// defaultTerms is the placeholder catalogue of the pipeline tests.
func defaultTerms() []catalog.Term {
	return []catalog.Term{
		placeholder(catProfanity, "zqlow", "vi"),
		placeholder(catInsult, "zqơrốp", "vi"),
		placeholder(catSexual, "zqhigh", "vi"),
		placeholder(catSelfHarm, "zqcrit", "vi"),
		placeholder(catInsult, "zqenglish", "en"),
	}
}

// publishPlatform inserts the next platform catalogue version with terms (as publishSensitiveCatalog does) and makes
// the pipeline reload it.
func (c *chatEnv) publishPlatform(terms ...catalog.Term) int64 {
	c.t.Helper()
	ctx := context.Background()
	rows, err := c.su.QueryContext(ctx, `SELECT category_id, code, severity, default_action, enabled, allow_tenant_toggle
		FROM chat.sensitive_term_categories WHERE deleted_at IS NULL ORDER BY code`)
	if err != nil {
		c.t.Fatal(err)
	}
	var cats []catalog.Category
	for rows.Next() {
		var k catalog.Category
		if err := rows.Scan(&k.ID, &k.Code, &k.Severity, &k.DefaultAction, &k.Enabled, &k.AllowTenantToggle); err != nil {
			c.t.Fatal(err)
		}
		cats = append(cats, k)
	}
	_ = rows.Close()
	var v int64
	if err := c.su.QueryRowContext(ctx, `SELECT coalesce(max(catalog_version), 0) + 1 FROM chat.sensitive_catalog_versions
		WHERE tenant_id IS NULL`).Scan(&v); err != nil {
		c.t.Fatal(err)
	}
	snap, _ := json.Marshal(catalog.Snapshot{Scope: "platform", Version: v, Categories: cats, Terms: terms, Allowlist: []catalog.AllowEntry{}})
	c.exec(`INSERT INTO chat.sensitive_catalog_versions (catalog_version_id, tenant_id, catalog_version, scope, published_at, change_summary,
		term_count, checksum, snapshot) VALUES ($1, NULL, $2, 'platform', now(), 'test', $3, 'test', $4::jsonb)`,
		uuid.New(), v, len(terms), string(snap))
	c.reload()
	return v
}

// reload makes the pipeline re-read the catalogue of the test tenants and drops the cached switches.
func (c *chatEnv) reload() {
	c.t.Helper()
	ctx := context.Background()
	for _, tn := range []uuid.UUID{c.tenantA, c.tenantB, c.tenantU} {
		tn := tn
		c.pipe.Catalogs.Invalidate(ctx, &tn)
		if err := c.facts.Cache.Invalidate(ctx, "switches:"+tn.String()); err != nil {
			c.t.Fatal(err)
		}
	}
}

// tenantSwitch configures the switches of a tenant directly (the US3 commands have their own tests).
func (c *chatEnv) tenantSwitch(tenant uuid.UUID, enabledAll bool, steps map[string]bool) {
	c.t.Helper()
	b, _ := json.Marshal(steps)
	exp := c.clock.Now().Add(time.Hour)
	c.exec(`INSERT INTO chat.moderation_switches (switch_id, tenant_id, scope, enabled_all, steps, reason, expires_at)
		VALUES ($1, $2, 'tenant', $3, $4::jsonb, 'integration test switch', $5)`, uuid.New(), tenant, enabledAll, string(b), exp)
	c.reload()
}
