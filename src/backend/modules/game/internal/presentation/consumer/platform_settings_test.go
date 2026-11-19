package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// store is a TEST DOUBLE of write.PlatformSettingWriter (R17) with the source_version rule of the SQL upsert.
type store struct {
	rows  map[string]write.RefPlatformSetting
	calls int
	err   error
}

func (s *store) UpsertPlatformSetting(_ context.Context, r write.RefPlatformSetting) (bool, error) {
	s.calls++
	if s.err != nil {
		return false, s.err
	}
	if cur, ok := s.rows[r.Key]; ok && cur.SourceVersion >= r.SourceVersion {
		return false, nil
	}
	s.rows[r.Key] = r
	return true, nil
}

func settingsEnvelope(t *testing.T, payload map[string]any, at time.Time) outbox.Envelope {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return outbox.Envelope{EventID: uuid.Must(uuid.NewV7()), EventType: "platform_settings.changed", OccurredAt: at, Payload: raw}
}

func curfewPayload(start, end string) map[string]any {
	return map[string]any{"key": "platform_curfew", "scope": "platform", "tenant_id": nil, "school_id": nil, "flag_key": nil,
		"before": map[string]any{"start": "21:30", "end": "06:30"}, "after": map[string]any{"start": start, "end": end},
		"curfew_window": map[string]any{"start": start, "end": end}, "reason": "Điều chỉnh", "actor_id": uuid.NewString()}
}

func newPlatformSettings() (PlatformSettings, *store) {
	s := &store{rows: map[string]write.RefPlatformSetting{}}
	return PlatformSettings{Settings: command.PlatformSettings{Refs: s}}, s
}

func stored(t *testing.T, s *store) string {
	t.Helper()
	return string(s.rows[write.KeyPlatformCurfew].Value)
}

// OQ F014-2: platform_settings.changed (platform_curfew) is stored; a redelivery of the same event (pkg/outbox dedupes
// by event_id; even without it the source_version keeps the row) changes nothing; an older event never wins.
func TestPlatformSettingsAppliesPlatformCurfewIdempotently(t *testing.T) {
	ctx := context.Background()
	c, s := newPlatformSettings()
	t0 := time.Date(2026, 11, 20, 4, 40, 0, 0, time.UTC)
	e := settingsEnvelope(t, curfewPayload("21:30", "04:00"), t0)
	if err := c.Handle(ctx, e); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, s); got != `{"start":"21:30","end":"04:00"}` {
		t.Fatalf("stored %s", got)
	}
	if err := c.Handle(ctx, e); err != nil { // redelivery
		t.Fatal(err)
	}
	if got := s.rows[write.KeyPlatformCurfew].SourceVersion; got != t0.UnixMicro() {
		t.Fatalf("source_version %d", got)
	}
	older := settingsEnvelope(t, curfewPayload("20:00", "07:00"), t0.Add(-time.Minute))
	if err := c.Handle(ctx, older); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, s); got != `{"start":"21:30","end":"04:00"}` {
		t.Fatalf("older event overrode the value: %s", got)
	}
	newer := settingsEnvelope(t, curfewPayload("22:00", "06:00"), t0.Add(time.Minute))
	if err := c.Handle(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, s); got != `{"start":"22:00","end":"06:00"}` {
		t.Fatalf("newer event not applied: %s", got)
	}
}

// Other keys and scopes of platform_settings.changed are acknowledged without writing anything.
func TestPlatformSettingsIgnoresOtherKeys(t *testing.T) {
	ctx := context.Background()
	c, s := newPlatformSettings()
	tenant := uuid.New()
	for _, p := range []map[string]any{
		{"key": "student_cleanup", "scope": "platform", "before": nil, "after": map[string]any{"default_days": 180}, "reason": "x", "actor_id": nil},
		{"key": "chat_retention", "scope": "platform", "before": nil, "after": nil, "reason": nil, "actor_id": nil},
		{"key": "portal.users", "scope": "feature_flag", "flag_key": "portal.users", "before": nil, "after": nil, "reason": nil, "actor_id": nil},
		{"key": "curfew_window", "scope": "school", "tenant_id": tenant.String(), "school_id": uuid.NewString(),
			"curfew_window": map[string]any{"start": "20:00", "end": "07:00"}, "before": nil, "after": nil, "reason": nil, "actor_id": nil},
	} {
		e := settingsEnvelope(t, p, time.Now())
		if p["scope"] == "school" {
			e.TenantID = &tenant
		}
		if err := c.Handle(ctx, e); err != nil {
			t.Fatalf("%v: %v", p["key"], err)
		}
	}
	if s.calls != 0 || len(s.rows) != 0 {
		t.Fatalf("other keys written: calls=%d rows=%v", s.calls, s.rows)
	}
}

// A malformed platform_curfew is refused at once (DLQ), never stored; a store error is transient (retry).
func TestPlatformSettingsRejectsInvalidCurfew(t *testing.T) {
	ctx := context.Background()
	c, s := newPlatformSettings()
	tenant := uuid.New()
	bad := []outbox.Envelope{
		settingsEnvelope(t, curfewPayload("21:30", "21:30"), time.Now()),
		settingsEnvelope(t, curfewPayload("25:00", "04:00"), time.Now()),
		settingsEnvelope(t, map[string]any{"key": "platform_curfew", "scope": "platform", "before": nil, "after": nil,
			"curfew_window": nil, "reason": nil, "actor_id": nil}, time.Now()),
		{EventID: uuid.New(), EventType: "platform_settings.changed", OccurredAt: time.Now(), Payload: []byte(`[1]`)},
	}
	withTenant := settingsEnvelope(t, curfewPayload("21:30", "04:00"), time.Now())
	withTenant.TenantID = &tenant
	bad = append(bad, withTenant)
	for i, e := range bad {
		if err := c.Handle(ctx, e); !errors.Is(err, outbox.ErrPermanent) {
			t.Fatalf("case %d: err = %v, want permanent", i, err)
		}
	}
	if len(s.rows) != 0 {
		t.Fatalf("invalid curfew stored: %v", s.rows)
	}
	// after without curfew_window still carries the value for this key.
	p := curfewPayload("21:00", "05:00")
	p["curfew_window"] = nil
	if err := c.Handle(ctx, settingsEnvelope(t, p, time.Now())); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, s); got != `{"start":"21:00","end":"05:00"}` {
		t.Fatalf("stored %s", got)
	}
	s.err = errors.New("db down")
	if err := c.Handle(ctx, settingsEnvelope(t, curfewPayload("21:30", "04:00"), time.Now())); err == nil || errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("store error must be transient: %v", err)
	}
}
