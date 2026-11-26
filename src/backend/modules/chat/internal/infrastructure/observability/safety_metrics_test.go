package observability

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// MONITORING §3: eduworld_reports_total{target_type,reason} (no eduworld_chat_reports_total any more) and
// eduworld_moderation_school_notifications_total{status} are registered with the Safety Team metrics and move per call.
func TestSafetyCountersRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	safetyOnce = sync.Once{} // register on this test registry
	RegisterSafety(reg)
	s := Safety{}
	s.Report("message", "bullying")
	s.Report("user", "grooming")
	s.SchoolNotification("sent")
	s.SchoolNotification("acknowledged")
	s.SchoolNotification("overdue")
	want := `
# HELP eduworld_moderation_school_notifications_total School safety notifications by status: sent, acknowledged, overdue (first reminder past ack_due_at).
# TYPE eduworld_moderation_school_notifications_total counter
eduworld_moderation_school_notifications_total{status="acknowledged"} 1
eduworld_moderation_school_notifications_total{status="overdue"} 1
eduworld_moderation_school_notifications_total{status="sent"} 1
# HELP eduworld_reports_total Reports created by students, by target type (message, user) and reason (MODERATION §5).
# TYPE eduworld_reports_total counter
eduworld_reports_total{reason="bullying",target_type="message"} 1
eduworld_reports_total{reason="grooming",target_type="user"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "eduworld_reports_total",
		"eduworld_moderation_school_notifications_total"); err != nil {
		t.Fatal(err)
	}
	if n, _ := testutil.GatherAndCount(reg, "eduworld_chat_reports_total"); n != 0 {
		t.Fatal("eduworld_chat_reports_total still exposed")
	}
}

// eduworld_chat_filter_duration_seconds{stage} replaces eduworld_chat_filter_latency_seconds (no label).
func TestFilterDurationByStage(t *testing.T) {
	reg := prometheus.NewRegistry()
	moderationOnce = sync.Once{}
	RegisterModeration(reg)
	for _, st := range []string{"sanctions", "catalog", "evaluate", "total"} {
		Moderation{}.FilterDuration(st, 3*time.Millisecond)
	}
	if n, err := testutil.GatherAndCount(reg, "eduworld_chat_filter_duration_seconds"); err != nil || n != 4 {
		t.Fatalf("filter duration series %d %v", n, err)
	}
	if n, _ := testutil.GatherAndCount(reg, "eduworld_chat_filter_latency_seconds"); n != 0 {
		t.Fatal("eduworld_chat_filter_latency_seconds still exposed")
	}
}
