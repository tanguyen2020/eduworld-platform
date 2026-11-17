//go:build integration

package sample_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
	pwrite "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/infrastructure/persistence/write"
)

type result struct {
	status   int
	code     string
	replayed bool
	id       string
}

// F003-AC-13, AC-14, AC-15, AC-16 with the sample CreateWidget command.
func TestIdempotentCreate(t *testing.T) {
	e := sample.Setup(context.Background(), t)
	tenancy.Configure(tenancy.Hooks{Recorder: e.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	store := idempotency.NewStore(e.App, 0, nil)
	create := command.Create{Handlers: command.Handlers{UoW: e.UoW, Repo: pwrite.WidgetRepo{}, Events: pwrite.Publisher{W: e.Writer}}}
	calls := 0
	var mu sync.Mutex
	op := httpx.Operation{ID: "sampleCreateWidget", Method: http.MethodPost, Pattern: "/sample/widgets",
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated}
	h := httpx.Idempotency(store, op)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		var in dto.CreateWidget
		_ = json.NewDecoder(r.Body).Decode(&in)
		id, err := create.Handle(r.Context(), in)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(dto.Created{ID: id})
	}))
	userA := authztest.Staff(authztest.TenantA, "tenant_admin")
	userB := authztest.Staff(authztest.TenantB, "tenant_admin")
	do := func(ctx context.Context, key, body string) result {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sample/widgets", strings.NewReader(body)).WithContext(ctx)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		r := result{status: rec.Code, replayed: rec.Header().Get("Idempotent-Replayed") == "true"}
		r.code, _ = out["code"].(string)
		r.id, _ = out["id"].(string)
		return r
	}
	ctxA := authztest.Ctx(context.Background(), userA)
	ctxB := authztest.Ctx(context.Background(), userB)

	if r := do(ctxA, "", `{"code":"i-0","name":"x"}`); r.status != 428 || r.code != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("missing key: %+v", r)
	}
	before := calls
	if r := do(ctxA, uuid.NewString(), `{"code":"i-0","name":"x"}`); r.status != 400 || calls != before {
		t.Fatalf("UUIDv4 key: %+v (handler calls %d)", r, calls-before)
	}

	key := ids.New().String()
	first := do(ctxA, key, `{"code":"i-1","name":"One"}`)
	if first.status != 201 || first.replayed || first.id == "" {
		t.Fatalf("first: %+v", first)
	}
	again := do(ctxA, key, `{"name":"One","code":"i-1"}`)
	if again.status != 201 || !again.replayed || again.id != first.id {
		t.Fatalf("replay: %+v", again)
	}
	if r := do(ctxA, key, `{"code":"i-1","name":"Other"}`); r.status != 409 || r.code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("reused key: %+v", r)
	}
	// AC-16: user B with the same key and route is not given A's response.
	if r := do(ctxB, key, `{"code":"i-1","name":"One"}`); r.status != 201 || r.replayed || r.id == first.id {
		t.Fatalf("other user: %+v", r)
	}

	// AC-14: two concurrent requests with one key → one business result, the other replays it.
	ckey := ids.New().String()
	var wg sync.WaitGroup
	results := make([]result, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = do(ctxA, ckey, `{"code":"i-2","name":"Two"}`)
		}(i)
	}
	wg.Wait()
	if results[0].status != 201 || results[1].status != 201 || results[0].id != results[1].id || results[0].replayed == results[1].replayed {
		t.Fatalf("concurrent: %+v", results)
	}
	var widgets int
	_ = e.Platform.QueryRow(`SELECT count(*) FROM sample.widgets WHERE code = 'i-2'`).Scan(&widgets)
	if widgets != 1 {
		t.Fatalf("concurrent requests created %d widgets", widgets)
	}

	// AC-15: a failed command leaves no record, so the same key works after the cause is fixed.
	fkey := ids.New().String()
	if r := do(ctxA, fkey, `{"code":"i-1","name":"dup"}`); r.status != 409 || r.code != "CONFLICT" {
		t.Fatalf("duplicate code: %+v", r)
	}
	var records int
	_ = e.Migrator.QueryRow(`SELECT count(*) FROM infra.idempotency_records WHERE idem_key = $1`, fkey).Scan(&records)
	if records != 0 {
		t.Fatal("record written although the command rolled back")
	}
	del := command.Delete{Handlers: command.Handlers{UoW: e.UoW, Repo: pwrite.WidgetRepo{}, Events: pwrite.Publisher{W: e.Writer}}}
	if err := del.Handle(ctxA, uuid.MustParse(first.id), "retry test"); err != nil {
		t.Fatal(err)
	}
	if r := do(ctxA, fkey, `{"code":"i-1","name":"dup"}`); r.status != 201 || r.replayed {
		t.Fatalf("retry after rollback: %+v", r)
	}

	// Expired records are ignored by Lookup (never deleted by the application).
	k, issued, _ := idempotency.ParseKey(key, time.Now(), idempotency.DefaultTTL)
	late := idempotency.NewStore(e.App, time.Hour, fakeClock{time.Now().Add(25 * time.Hour)})
	if rec, err := late.Lookup(context.Background(), idempotency.Key{UserID: userA.UserID, Route: "/api/v1/sample/widgets", Key: k, IssuedOn: issued}); err != nil || rec != nil {
		t.Fatalf("expired record returned: %+v %v", rec, err)
	}
}

type fakeClock struct{ t time.Time }

func (f fakeClock) Now() time.Time { return f.t }
