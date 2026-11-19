//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/webhook"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/sisinternal"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/tests/fakesis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// failingNonces is a test double of port.WebhookNonces whose store is down (tests only).
type failingNonces struct{}

func (failingNonces) Claim(context.Context, uuid.UUID) (bool, error) {
	return false, errors.New("valkey down")
}
func (failingNonces) Release(context.Context, uuid.UUID) error { return nil }

// F017-AC-18…AC-21 and AC-14 (webhook side) with Valkey (nonce, lock) and NATS JetStream (IDENTITY_SIS_JOBS,
// DLQ_IDENTITY): signature, skew, replay, 404 cases, 202 after the ack, processing as a run of mode webhook,
// redelivery and dead letter.
func TestSisContractWebhook(t *testing.T) {
	ctx := context.Background()
	s := newSisAPI(ctx, t, setup(ctx, t))
	conn := s.create(t, true)
	s.trigger(ctx, t, conn, sis.ModeFull, false)

	_, vkAddr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{vkAddr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	_, natsURL := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if err := messaging.EnsureSisJobsStream(ctx, js, 1); err != nil {
		t.Fatal(err)
	}
	if err := natsx.EnsureStreams(ctx, js, "identity", natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	secretsStore := memSecrets{"sis/th-demo-a": "k-th-demo-a", "sis/th-demo-a-webhook": "whsec-th-demo-a"}
	recv := command.ReceiveSisWebhook{UoW: s.uow, Repo: write.SisContractRepo{}, Secrets: secretsStore, Verifier: webhook.Verifier{Validator: v},
		Nonces: cache.SisWebhookNonces{C: vk}, Jobs: messaging.JobQueue{JS: js}, Metrics: metrics.Sync{}}
	r := chi.NewRouter()
	sisinternal.Register(r, sisinternal.Webhook{Receive: recv})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	hs3 := schoolExt + "-HS3"
	body := func(delivery string, status string) []byte {
		b, _ := json.Marshal(map[string]any{"delivery_id": delivery, "contract_version": "1.2.1", "changes": []map[string]any{{"sequence": 9,
			"entity_kind": "student", "operation": "upsert", "external_id": hs3, "occurred_at": fakesis.T,
			"record": fakesis.Student(hs3, schoolExt, "A003", "Lê Minh Chi", "hs.chi.thdemo001", status)}}})
		return b
	}
	post := func(id uuid.UUID, b []byte, secret string, at time.Time) int {
		t.Helper()
		ts := strconv.FormatInt(at.Unix(), 10)
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/sis/webhooks/"+id.String(), bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-EduWorld-Timestamp", ts)
		req.Header.Set("X-EduWorld-Signature", webhook.Sign([]byte(secret), ts, b))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	stream := func() uint64 {
		st, err := js.Stream(ctx, messaging.SisJobsStream)
		if err != nil {
			t.Fatal(err)
		}
		info, err := st.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return info.State.Msgs
	}
	now := time.Now()
	delivery := uuid.New()

	// AC-18: wrong secret, stale or future timestamp → 401 (no reason).
	if c := post(conn, body(delivery.String(), "suspended"), "wrong", now); c != 401 {
		t.Fatalf("wrong signature: %d", c)
	}
	if c := post(conn, body(delivery.String(), "suspended"), "whsec-th-demo-a", now.Add(-6*time.Minute)); c != 401 {
		t.Fatalf("stale timestamp: %d", c)
	}
	// AC-19: 202 after the JetStream ack with Nats-Msg-Id = delivery_id; a replay → 409, no second job.
	if c := post(conn, body(delivery.String(), "suspended"), "whsec-th-demo-a", now); c != 202 {
		t.Fatalf("valid delivery: %d", c)
	}
	if c := post(conn, body(delivery.String(), "suspended"), "whsec-th-demo-a", now); c != 409 || stream() != 1 {
		t.Fatalf("replay: %d, %d jobs", c, stream())
	}
	if testutil.ToFloat64(metrics.WebhookCounter().WithLabelValues(command.WebhookReplay)) < 1 {
		t.Fatal("eduworld_sis_webhook_total{result=replay}")
	}
	// Valkey down → 503 (fail closed).
	down := recv
	down.Nonces = failingNonces{}
	if _, err := down.Handle(ctx, conn, "x", "1", nil); codeOfErr(err) != "UNAUTHENTICATED" {
		t.Fatalf("signature first: %v", err)
	}
	b := body(uuid.NewString(), "active")
	ts := strconv.FormatInt(now.Unix(), 10)
	if _, err := down.Handle(ctx, conn, webhook.Sign([]byte("whsec-th-demo-a"), ts, b), ts, b); codeOfErr(err) != "UNAVAILABLE" {
		t.Fatalf("valkey down: %v", err)
	}
	// AC-20: unknown connection, connection without webhook secret, without the capability → 404.
	if c := post(uuid.New(), body(uuid.NewString(), "active"), "whsec-th-demo-a", now); c != 404 {
		t.Fatalf("unknown connection: %d", c)
	}
	noSecret := s.createSecond(ctx, t)
	if c := post(noSecret, body(uuid.NewString(), "active"), "whsec-th-demo-a", now); c != 404 {
		t.Fatalf("no webhook secret: %d", c)
	}

	// AC-21: the worker processes the batch as a run of mode webhook; the student is locked.
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_ = messaging.ConsumeSisJobs(cctx, js, messaging.SisJobHandlers{WebhookBatch: s.sync.ExecuteWebhook, ScheduledSync: s.sync.Execute})
	}()
	deadline := time.Now().Add(30 * time.Second)
	for count(ctx, t, s.platform, `SELECT count(*) FROM sis.sync_runs WHERE mode = 'webhook' AND delivery_id = $1 AND status = 'succeeded'`, delivery) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("webhook batch not processed as a run of mode webhook")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if n := count(ctx, t, s.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.chi.thdemo001' AND status = 'locked'`); n != 1 {
		t.Fatal("webhook change not applied")
	}
	cancel()

	// AC-21: a batch that keeps failing is redelivered then dead-lettered to DLQ_IDENTITY (5 deliveries).
	var failing atomic.Int32
	fctx, fcancel := context.WithCancel(ctx)
	defer fcancel()
	go func() {
		_ = messaging.ConsumeSisJobs(fctx, js, messaging.SisJobHandlers{WebhookRetryDelay: 50 * time.Millisecond,
			WebhookBatch: func(context.Context, port.WebhookJob) error { failing.Add(1); return errors.New("database down") }})
	}()
	if c := post(conn, body(uuid.NewString(), "active"), "whsec-th-demo-a", time.Now()); c != 202 {
		t.Fatalf("second delivery: %d", c)
	}
	deadline = time.Now().Add(30 * time.Second)
	for {
		st, err := js.Stream(ctx, natsx.DLQStream("identity"))
		if err != nil {
			t.Fatal(err)
		}
		info, err := st.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if info.State.Msgs == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("not dead-lettered after %d deliveries", failing.Load())
		}
		time.Sleep(100 * time.Millisecond)
	}
	fcancel()
	if failing.Load() != 5 {
		t.Fatalf("deliveries before DLQ = %d, want 5", failing.Load())
	}

	// AC-14: a paused connection receives no webhook.
	paused := sis.StatusPaused
	if _, err := s.contract.Update(s.sa, conn, s.version(ctx, t, conn), command.ContractPatch{Status: &paused, Reason: "maintenance"}); err != nil {
		t.Fatal(err)
	}
	if c := post(conn, body(uuid.NewString(), "active"), "whsec-th-demo-a", time.Now()); c != 404 {
		t.Fatalf("paused connection: %d", c)
	}
}

// createSecond creates a contract_v1 connection on the other school of tenant A, without webhook secret.
func (s *sisAPI) createSecond(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	other := uuid.MustParse(s.a.otherSchool)
	in := command.ContractCreateInput{ContractVersion: "1.2.0", SecretRef: "sis/th-demo-a", Reason: "pilot",
		Config: sis.ContractConfig{BaseURL: s.fake.URL + "/eduworld/v1", AuthType: sis.AuthAPIKey,
			Schools: []sis.ContractSchool{{ExternalID: "TH-DEMO-002", SchoolID: other}}}}
	out, err := s.conns.CreateContract(s.sa, s.ta, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.contract.CheckHealth(s.sa, out.ConnectionID); err != nil {
		t.Fatal(err)
	}
	_ = ctx
	return out.ConnectionID
}
