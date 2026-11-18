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
)

// Stream and subjects of the SIS jobs.
const (
	SisJobsStream      = "IDENTITY_SIS_JOBS"
	SisJobsSubjects    = "ew.job.identity.sis.>"
	LoginRefreshSubj   = "ew.job.identity.sis.login_refresh"
	CsvImportSubj      = "ew.job.identity.sis.csv_import"
	SisJobsDuplicates  = 10 * time.Minute // Nats-Msg-Id dedupe window of login_refresh (F011-AC-21)
	sisJobsConsumer    = "identity_sis_jobs"
	sisJobsMaxDeliver  = 3
	sisJobsAckWait     = 30 * time.Second
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
)

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
}

// ConsumeSisJobs runs the durable pull consumer of IDENTITY_SIS_JOBS until ctx ends: a malformed job is
// terminated, a failing one is redelivered up to MaxDeliver.
func ConsumeSisJobs(ctx context.Context, js jetstream.JetStream, h SisJobHandlers) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, SisJobsStream, jetstream.ConsumerConfig{Durable: sisJobsConsumer,
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: sisJobsAckWait, MaxDeliver: sisJobsMaxDeliver, FilterSubject: SisJobsSubjects})
	if err != nil {
		return fmt.Errorf("messaging: consumer %s: %w", sisJobsConsumer, err)
	}
	cc, err := cons.Consume(func(m jetstream.Msg) {
		var run func() error
		switch {
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
			_ = m.Term() // scheduled_sync / webhook_batch belong to F017
			return
		}
		if err := run(); err != nil {
			slog.WarnContext(ctx, "sis job failed", "subject", m.Subject(), "error", err)
			_ = m.Nak()
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
