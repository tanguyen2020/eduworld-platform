package analytics

import (
	"testing"
	"time"
)

// MONITORING §3: chat_seq_gap and content_download are metric-only telemetry with closed labels; crash counts one.
func TestClientMetricTelemetry(t *testing.T) {
	ok := map[string]map[string]any{
		"chat_seq_gap":     {"channel_type": "party", "gap_count": float64(3)},
		"content_download": {"source": "lan_cache", "bytes": float64(1 << 20)},
		"crash":            {"error_code": "E_GPU", "fatal": true},
	}
	for typ, attrs := range ok {
		if err := ValidateTelemetry(0, typ, attrs); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
		if Kept(typ) != (typ == "crash") {
			t.Errorf("%s kept in analytics = %v", typ, Kept(typ))
		}
	}
	if m := MetricOf("chat_seq_gap", ok["chat_seq_gap"]); m != (ClientMetric{Kind: "chat_seq_gap", Label: "party", Value: 3}) {
		t.Errorf("seq gap metric %+v", m)
	}
	if m := MetricOf("content_download", ok["content_download"]); m != (ClientMetric{Kind: "content_download", Label: "lan_cache", Value: 1 << 20}) {
		t.Errorf("download metric %+v", m)
	}
	if m := MetricOf("crash", ok["crash"]); m != (ClientMetric{Kind: "crash", Value: 1}) {
		t.Errorf("crash metric %+v", m)
	}
	if m := MetricOf("screen_view", map[string]any{"screen_code": "hub"}); m.Kind != "" {
		t.Errorf("screen_view metric %+v", m)
	}
	for name, c := range map[string]struct {
		typ   string
		attrs map[string]any
	}{
		"class channel (ADR-0095)": {"chat_seq_gap", map[string]any{"channel_type": "class", "gap_count": float64(1)}},
		"gap 0":                    {"chat_seq_gap", map[string]any{"channel_type": "world", "gap_count": float64(0)}},
		"gap fraction":             {"chat_seq_gap", map[string]any{"channel_type": "world", "gap_count": 1.5}},
		"free source":              {"content_download", map[string]any{"source": "usb", "bytes": float64(1)}},
		"negative bytes":           {"content_download", map[string]any{"source": "cdn", "bytes": float64(-1)}},
		"too many bytes":           {"content_download", map[string]any{"source": "cdn", "bytes": float64(MaxDownloadBytes + 1)}},
		"attribute of another":     {"content_download", map[string]any{"source": "cdn", "bytes": float64(1), "gap_count": float64(1)}},
	} {
		if err := ValidateTelemetry(2, c.typ, c.attrs); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// Valid metric-only events are dropped from analytics as not_in_catalog (like network_sample).
	now := time.Now()
	if r := DropTelemetry("chat_seq_gap", now, now, true, false, Config{LateWindow: time.Hour}); r != DropNotInCatalog {
		t.Errorf("drop %q", r)
	}
}
