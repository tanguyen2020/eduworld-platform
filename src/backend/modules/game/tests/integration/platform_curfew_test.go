//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func curfewChanged(start, end string) map[string]any {
	return map[string]any{"key": "platform_curfew", "scope": "platform", "tenant_id": nil, "school_id": nil, "flag_key": nil,
		"before": map[string]any{"start": "21:30", "end": "06:30"}, "after": map[string]any{"start": start, "end": end},
		"curfew_window": map[string]any{"start": start, "end": end}, "reason": "Điều chỉnh theo chính sách",
		"actor_id": "0193a5f4-6c1e-7b2a-9d1f-000000000001"}
}

// OQ F014-2 (F015-AC-21, ADR-0078): a System Admin changes platform_curfew in module identity
// (putPlatformCurfewSettings → platform_settings.changed); the consumer game.platform_settings projects it into
// ref.platform_settings and createGameSession and the job curfew-session-closer apply it at once, without a restart
// and without the new EW_PLATFORM_CURFEW (which stays the default before the first event). A redelivered event is
// processed once, other keys are ignored, an older event never overrides a newer value.
func TestPlatformCurfewChangeTakesEffectWithoutRestart(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{curfew: "21:30-06:30"}) // EW_PLATFORM_CURFEW at start-up
	b := newBus(ctx, t)
	b.run(ctx, t, consumer.PlatformSettingsOptions(consumer.PlatformSettings{Settings: command.PlatformSettings{Refs: write.RefPlatformSettings{}}},
		e.uow, e.app, b.js))
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")

	// 04:50 ICT: inside the start-up default 21:30–06:30.
	e.clock.Set(time.Date(2026, 11, 20, 4, 50, 0, 0, ict))
	r := e.start(t, an, e.device(an), newKey(), nil)
	if r.status != http.StatusForbidden || r.code() != string(problem.CurfewActive) {
		t.Fatalf("default curfew: %d %s", r.status, r.raw)
	}
	if at, _ := time.Parse(time.RFC3339, r.body["reopen_at"].(string)); !at.Equal(time.Date(2026, 11, 20, 6, 30, 0, 0, ict)) {
		t.Fatalf("reopen_at %v", r.body["reopen_at"])
	}

	// The System Admin moves the end to 04:00; the event is delivered twice (redelivery of the same event_id).
	changed := b.publish(ctx, t, "platform_settings.changed", nil, curfewChanged("21:30", "04:00"))
	b.publishEnvelope(ctx, t, changed)
	e.processed(ctx, t, messaging.PlatformSettingsName, changed)
	time.Sleep(300 * time.Millisecond) // let the duplicate delivery run
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = $1 AND event_id = $2`,
		messaging.PlatformSettingsName, changed.EventID); n != 1 {
		t.Fatalf("processed_events rows = %d", n)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.platform_settings WHERE key = 'platform_curfew'
		AND value = '{"start":"21:30","end":"04:00"}'::jsonb AND source_version = $1`, changed.OccurredAt.UTC().UnixMicro()); n != 1 {
		t.Fatal("platform_curfew not projected")
	}
	r = e.start(t, an, e.device(an), newKey(), nil)
	if r.status != http.StatusCreated {
		t.Fatalf("after the change, same process: %d %s", r.status, r.raw)
	}
	id := r.body["game_session_id"].(string)

	// Other keys of the same event are ignored; an older platform_curfew event (late delivery) never wins.
	other := b.publish(ctx, t, "platform_settings.changed", nil, map[string]any{"key": "student_cleanup", "scope": "platform",
		"tenant_id": nil, "school_id": nil, "flag_key": nil, "before": nil, "after": map[string]any{"default_days": 180},
		"curfew_window": nil, "reason": "x", "actor_id": nil})
	e.processed(ctx, t, messaging.PlatformSettingsName, other)
	older, err := b.w.Envelope(ctx, outbox.Message{Type: "platform_settings.changed", Subject: outbox.Subject{Type: "platform_setting",
		ID: "platform_curfew"}, Platform: true, OccurredAt: changed.OccurredAt.Add(-time.Minute), Payload: curfewChanged("21:30", "06:30")})
	if err != nil {
		t.Fatal(err)
	}
	b.publishEnvelope(ctx, t, older)
	e.processed(ctx, t, messaging.PlatformSettingsName, older)
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.platform_settings`); n != 1 {
		t.Fatalf("ref.platform_settings rows = %d (other keys must not be stored)", n)
	}
	if n := count(ctx, t, e.migrator, `SELECT count(*) FROM ref.platform_settings WHERE value = '{"start":"21:30","end":"04:00"}'::jsonb`); n != 1 {
		t.Fatal("an older event overrode platform_curfew")
	}

	// A later change to 21:30–05:00: the job curfew-session-closer ends the running session at its next tick.
	later := b.publish(ctx, t, "platform_settings.changed", nil, curfewChanged("21:30", "05:00"))
	e.processed(ctx, t, messaging.PlatformSettingsName, later)
	e.clock.Set(time.Date(2026, 11, 20, 4, 55, 0, 0, ict))
	if n, err := e.sessions.CloseSessionsAtCurfew(ctx, 10); err != nil || n != 1 {
		t.Fatalf("curfew closer: %d %v", n, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_sessions WHERE game_session_id = $1 AND end_reason = 'curfew'`, id); n != 1 {
		t.Fatal("session not ended by the changed curfew")
	}
	r = e.start(t, an, e.device(an), newKey(), nil)
	if r.status != http.StatusForbidden || r.code() != string(problem.CurfewActive) {
		t.Fatalf("changed curfew 21:30-05:00 at 04:55: %d %s", r.status, r.raw)
	}
	if at, _ := time.Parse(time.RFC3339, r.body["reopen_at"].(string)); !at.Equal(time.Date(2026, 11, 20, 5, 0, 0, 0, ict)) {
		t.Fatalf("reopen_at %v", r.body["reopen_at"])
	}
}
