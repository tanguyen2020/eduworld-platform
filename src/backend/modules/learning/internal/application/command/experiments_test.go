package command

import (
	"encoding/json"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
)

// F035-AC-26, AC-27: an arm ≥ 5 points under the control trips the guardrail; small cells are hidden and never trip it.
func TestExperimentResultsOf(t *testing.T) {
	arms, _ := json.Marshal([]experiment.Arm{{Code: "control", AllocationPct: 50, IsControl: true}, {Code: "b", AllocationPct: 50}})
	e := write.ExperimentRow{Code: "x", Arms: arms}
	cfg := DefaultExperimentConfig()
	ll := 0.42
	res, breached := ExperimentResultsOf(e, []write.ArmMetric{
		{ArmCode: "control", Units: 3, Students: 30, Answers: 100, Correct: 80, ActiveMinutes: 300, ActiveStudents: 30},
		{ArmCode: "b", Units: 3, Students: 30, Answers: 100, Correct: 75, LogLoss: &ll, ActiveMinutes: 330, ActiveStudents: 30},
	}, cfg)
	if !breached {
		t.Fatal("80 % → 75 % is a 5-point drop")
	}
	rows := res["arms"].([]map[string]any)
	if rows[1]["primary_metric"] != 0.75 || rows[1]["next_item_log_loss"] != 0.42 || rows[1]["active_minutes_avg"] != 11.0 || rows[1]["suppressed"] != false {
		t.Fatalf("arm b %v", rows[1])
	}
	_, breached = ExperimentResultsOf(e, []write.ArmMetric{
		{ArmCode: "control", Students: 30, Answers: 100, Correct: 80}, {ArmCode: "b", Students: 30, Answers: 100, Correct: 76},
	}, cfg)
	if breached {
		t.Fatal("a 4-point drop trips the guardrail")
	}
	res, breached = ExperimentResultsOf(e, []write.ArmMetric{
		{ArmCode: "control", Students: 30, Answers: 100, Correct: 80}, {ArmCode: "b", Students: 4, Answers: 40, Correct: 0},
	}, cfg)
	rows = res["arms"].([]map[string]any)
	if breached || rows[1]["suppressed"] != true || rows[1]["primary_metric"] != nil {
		t.Fatalf("small cell: breached %v, %v", breached, rows[1])
	}
}
