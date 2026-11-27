package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/cases"
	"github.com/tanguyen2020/eduworld-platform/src/backend/test/security/authz-matrix/internal/spec"
)

const (
	tenantA = "aaaaaaaa-0000-4000-8000-000000000001"
	tenantB = "bbbbbbbb-0000-4000-8000-000000000002"
	idOfB   = "bbbbbbbb-0000-4000-8000-0000000000b1"
)

// standIn is a STAND-IN gateway for the runner tests only (AGENTS R17): it reproduces the answers of pkg/httpx
// (guards, tenant header, contract check) for the fixture contract. It is not the platform and proves nothing
// about it — the staging run of `make authz-matrix ENV=staging` does.
type standIn struct {
	leakCrossTenant bool // answer 200 to a resource of tenant B (the bug the matrix must catch)
	serveFacade     bool // answer 200 on /internal/v1/* (the bug SCOPE=internal must catch)
	dropMaxLength   bool // 422 without naming the field
	hideInternal    bool // public gateway: x-internal routes (/game-admin/*) are not routed
}

func problem(w http.ResponseWriter, status int, code string, fields ...map[string]string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "code": code, "errors": fields})
}

func (s standIn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/api/v1")
	if strings.HasPrefix(r.URL.Path, "/internal/") {
		if s.serveFacade {
			w.WriteHeader(200)
			return
		}
		problem(w, 404, "NOT_FOUND")
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") || (s.hideInternal && strings.HasPrefix(r.URL.Path, "/api/v1/game-admin/")) {
		problem(w, 404, "NOT_FOUND")
		return
	}
	switch p {
	case "/auth/game/login":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if u, _ := in["username"].(string); len(u) > 128 {
			problem(w, 422, "VALIDATION_FAILED", map[string]string{"field": "username", "code": "MAX_LENGTH"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tokens": map[string]string{"access_token": "tok-student"}})
		return
	case "/auth/portal/login":
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok-" + strings.TrimPrefix(in["username"], "user-")})
		return
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer tok-")
	if auth == "" || auth == r.Header.Get("Authorization") {
		problem(w, 401, "UNAUTHENTICATED")
		return
	}
	role := cases.Role(auth)
	tenantScoped := !strings.HasPrefix(p, "/game-admin/") && p != "/me/tenants"
	hdr := r.Header.Get("X-Tenant-Id")
	if tenantScoped && hdr == "" && !strings.HasPrefix(p, "/leaderboards/") {
		problem(w, 400, "TENANT_HEADER_REQUIRED")
		return
	}
	if hdr != "" && hdr != tenantA {
		problem(w, 403, "TENANT_MISMATCH")
		return
	}
	switch {
	case strings.HasPrefix(p, "/game/") && role.Staff():
		problem(w, 403, "GAME_ACCESS_STUDENT_ONLY")
		return
	case (strings.HasPrefix(p, "/cms/") || strings.HasPrefix(p, "/game-admin/")) && role == cases.Student:
		problem(w, 403, "PORTAL_ACCESS_STAFF_ONLY")
		return
	case strings.HasPrefix(p, "/game-admin/") && !role.Platform():
		problem(w, 403, "PERMISSION_DENIED")
		return
	}
	if r.Method == http.MethodPatch {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if t, _ := in["title"].(string); len(t) > 255 {
			if s.dropMaxLength {
				problem(w, 422, "VALIDATION_FAILED", map[string]string{"field": "body", "code": "INVALID"})
			} else {
				problem(w, 422, "VALIDATION_FAILED", map[string]string{"field": "title", "code": "MAX_LENGTH"})
			}
			return
		}
	}
	if strings.Contains(p, idOfB) && !s.leakCrossTenant {
		problem(w, 404, "NOT_FOUND")
		return
	}
	w.WriteHeader(200)
	_, _ = io.WriteString(w, "{}")
}

func fixtures() Fixtures {
	f := Fixtures{ClientVersion: "1.0.0", Platform: "web", Accounts: map[cases.Role]Account{}}
	f.Tenants.A, f.Tenants.B = tenantA, tenantB
	f.Accounts[cases.Student] = Account{Username: "hs.fixture", Kind: "game"}
	for _, r := range cases.Roles[1:] {
		f.Accounts[r] = Account{Username: "user-" + string(r), Kind: "portal"}
	}
	f.IDs = map[string]map[string]string{"b": {"player_id": idOfB, "thing_id": idOfB, "leaderboard_id": idOfB}}
	return f
}

func newRunner(t *testing.T, s standIn) (*Runner, []spec.Operation) {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	ops, err := spec.Load(context.Background(), "../spec/testdata/mini.yaml", "../spec/testdata/mini_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	return &Runner{BaseURL: srv.URL + "/api/v1", Fixtures: fixtures(), Creds: func(cases.Role) (string, string) { return "", "pw" }}, ops
}

func count(rs []Result, v Verdict) int {
	n := 0
	for _, r := range rs {
		if r.Verdict == v {
			n++
		}
	}
	return n
}

func TestRunMatrixAgainstStandIn(t *testing.T) {
	r, ops := newRunner(t, standIn{})
	rs := r.RunMatrix(context.Background(), cases.Generate(ops, cases.Overrides{}))
	for _, x := range rs {
		if x.Verdict == Fail || x.Verdict == Skipped {
			t.Fatalf("unexpected %s: %+v", x.Verdict, x)
		}
	}
	if count(rs, Pass) == 0 || count(rs, Observed) == 0 {
		t.Fatalf("pass=%d observed=%d", count(rs, Pass), count(rs, Observed))
	}
}

// A 2xx on a resource of another tenant is a failed case flagged as a leak.
func TestRunMatrixCatchesLeak(t *testing.T) {
	r, ops := newRunner(t, standIn{leakCrossTenant: true})
	rep := &Report{Results: r.RunMatrix(context.Background(), cases.Generate(ops, cases.Overrides{}))}
	rep.Summarise()
	if rep.Summary.Fail == 0 {
		t.Fatal("leak not caught")
	}
	leak := false
	for _, x := range rep.Results {
		if x.Verdict == Fail && strings.HasPrefix(x.Note, "LEAK") && x.Relation == cases.OtherTenant {
			leak = true
		}
	}
	if !leak || rep.Gate(false) == nil {
		t.Fatalf("leak=%v gate=%v", leak, rep.Gate(false))
	}
	var md bytes.Buffer
	if err := rep.WriteMarkdown(&md); err != nil || !strings.Contains(md.String(), "KHÔNG ĐẠT") || strings.Contains(md.String(), "tok-") {
		t.Fatalf("markdown: %v\n%s", err, md.String())
	}
}

func TestRunMatrixWithoutCredentialsSkips(t *testing.T) {
	r, ops := newRunner(t, standIn{})
	r.Creds = func(cases.Role) (string, string) { return "", "" }
	rep := &Report{Results: r.RunMatrix(context.Background(), cases.Generate(ops, cases.Overrides{}))}
	rep.Summarise()
	if rep.Summary.Skipped == 0 || rep.Gate(true) == nil {
		t.Fatalf("skipped=%d strict gate=%v", rep.Summary.Skipped, rep.Gate(true))
	}
}

func TestRunInternal(t *testing.T) {
	paths, err := spec.InternalPaths("../spec/testdata/modules")
	if err != nil {
		t.Fatal(err)
	}
	// a port nobody listens on: the direct probe must pass on a refused connection
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := l.Addr().String()
	_ = l.Close()

	r, ops := newRunner(t, standIn{hideInternal: true})
	internal := httptest.NewServer(standIn{})
	defer internal.Close()
	r.InternalURL = internal.URL + "/api/v1" // separate internal gateway: x-internal probes run
	rs := r.RunInternal(context.Background(), cases.InternalProbes(ops, paths), []DirectTarget{DirectTarget(closed)}, paths)
	if count(rs, Fail) != 0 || count(rs, Pass) == 0 {
		t.Fatalf("clean target: %+v", rs)
	}

	bad, _ := newRunner(t, standIn{serveFacade: true})
	rs = bad.RunInternal(context.Background(), cases.InternalProbes(ops, paths), nil, paths)
	if count(rs, Fail) == 0 {
		t.Fatal("facade served over HTTP not caught")
	}
	// without a separate internal gateway the x-internal probes are skipped, not failed
	for _, x := range rs {
		if x.Rule == "x_internal_not_public" && x.Verdict != Skipped {
			t.Fatalf("x-internal probe without gateway split: %+v", x)
		}
	}
}

func TestRunLength(t *testing.T) {
	r, ops := newRunner(t, standIn{})
	ls, _ := cases.LengthCases(ops)
	var picked []cases.LengthCase
	for _, c := range ls {
		if (c.OpID == "gameLogin" && c.Field == "username") || c.OpID == "updateThing" {
			picked = append(picked, c)
		}
	}
	rs := r.RunLength(context.Background(), picked, cases.HeaderCases(ops), cases.Overrides{})
	for _, x := range rs {
		if x.Verdict != Pass {
			t.Fatalf("%+v", x)
		}
	}
	if len(rs) != 2+3 {
		t.Fatalf("results = %d", len(rs))
	}
	bad, _ := newRunner(t, standIn{dropMaxLength: true})
	rs = bad.RunLength(context.Background(), picked, nil, cases.Overrides{})
	if count(rs, Fail) != 1 {
		t.Fatalf("422 without field not caught: %+v", rs)
	}
}

func TestEnvCredentials(t *testing.T) {
	env := map[string]string{"EW_AUTHZ_TOKEN_STUDENT": "t1", "EW_AUTHZ_PASSWORD_SCHOOL_ADMIN": "p2"}
	c := EnvCredentials(func(k string) string { return env[k] })
	if tok, _ := c(cases.Student); tok != "t1" {
		t.Fatal(tok)
	}
	if _, pw := c(cases.SchoolAdmin); pw != "p2" {
		t.Fatal(pw)
	}
	if tok, pw := c(cases.SystemAdmin); tok != "" || pw != "" {
		t.Fatal(fmt.Sprint(tok, pw))
	}
}
