//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

var anonCalls = struct {
	sync.Mutex
	n map[string]int // "<module>:<user_id>:<reason>"
}{n: map[string]int{}}

// recordingAnonymizer is a TEST DOUBLE of anonymizeUserData of a module (the facades have their own tests).
type recordingAnonymizer struct{ name string }

func (r recordingAnonymizer) Name() string { return r.name }
func (r recordingAnonymizer) Anonymize(_ context.Context, user, _ uuid.UUID, reason string) (port.StudentDataResult, error) {
	anonCalls.Lock()
	defer anonCalls.Unlock()
	anonCalls.n[r.name+":"+user.String()+":"+reason]++
	return port.StudentDataResult{Status: saga.Done}, nil
}

func anonCount(module string, user uuid.UUID, reason string) int {
	anonCalls.Lock()
	defer anonCalls.Unlock()
	return anonCalls.n[module+":"+user.String()+":"+reason]
}

// F016 US5: F016-AC-20..AC-23 (data request and retention-anonymizer).
func TestUserAnonymize(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	h, idp := realAuthHTTP(ctx, t, e)
	a := seedTenant(ctx, t, e.platform, "pgd-an-a")
	b := seedTenant(ctx, t, e.platform, "pgd-an-b")
	st := enrolled(ctx, t, e.platform, a, "hs.an.xoa")
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'deactivated', email = 'hs.an@example.invalid' WHERE user_id = $1`, st); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.students SET student_code = 'HS-AN-01', national_id_hash = 'h', date_of_birth_enc = '\x01'
		WHERE student_id = $1`, st); err != nil {
		t.Fatal(err)
	}
	iss := idp.Issuer()
	portalUser(ctx, t, e, iss, "sa.an", "system_admin", nil, "platform", nil)
	portalUser(ctx, t, e, iss, "ad.donvi.an", "tenant_admin", &a.tenant, "tenant", nil)
	now := time.Now()
	tok := func(u string, mfa bool) string { return idp.Token("sub-"+u, httpxtest.PortalClient, mfa, now) }
	hdr := func(tenant string) map[string]string {
		m := map[string]string{"Idempotency-Key": uuid.Must(uuid.NewV7()).String()}
		if tenant != "" {
			m["X-Tenant-Id"] = tenant
		}
		return m
	}
	body := func(kind string, user uuid.UUID) map[string]any {
		return map[string]any{"request_type": kind, "subject_user_id": user, "requester_kind": "parent_via_school",
			"reference": "CV-2026-118", "reason": "Phụ huynh yêu cầu xóa dữ liệu qua nhà trường"}
	}

	// AC-22: only platform.data_request.write; X-Tenant-Id must be the tenant of the subject.
	if r := h.do(t, "POST", "/admin/data-requests", tok("ad.donvi.an", false), hdr(a.tenant), body("anonymize", st)); r.status != 403 {
		t.Fatalf("tenant admin: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", "/admin/data-requests", tok("sa.an", true), hdr(b.tenant), body("anonymize", st)); r.status != 404 {
		t.Fatalf("other tenant: %d %v", r.status, r.body)
	}
	if r := h.do(t, "POST", "/admin/data-requests", tok("sa.an", true), hdr(a.tenant), body("export", st)); r.status != 422 {
		t.Fatalf("export: %d %v", r.status, r.body)
	}
	// AC-20, AC-21: the saga runs every participant, overwrites the PII, keeps every row.
	r := h.do(t, "POST", "/admin/data-requests", tok("sa.an", true), hdr(a.tenant), body("anonymize", st))
	if r.status != 202 || r.body["status"] != "completed" || r.body["request_type"] != "anonymize" {
		t.Fatalf("create: %d %v", r.status, r.body)
	}
	reqID := r.body["data_request_id"].(string)
	if anonCount("game", st, "data_subject_request") != 1 || anonCount("learning", st, "data_subject_request") != 1 {
		t.Fatal("participants not called")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'anonymized' AND username = 'anon.' || $1::text
		AND email IS NULL AND display_name IS NULL AND anonymized_at IS NOT NULL`, st) != 1 {
		t.Fatal("user PII")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND full_name = 'anon' AND student_code IS NULL
		AND national_id_hash IS NULL AND date_of_birth_enc IS NULL`, st) != 1 {
		t.Fatal("student PII")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.erasure_log WHERE user_id = $1 AND legal_basis = 'data_request:' || $2`, st, reqID) != 1 {
		t.Fatal("erasure log")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.deletion_requests WHERE request_id = $1 AND status = 'executed'
		AND requester_kind = 'parent_via_school' AND erasure_id IS NOT NULL`, reqID) != 1 {
		t.Fatal("request not executed")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_status_history WHERE user_id = $1 AND to_status = 'anonymized' AND source = 'anonymize'`, st) != 1 {
		t.Fatal("status history")
	}
	if count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope::text LIKE '%data_request.created%' AND envelope::text LIKE '%' || $1 || '%'`, reqID) != 1 {
		t.Fatal("audit with the request id")
	}
	// AC-23: user.anonymized carries identifiers and status only.
	var payload json.RawMessage
	if err := e.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'user.anonymized'
		AND envelope->'payload'->>'user_id' = $1`, st.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	_ = json.Unmarshal(payload, &p)
	for k := range p {
		switch k {
		case "user_id", "tenant_id", "user_type", "trigger", "data_request_id", "anonymized_at":
		default:
			t.Fatalf("field %s in user.anonymized", k)
		}
	}
	if p["trigger"] != "data_request" || p["data_request_id"] != reqID {
		t.Fatalf("payload: %v", p)
	}
	// The Keycloak worker disables and clears the user on user.anonymized.
	linkLocal(ctx, t, e, st, "kc-hs.an.xoa")
	kc := &kcAccess{disabled: map[string]int{}, enabled: map[string]int{}, logouts: map[string]int{}}
	c := consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: e.uow, Repo: write.LifecycleRepo{}, KC: kc, Anon: kc, Issuers: []string{localIssuer}}}
	if err := c.Handle(ctx, outbox.Envelope{EventID: uuid.New(), EventType: "user.anonymized", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if kc.disabled["kc-hs.an.xoa"] != 1 || kc.anonymized["kc-hs.an.xoa"] != 1 {
		t.Fatalf("keycloak: %+v", kc)
	}
	// Already anonymized → 409; reads only for platform.data_request.write.
	if r := h.do(t, "POST", "/admin/data-requests", tok("sa.an", true), hdr(a.tenant), body("anonymize", st)); r.status != 409 {
		t.Fatalf("second request: %d %v", r.status, r.body)
	}
	if r := h.do(t, "GET", "/admin/data-requests/"+reqID, tok("sa.an", true), nil, nil); r.status != 200 || r.body["status"] != "completed" {
		t.Fatalf("get: %d %v", r.status, r.body)
	}
	if r := h.do(t, "GET", "/admin/data-requests?tenant_ids="+b.tenant, tok("sa.an", true), nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 0 {
		t.Fatalf("list other tenant: %d %v", r.status, r.body)
	}
	if r := h.do(t, "GET", "/admin/data-requests?status=completed", tok("sa.an", true), nil, nil); r.status != 200 || len(r.body["data"].([]any)) != 1 {
		t.Fatalf("list: %d %v", r.status, r.body)
	}
	if r := h.do(t, "GET", "/admin/data-requests/"+reqID, tok("ad.donvi.an", false), map[string]string{"X-Tenant-Id": a.tenant}, nil); r.status != 403 {
		t.Fatalf("tenant admin read: %d", r.status)
	}

	// NFR-PRIV-003: retention-anonymizer — deactivated over a year ago → anonymized; recently → kept.
	old := enrolled(ctx, t, e.platform, a, "hs.an.cu")
	recent := enrolled(ctx, t, e.platform, a, "hs.an.moi")
	for id, days := range map[uuid.UUID]int{old: 400, recent: 10} {
		if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET status = 'deactivated' WHERE user_id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := e.platform.ExecContext(ctx, `INSERT INTO identity.user_status_history (history_id, tenant_id, user_id, from_status, to_status,
			source, changed_at) VALUES ($1, $2, $3, 'active', 'deactivated', 'sis', now() - make_interval(days => $4))`, uuid.New(), a.tenant, id, days); err != nil {
			t.Fatal(err)
		}
	}
	job := command.UserAnonymize{UoW: e.uow, Repo: write.AnonymizeRepo{}, Saga: saga.Engine{UoW: e.uow, Repo: write.SagaRepo{}},
		Participants: []port.UserDataAnonymizer{recordingAnonymizer{"game"}}, Lifecycle: write.LifecycleRepo{}, Events: e.deps.Events, Audit: e.deps.Audit}
	if n, err := job.RunRetention(ctx); err != nil || n != 1 {
		t.Fatalf("retention: %d %v", n, err)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'anonymized'`, old) != 1 ||
		count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'deactivated'`, recent) != 1 {
		t.Fatal("retention selection")
	}
	if anonCount("game", old, "retention_expired") != 1 || count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events
		WHERE event_type = 'user.anonymized' AND envelope->'payload'->>'user_id' = $1 AND envelope->'payload'->>'trigger' = 'retention'`, old.String()) != 1 {
		t.Fatal("retention saga")
	}
	if n, err := job.RunRetention(ctx); err != nil || n != 0 { // idempotent
		t.Fatalf("second retention run: %d %v", n, err)
	}
	// AC-21: no row removed.
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id IN ($1, $2)`, st, old) != 2 {
		t.Fatal("rows deleted")
	}
}
