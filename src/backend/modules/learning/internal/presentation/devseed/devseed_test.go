package devseed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
			if in["password"] != "pw" || in["device_id"] == "" || r.Header.Get("Authorization") != "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"INVALID_CREDENTIALS"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-` + in["username"].(string) + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/tenants":
			lookups++
			_, _ = w.Write([]byte(`{"data":[{"tenant_id":"` + tenantA + `","code":"th-demo-a","status":"active"},
				{"tenant_id":"` + tenantB + `","code":"pgd-demo","status":"active"},{"tenant_id":"x","code":"th-demo-a-old","status":"disabled"}]}`))
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
	missing := &AdminAssigner{HTTP: srv.Client(), BaseURL: base, Token: tok, Codes: []string{"khong-co"}}
	if err := missing.Assign(ctx, "lesson", id); err == nil {
		t.Fatal("unknown tenant code accepted")
	}
}
