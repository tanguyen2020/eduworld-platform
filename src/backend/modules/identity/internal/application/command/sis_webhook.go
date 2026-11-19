package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Webhook results of eduworld_sis_webhook_total.
const (
	WebhookAccepted     = "accepted"
	WebhookReplay       = "replay"
	WebhookUnauthorized = "unauthorized"
	WebhookNotFound     = "not_found"
	WebhookInvalid      = "invalid"
	WebhookUnavailable  = "unavailable"
)

// ReceiveSisWebhook implements receiveSisChanges (F017-AC-18…AC-20): no person is involved, the HMAC of
// the connection's webhook secret authenticates the SIS. The batch is only queued (202 after the JetStream
// ack); a run of mode webhook processes it in the worker.
type ReceiveSisWebhook struct {
	UoW      write.UnitOfWork
	Repo     write.SisContractRepository
	Secrets  port.SecretStore
	Verifier port.WebhookVerifier
	Nonces   port.WebhookNonces
	Jobs     port.SisSyncJobQueue
	Metrics  port.SisSyncMetrics // optional
	Clock    func() time.Time
}

func (h ReceiveSisWebhook) observe(result string) {
	if h.Metrics != nil {
		h.Metrics.Webhook(result)
	}
}

// Handle verifies, deduplicates and queues one delivery; it returns the delivery id.
func (h ReceiveSisWebhook) Handle(ctx context.Context, connectionID uuid.UUID, signature, timestamp string, body []byte) (out uuid.UUID, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ReceiveSisWebhook")
	defer func() { done(err) }()
	notFound := func() (uuid.UUID, error) {
		h.observe(WebhookNotFound)
		return uuid.Nil, problem.New(problem.NotFound, "")
	}
	// The route carries no tenant: the connection id is resolved under the platform_job bypass (audited).
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		return out, err
	}
	var tenantID uuid.UUID
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		tenantID, err = h.Repo.TenantOfAny(ctx, connectionID)
		return err
	}); err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.NotFound {
			return notFound()
		}
		return out, err
	}
	sc, err := authz.EventScope(tenantID)
	if err != nil {
		return out, err
	}
	ctx = tenancy.WithScope(ctx, sc)
	var c *write.ContractConnectionRow
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		c, err = h.Repo.GetContract(ctx, connectionID)
		return err
	}); err != nil {
		return out, err
	}
	// F017-AC-14, AC-20: inactive connections, connections without webhook capability or secret do not exist here.
	if c.ProviderKind != sis.ProviderContractV1 || !c.State.Active() || !sis.HasCapability(c.Capabilities, sis.CapWebhook) || c.WebhookSecretRef == "" {
		return notFound()
	}
	secret, err := h.Secrets.Get(ctx, c.WebhookSecretRef)
	if err != nil {
		h.observe(WebhookUnavailable)
		return out, problem.New(problem.Unavailable, "").Retry()
	}
	now := time.Now()
	if h.Clock != nil {
		now = h.Clock()
	}
	batch, err := h.Verifier.Verify(secret, signature, timestamp, body, now)
	switch {
	case errors.Is(err, port.ErrWebhookSignature):
		h.observe(WebhookUnauthorized)
		return out, problem.New(problem.Unauthenticated, "") // never says why (F017-AC-18)
	case err != nil:
		h.observe(WebhookInvalid)
		return out, problem.New(problem.BadRequest, "the body is not a ChangeBatch of the EduWorld SIS Contract v1")
	}
	fresh, err := h.Nonces.Claim(ctx, batch.DeliveryID)
	if err != nil { // fail closed (plan F017: Valkey error → 503)
		h.observe(WebhookUnavailable)
		return out, problem.New(problem.Unavailable, "").Retry()
	}
	if !fresh {
		h.observe(WebhookReplay)
		return out, problem.New(problem.Conflict, "delivery_id already received")
	}
	job := port.WebhookJob{TenantID: tenantID, ConnectionID: connectionID, DeliveryID: batch.DeliveryID, Changes: batch.Changes,
		Violations: len(batch.Violations)}
	if err := h.Jobs.EnqueueWebhookBatch(ctx, job); err != nil {
		_ = h.Nonces.Release(context.WithoutCancel(ctx), batch.DeliveryID) // the SIS sends it again
		h.observe(WebhookUnavailable)
		return out, problem.New(problem.Unavailable, "").Retry()
	}
	h.observe(WebhookAccepted)
	return batch.DeliveryID, nil
}

// ScheduleSisSync is the job sis-scheduler (F017 T028): every tick it queues the full sync of each active
// contract_v1 connection whose schedule (sis_connections.schedule, default sis_sync_schedule) matches the
// current minute, and a delta sync when the facade declares the delta capability. Jobs are deduplicated by
// Nats-Msg-Id per connection, mode and minute, so several worker replicas queue one job.
type ScheduleSisSync struct {
	UoW   write.UnitOfWork
	Repo  write.SisContractRepository
	Jobs  port.SisSyncJobQueue
	Clock func() time.Time
}

// RunOnce queues the due runs and returns their number.
func (h ScheduleSisSync) RunOnce(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.job.SisScheduler")
	defer func() { done(err) }()
	now := time.Now()
	if h.Clock != nil {
		now = h.Clock()
	}
	now = now.Truncate(time.Minute)
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonPlatformJob)
	if err != nil {
		return 0, err
	}
	var conns []write.ContractConnectionRow
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		conns, err = h.Repo.ActiveContractConnections(ctx)
		return err
	}); err != nil {
		return 0, err
	}
	full, _ := sis.ParseCron(sis.DefaultFullSchedule)
	delta, _ := sis.ParseCron(sis.DefaultDeltaSchedule)
	minute := now.UTC().Format("200601021504")
	for _, c := range conns {
		cron := full
		if c.Schedule != "" {
			if parsed, err := sis.ParseCron(c.Schedule); err == nil {
				cron = parsed
			}
		}
		mode := ""
		switch {
		case cron.Matches(now):
			mode = sis.ModeFull
		case sis.HasCapability(c.Capabilities, sis.CapDelta) && delta.Matches(now):
			mode = sis.ModeDelta
		default:
			continue
		}
		job := port.SyncJob{TenantID: c.TenantID, ConnectionID: c.ID, Mode: mode, Trigger: sis.TriggerSchedule}
		if err := h.Jobs.EnqueueSync(ctx, job, "sched:"+c.ID.String()+":"+mode+":"+minute); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
