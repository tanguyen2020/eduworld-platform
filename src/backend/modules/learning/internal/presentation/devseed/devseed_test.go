package devseed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// The seed file of the repository is valid and keeps the dev markers (R2: content only in seed data files).
func TestSeedFileValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "seed", "content", "math_g3.dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	if f.GradeLevel != 3 || f.SubjectCode != "math" || len(f.TenantCodes) == 0 || !strings.HasPrefix(f.QuestionSet.Title, "[DEV]") {
		t.Fatalf("unexpected seed file %+v", f)
	}
	// The pool of the DEV mini-games (seed/minigames of module game): true_false_race draws 5 true_false of difficulty
	// 1–3, multiplication_card_match 3 matching of difficulty 2–3; no pool question is also a question of the set.
	types := map[string]int{}
	for _, q := range f.PoolQuestions {
		d, _ := q.Body["difficulty"].(float64)
		switch q.Body["type"] {
		case "true_false":
			if d >= 1 && d <= 3 {
				types["true_false"]++
			}
		case "matching":
			if d >= 2 && d <= 3 {
				types["matching"]++
			}
		}
	}
	if types["true_false"] < 5 || types["matching"] < 3 || !slices.Contains(f.TenantCodes, "common") {
		t.Fatalf("mini-game pool %v, tenants %v", types, f.TenantCodes)
	}
	dup := f
	dup.PoolQuestions = append(slices.Clone(f.PoolQuestions), f.Questions[0])
	if dup.Validate() == nil {
		t.Fatal("pool question key equal to a question key accepted")
	}
	f.Questions = append(f.Questions, f.Questions[0])
	if f.Validate() == nil {
		t.Fatal("duplicate question key accepted")
	}
}

// MOCK (TEST FAKE) of the portal login and the System Admin API: PortalLogin sends no password anywhere else than the
// body, AdminAssigner resolves the demo tenants by code once and assigns with an Idempotency-Key.
func TestPortalLoginAndAdminAssigner(t *testing.T) {
	var mu sync.Mutex
	var assigned []map[string]any
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	lookups := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/portal/login":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			// locale must be a contract Locale (vi-VN|en-US, ADR-0056), not a ContentLanguage
			if in["password"] != "pw" || in["device_id"] == "" || r.Header.Get("Authorization") != "" ||
				(in["locale"] != "vi-VN" && in["locale"] != "en-US") {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"INVALID_CREDENTIALS"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-` + in["username"].(string) + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/tenants":
			lookups++
			_, _ = w.Write([]byte(`{"data":[{"tenant_id":"` + tenantA + `","code":"th-demo-a","status":"active"},
				{"tenant_id":"` + tenantB + `","code":"pgd-demo","status":"active"},{"tenant_id":"x","code":"th-demo-a-old","status":"disabled"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/tenant-assignments":
			// listTenantAssignments: the tenants already holding the resource (from the assignments recorded so far).
			var rows []string
			for _, in := range assigned {
				if in["resource_type"] == r.URL.Query().Get("resource_type") && in["resource_id"] == r.URL.Query().Get("resource_id") {
					for _, t := range in["tenant_ids"].([]any) {
						rows = append(rows, `{"tenant_id":"`+t.(string)+`"}`)
					}
				}
			}
			_, _ = w.Write([]byte(`{"data":[` + strings.Join(rows, ",") + `],"page":{"has_more":false,"limit":100}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/tenant-assignments":
			if r.Header.Get("Authorization") != "Bearer tok-sa.admin" || r.Header.Get("Idempotency-Key") == "" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			assigned = append(assigned, in)
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	base := srv.URL + "/api/v1"
	if _, err := PortalLogin(ctx, srv.Client(), base, "sa.admin", "wrong"); err == nil || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("wrong password: %v", err)
	}
	tok, err := PortalLogin(ctx, srv.Client(), base, "sa.admin", "pw")
	if err != nil || tok != "tok-sa.admin" {
		t.Fatalf("login: %q %v", tok, err)
	}
	a := &AdminAssigner{HTTP: srv.Client(), BaseURL: base, Token: tok, Codes: []string{"th-demo-a", "pgd-demo"}}
	id := uuid.New()
	for _, rt := range []string{"curriculum", "question_set"} {
		if err := a.Assign(ctx, rt, id); err != nil {
			t.Fatal(err)
		}
	}
	if lookups != 2 || len(assigned) != 2 {
		t.Fatalf("lookups %d (want 2, one per code), assignments %d", lookups, len(assigned))
	}
	ts, _ := assigned[0]["tenant_ids"].([]any)
	if assigned[0]["resource_type"] != "curriculum" || assigned[0]["resource_id"] != id.String() || len(ts) != 2 || ts[0] != tenantA || ts[1] != tenantB {
		t.Fatalf("assignment body %v", assigned[0])
	}
	// Already assigned to every demo tenant: nothing is sent again (a second seed run writes no tenant_data.assigned).
	if err := a.Assign(ctx, "curriculum", id); err != nil || len(assigned) != 2 {
		t.Fatalf("second assign: %v, assignments %d", err, len(assigned))
	}
	missing := &AdminAssigner{HTTP: srv.Client(), BaseURL: base, Token: tok, Codes: []string{"khong-co"}}
	if err := missing.Assign(ctx, "lesson", id); err == nil {
		t.Fatal("unknown tenant code accepted")
	}
}
