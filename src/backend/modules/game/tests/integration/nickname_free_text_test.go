//go:build integration

package integration

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T018 — F136-AC-03, AC-04 (TC-GAM-011): a typed nickname is normalised and stored with nickname_key, the history row
// "initial" and game_profile.updated in the same transaction; the suggestion path still works; both fields, or
// nickname null, are 422 VALIDATION_FAILED.
func TestNicknameFreeText(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.putAvatar(t, an, avatar()); r.status != http.StatusOK {
		t.Fatalf("avatar: %s", r.raw)
	}
	opt := e.options.put(an.id, "Cú Mèo Chăm Chỉ 27")
	h := th(an)
	h["Content-Type"] = "application/merge-patch+json"
	for name, body := range map[string]map[string]any{
		"both":          {"nickname": "Siêu Nhân Sao Băng", "nickname_option_id": opt},
		"nickname null": {"nickname": nil},
	} {
		if r := e.do(t, http.MethodPatch, "/game/profile", e.gtoken(an), h, body); r.status != http.StatusUnprocessableEntity ||
			r.code() != string(problem.ValidationFailed) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	r := e.typeNickname(t, an, "  Siêu   Nhân Sao  Băng ")
	if r.status != http.StatusOK || r.body["nickname"] != "Siêu Nhân Sao Băng" || r.body["setup_required"] != false {
		t.Fatalf("typed: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE student_id = $1 AND nickname = 'Siêu Nhân Sao Băng'
		AND nickname_key = 'siêu nhân sao băng'`, an.id); n != 1 {
		t.Fatalf("stored profile: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.nickname_changes WHERE student_id = $1 AND reason = 'initial'
		AND new_nickname = 'Siêu Nhân Sao Băng' AND old_nickname IS NULL AND changed_by = $1`, an.id); n != 1 {
		t.Fatalf("history: %d", n)
	}
	ups := e.payloads(ctx, t, "game_profile.updated")
	if last := ups[len(ups)-1]; last["nickname"] != "Siêu Nhân Sao Băng" || last["student_id"] != an.id.String() {
		t.Fatalf("game_profile.updated %v", last)
	}
	if e.audits(ctx, t, "game_profile.nickname_changed") != 1 {
		t.Fatal("audit game_profile.nickname_changed")
	}
	// The suggestion path keeps working (F070-AC-08) and the option is not screened.
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	calls := e.nickMod.count()
	if r := e.patchProfile(t, binh, map[string]any{"nickname_option_id": e.options.put(binh.id, "Sóc Nâu Ham Học 42")}); r.status != http.StatusOK {
		t.Fatalf("suggestion: %d %s", r.status, r.raw)
	}
	if e.nickMod.count() != calls {
		t.Fatal("a suggestion must not call the moderation")
	}
}

// T013 — F136-AC-13 (TC-GAM-013): uniqueness ignores case: "An An" taken → "an an" is 409 NICKNAME_UNAVAILABLE; two
// concurrent requests of the same key in a tenant: exactly one wins (unique partial (tenant_id, nickname_key)).
func TestNicknameKeyUniqueConcurrent(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.typeNickname(t, an, "An An Vui"); r.status != http.StatusOK {
		t.Fatalf("an: %d %s", r.status, r.raw)
	}
	if r := e.typeNickname(t, binh, "an AN vui"); r.status != http.StatusConflict || r.code() != string(problem.NicknameUnavailable) {
		t.Fatalf("case-insensitive duplicate: %d %s", r.status, r.raw)
	}
	var wg sync.WaitGroup
	statuses := make([]int, 6)
	students := make([]student, len(statuses))
	for i := range students {
		students[i] = e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	}
	for i, s := range students {
		wg.Add(1)
		go func(i int, s student) {
			defer wg.Done()
			name := "Sao Băng Đêm"
			if i%2 == 1 {
				name = "SAO BĂNG ĐÊM"
			}
			statuses[i] = e.typeNickname(t, s, name).status
		}(i, s)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, st := range statuses {
		switch st {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		}
	}
	if ok != 1 || conflict != len(statuses)-1 {
		t.Fatalf("concurrent: %v, want one 200 and the rest 409", statuses)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.game_profiles WHERE tenant_id = $1 AND nickname_key = 'sao băng đêm'
		AND deleted_at IS NULL`, e.tenantA); n != 1 {
		t.Fatalf("live rows with the key: %d", n)
	}
}

// T013 — F136-AC-15 (TC-GAM-013): nickname_attempt_rate (10 per 10 minutes, refusals counted) → the 11th typed
// submission is 429 RATE_LIMITED with Retry-After, before any facade call; never a sanction (no audit of one).
func TestNicknameAttemptRate(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	for i := 0; i < 10; i++ {
		if r := e.typeNickname(t, an, "Minh "+placeholderTerm); r.status != http.StatusUnprocessableEntity {
			t.Fatalf("attempt %d: %d %s", i+1, r.status, r.raw)
		}
	}
	calls := e.nickMod.count()
	r := e.typeNickname(t, an, "Siêu Nhân Sao Băng")
	if r.status != http.StatusTooManyRequests || r.code() != string(problem.RateLimited) || r.header.Get("Retry-After") == "" {
		t.Fatalf("11th: %d %s %v", r.status, r.raw, r.header)
	}
	if e.nickMod.count() != calls {
		t.Fatal("a rate-limited submission must not reach the moderation")
	}
	if e.events(ctx, t, "sanction.applied") != 0 {
		t.Fatal("never a sanction for trying")
	}
	e.clock.Add(11 * time.Minute)
	if r := e.typeNickname(t, an, "Siêu Nhân Sao Băng"); r.status != http.StatusOK {
		t.Fatalf("after the window: %d %s", r.status, r.raw)
	}
}

// T013 — the Valkey adapter of the attempt counter: one sliding window per student and tenant, numbers only (no text).
func TestNicknameAttemptCounterValkey(t *testing.T) {
	ctx := context.Background()
	_, addr := testkit.StartValkey(ctx, t)
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctr := cache.NicknameAttempts{C: c, Limit: 3, Window: 10 * time.Minute}
	tenant, a, b := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 11, 28, 3, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if ok, _, err := ctr.Hit(ctx, tenant, a, now.Add(time.Duration(i)*time.Minute)); err != nil || !ok {
			t.Fatalf("hit %d: %v %v", i, ok, err)
		}
	}
	ok, wait, err := ctr.Hit(ctx, tenant, a, now.Add(3*time.Minute))
	if err != nil || ok || wait <= 0 || wait > 7*time.Minute {
		t.Fatalf("4th: %v %v %v", ok, wait, err)
	}
	if ok, _, _ := ctr.Hit(ctx, tenant, b, now); !ok {
		t.Fatal("another student has its own window")
	}
	if ok, _, _ := ctr.Hit(ctx, tenant, a, now.Add(21*time.Minute)); !ok {
		t.Fatal("the window slides")
	}
	keys, err := c.Do(ctx, c.B().Keys().Pattern("ew:game:nick:attempts:*").Build()).AsStrSlice()
	if err != nil || len(keys) != 2 || !strings.Contains(keys[0], tenant.String()) {
		t.Fatalf("keys %v %v", keys, err)
	}
	ttl, err := c.Do(ctx, c.B().Pttl().Key(keys[0]).Build()).AsInt64()
	if err != nil || ttl <= 0 || ttl > (10*time.Minute).Milliseconds() {
		t.Fatalf("ttl %d %v", ttl, err)
	}
}

// T013 — F136-AC-11, AC-12, AC-14, AC-16, AC-30 (TC-GAM-013): every content refusal is the same 422
// NICKNAME_NOT_ALLOWED with a generic message in the language of Accept-Language and no echo; a facade failure is 503
// and nothing is stored; the refused text is in no log line, audit or outbox event (result code only).
func TestNicknameRejectNoLeak(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.nickPersonal.blocked[an.id] = []string{"Nguyen Thi Placeholder"}
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)
	refused := []string{"Minh zalo_minhxinh", "Siêu " + placeholderTerm, "Nguyen Thi Placeholder", "Minh 123456"}
	var details []string
	for i, text := range refused {
		lang := "vi-VN"
		if i%2 == 1 {
			lang = "en-US"
		}
		r := e.typeNickname(t, an, text, lang)
		if r.status != http.StatusUnprocessableEntity || r.code() != string(problem.NicknameNotAllowed) {
			t.Fatalf("%q: %d %s", text, r.status, r.raw)
		}
		if _, has := r.body["errors"]; has {
			t.Fatalf("%q: field errors leak the rule: %s", text, r.raw)
		}
		for _, w := range strings.Fields(text) {
			if len(w) >= 4 && strings.Contains(strings.ToLower(r.raw), strings.ToLower(w)) {
				t.Fatalf("%q: response echoes %q: %s", text, w, r.raw)
			}
		}
		details = append(details, lang+":"+r.body["detail"].(string))
	}
	if details[0] == details[1] || !strings.HasPrefix(details[0], "vi-VN:") || details[0] != "vi-VN:"+strings.TrimPrefix(details[2], "vi-VN:") {
		t.Fatalf("one generic message per language: %v", details)
	}
	e.nickMod.down = true
	if r := e.typeNickname(t, an, "Siêu Nhân Sao Băng"); r.status != http.StatusServiceUnavailable || r.code() != string(problem.Unavailable) {
		t.Fatalf("moderation down: %d %s", r.status, r.raw)
	}
	e.nickMod.down, e.nickPersonal.down = false, true
	if r := e.typeNickname(t, an, "Siêu Nhân Sao Băng"); r.status != http.StatusServiceUnavailable {
		t.Fatalf("identity down: %d %s", r.status, r.raw)
	}
	if p := e.profile(t, an); p.body["nickname"] != nil {
		t.Fatalf("stored while unavailable: %s", p.raw)
	}
	if n := e.audits(ctx, t, "game_profile.nickname_refused"); n != len(refused) {
		t.Fatalf("refusal audits %d, want %d", n, len(refused))
	}
	for _, text := range append(refused, "Siêu Nhân Sao Băng") {
		for _, w := range strings.Fields(text) {
			if len(w) < 4 || w == "Siêu" || w == "Minh" {
				continue
			}
			if strings.Contains(strings.ToLower(logs.String()), strings.ToLower(w)) {
				t.Fatalf("log leaks %q", w)
			}
			if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE lower(envelope::text) LIKE '%' || lower($1) || '%'`, w); n != 0 {
				t.Fatalf("outbox/audit leaks %q", w)
			}
		}
	}
}
