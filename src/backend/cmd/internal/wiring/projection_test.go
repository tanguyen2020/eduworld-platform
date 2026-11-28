package wiring_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

func payloadOf(t *testing.T, e outbox.Envelope) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func set(types ...string) map[string]bool {
	out := map[string]bool{}
	for _, t := range types {
		out[t] = true
	}
	return out
}

// F003-AC-39, F003 Clarifications 2026-11-28: each listProjectionSnapshot item becomes the projection events it stands for,
// in the tenant of the item, with its source_version; a soft-deleted source row never projects as active; the student
// event follows what the consumer subscribes (placement for learning/competition, student.synced for game/chat), then
// cleaned_up at deleted_at (deleted) or restored at the read time (live).
func TestSnapshotEnvelopes(t *testing.T) {
	tenant, school, student := uuid.New(), uuid.New(), uuid.New()
	readAt := time.Date(2026, 11, 28, 9, 0, 0, 0, time.UTC)
	deletedAt := readAt.Add(-48 * time.Hour)

	envs, err := wiring.SnapshotEnvelopes(identityapi.ProjectionSnapshotItem{EntityType: "tenant", EntityId: tenant, TenantId: tenant, SourceVersion: 7,
		Deleted: true, Data: map[string]any{"code": "th-a", "tenant_type": "school", "status": "active"}}, set("tenant.updated"), readAt)
	if err != nil || len(envs) != 1 || envs[0].EventType != "tenant.updated" || *envs[0].TenantID != tenant || !envs[0].OccurredAt.Equal(readAt) {
		t.Fatalf("tenant: %+v %v", envs, err)
	}
	if p := payloadOf(t, envs[0]); p["status"] != "disabled" || p["source_version"] != float64(7) || p["tenant_id"] != tenant.String() {
		t.Fatalf("deleted tenant payload %v", p)
	}
	if _, err := wiring.SnapshotEnvelopes(identityapi.ProjectionSnapshotItem{EntityType: "school", EntityId: school, TenantId: tenant},
		set("tenant.updated"), readAt); err == nil {
		t.Fatal("a consumer without school.updated cannot project schools")
	}

	live := identityapi.ProjectionSnapshotItem{EntityType: "student", EntityId: student, TenantId: tenant, SourceVersion: 3,
		Data: map[string]any{"school_id": school.String(), "grade_level": float64(4), "status": "active"}}
	envs, err = wiring.SnapshotEnvelopes(live, set("student.placement_changed", "student.synced", "student.cleaned_up", "student.restored"), readAt)
	if err != nil || len(envs) != 2 || envs[0].EventType != "student.placement_changed" || envs[1].EventType != "student.restored" ||
		!envs[1].OccurredAt.Equal(readAt) {
		t.Fatalf("live student (placement): %+v %v", envs, err)
	}
	if p := payloadOf(t, envs[0]); p["student_id"] != student.String() || p["tenant_id"] != tenant.String() || p["source_version"] != float64(3) {
		t.Fatalf("placement payload %v", p)
	}

	gone := live
	gone.Deleted, gone.DeletedAt = true, &deletedAt
	gone.Data = map[string]any{"school_id": school.String(), "grade_level": float64(4), "status": "inactive"}
	envs, err = wiring.SnapshotEnvelopes(gone, set("student.synced", "student.cleaned_up", "student.restored"), readAt)
	if err != nil || len(envs) != 2 || envs[0].EventType != "student.synced" || envs[1].EventType != "student.cleaned_up" ||
		!envs[1].OccurredAt.Equal(deletedAt) {
		t.Fatalf("deleted student (synced): %+v %v", envs, err)
	}
	if p := payloadOf(t, envs[0]); p["action"] != "update" || p["status"] != "deactivated" || p["grade_level"] != float64(4) {
		t.Fatalf("synced payload %v", p)
	}
	// An active placement never sets a status on student.synced: the stored status (locked, deactivated) is kept.
	envs, _ = wiring.SnapshotEnvelopes(live, set("student.synced"), readAt)
	if p := payloadOf(t, envs[0]); len(envs) != 1 || p["status"] != nil {
		t.Fatalf("active synced: %v", p)
	}
	if _, err := wiring.SnapshotEnvelopes(live, set("tenant.updated"), readAt); err == nil {
		t.Fatal("a consumer without a student event cannot project students")
	}
}
