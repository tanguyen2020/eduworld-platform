package analytics

import (
	"fmt"
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
	return nil
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
