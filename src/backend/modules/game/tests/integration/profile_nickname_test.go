//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func nicknames(t *testing.T, r resp) []string {
	t.Helper()
	data, _ := r.body["data"].([]any)
	out := make([]string, 0, len(data))
	for _, d := range data {
		m := d.(map[string]any)
		if _, err := uuid.Parse(m["nickname_option_id"].(string)); err != nil {
			t.Fatalf("option id %v", m)
		}
		out = append(out, m["nickname"].(string))
	}
	return out
}

// T019 (F070-AC-07): suggestNicknames answers count suggestions (default 6, up to 10) in the language of
// Accept-Language, each ≤ 24 characters, distinct and not used by a live profile of the tenant; names used in
// another tenant do not count.
func TestSuggestNicknames(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	r := e.suggest(t, s, 0, "vi-VN")
	vi := nicknames(t, r)
	if r.status != http.StatusOK || len(vi) != 6 {
		t.Fatalf("default count: %d %s", r.status, r.raw)
	}
	seen := map[string]bool{}
	for _, n := range vi {
		if utf8.RuneCountInString(n) > 24 || seen[n] {
			t.Fatalf("suggestion %q", n)
		}
		seen[n] = true
	}
	en := nicknames(t, e.suggest(t, s, 10, "en-US,en;q=0.9"))
	if len(en) != 10 {
		t.Fatalf("en: %v", en)
	}
	// English words of the seed (adjective + noun + 2 digits); Vietnamese ones have diacritics.
	for _, n := range en {
		if strings.ContainsAny(n, "ăâđêôơưàáạảãèéẹẻẽ") {
			t.Fatalf("en suggestion %q", n)
		}
	}
	if r := e.suggest(t, s, 11, ""); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("count 11: %d", r.status)
	}

	// Taken names are never suggested: a catalog with 1 adjective × 1 noun × 90 numbers, 85 taken in tenant A.
	e.publishCatalog(ctx, t, catalogPayload(t, map[string]any{"vi": map[string]any{"adjectives": []any{"Vui Vẻ"}, "nouns": []any{"Cú Mèo"}},
		"en": map[string]any{"adjectives": []any{"Happy"}, "nouns": []any{"Owl"}}}))
	for i := 10; i < 95; i++ {
		o := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
		e.setNickname(ctx, t, o, fmt.Sprintf("Cú Mèo Vui Vẻ %d", i))
	}
	other := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	e.setNickname(ctx, t, other, "Cú Mèo Vui Vẻ 95") // another tenant: still free in tenant A
	free := nicknames(t, e.suggest(t, s, 10, "vi"))
	if len(free) != 5 {
		t.Fatalf("free names: %v", free)
	}
	for _, n := range free {
		var num int
		if _, err := fmt.Sscanf(strings.TrimPrefix(n, "Cú Mèo Vui Vẻ "), "%d", &num); err != nil || num < 95 {
			t.Fatalf("taken name suggested: %q", n)
		}
	}
}

// setNickname gives o a profile with nickname n directly (fixture of other students).
func (e *env) setNickname(ctx context.Context, t *testing.T, o student, n string) {
	t.Helper()
	if _, err := e.platform.ExecContext(ctx, `INSERT INTO game.game_profiles (game_profile_id, tenant_id, student_id, school_id, created_at,
		updated_at, settings, nickname, nickname_key) VALUES ($1, $2, $3, $4, now(), now(), '{}', $5::varchar, lower($5::varchar))`, uuid.New(), o.tenant, o.id,
		o.school, n); err != nil {
		t.Fatal(err)
	}
}

// T020 (F070-AC-08, AC-09, AC-10): a received option sets the nickname (history "initial", game_profile.updated,
// setup_required false with an avatar); an unknown, expired or other student's option, or a name taken meanwhile in
// the tenant, is 409 NICKNAME_UNAVAILABLE. F136 (ADR-0105) replaces F070-AC-10: a typed "nickname" is accepted through
// moderation (nickname_free_text_test.go); "nickname" together with "nickname_option_id" is 422 VALIDATION_FAILED.
func TestChangeNickname(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.putAvatar(t, an, avatar()); r.status != http.StatusOK {
		t.Fatalf("avatar: %s", r.raw)
	}
	r := e.suggest(t, an, 6, "vi")
	data := r.body["data"].([]any)
	first := data[0].(map[string]any)
	if r := e.patchProfile(t, an, map[string]any{"nickname": "Tên tự gõ", "nickname_option_id": first["nickname_option_id"]}); r.status != http.StatusUnprocessableEntity ||
		r.code() != string(problem.ValidationFailed) {
		t.Fatalf("nickname and option together: %d %s", r.status, r.raw)
	}
	if r := e.patchProfile(t, binh, map[string]any{"nickname_option_id": first["nickname_option_id"]}); r.status != http.StatusConflict ||
		r.code() != string(problem.NicknameUnavailable) {
		t.Fatalf("option of another student: %d %s", r.status, r.raw)
	}
	if r := e.patchProfile(t, an, map[string]any{"nickname_option_id": uuid.NewString()}); r.status != http.StatusConflict ||
		r.code() != string(problem.NicknameUnavailable) {
		t.Fatalf("unknown option: %d %s", r.status, r.raw)
	}
	r = e.patchProfile(t, an, map[string]any{"nickname_option_id": first["nickname_option_id"]})
	if r.status != http.StatusOK || r.body["nickname"] != first["nickname"] || r.body["setup_required"] != false {
		t.Fatalf("set: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'initial'
		AND new_nickname = $2 AND old_nickname IS NULL`, an.id, first["nickname"]); n != 1 {
		t.Fatalf("history: %d", n)
	}
	ups := e.payloads(ctx, t, "game_profile.updated")
	last := ups[len(ups)-1]
	if last["nickname"] != first["nickname"] || last["student_id"] != an.id.String() || last["tenant_id"] != an.tenant.String() {
		t.Fatalf("game_profile.updated %v", last)
	}
	// Options are cleared once a nickname is chosen; an expired option is refused.
	if r := e.patchProfile(t, an, map[string]any{"nickname_option_id": data[1].(map[string]any)["nickname_option_id"]}); r.status != http.StatusConflict {
		t.Fatalf("option after choice: %d %s", r.status, r.raw)
	}
	old := e.options.put(binh.id, "Sóc Nâu Ham Học 42")
	e.clock.Add(11 * time.Minute)
	if r := e.patchProfile(t, binh, map[string]any{"nickname_option_id": old}); r.status != http.StatusConflict ||
		r.code() != string(problem.NicknameUnavailable) {
		t.Fatalf("expired option: %d %s", r.status, r.raw)
	}
	// Taken meanwhile: binh received the name an now holds (case-insensitive).
	taken := e.options.put(binh.id, strings.ToUpper(first["nickname"].(string)))
	if r := e.patchProfile(t, binh, map[string]any{"nickname_option_id": taken}); r.status != http.StatusConflict ||
		r.code() != string(problem.NicknameUnavailable) {
		t.Fatalf("taken: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND nickname IS NOT NULL`, binh.id); n != 0 {
		t.Fatal("binh got a nickname")
	}
	// settings and preferred_language (merge patch) change nothing other players see: no event.
	before := e.events(ctx, t, "game_profile.updated")
	r = e.patchProfile(t, an, map[string]any{"settings": map[string]any{"music": 0.5}, "preferred_language": "en-US"})
	if r.status != http.StatusOK || r.body["preferred_language"] != "en-US" || r.body["settings"].(map[string]any)["music"] != 0.5 {
		t.Fatalf("settings: %d %s", r.status, r.raw)
	}
	if e.events(ctx, t, "game_profile.updated") != before {
		t.Fatal("settings published game_profile.updated")
	}
	if r := e.patchProfile(t, an, map[string]any{"preferred_language": "fr-FR"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unsupported language: %d %s", r.status, r.raw)
	}
	e.checkEventSchemas(ctx, t)
}

// T021 (F070-AC-11): after the initial nickname, a student changes it nickname_change_monthly_limit (1) times per
// calendar month; the next change is 409 NICKNAME_CHANGE_LIMIT with retry_after = first day of next month (ICT); a
// school setting may only lower the limit.
func TestNicknameMonthlyLimit(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	e.clock.Set(time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC))
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.ready(t, s, "Cú Mèo Chăm Chỉ 27")
	if r := e.patchProfile(t, s, map[string]any{"nickname_option_id": e.options.put(s.id, "Sóc Nâu Vui Vẻ 12")}); r.status != http.StatusOK {
		t.Fatalf("first change: %d %s", r.status, r.raw)
	}
	r := e.patchProfile(t, s, map[string]any{"nickname_option_id": e.options.put(s.id, "Cá Heo Tò Mò 33")})
	if r.status != http.StatusConflict || r.code() != string(problem.NicknameChangeLimit) {
		t.Fatalf("second change: %d %s", r.status, r.raw)
	}
	want := time.Date(2026, 12, 1, 0, 0, 0, 0, ict).UTC().Format(time.RFC3339)
	if r.body["retry_after"] != want {
		t.Fatalf("retry_after %v, want %s", r.body["retry_after"], want)
	}
	e.clock.Set(time.Date(2026, 11, 30, 17, 1, 0, 0, time.UTC)) // 1 Dec 00:01 ICT
	if r := e.patchProfile(t, s, map[string]any{"nickname_option_id": e.options.put(s.id, "Cá Heo Tò Mò 33")}); r.status != http.StatusOK {
		t.Fatalf("next month: %d %s", r.status, r.raw)
	}
	// A school policy of 0 changes per month forbids player changes (it cannot raise the platform limit).
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.schools SET settings_subset = '{"nickname_change_monthly_limit": 0}'
		WHERE school_id = $1`, s.school); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(time.Date(2027, 1, 5, 3, 0, 0, 0, time.UTC))
	if r := e.patchProfile(t, s, map[string]any{"nickname_option_id": e.options.put(s.id, "Rái Cá Khéo Tay 44")}); r.status != http.StatusConflict ||
		r.code() != string(problem.NicknameChangeLimit) {
		t.Fatalf("school limit 0: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'player_change'`, s.id); n != 2 {
		t.Fatalf("player changes: %d", n)
	}
}
