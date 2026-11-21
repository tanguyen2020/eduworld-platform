//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// F015 T054 × F071-AC-03: school.updated as module identity publishes it (settings_subset with enabled_subject_codes,
// F015 T054) goes through the consumer game.projection_identity and hides the turned-off subject from listWorldAreas;
// the next school.updated without the key (School Admin set it back to null) shows every subject again.
func TestSubjectDisabledFromSchoolUpdated(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.publishWorld(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, an, 5)
	c := consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}}
	schoolUpdated := func(version int64, subset map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"school_id": e.schoolA.String(), "tenant_id": e.tenantA.String(), "status": "active",
			"settings_subset": subset, "changed_fields": []string{"settings.enabled_subject_codes"}, "source_version": version})
		tenant := e.tenantA
		env := outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: "school.updated", SchemaVersion: 1, OccurredAt: time.Now().UTC(),
			RecordedAt: time.Now().UTC(), Producer: "identity/test", TenantID: &tenant, Payload: raw}
		sc, err := authz.EventScope(tenant)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return c.Handle(ctx, env) }); err != nil {
			t.Fatal(err)
		}
	}
	base := func() map[string]any {
		return map[string]any{"allowed_hours": nil, "curfew_window": nil, "student_club_creation": false, "daily_play_limit_minutes": nil}
	}
	if _, ok := e.listAreas(t, an)["learning_zone.history"]; !ok {
		t.Fatal("history hidden before any school setting")
	}
	off := base()
	off["enabled_subject_codes"] = []string{"math", "vietnamese", "english", "science"}
	schoolUpdated(1000, off)
	if _, ok := e.listAreas(t, an)["learning_zone.history"]; ok {
		t.Fatal("history listed while the school turned it off")
	}
	if _, ok := e.listAreas(t, an)["learning_zone.math"]; !ok {
		t.Fatal("math hidden while on")
	}
	schoolUpdated(1001, base())
	if _, ok := e.listAreas(t, an)["learning_zone.history"]; !ok {
		t.Fatal("history still hidden after enabled_subject_codes left the subset")
	}
}
