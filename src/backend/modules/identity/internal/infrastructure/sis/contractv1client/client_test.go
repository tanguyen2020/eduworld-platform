package contractv1client_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contractv1client"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/tests/fakesis"
)

// secrets is a test double of port.SecretStore (in-memory, tests only).
type secrets map[string]string

func (s secrets) Get(_ context.Context, ref string) ([]byte, error) {
	v, ok := s[ref]
	if !ok {
		return nil, port.ErrSisSecret
	}
	return []byte(v), nil
}

// sleeps records the waits of the client instead of sleeping (test double).
type sleeps struct {
	mu sync.Mutex
	d  []time.Duration
}

func (s *sleeps) sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d = append(s.d, d)
	return nil
}

func (s *sleeps) waits() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.d...)
}

func newClient(t *testing.T, sis *fakesis.Server, auth string) (port.SisProvider, *sleeps, uuid.UUID) {
	t.Helper()
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	sl := &sleeps{}
	f := &contractv1client.Factory{Secrets: secrets{"sis/key": "k-1", "sis/oauth": `{"client_id":"ew","client_secret":"s3"}`}, Validator: v,
		Sleep: sl.sleep, Rate: 1000}
	id := uuid.New()
	c := port.ContractConnection{ID: id, BaseURL: sis.URL + "/eduworld/v1", AuthType: auth, SecretRef: "sis/key"}
	if auth == "oauth2_client_credentials" {
		c.SecretRef, c.TokenURL = "sis/oauth", sis.URL+"/oauth2/token"
	}
	p, err := f.For(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return p, sl, id
}

// F017-AC-05: pages through every list operation with the cursor; F017-AC-06: unknown fields dropped and
// counted, a record outside the contract is a violation, the others are kept.
func TestClientPagesAndValidation(t *testing.T) {
	s := fakesis.New() // MOCK
	defer s.Close()
	s.APIKey = "k-1"
	s.Lock()
	s.Students["TH-DEMO-001"][1]["status"] = "expelled" // enum violation
	s.Extra[fakesis.OpStudents] = map[string]any{"phone": "0900000000"}
	s.Unlock()
	p, _, id := newClient(t, s, "api_key")
	ctx := context.Background()
	h, err := p.Health(ctx)
	if err != nil || h.ContractVersion != "1.2.1" || len(h.Capabilities) != 4 {
		t.Fatalf("health = %+v %v", h, err)
	}
	page1, err := p.ListStudents(ctx, "TH-DEMO-001", "")
	if err != nil || !page1.HasMore || page1.NextCursor != "c2" || len(page1.Records) != 1 || len(page1.Violations) != 1 {
		t.Fatalf("page 1 = %+v %v", page1, err)
	}
	if _, ok := page1.Records[0]["phone"]; ok || page1.UnknownFields != 2 {
		t.Fatalf("unknown field stored or not counted: %+v", page1)
	}
	if got := testutil.ToFloat64(metrics.UnknownFieldsCounter().WithLabelValues(id.String(), "student")); got != 2 {
		t.Fatalf("eduworld_sis_contract_unknown_fields_total = %v", got)
	}
	if page1.Violations[0].Field != "status" || page1.Violations[0].ExternalID != "TH-DEMO-001-HS2" {
		t.Fatalf("violation = %+v", page1.Violations[0])
	}
	page2, err := p.ListStudents(ctx, "TH-DEMO-001", page1.NextCursor)
	if err != nil || page2.HasMore || len(page2.Records) != 1 {
		t.Fatalf("page 2 = %+v %v", page2, err)
	}
	if q := s.Queries[fakesis.OpStudents]; len(q) != 2 || q[1] != "cursor=c2&limit=500" {
		t.Fatalf("queries = %v", q)
	}
	years, err := p.ListAcademicYears(ctx, "TH-DEMO-001")
	if err != nil || len(years.Records) != 1 {
		t.Fatalf("years = %+v %v", years, err)
	}
	// Edge case: has_more without a new cursor never loops.
	s.Lock()
	s.StuckCursor = true
	s.Unlock()
	if _, err := p.ListStudents(ctx, "TH-DEMO-001", "c2"); !errors.Is(err, port.ErrSisPagination) {
		t.Fatalf("repeated cursor: %v", err)
	}
}

// F017-AC-10, AC-11: 5xx and 429 are retried with exponential backoff + jitter and Retry-After; 401 is not
// retried (F017-AC-12); retries are bounded (sis_retry_backoff: 6).
func TestClientRetries(t *testing.T) {
	s := fakesis.New() // MOCK
	defer s.Close()
	p, sl, _ := newClient(t, s, "api_key")
	ctx := context.Background()
	s.Lock()
	s.Fail[fakesis.OpSchools] = []int{503, 503}
	s.Unlock()
	if _, err := p.ListSchools(ctx, ""); err != nil {
		t.Fatal(err)
	}
	w := sl.waits()
	if s.CallsOf(fakesis.OpSchools) != 3 || len(w) < 2 || w[0] < time.Second || w[0] >= 2*time.Second || w[1] < 2*time.Second || w[1] >= 3*time.Second {
		t.Fatalf("calls %d waits %v: want 1 s and 2 s + jitter", s.CallsOf(fakesis.OpSchools), w)
	}
	s.Lock()
	s.Fail[fakesis.OpSchools], s.RetryAfter = []int{429}, "2"
	s.Unlock()
	if _, err := p.ListSchools(ctx, ""); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range sl.waits()[len(w):] {
		found = found || d == 2*time.Second
	}
	if !found {
		t.Fatalf("Retry-After: 2 not honoured: %v", sl.waits())
	}
	s.Lock()
	s.Fail[fakesis.OpSchools] = []int{503, 503, 503, 503, 503, 503, 503, 503}
	s.Unlock()
	before := s.CallsOf(fakesis.OpSchools)
	if _, err := p.ListSchools(ctx, ""); !errors.Is(err, port.ErrSisUnavailable) || s.CallsOf(fakesis.OpSchools)-before != 7 {
		t.Fatalf("exhausted retries: %v after %d calls", err, s.CallsOf(fakesis.OpSchools)-before)
	}
	s.Lock()
	s.Fail[fakesis.OpSchools] = []int{401}
	s.Unlock()
	before = s.CallsOf(fakesis.OpSchools)
	if _, err := p.ListSchools(ctx, ""); !errors.Is(err, port.ErrSisAuth) || s.CallsOf(fakesis.OpSchools)-before != 1 {
		t.Fatalf("401: %v, %d calls", err, s.CallsOf(fakesis.OpSchools)-before)
	}
	s.Lock()
	s.Fail[fakesis.OpChanges] = []int{http.StatusGone}
	s.Unlock()
	if _, err := p.ListChanges(ctx, "wm-1"); !errors.Is(err, port.ErrSisWatermarkExpired) {
		t.Fatalf("410: %v", err)
	}
}

// F017-AC-11: the rate of a connection is limited (sis_client_rate) and halved after a 429.
func TestClientRateLimit(t *testing.T) {
	s := fakesis.New() // MOCK
	defer s.Close()
	v, _ := contract.New()
	sl := &sleeps{}
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	f := &contractv1client.Factory{Secrets: secrets{"k": "x"}, Validator: v, Sleep: sl.sleep, Rate: 10, Now: func() time.Time { return now }}
	p, err := f.For(context.Background(), port.ContractConnection{ID: uuid.New(), BaseURL: s.URL, AuthType: "api_key", SecretRef: "k"})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := p.Health(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	w := sl.waits() // the clock is frozen: each request books the next 100 ms slot
	if len(w) != 2 || w[0] != 100*time.Millisecond || w[1] != 200*time.Millisecond {
		t.Fatalf("pacing waits %v, want 100 ms then 200 ms (10 req/s)", w)
	}
}

// F017-AC-12 + edge case: an expired OAuth2 token is renewed once; a refused token is auth_error.
func TestClientOAuth2(t *testing.T) {
	s := fakesis.New() // MOCK
	defer s.Close()
	s.ClientID, s.ClientSecret, s.BearerToken = "ew", "s3", "tok-1"
	p, _, _ := newClient(t, s, "oauth2_client_credentials")
	ctx := context.Background()
	if _, err := p.Health(ctx); err != nil || s.CallsOf(fakesis.OpToken) != 1 {
		t.Fatalf("token flow: %v, %d token calls", err, s.CallsOf(fakesis.OpToken))
	}
	s.Lock()
	s.BearerToken = "tok-2" // the facade revoked tok-1 mid-run
	s.Unlock()
	if _, err := p.Health(ctx); err != nil || s.CallsOf(fakesis.OpToken) != 2 {
		t.Fatalf("renewal: %v, %d token calls", err, s.CallsOf(fakesis.OpToken))
	}
	s.Lock()
	s.ClientSecret = "rotated"
	s.BearerToken = "tok-3"
	s.Unlock()
	if _, err := p.Health(ctx); !errors.Is(err, port.ErrSisAuth) {
		t.Fatalf("refused credential: %v", err)
	}
}

// F017-AC-22: lookupStudent is called once, without retry; unknown fields are dropped.
func TestClientLookup(t *testing.T) {
	s := fakesis.New() // MOCK
	defer s.Close()
	p, _, _ := newClient(t, s, "api_key")
	s.Lock()
	s.Lookups["hs.an.3a"] = fakesis.Student("HS-1", "TH-DEMO-001", "A1", "An", "hs.an.3a", "active")
	s.Extra[fakesis.OpLookup] = map[string]any{"address": "x"}
	s.Fail[fakesis.OpLookup] = []int{503}
	s.Unlock()
	if _, err := p.LookupStudent(context.Background(), "TH-DEMO-001", "hs.an.3a"); !errors.Is(err, port.ErrSisUnavailable) || s.CallsOf(fakesis.OpLookup) != 1 {
		t.Fatalf("5xx: %v after %d calls (no retry)", err, s.CallsOf(fakesis.OpLookup))
	}
	rec, err := p.LookupStudent(context.Background(), "TH-DEMO-001", "hs.an.3a")
	if err != nil || rec["student_external_id"] != "HS-1" || rec["address"] != nil {
		t.Fatalf("lookup = %v %v", rec, err)
	}
	if _, err := p.LookupStudent(context.Background(), "TH-DEMO-001", "nobody"); !errors.Is(err, port.ErrSisNotFound) {
		t.Fatalf("404: %v", err)
	}
}

// F017-AC-15: changes are validated (record required for upsert, DTO of the kind) and sorted by sequence.
func TestParseChanges(t *testing.T) {
	v, _ := contract.New()
	items := []map[string]any{
		{"sequence": float64(7), "entity_kind": "enrollment", "operation": "delete", "external_id": "HS-1|C-1", "occurred_at": fakesis.T},
		{"sequence": float64(5), "entity_kind": "student", "operation": "upsert", "external_id": "HS-1", "occurred_at": fakesis.T,
			"record": fakesis.Student("HS-1", "TH", "1", "An", "hs.an", "suspended")},
		{"sequence": float64(6), "entity_kind": "student", "operation": "upsert", "external_id": "HS-2", "occurred_at": fakesis.T},
		{"sequence": float64(8), "entity_kind": "teacher", "operation": "upsert", "external_id": "T-1", "occurred_at": fakesis.T},
	}
	ch, bad, _ := contractv1client.ParseChanges(v, items)
	if len(ch) != 2 || ch[0].Sequence != 5 || ch[1].Operation != "delete" || len(bad) != 2 {
		t.Fatalf("changes %+v violations %+v", ch, bad)
	}
}
