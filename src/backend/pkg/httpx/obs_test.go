package httpx_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// The server labels metrics/logs with the route pattern, module and ProblemCode (F002-AC-20, F002-AC-24).
func TestServerFeedsObservability(t *testing.T) {
	e := newEnv(t)
	var buf bytes.Buffer
	obs, err := observability.New(observability.Options{Logger: observability.NewLogger(&buf, observability.LoggerOptions{Binary: "core-api"}),
		Registerer: prometheus.NewRegistry(), Binary: "core-api"})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := httpx.NewServer(httpx.Deps{Auth: authCfg(e.idp), Keys: e.jwks, Resolver: e.res, Clock: e.clk, Middlewares: []func(http.Handler) http.Handler{obs.HTTP}})
	if err != nil {
		t.Fatal(err)
	}
	op := cmsOp
	op.Module = "learning"
	op.Pattern = "/cms/learning/questions/{question_id}"
	if err := srv.Handle(op, http.HandlerFunc(echo)); err != nil {
		t.Fatal(err)
	}
	e.srv = srv
	ta := uuid.New()
	c := e.idp.baseClaims(gameClient)
	e.res.set(c["sub"].(string), authz.PrincipalData{UserID: uuid.New(), UserType: authz.UserTypeStudent, TenantID: &ta})
	r := e.do(t, http.MethodGet, "/api/v1/cms/learning/questions/"+uuid.NewString(), e.idp.sign(tok{claims: c}), map[string]string{"X-Tenant-Id": ta.String()})
	if r.code != "PORTAL_ACCESS_STAFF_ONLY" {
		t.Fatalf("code %q", r.code)
	}
	n := testutil.ToFloat64(obs.RequestsCounter().WithLabelValues("learning", "/api/v1/cms/learning/questions/{question_id}", "GET", "4xx", "PORTAL_ACCESS_STAFF_ONLY"))
	if n != 1 {
		t.Fatalf("metric = %v", n)
	}
	if !strings.Contains(buf.String(), `"error.code":"PORTAL_ACCESS_STAFF_ONLY"`) || strings.Contains(buf.String(), "eyJ") {
		t.Fatalf("access log: %s", buf.String())
	}
	// 404 for unknown paths is labelled unmatched.
	e.do(t, http.MethodGet, "/api/v1/nope", "", nil)
	if n := testutil.ToFloat64(obs.RequestsCounter().WithLabelValues("", "unmatched", "GET", "4xx", "NOT_FOUND")); n != 1 {
		t.Fatalf("unmatched = %v", n)
	}
}
