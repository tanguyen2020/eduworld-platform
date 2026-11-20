// Package natsstream is the NATS JetStream transport of chat messages (ADR-0035, EVENT_DESIGN §4.10): stream
// CHAT_MESSAGES (ew.chat.<tenant_id>.msg|evt|cursor.<channel_id>) is the durable write path before "Sent", read by the
// durable consumer "persister" of worker-chat. Game clients and the portal never reach NATS (R25).
package natsstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
)

// Names of the chat stream and its persister consumer (EVENT_DESIGN §4.10); dead letters go to DLQ_CHAT
// (ew.dlq.chat.persister), the DLQ stream of module chat.
const (
	StreamName      = "CHAT_MESSAGES"
	PersisterName   = "persister"
	PersisterFilter = "ew.chat.*.msg.*"
	DuplicateWindow = 2 * time.Minute // jetstream_duplicate_window
)

// Subjects of CHAT_MESSAGES.
var Subjects = []string{"ew.chat.*.msg.*", "ew.chat.*.evt.*", "ew.chat.*.cursor.*"}

// MessageSubject is ew.chat.<tenant_id>.msg.<channel_id>.
func MessageSubject(e message.Envelope) string {
	return "ew.chat." + e.TenantID.String() + ".msg." + e.ChannelID.String()
}

// StreamOptions configure EnsureStream.
type StreamOptions struct {
	MaxAge   time.Duration // 72 h
	Replicas int
}

// EnsureStream creates or updates CHAT_MESSAGES (file storage, max_age, duplicate_window 2 min) and the module DLQ stream
// DLQ_CHAT, idempotently (T002). Run by the send path and worker-chat at start.
func EnsureStream(ctx context.Context, js jetstream.JetStream, o StreamOptions) error {
	if o.Replicas < 1 {
		o.Replicas = 1
	}
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: StreamName, Subjects: Subjects, Storage: jetstream.FileStorage,
		MaxAge: o.MaxAge, Duplicates: DuplicateWindow, Replicas: o.Replicas}); err != nil {
		return fmt.Errorf("chat: stream %s: %w", StreamName, err)
	}
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: natsx.DLQStream("chat"), Subjects: []string{"ew.dlq.chat.>"},
		MaxAge: natsx.DefaultDLQRetention, Duplicates: natsx.DefaultDuplicateWindow, Replicas: o.Replicas}); err != nil {
		return fmt.Errorf("chat: stream %s: %w", natsx.DLQStream("chat"), err)
	}
	return nil
}

// Publisher implements port.ChatStream: Nats-Msg-Id = message_id, waits for the PubAck at most Timeout (and never past
// the deadline of the command minus a margin, so the module can still answer CHAT_TEMPORARILY_UNAVAILABLE before the
// gateway's nats_request_timeout, F115-AC-03).
type Publisher struct {
	JS      jetstream.JetStream
	Timeout time.Duration // chat_publish_ack_timeout
	Margin  time.Duration // kept before the command deadline (default 150 ms)
}

var _ port.ChatStream = Publisher{}

// Publish implements port.ChatStream.
func (p Publisher) Publish(ctx context.Context, e message.Envelope) error {
	if p.JS == nil {
		return port.ErrUnavailable
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	timeout := p.Timeout
	if dl, ok := ctx.Deadline(); ok {
		margin := p.Margin
		if margin <= 0 {
			margin = 150 * time.Millisecond
		}
		if left := time.Until(dl) - margin; left < timeout {
			timeout = left
		}
	}
	if timeout <= 0 {
		return port.ErrUnavailable
	}
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	m := nats.NewMsg(MessageSubject(e))
	m.Data = b
	m.Header.Set(natsx.HeaderTenantID, e.TenantID.String())
	if e.TraceID != nil {
		m.Header.Set("Ew-Trace-Id", *e.TraceID)
	}
	ack, err := p.JS.PublishMsg(cctx, m, jetstream.WithMsgID(e.MessageID.String()), jetstream.WithExpectStream(StreamName))
	if err != nil {
		return fmt.Errorf("%w: %w", port.ErrUnavailable, err)
	}
	if ack == nil || ack.Stream != StreamName {
		return fmt.Errorf("%w: %w", port.ErrUnavailable, errors.New("unexpected stream ack"))
	}
	return nil
}
