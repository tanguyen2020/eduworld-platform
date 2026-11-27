package router

import "testing"

// subject_group of eduworld_nats_request_* is the first segment of the command type (bounded by the AsyncAPI).
func TestSubjectGroup(t *testing.T) {
	for typ, want := range map[string]string{"battle.answer.submit": "battle", "hub.player.move": "hub", "ping": "ping", "": "unknown"} {
		if got := SubjectGroup(typ); got != want {
			t.Errorf("SubjectGroup(%q) = %q, want %q", typ, got, want)
		}
	}
	for typ := range Routes {
		if g := SubjectGroup(typ); g == "" || len(g) > 32 {
			t.Errorf("route %q: subject_group %q not a short bounded value", typ, g)
		}
	}
}
