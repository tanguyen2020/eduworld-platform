package featuregate_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

const mergedSpec = "../../../../../specs/openapi/eduworld-api.v1.yaml"

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

type specCase struct {
	opID   string
	method string
	url    string
	action string
}

// SC-006, F012-AC-40 (test generated from OpenAPI): every operation of the merged spec with x-portal-action
// is gated — action or parent menu disabled → 403 FEATURE_DISABLED with feature_key, except can_disable =
// false actions, which are never blocked. Registration also proves every x-portal-action is in the catalog.
func TestEveryPortalOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("loads the merged spec")
	}
	doc, err := httpx.LoadSpec(context.Background(), mergedSpec)
	if err != nil {
		t.Fatal(err)
	}
	cat := loadCatalog(t)
	idp := httpxtest.NewIDP(t)
	clk := clock.NewFake(time.Now().UTC())
	src := &fakeSource{}
	gate := newGate(t, src, clk)
	res := &httpxtest.Resolver{}
	srv, err := httpx.NewServer(httpx.Deps{Auth: idp.AuthConfig(), Keys: idp.JWKS(t), Resolver: res, Clock: clk, Spec: doc, FeatureGate: gate})
	if err != nil {
		t.Fatal(err)
	}
	res.Set("sa", authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStaff, IsSystemAdmin: true,
		Grants: []authz.Grant{{Role: "system_admin", ScopeKind: authz.RoleScopePlatform}}})
	token := idp.Token("sa", httpxtest.PortalClient, clk.Now())

	var cases []specCase
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for path, item := range doc.Paths.Map() {
		for method, o := range item.Operations() {
			xa, has := o.Extensions["x-portal-action"]
			if !has {
				continue
			}
			op := httpx.Operation{ID: o.OperationID, Method: method, Pattern: path, Tenant: httpx.TenantOptional}
			if err := srv.Handle(op, ok); err != nil {
				t.Fatalf("register %s: %v", o.OperationID, err)
			}
			fill := func(param, value string) string {
				return pathParamRe.ReplaceAllStringFunc(path, func(m string) string {
					if m[1:len(m)-1] == param {
						return value
					}
					return uuid.NewString()
				})
			}
			switch v := xa.(type) {
			case string:
				cases = append(cases, specCase{o.OperationID, method, httpx.BasePath + fill("", ""), v})
			case map[string]any:
				param, _ := v["param"].(string)
				m, _ := v["map"].(map[string]any)
				for val, a := range m {
					s, _ := a.(string)
					u := fill(param, val)
					if !strings.Contains(path, "{"+param+"}") {
						u += "?" + param + "=" + val
					}
					cases = append(cases, specCase{o.OperationID + "[" + param + "=" + val + "]", method, httpx.BasePath + u, s})
				}
			default:
				t.Fatalf("%s: x-portal-action has type %T", o.OperationID, xa)
			}
		}
	}
	if len(cases) == 0 {
		t.Fatal("no operation with x-portal-action in the merged spec")
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].opID < cases[j].opID })

	call := func(c specCase) (int, string, string) {
		req := httptest.NewRequest(c.method, c.url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		code, _ := body["code"].(string)
		fk, _ := body["feature_key"].(string)
		return rec.Code, code, fk
	}
	protected := 0
	for _, c := range cases {
		a, found := cat.Action(c.action)
		if !found {
			t.Fatalf("%s: action %s not in catalog", c.opID, c.action)
		}
		for _, off := range []struct{ menus, actions []string }{{[]string{a.Menu}, nil}, {nil, []string{c.action}}} {
			src.set(nil, off.menus, off.actions)
			gate.Invalidate(nil)
			status, code, fk := call(c)
			want := c.action
			if off.menus != nil {
				want = a.Menu
			}
			if !a.CanDisable {
				if code == "FEATURE_DISABLED" {
					t.Errorf("%s %s: protected action %s blocked", c.method, c.url, c.action)
				}
				continue
			}
			if status != http.StatusForbidden || code != "FEATURE_DISABLED" || fk != want {
				t.Errorf("%s (%s %s): status %d code %q feature_key %q, want 403 FEATURE_DISABLED %q", c.opID, c.method, c.url, status, code, fk, want)
			}
		}
		if !a.CanDisable {
			protected++
		}
	}
	t.Logf("%d portal operation/action pairs checked (%d with can_disable = false)", len(cases), protected)
}
