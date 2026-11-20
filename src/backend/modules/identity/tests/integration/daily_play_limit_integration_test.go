//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/settings"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/gameauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/settingscfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F013-AC-33 (OQ-17): with daily_play_limit_minutes on the school and the time played today (projection of
// game_session.ended) at the limit, gameLogin, selectGameLoginTenant and refreshGameToken mode = resume are refused
// with 403 DAILY_PLAY_LIMIT_REACHED + reopen_at = 00:00 ICT the next day, after SSO and without revoking the game
// session stored on the device; a school without the limit and the common tenant are not limited.
func TestDailyPlayLimitAtGameLogin(t *testing.T) {
	ctx := context.Background()
	g := newGameIntegration(ctx, t, gameOpts{})
	defaults, err := settingscfg.DefaultsFrom(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	policy := gameauth.SettingsPolicy{Reader: read.SettingsModel{Session: g.read}, PlayTime: read.PlayTimeModel{Session: g.read}, Defaults: defaults}
	g.auth.DailyLimit = policy
	loc := policy.Location()
	a := seedTenant(ctx, t, g.platform, "pgd-daily-a")
	b := seedTenant(ctx, t, g.platform, "pgd-daily-b")
	an := g.student(ctx, t, a, "hs.choi.nhieu")
	ended := consumer.DailyPlayTime{PlayTime: command.RecordPlayTime{PlayTime: write.PlayTimeRepo{}, Location: loc}}
	// The sessions end today (ICT) at least one hour after midnight, so they never spill into yesterday.
	now := time.Now().In(loc)
	endedAt := now
	if early := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, loc); now.Before(early) {
		endedAt = early
	}
	play := func(tenant string, user uuid.UUID, d time.Duration) {
		t.Helper()
		ev := gameEvent(t, "game_session.ended", tenant, map[string]any{"game_session_id": uuid.NewString(), "student_id": user.String(),
			"platform": "android", "duration_ms": d.Milliseconds(), "end_reason": "logout", "ended_at": endedAt.UTC()})
		sc, err := authz.EventScope(*ev.TenantID)
		if err != nil {
			t.Fatal(err)
		}
		if err := g.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return ended.Handle(ctx, ev) }); err != nil {
			t.Fatal(err)
		}
	}
	setLimit := func(school, settings string) {
		t.Helper()
		if _, err := g.platform.ExecContext(ctx, `UPDATE org.schools SET settings = $2::jsonb WHERE school_id = $1`, school, settings); err != nil {
			t.Fatal(err)
		}
	}
	dev := uuid.New()
	// No limit on the school: 3 hours played do not matter.
	play(a.tenant, an, 3*time.Hour)
	if r := g.login(t, "hs.choi.nhieu", dev); r.code != 200 {
		t.Fatalf("no limit: %d %v", r.code, r.body)
	}
	// Limit 240 minutes: 3 hours played → still allowed; the session stored on the device is kept for resume.
	setLimit(a.school, `{"daily_play_limit_minutes": 240}`)
	r := g.login(t, "hs.choi.nhieu", dev)
	if r.code != 200 {
		t.Fatalf("under the limit: %d %v", r.code, r.body)
	}
	rt := tokensOf(t, r)["refresh_token"].(string)
	// One more hour (split by the consumer, then added) → the limit is reached.
	play(a.tenant, an, time.Hour)
	logouts := g.kc.logouts
	r = g.login(t, "hs.choi.nhieu", dev)
	reopen := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	if endedAt.After(now) { // the ended_at moved to 01:00 is still today
		reopen = time.Date(endedAt.Year(), endedAt.Month(), endedAt.Day()+1, 0, 0, 0, 0, loc)
	}
	if r.code != 403 || codeOf(r) != string(problem.DailyPlayLimitReached) || r.body["reopen_at"] != reopen.UTC().Format(time.RFC3339) {
		t.Fatalf("limit reached: %d %v (want reopen_at %s)", r.code, r.body, reopen.UTC().Format(time.RFC3339))
	}
	if g.kc.logouts != logouts+1 {
		t.Fatal("no Keycloak session may survive a refused login")
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.login_events WHERE user_id = $1 AND result = 'daily_play_limit_reached'`, an); n != 1 {
		t.Fatal("journal result daily_play_limit_reached")
	}
	// SSO first: a wrong password is still INVALID_CREDENTIALS.
	wrong := gameLogin("hs.choi.nhieu", dev)
	wrong["password"] = "sai"
	if r := g.post(t, "/auth/game/login", wrong, nil); codeOf(r) != string(problem.InvalidCredentials) {
		t.Fatalf("order: %v", r.body)
	}
	// Resume is refused without revoking the session; renew still works.
	if r := g.refresh(t, rt, dev, a.tenant, "resume", "1.4.0"); r.code != 403 || codeOf(r) != string(problem.DailyPlayLimitReached) || r.body["reopen_at"] == nil {
		t.Fatalf("resume at the limit: %d %v", r.code, r.body)
	}
	if n := count(ctx, t, g.platform, `SELECT count(*) FROM identity.game_auth_sessions WHERE refresh_token_hash = $1 AND revoked_at IS NULL`,
		gamelogin.HashToken(rt)); n != 1 {
		t.Fatal("the daily limit revoked the session")
	}
	if r := g.refresh(t, rt, dev, a.tenant, "renew", ""); r.code != 200 {
		t.Fatalf("renew at the limit: %d %v", r.code, r.body)
	}
	// selectGameLoginTenant of the limited tenant is refused too (second tenant offered by the TEST DOUBLE extraMembership).
	g.members.extra = &gamelogin.Membership{TenantID: uuid.MustParse(b.tenant), TenantType: "unit", TenantName: "pgd-daily-b", TenantStatus: "active",
		MemberKind: "student"}
	r = g.login(t, "hs.choi.nhieu", dev)
	ticket, _ := r.body["login_ticket"].(string)
	if r.code != 403 || codeOf(r) != string(problem.TenantNotSelected) || ticket == "" {
		t.Fatalf("two tenants: %d %v", r.code, r.body)
	}
	if r := g.post(t, "/auth/game/tenant:select", map[string]any{"login_ticket": ticket, "tenant_id": a.tenant, "device_id": dev.String()}, nil); r.code != 403 ||
		codeOf(r) != string(problem.DailyPlayLimitReached) {
		t.Fatalf("select at the limit: %d %v", r.code, r.body)
	}
	g.members.extra = nil
	// Turning the limit off lets the student in again.
	setLimit(a.school, `{"daily_play_limit_minutes": null}`)
	if r := g.login(t, "hs.choi.nhieu", dev); r.code != 200 {
		t.Fatalf("limit off: %d %v", r.code, r.body)
	}
	// Common tenant: a limit written on the virtual school is never applied.
	common := uuid.New()
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO identity.users (user_id, created_at, updated_at, user_type, status, username, display_name, primary_tenant_id, provisioned_at)
		VALUES ($1, now(), now(), 'student', 'active', 'hs.chung.choi', 'hs.chung.choi', $2, now())`, common, commonTenant); err != nil {
		t.Fatal(err)
	}
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO org.students (student_id, tenant_id, created_at, updated_at, school_id, grade_level, full_name)
		VALUES ($1, $2, now(), now(), $3, 4, 'Hoc Sinh Chung')`, common, commonTenant, commonSchool); err != nil {
		t.Fatal(err)
	}
	if _, err := g.platform.ExecContext(ctx, `INSERT INTO org.enrollments (enrollment_id, tenant_id, created_at, updated_at, school_id, student_id, class_id, status, start_date, source)
		VALUES ($1, $2, now(), now(), $3, $4, $5, 'active', current_date, 'platform_common')`, uuid.New(), commonTenant, commonSchool, common, commonG4); err != nil {
		t.Fatal(err)
	}
	g.identity(ctx, t, common, "hs.chung.choi")
	setLimit(commonSchool, `{"daily_play_limit_minutes": 15}`)
	play(commonTenant, common, time.Hour)
	if r := g.login(t, "hs.chung.choi", dev); r.code != 200 {
		t.Fatalf("common tenant: %d %v", r.code, r.body)
	}
}

// The projection identity.daily_play_time: a session over midnight is split over both ICT days, RLS keeps the rows
// of the tenant and a payload without duration is refused (DLQ).
func TestDailyPlayTimeProjection(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	f := seedTenant(ctx, t, e.platform, "pgd-play-time")
	other := seedTenant(ctx, t, e.platform, "pgd-play-time-b")
	an := enrolled(ctx, t, e.platform, f, "hs.qua.dem")
	loc := time.FixedZone("ICT", 7*3600)
	c := consumer.DailyPlayTime{PlayTime: command.RecordPlayTime{PlayTime: write.PlayTimeRepo{}, Location: loc}}
	ev := gameEvent(t, "game_session.ended", f.tenant, map[string]any{"game_session_id": uuid.NewString(), "student_id": an.String(),
		"platform": "web", "duration_ms": (90 * time.Minute).Milliseconds(), "end_reason": "timeout",
		"ended_at": time.Date(2026, 11, 21, 0, 30, 0, 0, loc).UTC()})
	sc, _ := authz.EventScope(*ev.TenantID)
	if err := e.uow.InTenantTx(ctx, sc, func(ctx context.Context) error { return c.Handle(ctx, ev) }); err != nil {
		t.Fatal(err)
	}
	pt := read.PlayTimeModel{Session: e.read}
	tenantCtx := func(t *testing.T, tenant string) context.Context {
		t.Helper()
		sc, err := authz.EventScope(uuid.MustParse(tenant))
		if err != nil {
			t.Fatal(err)
		}
		return tenancy.WithScope(ctx, sc)
	}
	for day, want := range map[string]time.Duration{"2026-11-20": time.Hour, "2026-11-21": 30 * time.Minute} {
		d, _ := time.Parse(time.DateOnly, day)
		got, err := pt.PlayedOn(tenantCtx(t, f.tenant), uuid.MustParse(f.tenant), an, settings.DateOf(d, time.UTC))
		if err != nil || got != want {
			t.Fatalf("%s: %v %v", day, got, err)
		}
	}
	// Another tenant's scope sees nothing (RLS).
	if got, err := pt.PlayedOn(tenantCtx(t, other.tenant), uuid.MustParse(f.tenant), an, settings.PlayDate{Year: 2026, Month: 11, Day: 20}); err != nil || got != 0 {
		t.Fatalf("other tenant: %v %v", got, err)
	}
	bad := gameEvent(t, "game_session.ended", f.tenant, map[string]any{"student_id": an.String(), "ended_at": time.Now().UTC()})
	if err := c.Handle(ctx, bad); err == nil {
		t.Fatal("payload without duration accepted")
	}
}
