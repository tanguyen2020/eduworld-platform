package observability

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// MONITORING §3: the gateway registers eduworld_ws_connections{node}, eduworld_ws_connect_total{result} (every result
// exposed from start) and eduworld_players_online{platform,grade_band}; no eduworld_rt_connections any more.
func TestGatewayMetricsRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg, "rt-1")
	m.PlayersOnline.WithLabelValues(PlayerLabels("windows", "g4_5")).Inc()
	want := `
# HELP eduworld_ws_connect_total WebSocket handshakes by result (ok, ticket_invalid, rejected).
# TYPE eduworld_ws_connect_total counter
eduworld_ws_connect_total{result="ok"} 0
eduworld_ws_connect_total{result="rejected"} 0
eduworld_ws_connect_total{result="ticket_invalid"} 0
# HELP eduworld_ws_connections Open WebSocket connections of the gateway node.
# TYPE eduworld_ws_connections gauge
eduworld_ws_connections{node="rt-1"} 0
# HELP eduworld_players_online Students connected to this gateway node (registered and present), by client platform and grade band.
# TYPE eduworld_players_online gauge
eduworld_players_online{grade_band="g4_5",platform="windows"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "eduworld_ws_connect_total", "eduworld_ws_connections",
		"eduworld_players_online"); err != nil {
		t.Fatal(err)
	}
	if n, _ := testutil.GatherAndCount(reg, "eduworld_rt_connections"); n != 0 {
		t.Fatalf("eduworld_rt_connections still exposed (%d series)", n)
	}
}

// Ticket values outside the closed lists never create a series: they read "unknown".
func TestPlayerLabelsBounded(t *testing.T) {
	for _, c := range []struct{ platform, band, wantP, wantB string }{
		{"android", "g8_9", "android", "g8_9"},
		{"", "", "unknown", "unknown"},
		{"<script>", "g10", "unknown", "unknown"},
		{"ios", "G1_3", "ios", "unknown"},
	} {
		p, b := PlayerLabels(c.platform, c.band)
		if p != c.wantP || b != c.wantB {
			t.Errorf("PlayerLabels(%q,%q) = %q,%q", c.platform, c.band, p, b)
		}
	}
}

// A refused ticket is ticket_invalid; the gateway's own Valkey failure is rejected.
func TestConnectResult(t *testing.T) {
	for reason, want := range map[string]string{"missing": ConnectTicketInvalid, "malformed": ConnectTicketInvalid,
		"unknown": ConnectTicketInvalid, "unavailable": ConnectRejected} {
		if got := ConnectResult(reason); got != want {
			t.Errorf("ConnectResult(%q) = %q, want %q", reason, got, want)
		}
	}
}
