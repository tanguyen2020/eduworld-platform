package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// StreamConfig is the work-queue stream GAME_RENDER_JOBS (the container creates it too when it starts first).
func StreamConfig() jetstream.StreamConfig {
	return jetstream.StreamConfig{Name: Stream, Subjects: []string{SubjectAll}, Retention: jetstream.WorkQueuePolicy, MaxAge: MaxAge,
		Duplicates: natsx.DefaultDuplicateWindow}
}

// EnsureStream creates the stream when missing (idempotent; an existing stream is kept).
func EnsureStream(ctx context.Context, js jetstream.JetStream) error {
	if _, err := js.Stream(ctx, Stream); err == nil {
		return nil
	} else if !errors.Is(err, jetstream.ErrStreamNotFound) {
		return fmt.Errorf("stream %s: %w", Stream, err)
	}
	if _, err := js.CreateStream(ctx, StreamConfig()); err != nil && !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		return fmt.Errorf("create stream %s: %w", Stream, err)
	}
	return nil
}

// Queue implements port.RenderQueue on JetStream (Nats-Msg-Id = render_job_id:attempt dedupes a re-enqueue).
type Queue struct {
	JS jetstream.JetStream
}

var _ port.RenderQueue = Queue{}

// Enqueue implements port.RenderQueue.
func (q Queue) Enqueue(ctx context.Context, r port.RenderRequest) error {
	if err := EnsureStream(ctx, q.JS); err != nil {
		return err
	}
	b, err := json.Marshal(Job{RenderJobID: r.RenderJobID, Kind: r.Kind, Size: Size, Frames: r.Frames, Attempt: r.Attempt,
		Source: Source{AssetID: r.SourceID, Bucket: r.Bucket, Key: r.Key, MIME: r.MIME, SHA256: r.SHA256}})
	if err != nil {
		return err
	}
	_, err = q.JS.PublishMsg(ctx, &nats.Msg{Subject: SubjectRequest, Data: b},
		jetstream.WithMsgID(r.RenderJobID.String()+":"+strconv.Itoa(r.Attempt)))
	if err != nil {
		return fmt.Errorf("enqueue render %s: %w", r.RenderJobID, err)
	}
	return nil
}

// ResultHandler records one render result (command.GameMedia.RecordRenderResult).
type ResultHandler func(ctx context.Context, r port.RenderOutcome) error

// Outcome maps a Result of the wire to the application.
func Outcome(res Result) port.RenderOutcome {
	o := port.RenderOutcome{RenderJobID: res.RenderJobID, Attempt: res.Attempt, Done: res.Status == StatusDone, Error: res.Error}
	if res.Output != nil {
		o.Bucket, o.Key, o.MIME, o.SHA256, o.SizeBytes = res.Output.Bucket, res.Output.Key, res.Output.MIME, res.Output.SHA256, res.Output.SizeBytes
		o.Width, o.Height, o.Frames = res.Output.Width, res.Output.Height, res.Output.Frames
	}
	return o
}

// ConsumeResults pulls the results of the renderer (durable consumer game_render_results, explicit ack) until ctx ends:
// a recorded result is acked; a failure is redelivered after a delay; an undecodable message is terminated.
func ConsumeResults(ctx context.Context, js jetstream.JetStream, handle ResultHandler, log *slog.Logger) error {
	if err := EnsureStream(ctx, js); err != nil {
		return err
	}
	cons, err := js.CreateOrUpdateConsumer(ctx, Stream, jetstream.ConsumerConfig{Durable: ConsumerResults, FilterSubject: SubjectResult,
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: time.Minute, MaxDeliver: -1})
	if err != nil {
		return fmt.Errorf("consumer %s: %w", ConsumerResults, err)
	}
	for ctx.Err() == nil {
		batch, err := cons.Fetch(10, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("fetch render results: %w", err)
		}
		for m := range batch.Messages() {
			var res Result
			if err := json.Unmarshal(m.Data(), &res); err != nil || res.RenderJobID.String() == "00000000-0000-0000-0000-000000000000" {
				log.WarnContext(ctx, "invalid render result terminated", "subject", m.Subject())
				_ = m.Term()
				continue
			}
			if err := handle(ctx, Outcome(res)); err != nil {
				log.ErrorContext(ctx, "render result not recorded", "render_job_id", res.RenderJobID, "error", err)
				_ = m.NakWithDelay(10 * time.Second)
				continue
			}
			_ = m.Ack()
		}
		if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) && ctx.Err() == nil {
			return fmt.Errorf("fetch render results: %w", err)
		}
	}
	return nil
}

// LazyQueue resolves the JetStream context at the first enqueue (the binary connects NATS after the modules register).
type LazyQueue struct {
	Get func() (jetstream.JetStream, error)
}

var _ port.RenderQueue = LazyQueue{}

// Enqueue implements port.RenderQueue.
func (q LazyQueue) Enqueue(ctx context.Context, r port.RenderRequest) error {
	js, err := q.Get()
	if err != nil {
		return fmt.Errorf("render queue: %w", err)
	}
	return Queue{JS: js}.Enqueue(ctx, r)
}
