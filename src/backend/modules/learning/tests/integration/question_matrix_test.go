//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// Route matrix of F022 (F002-AC-28, NFR-SEC-006, TC-TEN-001, F022-AC-09): for every question operation, a principal
// without the permission is refused (role_denied) and a principal of tenant B never reaches a question of tenant A
// (cross_tenant: 404, or a search without it).
func TestQuestionRouteMatrix(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	qA := must(t, c.createQ(t, qlcmA, qc.body(qbFixture(t, "numeric"), nil)), http.StatusCreated, "question A").str("content_id")
	path := "/cms/learning/questions/" + qA
	stale := ifm(`W/"1"`)
	body := qc.body(qbFixture(t, "numeric"), nil)
	type call struct {
		user   string
		method string
		path   string
		h      hdr
		body   any
		want   int
	}
	ops := []struct {
		id            string
		denied, cross call
		check         func(resp) bool
	}{
		{"searchQuestions", call{noRoleA, "GET", "/cms/learning/questions", nil, nil, 403}, call{qlcmB, "GET", "/cms/learning/questions", nil, nil, 200},
			func(r resp) bool { return len(r.items("data")) == 0 }},
		{"createQuestion", call{noRoleA, "POST", "/cms/learning/questions", idem(), body, 403},
			// A question of tenant B cannot point at the curriculum of tenant A (not assigned to B).
			call{qlcmB, "POST", "/cms/learning/questions", idem(), body, 422}, nil},
		{"getCmsQuestion", call{noRoleA, "GET", path, nil, nil, 403}, call{qlcmB, "GET", path, nil, nil, 404}, nil},
		{"deleteQuestion", call{cmDuyet, "DELETE", path, stale, nil, 403}, call{qlcmB, "DELETE", path, stale, nil, 404}, nil},
		{"getCmsQuestionVersion", call{noRoleA, "GET", path + "/versions/1", nil, nil, 403}, call{qlcmB, "GET", path + "/versions/1", nil, nil, 404}, nil},
		{"updateQuestionDraft", call{noRoleA, "PATCH", path + "/versions/1", stale, map[string]any{"difficulty": 3}, 403},
			call{qlcmB, "PATCH", path + "/versions/1", stale, map[string]any{"difficulty": 3}, 404}, nil},
		{"createQuestionVersion", call{noRoleA, "POST", path + "/versions", idem(), map[string]any{"from_version": 1}, 403},
			call{qlcmB, "POST", path + "/versions", idem(), map[string]any{"from_version": 1}, 404}, nil},
	}
	for _, o := range ops {
		t.Run(o.id, func(t *testing.T) {
			must(t, c.do(t, o.denied.method, o.denied.path, o.denied.user, o.denied.h, o.denied.body), o.denied.want, "role denied")
			r := must(t, c.do(t, o.cross.method, o.cross.path, o.cross.user, o.cross.h, o.cross.body), o.cross.want, "cross tenant")
			if o.check != nil && !o.check(r) {
				t.Fatalf("cross tenant answer: %s", r.raw)
			}
		})
	}
	authztest.Cover(t, "searchQuestions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createQuestion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsQuestion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteQuestion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCmsQuestionVersion", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateQuestionDraft", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createQuestionVersion", authztest.CrossTenant, authztest.RoleDenied)
}
