//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// T023 — F136-AC-07 (server part), AC-08 (TC-GAM-013): after the first nickname, a typed change is a player_change
// counted by nickname_change_monthly_limit (1); beyond it 409 NICKNAME_CHANGE_LIMIT with retry_after (start of the next
// month, Asia/Ho_Chi_Minh) BEFORE the moderation is called; getGameProfile reports nickname_changes_left and
// nickname_change_available_at.
func TestNicknameChangeLimit(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	dung := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.ready(t, dung, "Cú Mèo Chăm Chỉ 27")
	p := e.profile(t, dung)
	if p.body["nickname_changes_left"] != float64(1) || p.body["nickname_change_available_at"] != nil {
		t.Fatalf("before the change: %s", p.raw)
	}
	if r := e.typeNickname(t, dung, "Rồng Xanh Bay Cao"); r.status != http.StatusOK {
		t.Fatalf("first change: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'player_change'
		AND old_nickname = 'Cú Mèo Chăm Chỉ 27' AND new_nickname = 'Rồng Xanh Bay Cao'`, dung.id); n != 1 {
		t.Fatalf("player_change history: %d", n)
	}
	calls := e.nickMod.count()
	r := e.typeNickname(t, dung, "Mèo Con Tinh Nghịch")
	if r.status != http.StatusConflict || r.code() != string(problem.NicknameChangeLimit) || r.body["retry_after"] == nil {
		t.Fatalf("second change: %d %s", r.status, r.raw)
	}
	if e.nickMod.count() != calls {
		t.Fatal("the monthly limit is checked before the moderation")
	}
	next := time.Date(2026, 12, 1, 0, 0, 0, 0, ict).UTC().Format(time.RFC3339)
	p = e.profile(t, dung)
	if p.body["nickname_changes_left"] != float64(0) || p.body["nickname_change_available_at"] != next {
		t.Fatalf("after the change: %s (want available at %s)", p.raw, next)
	}
	// The same nickname again is not a change (no limit, no facade call).
	if r := e.typeNickname(t, dung, "rồng xanh bay cao"); r.status != http.StatusConflict {
		// a different case is a different nickname for the student: counted, refused by the limit
		t.Fatalf("case change: %d %s", r.status, r.raw)
	}
	if r := e.typeNickname(t, dung, "Rồng Xanh  Bay Cao"); r.status != http.StatusOK {
		t.Fatalf("same nickname: %d %s", r.status, r.raw)
	}
	e.clock.Add(13 * 24 * time.Hour) // 2026-12-02: a new month
	if r := e.typeNickname(t, dung, "Mèo Con Tinh Nghịch"); r.status != http.StatusOK {
		t.Fatalf("next month: %d %s", r.status, r.raw)
	}
}

// T023 — F136-AC-09 (TC-GAM-011): a successful typed change publishes game_profile.updated with the new nickname only —
// no real name, username or school (R16) — for ref.player_profiles of competition and chat.
func TestNicknameProjectionUpdated(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.ready(t, an, "Cú Mèo Chăm Chỉ 27")
	before := e.events(ctx, t, "game_profile.updated")
	if r := e.typeNickname(t, an, "Siêu Nhân Sao Băng"); r.status != http.StatusOK {
		t.Fatalf("change: %d %s", r.status, r.raw)
	}
	ups := e.payloads(ctx, t, "game_profile.updated")
	if len(ups) != before+1 {
		t.Fatalf("game_profile.updated %d → %d", before, len(ups))
	}
	last := ups[len(ups)-1]
	if last["nickname"] != "Siêu Nhân Sao Băng" || last["tenant_id"] != an.tenant.String() {
		t.Fatalf("payload %v", last)
	}
	for _, k := range []string{"full_name", "username", "school_id", "school_name", "nickname_key"} {
		if _, ok := last[k]; ok {
			t.Fatalf("payload carries %s: %v", k, last)
		}
	}
}
