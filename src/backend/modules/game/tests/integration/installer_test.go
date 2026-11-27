//go:build integration

package integration

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/release"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

type installerProxy struct{ e *env }

func (p installerProxy) List(ctx context.Context, school uuid.UUID) (query.SchoolInstallerView, error) {
	if p.e.installers == nil {
		return query.SchoolInstallerView{}, problem.New(problem.Unavailable, "").Retry()
	}
	return p.e.installers.List(ctx, school)
}

// fakeCDN is a TEST DOUBLE of the release CDN (R17): it serves school-kit/production.json and its signature.
type fakeCDN struct {
	mu        sync.Mutex
	body, sig []byte
	down      bool
	hits      int
}

func (c *fakeCDN) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++
	if c.down {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	switch r.URL.Path {
	case "/releases/school-kit/production.json":
		_, _ = w.Write(c.body)
	case "/releases/school-kit/production.json.sig":
		_, _ = w.Write(c.sig)
	default:
		http.NotFound(w, r)
	}
}

type memKV struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (k *memKV) Get(_ context.Context, key string) ([]byte, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	return v, ok, nil
}

func (k *memKV) Set(_ context.Context, key string, v []byte, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = v
	return nil
}

func installerKit(id, band, released string) map[string]any {
	return map[string]any{"kit_id": id, "platform": "android", "client_version": "0.4.0", "grade_bands": []string{band},
		"url": "https://play.example.test/releases/school-kit/0.4.0/" + id + ".apk", "sha256": strings.Repeat("b", 64),
		"size_bytes": 412000000, "min_os": "Android 8.0", "released_at": released}
}

// F134-AC-13…AC-16 (TC-TEN-003, TC-TEN-024): the verified production index filtered by the grades of the school's active
// students, newest per band; bad signature / CDN down → 503 with no link; another school (same or other tenant) → 404;
// Academic Manager / student → 403; the answer holds the public kit fields only.
func TestSchoolClientInstallers(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	key, err := rsa.GenerateKey(rand.Reader, 2048) // TEST KEY generated per run
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	cdn := &fakeCDN{}
	publish := func(kits ...map[string]any) {
		b, _ := json.Marshal(map[string]any{"schema_version": 1, "channel": "production", "generated_at": "2026-11-27T02:00:00Z", "kits": kits})
		sum := sha256.Sum256(b)
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		cdn.mu.Lock()
		cdn.body, cdn.sig = b, []byte(base64.StdEncoding.EncodeToString(sig))
		cdn.mu.Unlock()
	}
	publish(installerKit("g13-old", "g1_3", "2026-11-20T00:00:00Z"), installerKit("g13", "g1_3", "2026-11-26T00:00:00Z"),
		installerKit("g45", "g4_5", "2026-11-26T00:00:00Z"), installerKit("g89", "g8_9", "2026-11-26T00:00:00Z"))
	srv := httptest.NewServer(cdn)
	t.Cleanup(srv.Close)
	kv := &memKV{m: map[string][]byte{}}
	newIndex := func() *release.SchoolKitIndex {
		ix, err := release.New(release.Config{BaseURL: srv.URL + "/releases", PublicKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})},
			nil, kv)
		if err != nil {
			t.Fatal(err)
		}
		return ix
	}
	e.installers = &query.SchoolInstallers{Read: e.read, Reads: read.Installers{}, Index: newIndex(), WebClientURL: "https://play.example.test/"}
	// School A: students in grades 3 and 4 (and a deleted one in grade 8).
	e.newStudent(ctx, t, e.tenantA, e.schoolA, "active") // grade 3
	s4 := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	gone := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.students SET grade_level = 4 WHERE student_id = $1`, s4.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.students SET grade_level = 8, deleted_at = now() WHERE student_id = $1`, gone.id); err != nil {
		t.Fatal(err)
	}
	hA := map[string]string{"X-Tenant-Id": e.tenantA.String()}
	path := "/tenant/client-installers?school_id="
	admin := e.staffToken(e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA)))
	r := e.do(t, http.MethodGet, path+e.schoolA.String(), admin, hA, nil)
	if r.status != http.StatusOK || r.body["channel"] != "production" || r.body["web_client_url"] != "https://play.example.test/" {
		t.Fatalf("school A: %d %s", r.status, r.raw)
	}
	data := r.body["data"].([]any)
	if len(data) != 2 || data[0].(map[string]any)["kit_id"] != "g13" || data[1].(map[string]any)["kit_id"] != "g45" {
		t.Fatalf("kits of grades 3–4: %s", r.raw)
	}
	// AC-16: only the public fields of a kit; every string within its bound.
	allowed := map[string]bool{"kit_id": true, "platform": true, "client_version": true, "grade_bands": true, "url": true, "sha256": true,
		"size_bytes": true, "min_os": true, "released_at": true}
	for _, d := range data {
		for k, v := range d.(map[string]any) {
			if !allowed[k] {
				t.Fatalf("field %s outside SchoolClientInstaller", k)
			}
			if s, ok := v.(string); ok && len(s) > 2048 {
				t.Fatalf("%s longer than its bound", k)
			}
		}
	}
	for _, leak := range []string{e.tenantA.String(), "127.0.0.1", "secret", "student"} {
		if strings.Contains(r.raw, leak) {
			t.Fatalf("answer leaks %q: %s", leak, r.raw)
		}
	}
	// School A2 has no student yet: every kit (newest per band).
	tadmin := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	if r := e.do(t, http.MethodGet, path+e.schoolA2.String(), tadmin, hA, nil); r.status != http.StatusOK || len(r.body["data"].([]any)) != 3 {
		t.Fatalf("school without students: %d %s", r.status, r.raw)
	}
	// AC-15: the school admin of A asks school A2 (same tenant) or school B (other tenant) → 404; Academic Manager → 403.
	if r := e.do(t, http.MethodGet, path+e.schoolA2.String(), admin, hA, nil); r.status != http.StatusNotFound {
		t.Fatalf("other school of the tenant: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, path+e.schoolB.String(), tadmin, hA, nil); r.status != http.StatusNotFound {
		t.Fatalf("school of tenant B: %d %s", r.status, r.raw)
	}
	am := e.staffToken(e.staff(e.tenantA, "academic_manager"))
	if r := e.do(t, http.MethodGet, path+e.schoolA.String(), am, hA, nil); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("academic manager: %d %s", r.status, r.raw)
	}
	st := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.do(t, http.MethodGet, path+e.schoolA.String(), e.gameToken(st, uuid.NewString()), hA, nil); r.status != http.StatusForbidden {
		t.Fatalf("student: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, path+"not-a-uuid", admin, hA, nil); r.status != http.StatusUnprocessableEntity && r.status != http.StatusBadRequest {
		t.Fatalf("bad school_id: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listSchoolClientInstallers", authztest.RoleDenied, authztest.CrossTenant)

	// The verified index is cached: the CDN is not asked again within the TTL.
	hits := cdn.hits
	if r := e.do(t, http.MethodGet, path+e.schoolA.String(), admin, hA, nil); r.status != http.StatusOK || cdn.hits != hits {
		t.Fatalf("cache: %d hits %d→%d", r.status, hits, cdn.hits)
	}

	// AC-14: one byte changed in the index (fresh adapter and cache) → 503 UNAVAILABLE, no link.
	kv.m = map[string][]byte{}
	cdn.mu.Lock()
	cdn.body = []byte(strings.Replace(string(cdn.body), "g13", "g14", 1))
	cdn.mu.Unlock()
	e.installers.Index = newIndex()
	r = e.do(t, http.MethodGet, path+e.schoolA.String(), admin, hA, nil)
	if r.status != http.StatusServiceUnavailable || r.code() != "UNAVAILABLE" || strings.Contains(r.raw, "https://play") {
		t.Fatalf("tampered index: %d %s", r.status, r.raw)
	}
	cdn.mu.Lock()
	cdn.down = true
	cdn.mu.Unlock()
	if r := e.do(t, http.MethodGet, path+e.schoolA.String(), admin, hA, nil); r.status != http.StatusServiceUnavailable {
		t.Fatalf("CDN down: %d %s", r.status, r.raw)
	}
	e.installers.Index = nil // not configured in this environment
	if r := e.do(t, http.MethodGet, path+e.schoolA.String(), admin, hA, nil); r.status != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured: %d %s", r.status, r.raw)
	}
}
