package query_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

type fakeFlags struct{ items []settings.CatalogItem }

func (f fakeFlags) Catalog(context.Context) (string, []settings.CatalogItem, error) {
	return "v1", f.items, nil
}

func (fakeFlags) Flags(context.Context) ([]settings.Flag, error) { return nil, nil }

// listFeatureFlags pages by key: the next_cursor of a full page is accepted by the next call and every catalog
// action is reached exactly once (the second page used to answer 400 "invalid cursor").
func TestListFeatureFlagsPaging(t *testing.T) {
	var items []settings.CatalogItem
	for i := range 45 {
		items = append(items, settings.CatalogItem{Key: fmt.Sprintf("action.ws.menu%02d.view", i), Kind: "action", CanDisable: true})
	}
	items = append(items, settings.CatalogItem{Key: "menu.ws.only", Kind: "menu", CanDisable: true})
	q := query.FeatureFlags{Reader: fakeFlags{items: items}, Cursors: codec(t)}
	ctx := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	seen := map[string]int{}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("paging does not end")
		}
		out, err := q.List(ctx, "action", nil, 20, cursor)
		if err != nil {
			t.Fatalf("page %d: %v", pages+1, err)
		}
		for _, v := range out.Data {
			seen[v.FlagKey]++
			if v.FlagKind != "action" {
				t.Fatalf("kind filter: got %s", v.FlagKey)
			}
		}
		if !out.Page.HasMore {
			break
		}
		cursor = *out.Page.NextCursor
	}
	if len(seen) != 45 {
		t.Fatalf("want 45 actions, got %d", len(seen))
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("%s listed %d times", k, n)
		}
	}
}
