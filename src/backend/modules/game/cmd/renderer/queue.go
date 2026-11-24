package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// consumerMaxDeliver bounds the redeliveries of a job the container could not finish (crash, shutdown, result not
// published); retries of a failed render are decided by the worker of module game from the published Result.
const consumerMaxDeliver = 5

// StreamConfig is the work-queue stream GAME_RENDER_JOBS (created by the worker of module game as well).
func StreamConfig() jetstream.StreamConfig {
	return jetstream.StreamConfig{Name: render.Stream, Subjects: []string{render.SubjectAll},
		Retention: jetstream.WorkQueuePolicy, MaxAge: render.MaxAge, Duplicates: natsx.DefaultDuplicateWindow}
}

// ConsumerConfig is the durable pull consumer of the container: requests only, explicit ack, AckWait above the Godot
// timeout plus the transfer time so a running job is never redelivered to another instance.
func ConsumerConfig(timeout time.Duration) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{Durable: render.ConsumerRenderer, FilterSubject: render.SubjectRequest,
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: AckWait(timeout), MaxDeliver: consumerMaxDeliver}
}

// AckWait is the Godot timeout plus 3 minutes for the download, the upload and the result.
func AckWait(timeout time.Duration) time.Duration { return timeout + 3*time.Minute }

// EnsureStream creates GAME_RENDER_JOBS when missing and keeps an existing one as it is: the worker of module game
// creates it too, and the first one to start wins (no update fight between the two binaries).
func EnsureStream(ctx context.Context, js jetstream.JetStream) error {
	if _, err := js.Stream(ctx, render.Stream); err == nil {
		return nil
	} else if !errors.Is(err, jetstream.ErrStreamNotFound) {
		return fmt.Errorf("stream %s: %w", render.Stream, err)
	}
	if _, err := js.CreateStream(ctx, StreamConfig()); err != nil && !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		return fmt.Errorf("create stream %s: %w", render.Stream, err)
	}
	return nil
}

// JetStreamPublisher publishes Results on SubjectResult, deduplicated by Nats-Msg-Id "result:<job id>:<attempt>".
type JetStreamPublisher struct{ JS jetstream.JetStream }

// ResultMsgID is the Nats-Msg-Id of a Result.
func ResultMsgID(res render.Result) string {
	return "result:" + res.RenderJobID.String() + ":" + strconv.Itoa(res.Attempt)
}

// PublishResult implements Publisher.
func (p JetStreamPublisher) PublishResult(ctx context.Context, res render.Result) error {
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = p.JS.PublishMsg(ctx, &nats.Msg{Subject: render.SubjectResult, Data: b}, jetstream.WithMsgID(ResultMsgID(res)))
	return err
}

// Consume pulls one job at a time from the durable consumer until ctx ends.
func Consume(ctx context.Context, js jetstream.JetStream, w *Worker, timeout time.Duration) error {
	if err := EnsureStream(ctx, js); err != nil {
		return err
	}
	cons, err := js.CreateOrUpdateConsumer(ctx, render.Stream, ConsumerConfig(timeout))
	if err != nil {
		return fmt.Errorf("consumer %s: %w", render.ConsumerRenderer, err)
	}
	w.Log.InfoContext(ctx, "render consumer started", "stream", render.Stream, "consumer", render.ConsumerRenderer,
		"ack_wait", AckWait(timeout).String())
	for ctx.Err() == nil {
		batch, err := cons.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("fetch: %w", err)
		}
		for m := range batch.Messages() {
			w.Handle(ctx, m)
		}
		if err := batch.Error(); err != nil && !errors.Is(err, nats.ErrTimeout) && ctx.Err() == nil {
			return fmt.Errorf("fetch: %w", err)
		}
	}
	return nil
}
