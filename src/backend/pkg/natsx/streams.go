// Package natsx holds the NATS JetStream conventions of EduWorld (ADR-0042, EVENT_DESIGN §3.1): event
// and DLQ streams per module, message headers, and the Principal cache invalidator.
package natsx

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Defaults from CONFIG_DEFAULTS (event_stream_retention, jetstream_duplicate_window) and EVENT_DESIGN §8.
const (
	DefaultEventRetention    = 7 * 24 * time.Hour
	DefaultDLQRetention      = 30 * 24 * time.Hour
	DefaultDuplicateWindow   = 2 * time.Minute
	HeaderTraceparent        = "traceparent"
	HeaderTenantID           = "Ew-Tenant-Id"
	HeaderOriginalSubject    = "Ew-Original-Subject"
	HeaderDeadLetterConsumer = "Ew-Consumer"
)

// StreamOptions configure EnsureStreams.
type StreamOptions struct {
	EventRetention  time.Duration
	DLQRetention    time.Duration
	DuplicateWindow time.Duration
	Replicas        int
	Storage         jetstream.StorageType
}

// EventStream returns EVT_<MODULE>; DLQStream returns DLQ_<MODULE>.
func EventStream(module string) string { return "EVT_" + upper(module) }

// DLQStream returns DLQ_<MODULE>.
func DLQStream(module string) string { return "DLQ_" + upper(module) }

// EventSubject is ew.evt.<module>.<event_type>.
func EventSubject(module, eventType string) string { return "ew.evt." + module + "." + eventType }

// DLQSubject is ew.dlq.<module>.<consumer name without the module prefix>.
func DLQSubject(module, consumer string) string { return "ew.dlq." + module + "." + consumer }

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

// EnsureStreams creates or updates EVT_<MODULE> and DLQ_<MODULE> idempotently (run by the worker
// profile that owns them at startup).
func EnsureStreams(ctx context.Context, js jetstream.JetStream, module string, o StreamOptions) error {
	if o.EventRetention <= 0 {
		o.EventRetention = DefaultEventRetention
	}
	if o.DLQRetention <= 0 {
		o.DLQRetention = DefaultDLQRetention
	}
	if o.DuplicateWindow <= 0 {
		o.DuplicateWindow = DefaultDuplicateWindow
	}
	if o.Replicas <= 0 {
		o.Replicas = 1
	}
	for _, cfg := range []jetstream.StreamConfig{
		{Name: EventStream(module), Subjects: []string{"ew.evt." + module + ".>"}, MaxAge: o.EventRetention,
			Duplicates: o.DuplicateWindow, Replicas: o.Replicas, Storage: o.Storage},
		{Name: DLQStream(module), Subjects: []string{"ew.dlq." + module + ".>"}, MaxAge: o.DLQRetention,
			Duplicates: o.DuplicateWindow, Replicas: o.Replicas, Storage: o.Storage},
	} {
		if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("natsx: stream %s: %w", cfg.Name, err)
		}
	}
	return nil
}
