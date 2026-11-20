package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
)

var ict = time.FixedZone("ICT", 7*3600)

// directory is a TEST DOUBLE of write.Directory (R17): projected platform_curfew and school windows.
type directory struct {
	platform *write.CurfewWindow
	school   *write.CurfewWindow
	err      error
	reads    int
	limit    *int // daily_play_limit_minutes of the school
}

func (d *directory) Student(context.Context, uuid.UUID, uuid.UUID) (*write.Student, error) {
	return nil, nil
}

func (d *directory) SchoolCurfew(context.Context, uuid.UUID, uuid.UUID) (*write.CurfewWindow, error) {
	return d.school, nil
}

func (d *directory) PlatformCurfew(context.Context) (*write.CurfewWindow, error) {
	d.reads++
	return d.platform, d.err
}

func (d *directory) DailyPlayLimit(context.Context, uuid.UUID, uuid.UUID) (*int, error) {
	return d.limit, nil
}

func window(t *testing.T, s string) gamesession.Window {
	t.Helper()
	w, err := gamesession.ParseWindow(s, ict)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// OQ F014-2: the effective platform_curfew is the value projected from platform_settings.changed once there is one;
// the start-up default (EW_PLATFORM_CURFEW) applies only before the first event, or if a stored value is malformed.
func TestPlatformWindow(t *testing.T) {
	def := window(t, "21:30-06:30")
	cases := []struct {
		name      string
		projected *write.CurfewWindow
		want      gamesession.Window
	}{
		{"no event yet: start-up default", nil, def},
		{"projected value replaces the default", &write.CurfewWindow{Start: "21:30", End: "04:00"}, window(t, "21:30-04:00")},
		{"malformed projected value: default", &write.CurfewWindow{Start: "25:00", End: "04:00"}, def},
		{"empty projected window: default, never no curfew", &write.CurfewWindow{Start: "22:00", End: "22:00"}, def},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := platformWindow(def, tc.projected, ict); got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

// The effective curfew of a student is the platform window united with the school window (ADR-0078); the platform
// window is the projected one, so a changed platform curfew moves the opening time without a restart.
func TestSessionsCurfewUsesProjectedPlatformCurfew(t *testing.T) {
	ctx := context.Background()
	d := &directory{}
	h := Sessions{Directory: d, Config: SessionConfig{PlatformCurfew: window(t, "21:30-06:30"), CurfewLocation: ict}}
	at := time.Date(2026, 11, 20, 4, 50, 0, 0, ict)

	cf, err := h.curfew(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if active, reopen := cf.Active(at); !active || !reopen.Equal(time.Date(2026, 11, 20, 6, 30, 0, 0, ict)) {
		t.Fatalf("default 21:30-06:30 at 04:50: active=%v reopen=%v", active, reopen)
	}

	d.platform = &write.CurfewWindow{Start: "21:30", End: "04:00"} // putPlatformCurfewSettings projected
	if cf, err = h.curfew(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if active, _ := cf.Active(at); active {
		t.Fatal("projected 21:30-04:00: 04:50 must be open")
	}

	d.school = &write.CurfewWindow{Start: "21:00", End: "05:00"} // the school tightens it
	if cf, err = h.curfew(ctx, uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if active, reopen := cf.Active(at); !active || !reopen.Equal(time.Date(2026, 11, 20, 5, 0, 0, 0, ict)) {
		t.Fatalf("school 21:00-05:00 ∪ platform 21:30-04:00 at 04:50: active=%v reopen=%v", active, reopen)
	}
	if active, _ := cf.Active(time.Date(2026, 11, 20, 5, 10, 0, 0, ict)); active {
		t.Fatal("05:10 is outside both windows")
	}
}

func TestSessionsCurfewReadError(t *testing.T) {
	boom := errors.New("db down")
	h := Sessions{Directory: &directory{err: boom}, Config: SessionConfig{PlatformCurfew: window(t, "21:30-06:30"), CurfewLocation: ict}}
	if _, err := h.curfew(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, boom) {
		t.Fatalf("err = %v (a read error must not lift the curfew silently)", err)
	}
}

// settingsStore is a TEST DOUBLE of write.PlatformSettingWriter (R17) with the source_version rule of the SQL upsert.
type settingsStore struct {
	rows map[string]write.RefPlatformSetting
}

func (s *settingsStore) UpsertPlatformSetting(_ context.Context, r write.RefPlatformSetting) (bool, error) {
	if cur, ok := s.rows[r.Key]; ok && cur.SourceVersion >= r.SourceVersion {
		return false, nil
	}
	s.rows[r.Key] = r
	return true, nil
}

func TestApplyPlatformCurfew(t *testing.T) {
	ctx := context.Background()
	s := &settingsStore{rows: map[string]write.RefPlatformSetting{}}
	h := PlatformSettings{Refs: s}
	if ok, err := h.ApplyPlatformCurfew(ctx, write.CurfewWindow{Start: "21:30", End: "04:00"}, 200); err != nil || !ok {
		t.Fatalf("apply: %v %v", ok, err)
	}
	if got := string(s.rows[write.KeyPlatformCurfew].Value); got != `{"start":"21:30","end":"04:00"}` {
		t.Fatalf("stored %s", got)
	}
	if ok, _ := h.ApplyPlatformCurfew(ctx, write.CurfewWindow{Start: "21:30", End: "04:00"}, 200); ok {
		t.Fatal("same version applied twice")
	}
	if ok, _ := h.ApplyPlatformCurfew(ctx, write.CurfewWindow{Start: "20:00", End: "06:30"}, 100); ok {
		t.Fatal("an older event overrode a newer value")
	}
	for _, w := range []write.CurfewWindow{{Start: "21:30", End: "21:30"}, {Start: "24:00", End: "06:00"}, {Start: "", End: "06:00"}} {
		if _, err := h.ApplyPlatformCurfew(ctx, w, 300); !errors.Is(err, ErrInvalidPlatformCurfew) {
			t.Fatalf("%+v: err = %v", w, err)
		}
	}
	if got := s.rows[write.KeyPlatformCurfew].SourceVersion; got != 200 {
		t.Fatalf("source_version = %d", got)
	}
}
