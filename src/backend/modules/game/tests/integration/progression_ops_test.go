//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F074-AC-24: a new reward rule version applies only to events from its effective time (active_from, else publish
// time); an event that happened before is rewarded by the version in effect then, and XP already granted is never taken
// back.
func TestRewardRuleVersioning(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.clock.Add(time.Hour)
	before := e.lessonEvent(s, uuid.New(), []int{3}, "practice") // occurred under v1
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice"))
	e.clock.Add(time.Hour)
	v2 := progressionItem(t, ruleLesson, func(m map[string]any) {
		m["grants"] = []any{map[string]any{"type": "xp", "amount": 30}}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{v2}, "lesson rule v2"); err != nil {
		t.Fatal(err)
	}
	future := progressionItem(t, ruleLesson, func(m map[string]any) {
		m["grants"] = []any{map[string]any{"type": "xp", "amount": 45}}
		m["active_from"] = e.clock.Now().Add(48 * time.Hour).Format(time.RFC3339)
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{future}, "lesson rule v3 from the day after tomorrow"); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(time.Minute)
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice")) // v2 (v3 not yet in effect)
	e.mustDeliver(ctx, t, uc, before)                                             // late delivery: still v1
	e.clock.Add(72 * time.Hour)
	e.mustDeliver(ctx, t, uc, e.lessonEvent(s, uuid.New(), []int{3}, "practice")) // v3
	rows := e.ledger(ctx, t, s)
	if len(rows) != 4 || rows[0].amount != 20 || *rows[0].version != 1 || rows[1].amount != 30 || *rows[1].version != 2 ||
		rows[2].amount != 20 || *rows[2].version != 1 || rows[3].amount != 45 || *rows[3].version != 3 {
		t.Fatalf("ledger %+v", rows)
	}
	if _, xp := e.xpOf(ctx, t, s); xp != 115 {
		t.Fatalf("xp_total %d (nothing taken back)", xp)
	}
}

// F074-AC-25, F074-AC-26 (AC-GAM-016): a grade change keeps XP, level, title and ledger and expires (no penalty) the
// active learning quests of the old grade, idempotent per school year; an unchanged grade changes nothing and writes no
// message.
func TestGradeTransition(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	if _, err := e.publish(ctx, t, progressionSeed(t), "seed progression v1"); err != nil {
		t.Fatal(err)
	}
	uc := e.progressionUC()
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	tok := e.gtoken(an)
	for _, q := range []string{learnQuest, questMath3} {
		if r := e.accept(t, an, tok, q, ""); r.status != http.StatusOK {
			t.Fatalf("accept %s: %d %s", q, r.status, r.raw)
		}
	}
	e.mustDeliver(ctx, t, uc, e.questEvent(an, "main"))
	level, xp := e.xpOf(ctx, t, an)
	gt := consumer.GradeTransition{Progression: uc}
	deliver := func(typ string, payload map[string]any) {
		env := e.envelope(typ, "identity", an.tenant, payload)
		if err := e.uow.InTenantTx(ctx, scopeOf(t, an.tenant), func(ctx context.Context) error { return gt.Handle(ctx, env) }); err != nil {
			t.Fatal(err)
		}
	}
	placement := func(prev, grade int) map[string]any {
		return map[string]any{"student_id": an.id, "tenant_id": an.tenant, "school_id": an.school, "grade_level": grade, "previous_grade_level": prev,
			"status": "active", "changed_fields": []string{"grade_level"}, "source": "sis", "source_version": 2}
	}
	// AC-26: same grade (stays in grade 3) and a sync without a grade change: nothing changes, no message.
	outbox := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`)
	deliver("student.placement_changed", placement(3, 3))
	deliver("student.synced", map[string]any{"student_id": an.id, "school_id": an.school, "action": "update", "sync_run_id": uuid.New(),
		"grade_level": 3, "changed_fields": []string{"status"}})
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`); n != outbox {
		t.Fatalf("unchanged grade wrote %d messages", n-outbox)
	}
	if st, _ := e.progressRow(ctx, t, an, learnQuest); st != "active" {
		t.Fatalf("learning quest %s", st)
	}
	// AC-25: 3 → 4, twice.
	deliver("student.placement_changed", placement(3, 4))
	if st, _ := e.progressRow(ctx, t, an, learnQuest); st != "expired" {
		t.Fatalf("learning quest of grade 3: %s", st)
	}
	if st, _ := e.progressRow(ctx, t, an, questMath3); st != "active" {
		t.Fatalf("side quest kept: %s", st)
	}
	if l, x := e.xpOf(ctx, t, an); l != level || x != xp {
		t.Fatalf("level/xp changed: %d %d → %d %d", level, xp, l, x)
	}
	var year string
	if err := e.platform.QueryRowContext(ctx, `SELECT grade_transition_year FROM game.game_profiles WHERE student_id = $1`, an.id).Scan(&year); err != nil ||
		year != command.SchoolYear(e.clock.Now()) {
		t.Fatalf("grade_transition_year %q %v", year, err)
	}
	abandoned := e.events(ctx, t, "quest.abandoned")
	outbox = count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`)
	deliver("student.synced", map[string]any{"student_id": an.id, "school_id": an.school, "action": "update", "sync_run_id": uuid.New(),
		"grade_level": 4, "changed_fields": []string{"grade_level"}})
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events`); n != outbox || e.events(ctx, t, "quest.abandoned") != abandoned {
		t.Fatalf("second transition wrote %d messages", n-outbox)
	}
	if n := len(e.ledger(ctx, t, an)); n != 1 {
		t.Fatalf("ledger changed: %d", n)
	}
}

// F074 T041 (OQ F016-2): mergeUserData moves the XP ledger of the source to the target by compensating entries
// (append-only), idempotent per merge_id; the target keeps the higher level and reaches the levels of the summed XP.
func TestMergeXPLedger(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	f := facade.Facade{Lifecycle: command.StudentLifecycle{UoW: e.uow, Sessions: write.SessionRepo{}, Profiles: write.ProfileRepo{},
		Presets: write.PresetRepo{}, XP: uc}}
	fctx := tenancy.WithScope(ctx, scopeOf(t, e.tenantA))
	from := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	into := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(from, "main"))                            // 150: level 2
	e.mustDeliver(ctx, t, uc, e.questEvent(from, "main"))                            // 300: level 2
	e.mustDeliver(ctx, t, uc, e.lessonEvent(into, uuid.New(), []int{3}, "practice")) // 20: level 1
	body := &api.MergeUserDataJSONRequestBody{MergeId: uuid.New(), TargetUserId: into.id, Reason: "same person"}
	for i := 0; i < 2; i++ {
		if _, err := f.MergeUserData(fctx, api.MergeUserDataRequestObject{UserId: from.id, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	src, dst := e.ledger(ctx, t, from), e.ledger(ctx, t, into)
	if len(src) != 3 || sumAmounts(src) != 0 || src[2].amount != -300 || src[2].sourceType != "merge" || src[2].ruleID.Valid {
		t.Fatalf("source ledger %+v", src)
	}
	if len(dst) != 2 || sumAmounts(dst) != 320 || dst[1].amount != 300 {
		t.Fatalf("target ledger %+v", dst)
	}
	if level, xp := e.xpOf(ctx, t, into); level != 2 || xp != 320 {
		t.Fatalf("target level %d xp %d", level, xp)
	}
	if seen := e.checkEventSchemas(ctx, t); seen["xp.awarded"] != 5 {
		t.Fatalf("xp.awarded %v", seen)
	}
}

// F074-AC-03 (reconciliation part, SC-003): xp-reconcile reports a profile whose xp_total differs from Σ ledger (metric
// and audit record) and corrects nothing.
func TestXPReconcile(t *testing.T) {
	ctx := context.Background()
	e := setupProgression(ctx, t)
	uc := e.progressionUC()
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	ok := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.mustDeliver(ctx, t, uc, e.questEvent(s, "main"))
	e.mustDeliver(ctx, t, uc, e.questEvent(ok, "main"))
	if n, err := uc.ReconcileXP(ctx, 100); err != nil || n != 0 {
		t.Fatalf("clean run: %d %v", n, err)
	}
	if _, err := e.platform.ExecContext(ctx, `UPDATE game.game_profiles SET xp_total = 999 WHERE student_id = $1`, s.id); err != nil {
		t.Fatal(err)
	}
	before := testutil.ToFloat64(observability.LedgerMismatches().WithLabelValues("xp"))
	if n, err := uc.ReconcileXP(ctx, 100); err != nil || n != 1 {
		t.Fatalf("mismatch run: %d %v", n, err)
	}
	if got := testutil.ToFloat64(observability.LedgerMismatches().WithLabelValues("xp")) - before; got != 1 {
		t.Fatalf("metric +%v", got)
	}
	if n := e.audits(ctx, t, "game.xp_ledger.mismatch"); n != 1 {
		t.Fatalf("audit %d", n)
	}
	if _, xp := e.xpOf(ctx, t, s); xp != 999 {
		t.Fatalf("reconcile corrected the cache: %d", xp)
	}
}
