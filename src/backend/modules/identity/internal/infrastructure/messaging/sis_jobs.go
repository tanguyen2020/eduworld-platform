// Package messaging holds the JetStream adapters of module identity outside the outbox: the SIS job
// work-queue IDENTITY_SIS_JOBS (SIS_INTEGRATION §4.5, §5).
package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// Stream and subjects of the SIS jobs.
const (
	SisJobsStream     = "IDENTITY_SIS_JOBS"
	SisJobsSubjects   = "ew.job.identity.sis.>"
	LoginRefreshSubj  = "ew.job.identity.sis.login_refresh"
	CsvImportSubj     = "ew.job.identity.sis.csv_import"
	ScheduledSyncSubj = "ew.job.identity.sis.scheduled_sync" // F017: manual, scheduled and fallback pull runs
	WebhookBatchSubj  = "ew.job.identity.sis.webhook_batch"  // F017: Nats-Msg-Id = delivery_id
	SisJobsDuplicates = 10 * time.Minute                     // Nats-Msg-Id dedupe window of login_refresh (F011-AC-21)
	sisJobsConsumer   = "identity_sis_jobs"
	sisJobsMaxDeliver = 3
	sisJobsAckWait    = 30 * time.Second
	// F017 (SIS_INTEGRATION §7): a failing webhook batch is redelivered after 5 minutes, at most 5 times,
	// then dead-lettered to DLQ_IDENTITY; a run waiting for the lock of its connection is retried every 30 s.
	webhookMaxDeliver = 5
	webhookRetryDelay = 5 * time.Minute
	lockRetryDelay    = 30 * time.Second
	runRetryDelay     = 30 * time.Second
	// SisJobsDLQSubj is the dead-letter subject of the SIS jobs (stream DLQ_IDENTITY, natsx.EnsureStreams).
	SisJobsDLQSubj = "ew.dlq.identity.sis_jobs"
	// runHeartbeat keeps a pull run (full sync of 50 000 students < 15 min) from being redelivered.
	runHeartbeat       = 15 * time.Second
	sisJobsMaxAge      = 24 * time.Hour
	loginRefreshPrefix = "student:"
)

// EnsureSisJobsStream creates or updates the work-queue stream (idempotent, at worker start-up).
func EnsureSisJobsStream(ctx context.Context, js jetstream.JetStream, replicas int) error {
	if replicas <= 0 {
		replicas = 1
	}
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: SisJobsStream, Subjects: []string{SisJobsSubjects},
		Retention: jetstream.WorkQueuePolicy, Duplicates: SisJobsDuplicates, MaxAge: sisJobsMaxAge, Replicas: replicas})
	if err != nil {
		return fmt.Errorf("messaging: stream %s: %w", SisJobsStream, err)
	}
	return nil
}

// JobQueue implements port.SisJobQueue.
type JobQueue struct{ JS jetstream.JetStream }

var (
	_ port.SisJobQueue       = JobQueue{}
	_ port.SisImportJobQueue = JobQueue{}
	_ port.SisSyncJobQueue   = JobQueue{}
)

// EnqueueSync publishes a scheduled_sync job deduplicated by msgID (run:<id>, sched:<conn>:<mode>:<minute>,
// fallback:<run>) and returns after the JetStream ack.
func (q JobQueue) EnqueueSync(ctx context.Context, job port.SyncJob, msgID string) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = q.JS.PublishMsg(ctx, &nats.Msg{Subject: ScheduledSyncSubj, Data: b, Header: nats.Header{}}, jetstream.WithMsgID(msgID))
	return err
}

// EnqueueWebhookBatch publishes a webhook_batch job with Nats-Msg-Id = delivery_id (F017-AC-19).
func (q JobQueue) EnqueueWebhookBatch(ctx context.Context, job port.WebhookJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = q.JS.PublishMsg(ctx, &nats.Msg{Subject: WebhookBatchSubj, Data: b, Header: nats.Header{}}, jetstream.WithMsgID(job.DeliveryID.String()))
	return err
}

// EnqueueCsvImport publishes a csv_import job deduplicated by import id.
func (q JobQueue) EnqueueCsvImport(ctx context.Context, job port.CsvImportJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = q.JS.PublishMsg(ctx, &nats.Msg{Subject: CsvImportSubj, Data: b, Header: nats.Header{}}, jetstream.WithMsgID("import:"+job.ImportID.String()))
	return err
}

// LoginRefreshMsgID is the dedupe key of a login_refresh job.
func LoginRefreshMsgID(job port.LoginRefreshJob) string {
	return loginRefreshPrefix + job.ConnectionID.String() + ":" + job.StudentExternalID
}

// EnqueueLoginRefresh publishes the job with Nats-Msg-Id so duplicates within 10 minutes are dropped.
func (q JobQueue) EnqueueLoginRefresh(ctx context.Context, job port.LoginRefreshJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	msg := &nats.Msg{Subject: LoginRefreshSubj, Data: b, Header: nats.Header{}}
	_, err = q.JS.PublishMsg(ctx, msg, jetstream.WithMsgID(LoginRefreshMsgID(job)))
	return err
}

// SisJobHandlers handle the SIS jobs (nil = not handled by this worker: the job is terminated).
type SisJobHandlers struct {
	LoginRefresh func(ctx context.Context, job port.LoginRefreshJob) error
	CsvImport    func(ctx context.Context, job port.CsvImportJob) error
	// F017: pull runs and webhook batches (command.SisSync.Execute / ExecuteWebhook).
	ScheduledSync func(ctx context.Context, job port.SyncJob) error
	WebhookBatch  func(ctx context.Context, job port.WebhookJob) error
	// WebhookRetryDelay overrides the 5 minutes between deliveries of a failing batch (tests).
	WebhookRetryDelay time.Duration
}

func delivered(m jetstream.Msg) uint64 {
	md, err := m.Metadata()
	if err != nil || md.NumDelivered == 0 {
		return 1
	}
	return md.NumDelivered
}

// deadLetter publishes a failed webhook batch to DLQ_IDENTITY and terminates it.
func deadLetter(ctx context.Context, js jetstream.JetStream, m jetstream.Msg) {
	msg := nats.NewMsg(SisJobsDLQSubj)
	msg.Data = m.Data()
	msg.Header.Set(natsx.HeaderOriginalSubject, m.Subject())
	msg.Header.Set(natsx.HeaderDeadLetterConsumer, sisJobsConsumer)
	if _, err := js.PublishMsg(ctx, msg, jetstream.WithMsgID("dlq:"+msgID(m))); err != nil {
		slog.ErrorContext(ctx, "sis job dlq publish failed", "subject", m.Subject(), "error", err)
		_ = m.NakWithDelay(time.Minute)
		return
	}
	slog.WarnContext(ctx, "sis job dead-lettered", "subject", m.Subject())
	_ = m.Term()
}

func msgID(m jetstream.Msg) string {
	if h := m.Headers(); h != nil {
		if id := h.Get(jetstream.MsgIDHeader); id != "" {
			return id
		}
	}
	md, err := m.Metadata()
	if err != nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", md.Sequence.Stream)
}

// ConsumeSisJobs runs the durable pull consumer of IDENTITY_SIS_JOBS until ctx ends: a malformed job is
// terminated, a failing one is redelivered up to MaxDeliver.
func ConsumeSisJobs(ctx context.Context, js jetstream.JetStream, h SisJobHandlers) error {
	// MaxDeliver is decided here per subject (webhook batches 5, other jobs 3), not by the server (F017).
	cons, err := js.CreateOrUpdateConsumer(ctx, SisJobsStream, jetstream.ConsumerConfig{Durable: sisJobsConsumer,
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: sisJobsAckWait, MaxDeliver: -1, FilterSubject: SisJobsSubjects})
	if err != nil {
		return fmt.Errorf("messaging: consumer %s: %w", sisJobsConsumer, err)
	}
	cc, err := cons.Consume(func(m jetstream.Msg) {
		var run func() error
		maxDeliver, retryDelay, dlq := uint64(sisJobsMaxDeliver), time.Duration(0), false
		switch {
		case m.Subject() == ScheduledSyncSubj && h.ScheduledSync != nil:
			var job port.SyncJob
			if err := json.Unmarshal(m.Data(), &job); err != nil || job.ConnectionID == uuid.Nil || job.TenantID == uuid.Nil {
				_ = m.Term()
				return
			}
			run, retryDelay = func() error { return withHeartbeat(ctx, m, func() error { return h.ScheduledSync(ctx, job) }) }, runRetryDelay
		case m.Subject() == WebhookBatchSubj && h.WebhookBatch != nil:
			var job port.WebhookJob
			if err := json.Unmarshal(m.Data(), &job); err != nil || job.ConnectionID == uuid.Nil || job.TenantID == uuid.Nil {
				_ = m.Term()
				return
			}
			run = func() error { return withHeartbeat(ctx, m, func() error { return h.WebhookBatch(ctx, job) }) }
			maxDeliver, retryDelay, dlq = webhookMaxDeliver, webhookRetryDelay, true
			if h.WebhookRetryDelay > 0 {
				retryDelay = h.WebhookRetryDelay
			}
		case m.Subject() == LoginRefreshSubj && h.LoginRefresh != nil:
			var job port.LoginRefreshJob
			if err := json.Unmarshal(m.Data(), &job); err != nil || job.StudentExternalID == "" {
				_ = m.Term()
				return
			}
			run = func() error { return h.LoginRefresh(ctx, job) }
		case m.Subject() == CsvImportSubj && h.CsvImport != nil:
			var job port.CsvImportJob
			if err := json.Unmarshal(m.Data(), &job); err != nil || job.ImportID == uuid.Nil {
				_ = m.Term()
				return
			}
			run = func() error { return h.CsvImport(ctx, job) }
		default:
			_ = m.Term() // not handled by this worker
			return
		}
		err := run()
		switch {
		case err == nil:
		case errors.Is(err, port.ErrRunLockBusy): // F017 edge case: queued until the run of the connection ends
			_ = m.NakWithDelay(lockRetryDelay)
			return
		case delivered(m) >= maxDeliver && dlq:
			slog.WarnContext(ctx, "sis job failed", "subject", m.Subject(), "error", err)
			deadLetter(ctx, js, m)
			return
		case delivered(m) >= maxDeliver:
			slog.WarnContext(ctx, "sis job failed, dropped", "subject", m.Subject(), "error", err)
			_ = m.Term()
			return
		default:
			slog.WarnContext(ctx, "sis job failed", "subject", m.Subject(), "error", err)
			if retryDelay > 0 {
				_ = m.NakWithDelay(retryDelay)
			} else {
				_ = m.Nak()
			}
			return
		}
		_ = m.Ack()
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	cc.Stop()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

// withHeartbeat keeps a long job (a pull run) from being redelivered while it runs.
func withHeartbeat(ctx context.Context, m jetstream.Msg, fn func() error) error {
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(runHeartbeat)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				_ = m.InProgress()
			}
		}
	}()
	return fn()
}
