package command

import "testing"

// The realtime ticket carries the grade band only (label of eduworld_players_online, MONITORING §3), never the grade.
func TestTicketGradeBand(t *testing.T) {
	g := func(n int) *int { return &n }
	for _, c := range []struct {
		grade *int
		want  string
	}{{nil, ""}, {g(1), "g1_3"}, {g(5), "g4_5"}, {g(7), "g6_7"}, {g(9), "g8_9"}, {g(0), ""}, {g(12), ""}} {
		if got := ticketGradeBand(c.grade); got != c.want {
			t.Errorf("ticketGradeBand(%v) = %q, want %q", c.grade, got, c.want)
		}
	}
}
