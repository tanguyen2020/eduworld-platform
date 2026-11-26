package analytics

import (
	"fmt"
	"slices"
	"sort"
	"time"
)

// Client telemetry types of GAME_ANALYTICS §3.2 and the only attributes each may carry (closed schema, F080-AC-01).
// type, client_event_id and occurred_at are common to every event. network_sample is in the contract enum but not in the
// analytics catalog: it is accepted and dropped (metric only).
var telemetryProps = map[string][]string{
	"screen_view":          {"screen_code", "duration_ms"},
	"tutorial_step":        {"step_code", "result"},
	"perf_sample":          {"preset", "renderer", "scene_code", "fps_p50", "fps_p10", "frame_ms_p95", "render_scale", "mem_mb"},
	"load_time":            {"kind", "ms"},
	"client_error":         {"error_code", "scene_code", "fatal", "crash_signature"},
	"crash":                {"error_code", "scene_code", "fatal", "crash_signature"},
	"network_sample":       {"rtt_p50_ms", "rtt_p95_ms", "reconnects"},
	"break_reminder_shown": {"reminder_no", "minutes_played"},
	// Metric-only types (MONITORING §3, not in the analytics catalog): eduworld_chat_seq_gaps_total{channel_type} and
	// eduworld_client_content_download_bytes_total{source}.
	"chat_seq_gap":     {"channel_type", "gap_count"},
	"content_download": {"source", "bytes"},
}

// analyticsProps are the attributes written to events_raw (crash_signature goes to error tracking only, F080-AC-04).
var analyticsProps = map[string][]string{
	"screen_view":          {"screen_code", "duration_ms"},
	"tutorial_step":        {"step_code", "result"},
	"perf_sample":          {"preset", "renderer", "scene_code", "fps_p50", "fps_p10", "frame_ms_p95", "render_scale", "mem_mb"},
	"load_time":            {"kind", "ms"},
	"client_error":         {"error_code", "scene_code", "fatal"},
	"crash":                {"error_code", "scene_code", "fatal"},
	"break_reminder_shown": {"reminder_no", "minutes_played"},
}

// ProductTelemetry are the types a tenant (or the platform) can switch off (analytics_product_telemetry_enabled, §3.2).
var ProductTelemetry = map[string]bool{"screen_view": true, "tutorial_step": true}

// FieldError is a 422 VALIDATION_FAILED detail (pointer into the batch).
type FieldError struct {
	Pointer string
	Reason  string
}

func (e FieldError) Error() string { return e.Pointer + ": " + e.Reason }

// ValidateTelemetry checks one event of a batch (index i) given its raw attributes: known type, only the attributes of
// its type, and the ranges of break_reminder_shown (reminder_no 1–20, minutes_played 0–1 440 — F080-AC-34).
func ValidateTelemetry(i int, typ string, attrs map[string]any) error {
	allowed, ok := telemetryProps[typ]
	if !ok {
		return FieldError{Pointer: fmt.Sprintf("/events/%d/type", i), Reason: "unknown telemetry type"}
	}
	set := map[string]bool{"type": true, "client_event_id": true, "occurred_at": true}
	for _, a := range allowed {
		set[a] = true
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !set[k] {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/%s", i, k), Reason: "attribute not allowed for " + typ}
		}
	}
	if typ == "break_reminder_shown" {
		if n, ok := attrs["reminder_no"].(float64); !ok || n != float64(int(n)) || n < 1 || n > 20 {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/reminder_no", i), Reason: "integer 1–20 required"}
		}
		if n, ok := attrs["minutes_played"].(float64); !ok || n != float64(int(n)) || n < 0 || n > 1440 {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/minutes_played", i), Reason: "integer 0–1440 required"}
		}
	}
	switch typ {
	case "chat_seq_gap":
		if c, _ := attrs["channel_type"].(string); !slices.Contains(ChatChannelTypes, c) {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/channel_type", i), Reason: "known channel type required"}
		}
		if n, ok := attrs["gap_count"].(float64); !ok || n != float64(int(n)) || n < 1 || n > 1000 {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/gap_count", i), Reason: "integer 1–1000 required"}
		}
	case "content_download":
		if s, _ := attrs["source"].(string); !slices.Contains(DownloadSources, s) {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/source", i), Reason: "preinstalled, lan_cache or cdn required"}
		}
		if n, ok := attrs["bytes"].(float64); !ok || n != float64(int64(n)) || n < 0 || n > MaxDownloadBytes {
			return FieldError{Pointer: fmt.Sprintf("/events/%d/bytes", i), Reason: "integer 0–10737418240 required"}
		}
	}
	return nil
}

// Closed values of the metric-only telemetry types (labels of MONITORING §3, never free text).
var (
	// ChatChannelTypes is ChannelType of the contract (ADR-0095: no class, no study_group).
	ChatChannelTypes = []string{"world", "area", "match", "party", "team", "direct"}
	// DownloadSources is the source of a content package (PERFORMANCE_BUDGET §10.1).
	DownloadSources = []string{"preinstalled", "lan_cache", "cdn"}
)

// MaxDownloadBytes caps one content_download event (10 GiB).
const MaxDownloadBytes = 10 << 30

// ClientMetric is what a valid telemetry event adds to the client metrics of MONITORING §3; Kind "" = nothing.
type ClientMetric struct {
	Kind  string // crash, chat_seq_gap, content_download
	Label string // channel_type or source
	Value float64
}

// MetricOf returns the client metric of a validated telemetry event (crash → crash count, chat_seq_gap → gap_count,
// content_download → bytes).
func MetricOf(typ string, attrs map[string]any) ClientMetric {
	switch typ {
	case "crash":
		return ClientMetric{Kind: typ, Value: 1}
	case "chat_seq_gap":
		c, _ := attrs["channel_type"].(string)
		n, _ := attrs["gap_count"].(float64)
		return ClientMetric{Kind: typ, Label: c, Value: n}
	case "content_download":
		s, _ := attrs["source"].(string)
		n, _ := attrs["bytes"].(float64)
		return ClientMetric{Kind: typ, Label: s, Value: n}
	}
	return ClientMetric{}
}

// TelemetryProps keeps the attributes of typ that go to analytics.
func TelemetryProps(typ string, attrs map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range analyticsProps[typ] {
		if v, ok := attrs[k]; ok {
			out[k] = v
		}
	}
	return out
}

// Kept reports whether a telemetry type is written to analytics (network_sample is not in the catalog).
func Kept(typ string) bool {
	_, ok := analyticsProps[typ]
	return ok
}

// DropReason is why a valid telemetry event is not stored (metric eduworld_telemetry_events_total{type,accepted}).
type DropReason string

// Reasons of DropTelemetry.
const (
	Keep              DropReason = ""
	DropQATenant      DropReason = "qa_tenant"
	DropProductOff    DropReason = "product_telemetry_disabled"
	DropFuture        DropReason = "future"
	DropLate          DropReason = "late"
	DropNotInCatalog  DropReason = "not_in_catalog"
	DropNoPlayerFound DropReason = "unknown_student"
)

// DropTelemetry decides whether a valid event is stored (F080-AC-03, AC-34, edge cases): a QA tenant never; a product
// event when product telemetry is off; an event from the future beyond skew or older than the late window; a type outside
// the catalog. break_reminder_shown, perf and crash events are kept even when product telemetry is off.
func DropTelemetry(typ string, at, now time.Time, productOn, qa bool, c Config) DropReason {
	switch {
	case qa:
		return DropQATenant
	case !Kept(typ):
		return DropNotInCatalog
	case ProductTelemetry[typ] && !productOn:
		return DropProductOff
	case at.After(now.Add(c.FutureSkew)):
		return DropFuture
	case now.Sub(at) > c.LateWindow:
		return DropLate
	}
	return Keep
}
