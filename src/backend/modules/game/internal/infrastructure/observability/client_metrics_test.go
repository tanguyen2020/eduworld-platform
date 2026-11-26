package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
)

// MONITORING §3/§5 (ClientCrashRateHigh): sessions and crashes share bounded labels; a free platform or version reads
// "unknown", a pre-release suffix is dropped.
func TestClientLabels(t *testing.T) {
	for _, c := range []struct{ p, v, wantP, wantV string }{
		{"windows", "1.2.3", "windows", "1.2.3"},
		{"android", "1.2.3-beta.4+build9", "android", "1.2.3"},
		{"tv", "1.2", "unknown", "unknown"},
		{"", "", "unknown", "unknown"},
	} {
		p, v := ClientLabels(c.p, c.v)
		if p != c.wantP || v != c.wantV {
			t.Errorf("ClientLabels(%q,%q) = %q,%q", c.p, c.v, p, v)
		}
	}
}

// Each client metric moves on its own event with the labels of MONITORING §3.
func TestClientMetricsMove(t *testing.T) {
	SessionMetrics{}.ClientSession("ios", "2.0.0-rc.1")
	if got := testutil.ToFloat64(ClientSessions().WithLabelValues("ios", "2.0.0")); got != 1 {
		t.Fatalf("client_sessions = %v", got)
	}
	a := AnalyticsMetrics{}
	a.Client(analytics.ClientMetric{Kind: "crash", Value: 1}, "ios", "2.0.0")
	a.Client(analytics.ClientMetric{Kind: "chat_seq_gap", Label: "team", Value: 4}, "ios", "2.0.0")
	a.Client(analytics.ClientMetric{Kind: "content_download", Label: "cdn", Value: 2048}, "ios", "2.0.0")
	a.Client(analytics.ClientMetric{}, "ios", "2.0.0") // nothing
	if got := testutil.ToFloat64(ClientCrashes().WithLabelValues("ios", "2.0.0")); got != 1 {
		t.Errorf("crashes = %v", got)
	}
	if got := testutil.ToFloat64(ChatSeqGaps().WithLabelValues("team")); got != 4 {
		t.Errorf("seq gaps = %v", got)
	}
	if got := testutil.ToFloat64(ContentDownloadBytes().WithLabelValues("cdn")); got != 2048 {
		t.Errorf("download bytes = %v", got)
	}
}
