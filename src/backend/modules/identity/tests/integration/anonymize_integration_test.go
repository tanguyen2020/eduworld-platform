//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
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

// F016 US5: F016-AC-20, AC-21, AC-23 through the job retention-anonymizer. F016-AC-22 (portal data-request intake
// createDataRequest) is out of Release 1 (owner 2026-11-22): personal-data requests are handled by operations.
func TestUserAnonymize(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-an-a")

	// NFR-PRIV-003: deactivated over a year ago → anonymized; recently → kept.
	old := enrolled(ctx, t, e.platform, a, "hs.an.cu")
	recent := enrolled(ctx, t, e.platform, a, "hs.an.moi")
	if _, err := e.platform.ExecContext(ctx, `UPDATE identity.users SET email = 'hs.an@example.invalid' WHERE user_id = $1`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE org.students SET student_code = 'HS-AN-01', national_id_hash = 'h', date_of_birth_enc = '\x01'
		WHERE student_id = $1`, old); err != nil {
		t.Fatal(err)
	}
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
		Participants: []port.UserDataAnonymizer{recordingAnonymizer{"game"}, recordingAnonymizer{"learning"}}, Lifecycle: write.LifecycleRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}
	if n, err := job.RunRetention(ctx); err != nil || n != 1 {
		t.Fatalf("retention: %d %v", n, err)
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'deactivated'`, recent) != 1 {
		t.Fatal("retention selection")
	}
	// AC-20, AC-21: the saga runs every participant, overwrites the PII, keeps every row.
	if anonCount("game", old, "retention_expired") != 1 || anonCount("learning", old, "retention_expired") != 1 {
		t.Fatal("participants not called")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id = $1 AND status = 'anonymized' AND username = 'anon.' || $1::text
		AND email IS NULL AND display_name IS NULL AND anonymized_at IS NOT NULL`, old) != 1 {
		t.Fatal("user PII")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM org.students WHERE student_id = $1 AND full_name = 'anon' AND student_code IS NULL
		AND national_id_hash IS NULL AND date_of_birth_enc IS NULL`, old) != 1 {
		t.Fatal("student PII")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM platform.erasure_log WHERE user_id = $1 AND legal_basis = 'retention_expired:student_data_retention'`, old) != 1 {
		t.Fatal("erasure log")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.user_status_history WHERE user_id = $1 AND to_status = 'anonymized' AND source = 'retention'`, old) != 1 {
		t.Fatal("status history")
	}
	// AC-23: user.anonymized carries identifiers and status only.
	var payload json.RawMessage
	if err := e.relay.QueryRowContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = 'user.anonymized'
		AND envelope->'payload'->>'user_id' = $1`, old.String()).Scan(&payload); err != nil {
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
	if p["trigger"] != "retention" || p["data_request_id"] != nil {
		t.Fatalf("payload: %v", p)
	}
	// The Keycloak worker disables and clears the user on user.anonymized.
	linkLocal(ctx, t, e, old, "kc-hs.an.cu")
	kc := &kcAccess{disabled: map[string]int{}, enabled: map[string]int{}, logouts: map[string]int{}}
	c := consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: e.uow, Repo: write.LifecycleRepo{}, KC: kc, Anon: kc, Issuers: []string{localIssuer}}}
	if err := c.Handle(ctx, outbox.Envelope{EventID: uuid.New(), EventType: "user.anonymized", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if kc.disabled["kc-hs.an.cu"] != 1 || kc.anonymized["kc-hs.an.cu"] != 1 {
		t.Fatalf("keycloak: %+v", kc)
	}
	if n, err := job.RunRetention(ctx); err != nil || n != 0 { // idempotent
		t.Fatalf("second retention run: %d %v", n, err)
	}
	// AC-21: no row removed.
	if count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE user_id IN ($1, $2)`, old, recent) != 2 {
		t.Fatal("rows deleted")
	}
}
