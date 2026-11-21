package query

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TEST DOUBLES (R17): toggles per scope and a workspace catalog by permission prefix.
type togglesFake struct {
	byScope map[string][2][]string
	err     error
}

func (f togglesFake) Disabled(_ context.Context, tenant *uuid.UUID) ([]string, []string, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	key := "platform"
	if tenant != nil {
		key = tenant.String()
	}
	v := f.byScope[key]
	return v[0], v[1], nil
}

type workspacesFake struct{}

func (workspacesFake) Workspaces(perms []string) []string {
	if slices.Contains(perms, "lms.student.read") {
		return []string{"school_lms"}
	}
	return []string{}
}

type meModelFake struct{ read.MeReadModel }

// F012-AC-39 (TC-CMS-MENU-06): disabled menus/actions of the session scope; F012-AC-10 no platform permission.
func TestGetMyPermissionsDisabled(t *testing.T) {
	tenant := uuid.New()
	toggles := togglesFake{byScope: map[string][2][]string{
		tenant.String(): {{"menu.school_lms.reports"}, {"action.school_lms.reports.export", "action.school_lms.assignments.create"}},
		"platform":      {{"menu.game_ops.quests"}, {"action.game_ops.quests.publish"}},
	}}
	q := Me{Model: meModelFake{}, Workspaces: workspacesFake{}, Toggles: toggles}
	ctx := authztest.Ctx(context.Background(), authztest.Staff(tenant, "school_admin"))
	out, err := q.GetMyPermissions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out.TenantID == nil || *out.TenantID != tenant || !slices.Equal(out.DisabledMenus, []string{"menu.school_lms.reports"}) ||
		len(out.DisabledActions) != 2 || !slices.Equal(out.Workspaces, []string{"school_lms"}) {
		t.Fatalf("tenant session = %+v", out)
	}
	for _, p := range out.Permissions {
		if authz.Permission(p).PlatformOnly() {
			t.Fatalf("platform permission %s in a tenant session", p)
		}
	}
	// System Admin without header: the platform defaults.
	sa := authztest.Ctx(context.Background(), authztest.Platform("system_admin"))
	if out, err = q.GetMyPermissions(sa); err != nil || out.TenantID != nil || !slices.Equal(out.DisabledMenus, []string{"menu.game_ops.quests"}) {
		t.Fatalf("platform session = %+v %v", out, err)
	}
	// The source unavailable: 503, never an empty list (no fail-open of the menu state).
	q.Toggles = togglesFake{err: errors.New("down")}
	var pe *problem.Error
	if _, err := q.GetMyPermissions(ctx); !errors.As(err, &pe) || pe.Code != problem.Unavailable {
		t.Fatalf("source down = %v", err)
	}
}
