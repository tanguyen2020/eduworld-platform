//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/questsql"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/worldsql"
)

// kxID is the id of fixture quest n of the keyset tests.
func kxID(n int) string { return fmt.Sprintf("0193a5f4-6c1e-7b2a-9d1f-0000000d%04d", n) }

// kxQuests is the fixture matrix of listQuests (grade 3, learning_zone.math unless edited): every branch of quest.Decide
// (subject, required_subject_enabled, window, discovery trigger, grade) and of quest.CanRepeat (once, repeatable with
// max and cooldown, per_period daily/weekly), the non-acceptable triggers and enough quests for several pages.
func kxQuests(now time.Time) map[int]command.ContentItem {
	day := 24 * time.Hour
	set := func(k string, v any) func(m map[string]any) { return func(m map[string]any) { m[k] = v } }
	edits := map[int]func(m map[string]any){
		1: nil, 2: nil, 3: nil, 4: nil, 5: nil, 6: nil, // once, quest_board
		7:  set("repeatability", map[string]any{"mode": "repeatable", "cooldown_hours": 24, "max_completions": 3}),
		8:  set("repeatability", map[string]any{"mode": "repeatable", "max_completions": 1}),
		9:  set("availability", map[string]any{"recurrence": "daily"}),
		10: set("availability", map[string]any{"recurrence": "weekly"}),
		11: set("availability", map[string]any{"end_at": now.Add(-day).Format(time.RFC3339)}),
		12: set("availability", map[string]any{"start_at": now.Add(day).Format(time.RFC3339)}),
		13: set("start_trigger", map[string]any{"kind": "discovery", "interactable_ids": []string{"discovery.hub.spark_tree"}}),
		14: set("start_trigger", map[string]any{"kind": "discovery", "interactable_ids": []string{"discovery.math.old_well"}}),
		15: set("start_trigger", map[string]any{"kind": "assignment"}),
		16: set("start_trigger", map[string]any{"kind": "assignment"}),
		17: set("subject_code", "vietnamese"),
		18: set("prerequisites", map[string]any{"required_subject_enabled": false}),
		19: set("grade_levels", []int{4}),
		20: set("prerequisites", map[string]any{"completed_quest_ids": []string{questMainA}}),
		21: set("repeatability", map[string]any{"mode": "repeatable", "cooldown_hours": 2}),
		22: nil, // active progress pinned to v1, then v2 published
		23: nil, // active progress, then unassigned
		24: set("quest_type", "side"),
	}
	out := map[int]command.ContentItem{}
	for n, edit := range edits {
		out[n] = learningQuest(kxID(n), fmt.Sprintf("kx.q%02d", n), edit)
	}
	return out
}

// progressFixture writes one quest_progress row of s on the current version of questID in the tenant transaction of s
// (RLS WITH CHECK), as the quest commands would.
func (e *env) progressFixture(ctx context.Context, t *testing.T, s student, questID, status, period string, started time.Time, completed *time.Time) {
	t.Helper()
	var p any
	if period != "" {
		p = period
	}
	if err := e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error {
		_, err := txOf(ctx).ExecContext(ctx, `INSERT INTO game.quest_progress (quest_progress_id, tenant_id, created_at, updated_at, student_id,
			school_id, quest_id, quest_version_id, quest_type, status, step_states, start_trigger, period_key, started_at, completed_at, ended_at)
		SELECT $1, $2, now(), now(), $3, $4, d.quest_id, d.current_version_id, v.quest_type, $5::text, '{}', v.start_trigger, $6, $7, $8,
			CASE WHEN $5::text = 'active' THEN NULL ELSE COALESCE($8::timestamptz, $7::timestamptz) END
		FROM game.quest_definitions d JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id WHERE d.quest_id = $9`,
			uuid.New(), s.tenant, s.id, s.school, status, p, started, completed, questID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// oldQuestList is the reference of the in-memory listQuests replaced on 2026-11-24 (owner rule: never cut a page in
// memory): every history of the student, every assigned quest, quest.Decide and quest.CanRepeat in Go, then the
// filters, the sort by (status rank, code, quest_id) and the whole list.
func oldQuestList(ctx context.Context, tenant, id uuid.UUID, reset quest.Reset, now time.Time, in query.ListInput) ([]query.QuestSummary, error) {
	c, w := questsql.Content{}, worldsql.Content{}
	type ent struct {
		row port.QuestRow
		sum query.QuestSummary
	}
	summaryOf := func(row port.QuestRow) query.QuestSummary {
		v := row.Version
		_, until := v.Window()
		return query.QuestSummary{QuestID: row.QuestID, QuestVersionID: row.VersionID, Code: v.Code, Name: v.Name, QuestType: v.QuestType,
			GradeLevels: v.GradeLevels, SubjectID: v.SubjectID, SubjectCode: v.SubjectCode, AreaID: v.AreaID, GiverNPCID: v.GiverNPCID,
			StoryChapterCode: v.StoryChapterCode, StartTrigger: v.StartTrigger.Kind, AvailableUntil: until}
	}
	entries := map[uuid.UUID]*ent{}
	st, err := w.Student(ctx, tenant, id)
	if err != nil || st == nil || st.Deleted || st.Status != "active" {
		return nil, err
	}
	hist, err := c.Histories(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	for qid, hs := range hist {
		if hs.Latest == nil || !hs.Latest.Active() {
			continue
		}
		row, err := c.QuestVersion(ctx, hs.Latest.VersionID)
		if err != nil {
			return nil, err
		}
		if row == nil {
			continue
		}
		sum := summaryOf(*row)
		sum.Status = quest.StatusActive
		v := questrules.ViewOf(hs.Latest, row.Version)
		sum.Progress = &v
		entries[qid] = &ent{row: *row, sum: sum}
	}
	rows, err := c.AssignedQuests(ctx, tenant, st.Grade)
	if err != nil {
		return nil, err
	}
	completed := map[uuid.UUID]bool{}
	for q, hs := range hist {
		completed[q] = hs.Completed > 0
	}
	f, err := questrules.Facts(ctx, w, c, tenant, id, st, completed)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if entries[row.QuestID] != nil {
			continue
		}
		v := row.Version
		d := quest.Decide(v, questrules.Student(st), f, now)
		if !d.Visible {
			continue
		}
		hs := hist[row.QuestID]
		sum := summaryOf(row)
		if hs != nil && hs.Latest != nil {
			pv := questrules.ViewOf(hs.Latest, v)
			sum.Progress = &pv
		}
		rep, retry := quest.CanRepeat(v, questrules.History(hs, v, reset, now), now)
		switch {
		case rep == quest.RepeatAlreadyCompleted:
			sum.Status, sum.LockedReason = quest.StatusCompleted, "already_completed"
		case v.AutoGranted() || !v.Acceptable():
			if hs == nil || hs.Latest == nil {
				continue
			}
			sum.Status = hs.Latest.Status
		case rep == quest.RepeatCooldown:
			sum.Status, sum.LockedReason, sum.RetryAfter = quest.StatusCompleted, "cooldown", retry
		default:
			sum.Status, sum.CanAccept = "available", d.Met
			if !d.Met {
				sum.LockedReason, sum.MissingRequirements = "prerequisite_not_met", d.Missing()
			}
		}
		entries[row.QuestID] = &ent{row: row, sum: sum}
	}
	inArea := func(v quest.Version, area string) bool {
		if v.AreaID == area || v.StartTrigger.AreaID == area {
			return true
		}
		for _, s := range v.Steps {
			if s.AreaID == area {
				return true
			}
		}
		return false
	}
	rank := map[string]int{quest.StatusActive: 0, "available": 1, quest.StatusCompleted: 2}
	key := func(s query.QuestSummary) string {
		r, ok := rank[s.Status]
		if !ok {
			r = 3
		}
		return fmt.Sprintf("%d:%s", r, s.Code)
	}
	var list []query.QuestSummary
	for _, e := range entries {
		switch {
		case in.Status != "" && e.sum.Status != in.Status, in.QuestType != "" && e.sum.QuestType != in.QuestType,
			in.AreaID != "" && !inArea(e.row.Version, in.AreaID):
			continue
		}
		list = append(list, e.sum)
	}
	slices.SortFunc(list, func(a, b query.QuestSummary) int {
		if c := strings.Compare(key(a), key(b)); c != 0 {
			return c
		}
		return strings.Compare(a.QuestID.String(), b.QuestID.String())
	})
	return list, nil
}

// allPages walks listQuests with query q and limit n through every page; it checks the page size, has_more and the
// stability of each cursor (the same cursor answers the same page).
func (e *env) allPages(t *testing.T, s student, q string, n int) ([]any, int) {
	t.Helper()
	var out []any
	cursor, pages := "", 0
	for {
		path := fmt.Sprintf("/game/quests?limit=%d", n)
		if q != "" {
			path += "&" + q
		}
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := e.do(t, http.MethodGet, path, e.gtoken(s), th(s), nil)
		if r.status != http.StatusOK {
			t.Fatalf("%s: %d %s", path, r.status, r.raw)
		}
		if again := e.do(t, http.MethodGet, path, e.gtoken(s), th(s), nil); again.raw != r.raw {
			t.Fatalf("%s: cursor not stable:\n%s\n%s", path, r.raw, again.raw)
		}
		pages++
		data, _ := r.body["data"].([]any)
		if len(data) > n {
			t.Fatalf("%s: %d rows for limit %d", path, len(data), n)
		}
		out = append(out, data...)
		pg := r.body["page"].(map[string]any)
		if pg["has_more"] != true {
			if pg["next_cursor"] != nil {
				t.Fatalf("%s: last page with a cursor", path)
			}
			return out, pages
		}
		if len(data) != n {
			t.Fatalf("%s: short page with has_more", path)
		}
		cursor = pg["next_cursor"].(string)
		if pages > 50 {
			t.Fatalf("%s: no end", path)
		}
	}
}

// Owner rule 2026-11-24 (API_DESIGN §5, CODE_PLAN §4.3): listQuests computes the status and the visibility of each quest
// (quest.Decide, quest.CanRepeat) and pages by keyset (status rank, code, quest_id) in SQL. On a fixture matrix of
// quests × histories × students × filters, the pages walked with small limits equal the replaced in-memory result
// exactly (same rows, same order, same fields), each cursor is stable, getQuest agrees with the list, and tenant B
// never sees a quest assigned to tenant A only or the progress of a student of A.
func TestQuestListKeysetEqualsInMemory(t *testing.T) {
	ctx := context.Background()
	e := setupQuests(ctx, t)
	now := e.clock.Now()
	day := 24 * time.Hour
	fx := kxQuests(now)
	var items []command.ContentItem
	for n := 1; n <= len(fx); n++ {
		items = append(items, fx[n])
	}
	e.publishQuests(ctx, t, items, e.tenantA)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	fresh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	em := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, em, 4)
	e.enableOnly(ctx, t, e.schoolA2, "vietnamese")
	other := e.newStudent(ctx, t, e.tenantA, e.schoolA2, "active")
	dung := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	// histories of an: every repeat branch, an abandoned and a non-acceptable quest with history, two active progress
	reset := questCfg().Reset
	ago := func(d time.Duration) *time.Time { x := now.Add(-d); return &x }
	e.progressFixture(ctx, t, an, kxID(1), "completed", "", now.Add(-3*day), ago(2*day))
	e.progressFixture(ctx, t, an, kxID(2), "abandoned", "", now.Add(-3*day), nil)
	e.progressFixture(ctx, t, an, kxID(7), "completed", "", now.Add(-2*time.Hour), ago(time.Hour))
	e.progressFixture(ctx, t, an, kxID(8), "completed", "", now.Add(-4*day), ago(3*day))
	e.progressFixture(ctx, t, an, kxID(9), "completed", reset.PeriodKey(quest.RecurrenceDaily, now), now.Add(-2*time.Hour), ago(time.Hour))
	e.progressFixture(ctx, t, an, kxID(10), "completed", reset.PeriodKey(quest.RecurrenceWeekly, now.Add(-7*day)), now.Add(-8*day), ago(8*day))
	e.progressFixture(ctx, t, an, kxID(16), "abandoned", "", now.Add(-day), nil)
	e.progressFixture(ctx, t, an, kxID(21), "completed", "", now.Add(-5*time.Hour), ago(4*time.Hour)) // cooldown over
	e.progressFixture(ctx, t, an, kxID(22), "active", "", now.Add(-time.Hour), nil)
	e.progressFixture(ctx, t, an, kxID(23), "active", "", now.Add(-time.Hour), nil)
	e.progressFixture(ctx, t, an, questMainA, "completed", "", now.Add(-5*day), ago(5*day))
	e.progressFixture(ctx, t, dung, questMainA, "active", "", now.Add(-time.Hour), nil)
	// an found the spark tree (exploration_progress, as the world discovery command records it): kx.q13 shows, kx.q14 not
	if err := e.uow.InTenantTx(ctx, scopeOf(t, an.tenant), func(ctx context.Context) error {
		_, err := txOf(ctx).ExecContext(ctx, `INSERT INTO game.exploration_progress (exploration_id, tenant_id, created_at, updated_at, student_id,
			area_id, discoveries, collectibles) VALUES ($1, $2, now(), now(), $3, 'main_hub', '["discovery.hub.spark_tree"]', '[]')`,
			uuid.New(), an.tenant, an.id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// a new version of 22 (an keeps its pinned v1), 23 unassigned (an keeps its active progress)
	e.publishQuests(ctx, t, []command.ContentItem{learningQuest(kxID(22), "kx.q22", func(m map[string]any) {
		m["name"] = map[string]any{"vi": "Phiên bản hai", "en": "Version two"}
	})}, e.tenantA)
	e.unassign(ctx, t, uuid.MustParse(kxID(23)), e.tenantA)

	filters := []string{"", "status=active", "status=available", "status=completed", "status=abandoned", "quest_type=learning",
		"quest_type=main", "quest_type=side", "area_id=learning_zone.math", "area_id=main_hub", "status=available&quest_type=learning"}
	students := map[string]student{"an": an, "fresh": fresh, "em": em, "other": other, "dung": dung}
	multiPage := false
	for name, s := range students {
		for _, f := range filters {
			in := query.ListInput{}
			for _, kv := range strings.Split(f, "&") {
				k, v, _ := strings.Cut(kv, "=")
				switch k {
				case "status":
					in.Status = v
				case "quest_type":
					in.QuestType = v
				case "area_id":
					in.AreaID = v
				}
			}
			var want []query.QuestSummary
			if err := e.uow.InTenantTx(ctx, scopeOf(t, s.tenant), func(ctx context.Context) error {
				var err error
				want, err = oldQuestList(ctx, s.tenant, s.id, reset, now, in)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(want)
			var wantJSON []any
			_ = json.Unmarshal(raw, &wantJSON)
			for _, n := range []int{2, 3, 100} {
				got, pages := e.allPages(t, s, f, n)
				if pages > 1 {
					multiPage = true
				}
				if len(got) == 0 && len(wantJSON) == 0 {
					continue
				}
				if !reflect.DeepEqual(got, wantJSON) {
					g, _ := json.Marshal(got)
					t.Fatalf("%s %q limit %d:\n got  %s\n want %s", name, f, n, g, raw)
				}
			}
			if f != "" {
				continue
			}
			// getQuest agrees with the list; a quest out of the list is 404
			listed := map[string]bool{}
			for _, w := range want {
				listed[w.QuestID.String()] = true
				d := e.do(t, http.MethodGet, "/game/quests/"+w.QuestID.String(), e.gtoken(s), th(s), nil)
				if d.status != http.StatusOK || d.body["status"] != w.Status || d.body["quest_version_id"] != w.QuestVersionID.String() ||
					d.body["can_accept"] != w.CanAccept {
					t.Fatalf("%s get %s: %d %s want %+v", name, w.Code, d.status, d.raw, w)
				}
			}
			for n := 1; n <= len(fx); n++ {
				if listed[kxID(n)] {
					continue
				}
				if d := e.do(t, http.MethodGet, "/game/quests/"+kxID(n), e.gtoken(s), th(s), nil); d.status != http.StatusNotFound {
					t.Fatalf("%s get hidden kx.q%02d: %d %s", name, n, d.status, d.raw)
				}
			}
		}
	}
	if !multiPage {
		t.Fatal("the matrix never needed more than one page")
	}
	// the matrix exercised every status of the old rules for an
	got := map[string]string{}
	all, _ := e.allPages(t, an, "", 100)
	for _, d := range all {
		m := d.(map[string]any)
		got[m["code"].(string)] = fmt.Sprint(m["status"], "/", m["locked_reason"])
	}
	for code, want := range map[string]string{"kx.q01": "completed/already_completed", "kx.q02": "available/<nil>",
		"kx.q07": "completed/cooldown", "kx.q08": "completed/already_completed", "kx.q09": "completed/already_completed",
		"kx.q10": "available/<nil>", "kx.q16": "abandoned/<nil>", "kx.q20": "available/<nil>", "kx.q21": "available/<nil>",
		"kx.q22": "active/<nil>", "kx.q23": "active/<nil>", "kx.q13": "available/<nil>"} {
		if got[code] != want {
			t.Fatalf("%s = %q, want %q (%v)", code, got[code], want, got)
		}
	}
	for _, hidden := range []string{"kx.q11", "kx.q12", "kx.q14", "kx.q15", "kx.q19"} {
		if _, ok := got[hidden]; ok {
			t.Fatalf("%s listed: %v", hidden, got)
		}
	}
	// tenant isolation: B lists none of the fixture (assigned to A only) and only its own progress
	for _, d := range func() []any { a, _ := e.allPages(t, dung, "", 100); return a }() {
		if c := d.(map[string]any)["code"].(string); strings.HasPrefix(c, "kx.") || c == "math.g3.test_learning" {
			t.Fatalf("tenant B lists %s", c)
		}
	}
	if act, _ := e.allPages(t, fresh, "status=active", 100); len(act) != 0 {
		t.Fatalf("a fresh student of A sees active progress: %v", act)
	}
	// a cursor of another list or a forged one is 400
	if r := e.do(t, http.MethodGet, "/game/quests?cursor=forged", e.gtoken(an), th(an), nil); r.status != http.StatusBadRequest {
		t.Fatalf("forged cursor: %d %s", r.status, r.raw)
	}
}
