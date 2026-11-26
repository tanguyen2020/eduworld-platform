//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/configschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	competitionhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Seeded competition configurations (migrations F104, F106, F107).
var (
	cfgMatchmaking = uuid.MustParse("01936a00-0000-7000-8000-000000000104")
	cfgRanking     = uuid.MustParse("01936a00-0000-7000-8000-000000000106")
	cfgBoards      = uuid.MustParse("01936a00-0000-7000-8000-000000000107")
	cfgSeasons     = uuid.MustParse("01936a00-0000-7000-8000-000000000108")
)

// cenv is the F108 environment: the party environment (migrated eduworld_competition with the seeds, pkg/httpx chain,
// fake OIDC, settable clock) with the competition game-admin operations mounted, and the apply use case.
type cenv struct {
	*penv
	cfg    command.GameConfigs
	apply  command.ConfigApply
	boards command.Leaderboards
}

func setupCfg(t *testing.T) *cenv {
	t.Helper()
	e := &cenv{}
	e.penv = setupParty(t, popts{mount: func(p *penv, s *httpx.Server) {
		e.cfg = command.GameConfigs{UoW: p.uow, Store: persistence.GameConfigStore{}, Schemas: configschema.Validator{}, Events: p.parties.Events,
			Audit: p.parties.Audit, Metrics: observability.GameConfig{}, Now: p.clock.Now}
		q := query.GameConfigs{Read: p.read, Reader: persistence.GameConfigStore{}}
		if err := competitionhttp.RegisterGameAdmin(s, competitionhttp.GameAdminHandlers{Commands: e.cfg, Queries: q}); err != nil {
			t.Fatal(err)
		}
		repo := persistence.RankingStore{}
		e.boards = command.Leaderboards{UoW: p.uow, Repo: repo, Ranking: repo, Seasons: repo, Store: cache.LeaderboardStore{},
			Events: p.parties.Events, Audit: p.parties.Audit, Metrics: observability.Ranking{}, Now: p.clock.Now}
		e.apply = command.ConfigApply{UoW: p.uow, Store: persistence.GameConfigStore{}, Boards: e.boards, Now: p.clock.Now}
	}})
	return e
}

// platform is a portal token of a platform principal with role.
func (e *cenv) platform(role string) string {
	sub := "kc-pl-" + uuid.NewString()
	e.res.Set(sub, authztest.Platform(role).PrincipalData)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// perms is a portal token of a platform principal holding exactly perms (one role per test actor).
func (e *cenv) perms(perms ...authz.Permission) string {
	sub := "kc-pp-" + uuid.NewString()
	d := authztest.Platform("competition_admin").PrincipalData
	d.Grants = []authz.Grant{{Role: "competition_admin", ScopeKind: authz.RoleScopePlatform, Permissions: perms}}
	e.res.Set(sub, d)
	return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
}

// admin calls a game-admin route (Idempotency-Key on writes, optional If-Match; a key reused when given).
func (e *cenv) admin(method, path, token, ifMatch, idem string, body any) presp {
	e.t.Helper()
	h := map[string]string{}
	if method != http.MethodGet {
		if idem == "" {
			idem = key()
		}
		h["Idempotency-Key"] = idem
	}
	if ifMatch != "" {
		h["If-Match"] = ifMatch
	}
	ct := ""
	if method == http.MethodPatch {
		ct = "application/merge-patch+json"
	}
	return e.do(method, "/game-admin"+path, token, h, body, ct)
}

func seedJSON(t *testing.T, file string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../seed/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func wantStatus(t *testing.T, what string, r presp, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: %d (want %d) %s", what, r.status, status, r.raw)
	}
}

func fieldErr(t *testing.T, r presp, field string) {
	t.Helper()
	if r.status != http.StatusUnprocessableEntity || r.body["code"] != "VALIDATION_FAILED" {
		t.Fatalf("want 422 VALIDATION_FAILED on %s: %d %s", field, r.status, r.raw)
	}
	errs, _ := r.body["errors"].([]any)
	for _, x := range errs {
		if m, _ := x.(map[string]any); m["field"] == field {
			return
		}
	}
	t.Fatalf("no error on %s: %s", field, r.raw)
}

// draftFrom creates a new draft version of cfg from its published version 1 with payload; returns the response.
func (e *cenv) draftFrom(typ string, cfg uuid.UUID, token string, payload map[string]any) presp {
	e.t.Helper()
	g := e.admin(http.MethodGet, "/"+typ+"/"+cfg.String()+"/versions/1", token, "", "", nil)
	wantStatus(e.t, "get v1", g, http.StatusOK)
	return e.admin(http.MethodPost, "/"+typ, token, "", "", map[string]any{"config_id": cfg, "based_on_version_id": g.body["config_version_id"],
		"title": "Bản nháp F108", "payload": payload})
}

// approve takes version no of cfg to approved (author submits, reviewer approves) and returns its ETag.
func (e *cenv) approve(typ string, cfg string, no int, author, reviewer string) string {
	e.t.Helper()
	base := "/" + typ + "/" + cfg + "/versions/" + strconv.Itoa(no)
	g := e.admin(http.MethodGet, base, author, "", "", nil)
	wantStatus(e.t, "get", g, http.StatusOK)
	s := e.admin(http.MethodPost, base+":transition", author, g.header.Get("ETag"), "", map[string]any{"action": "submit_review"})
	wantStatus(e.t, "submit", s, http.StatusOK)
	g = e.admin(http.MethodGet, base, reviewer, "", "", nil)
	a := e.admin(http.MethodPost, base+":transition", reviewer, g.header.Get("ETag"), "", map[string]any{"action": "approve"})
	wantStatus(e.t, "approve", a, http.StatusOK)
	return e.admin(http.MethodGet, base, reviewer, "", "", nil).header.Get("ETag")
}

// T011 — F108-AC-02, AC-03 (TC-CMS-AUTHZ-01, TC-CMS-AUTHZ-02, TC-CMS-WS-03): only portal staff with a platform role reach
// the routes; each type needs its own competition.config.* permission, review needs game.content.review, publish/rollback
// game.content.publish; every denial is 403 PERMISSION_DENIED with authz.denied; no tenant role holds competition.config.*.
func TestF108ConfigAuthz(t *testing.T) {
	e := setupCfg(t)
	before := len(e.audits("authz.denied"))
	tenantAdmin := func(role string) string {
		sub := "kc-ta-" + uuid.NewString()
		e.res.Set(sub, authztest.Staff(e.tenantA, role).PrincipalData)
		return e.idp.Token(sub, httpxtest.PortalClient, time.Now())
	}
	gameSub := "kc-pg-" + uuid.NewString()
	e.res.Set(gameSub, authztest.Platform("system_admin").PrincipalData)
	denied := map[string]string{
		"school admin":             tenantAdmin("school_admin"),
		"tenant admin":             tenantAdmin("tenant_admin"),
		"academic manager":         tenantAdmin("academic_manager"),
		"platform via game client": e.idp.Token(gameSub, httpxtest.GameClient, time.Now()),
		"content admin":            e.platform("content_admin"),
	}
	for name, tok := range denied {
		for _, typ := range []string{"matchmaking-configs", "ranking-rules", "competition-rewards"} {
			if r := e.admin(http.MethodGet, "/"+typ, tok, "", "", nil); r.status != http.StatusForbidden {
				t.Errorf("%s list %s: %d %s", name, typ, r.status, r.raw)
			}
		}
		if r := e.admin(http.MethodPost, "/ranking-rules", tok, "", "", map[string]any{"payload": seedJSON(t, "ranking_rules.json")}); r.status != http.StatusForbidden {
			t.Errorf("%s create: %d %s", name, r.status, r.raw)
		}
	}
	// No tenant role holds competition.config.* (R20, catalog checked at startup too).
	for _, r := range authz.SystemRoles() {
		if r.ScopeKind != authz.RoleScopeTenant {
			continue
		}
		for _, p := range r.Permissions {
			if len(p) >= 19 && string(p)[:19] == "competition.config." {
				t.Fatalf("tenant role %s holds %s", r.Name, p)
			}
		}
	}
	// A season writer may not write ranking rules, nor publish.
	season := e.perms(authz.PermCompetitionConfigSeasonWrite)
	r := e.draftFrom("ranking-rules", cfgRanking, e.platform("system_admin"), seedJSON(t, "ranking_rules.json"))
	wantStatus(t, "sa draft", r, http.StatusCreated)
	if r := e.admin(http.MethodPatch, "/ranking-rules/"+cfgRanking.String()+"/versions/2", season, r.header.Get("ETag"), "",
		map[string]any{"payload": seedJSON(t, "ranking_rules.json")}); r.status != http.StatusForbidden || r.body["code"] != "PERMISSION_DENIED" {
		t.Fatalf("season writer edits ranking rules: %d %s", r.status, r.raw)
	}
	writer := e.perms(authz.PermCompetitionConfigRankingWrite)
	reviewer := e.perms(authz.PermGameContentReview)
	etag := e.approve("ranking-rules", cfgRanking.String(), 2, e.platform("system_admin"), reviewer)
	if r := e.admin(http.MethodPost, "/ranking-rules/"+cfgRanking.String()+"/versions/2:publish", writer, etag, "", map[string]any{}); r.status != http.StatusForbidden {
		t.Fatalf("writer publishes: %d %s", r.status, r.raw)
	}
	if r := e.admin(http.MethodPost, "/ranking-rules/"+cfgRanking.String()+":rollback", reviewer, "", "", map[string]any{"to_version": 1, "reason": "x"}); r.status != http.StatusForbidden {
		t.Fatalf("reviewer rolls back: %d %s", r.status, r.raw)
	}
	// The writer of rewards uses competition.config.reward.write (RB-415).
	rewards := e.perms(authz.PermCompetitionConfigRewardWrite)
	rw := e.admin(http.MethodPost, "/competition-rewards", rewards, "", "", map[string]any{"payload": map[string]any{"schema_version": 1, "code": "season_1",
		"applies_to": "season", "name": "Mùa 1", "rules": []any{map[string]any{"rule_id": "t6", "tier_code": "T6", "reward_ref": map[string]any{"cosmetic_code": "cosmetic.crown"}}}}})
	wantStatus(t, "rewards create", rw, http.StatusCreated)
	if after := len(e.audits("authz.denied")); after <= before {
		t.Fatalf("authz.denied not recorded (%d → %d)", before, after)
	}
	// A type of another module is not served by competition (404).
	if r := e.admin(http.MethodGet, "/quests", e.platform("system_admin"), "", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("quests on competition: %d", r.status)
	}
	authztest.Cover(t, "listGameConfigs", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "createGameConfig", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "updateGameConfigDraft", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "transitionGameConfigVersion", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "publishGameConfigVersion", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "rollbackGameConfig", authztest.RoleDenied, authztest.CrossTenant)
}

// T010 — F108-AC-01, AC-04, AC-05, AC-06, AC-07 (TC-CMS-001…004): the eleven operations on the seven types; invariants
// with field errors; published versions immutable, drafts from based_on_version_id; If-Match 428/412; effective_from never
// in the past; idempotent publish (one release, one event); rollback with a reason to a version published before;
// unpublish only of templates and rewards; every write audited; the event matches its schema.
func TestF108ConfigWorkflow(t *testing.T) {
	e := setupCfg(t)
	sa := e.platform("system_admin")
	author := e.perms(authz.PermCompetitionConfigRankingWrite, authz.PermCompetitionConfigTournamentTemplateWrite)
	reviewer := e.perms(authz.PermGameContentReview, authz.PermGameContentPublish)
	// AC-01: every type is listed (seeds of F104/F106/F107).
	for typ, want := range map[string]int{"matchmaking-configs": 1, "ranking-rules": 1, "seasons": 1, "tournament-templates": 2,
		"challenge-templates": 3, "leaderboard-definitions": 1, "competition-rewards": 0} {
		r := e.admin(http.MethodGet, "/"+typ, sa, "", "", nil)
		wantStatus(t, "list "+typ, r, http.StatusOK)
		if d, _ := r.body["data"].([]any); len(d) != want {
			t.Fatalf("list %s: %d rows %s", typ, len(d), r.raw)
		}
	}
	// keyset pages paged in SQL
	p1 := e.admin(http.MethodGet, "/challenge-templates?limit=2", sa, "", "", nil)
	if d, _ := p1.body["data"].([]any); len(d) != 2 || p1.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("page 1: %s", p1.raw)
	}
	cur := p1.body["page"].(map[string]any)["next_cursor"].(string)
	if p2 := e.admin(http.MethodGet, "/challenge-templates?limit=2&cursor="+cur, sa, "", "", nil); len(p2.body["data"].([]any)) != 1 {
		t.Fatalf("page 2: %s", p2.raw)
	}
	rr := "/ranking-rules/" + cfgRanking.String()
	// AC-05: a published version is never edited.
	v1 := e.admin(http.MethodGet, rr+"/versions/1", sa, "", "", nil)
	if r := e.admin(http.MethodPatch, rr+"/versions/1", author, v1.header.Get("ETag"), "", map[string]any{"payload": seedJSON(t, "ranking_rules.json")}); r.status != http.StatusConflict || r.body["code"] != "CONTENT_NOT_EDITABLE" {
		t.Fatalf("edit published: %d %s", r.status, r.raw)
	}
	// AC-04: decreasing tiers are refused with the field.
	bad := seedJSON(t, "ranking_rules.json")
	bad["tiers"].(map[string]any)["levels"].([]any)[2].(map[string]any)["min_rating"] = 1200
	fieldErr(t, e.draftFrom("ranking-rules", cfgRanking, author, bad), "payload/tiers/levels/2/min_rating")
	// a draft from the published version (config_id + based_on_version_id)
	d := e.draftFrom("ranking-rules", cfgRanking, author, seedJSON(t, "ranking_rules.json"))
	wantStatus(t, "draft v2", d, http.StatusCreated)
	if d.body["version"] != float64(2) || d.body["based_on_version"] != float64(1) || d.body["status"] != "draft" {
		t.Fatalf("draft v2: %s", d.raw)
	}
	// AC-06: If-Match missing → 428; stale → 412.
	soft := seedJSON(t, "ranking_rules.json")
	soft["tiers"].(map[string]any)["hysteresis"] = 60
	if r := e.admin(http.MethodPatch, rr+"/versions/2", author, "", "", map[string]any{"payload": soft}); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no If-Match: %d %s", r.status, r.raw)
	}
	u := e.admin(http.MethodPatch, rr+"/versions/2", author, d.header.Get("ETag"), "", map[string]any{"payload": soft})
	wantStatus(t, "patch", u, http.StatusOK)
	if r := e.admin(http.MethodPatch, rr+"/versions/2", author, d.header.Get("ETag"), "", map[string]any{"payload": soft}); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	// reviewer ≠ author
	s := e.admin(http.MethodPost, rr+"/versions/2:transition", author, u.header.Get("ETag"), "", map[string]any{"action": "submit_review"})
	wantStatus(t, "submit", s, http.StatusOK)
	self := e.perms(authz.PermCompetitionConfigRankingWrite, authz.PermGameContentReview)
	g := e.admin(http.MethodGet, rr+"/versions/2", self, "", "", nil)
	if r := e.admin(http.MethodPost, rr+"/versions/2:transition", self, g.header.Get("ETag"), "", map[string]any{"action": "reject"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("reject without note: %d %s", r.status, r.raw)
	}
	g = e.admin(http.MethodGet, rr+"/versions/2", reviewer, "", "", nil)
	wantStatus(t, "approve", e.admin(http.MethodPost, rr+"/versions/2:transition", reviewer, g.header.Get("ETag"), "", map[string]any{"action": "approve"}), http.StatusOK)
	g = e.admin(http.MethodGet, rr+"/versions/2", reviewer, "", "", nil)
	// AC-05: effective_from in the past, publish_at → 422.
	past := e.clock.Now().Add(-time.Hour).Format(time.RFC3339)
	fieldErr(t, e.admin(http.MethodPost, rr+"/versions/2:publish", reviewer, g.header.Get("ETag"), "", map[string]any{"effective_from": past}), "effective_from")
	fieldErr(t, e.admin(http.MethodPost, rr+"/versions/2:publish", reviewer, g.header.Get("ETag"), "", map[string]any{"publish_at": e.clock.Now().Add(time.Hour).Format(time.RFC3339)}), "publish_at")
	// AC-07: the same Idempotency-Key twice → one release, one event.
	idem := key()
	monday := e.clock.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	p := e.admin(http.MethodPost, rr+"/versions/2:publish", reviewer, g.header.Get("ETag"), idem, map[string]any{"effective_from": monday, "change_note": "hysteresis 60"})
	wantStatus(t, "publish", p, http.StatusOK)
	again := e.admin(http.MethodPost, rr+"/versions/2:publish", reviewer, g.header.Get("ETag"), idem, map[string]any{"effective_from": monday, "change_note": "hysteresis 60"})
	wantStatus(t, "publish replay", again, http.StatusOK)
	if again.header.Get("Idempotent-Replayed") != "true" || again.body["release_id"] != p.body["release_id"] {
		t.Fatalf("replay: %s %s", again.header, again.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.content_releases WHERE config_id = $1 AND action = 'publish' AND effective_from IS NOT NULL`, cfgRanking); n != 1 {
		t.Fatalf("releases: %d", n)
	}
	if ev := e.events(command.EventConfigPublished); len(ev) != 1 || ev[0]["action"] != "publish" || ev[0]["version"] != float64(2) {
		t.Fatalf("events: %v", ev)
	}
	// the pointer moved at once; the runtime still reads v1 until effective_from (AC-08 timeline)
	if r := e.admin(http.MethodGet, rr+"/versions/2", sa, "", "", nil); r.body["is_current"] != true || r.body["status"] != "published" {
		t.Fatalf("pointer: %s", r.raw)
	}
	// diff, versions, releases
	if r := e.admin(http.MethodGet, rr+"/versions/2:diff?with=1", sa, "", "", nil); len(r.body["changes"].([]any)) != 1 {
		t.Fatalf("diff: %s", r.raw)
	}
	if r := e.admin(http.MethodGet, rr+"/versions", sa, "", "", nil); r.body["current_version"] != float64(2) || len(r.body["data"].([]any)) != 2 {
		t.Fatalf("versions: %s", r.raw)
	}
	// AC-05: rollback needs a reason and a version published before.
	fieldErr(t, e.admin(http.MethodPost, rr+":rollback", reviewer, "", "", map[string]any{"to_version": 1, "reason": " "}), "reason")
	dv := e.draftFrom("ranking-rules", cfgRanking, author, seedJSON(t, "ranking_rules.json"))
	wantStatus(t, "draft v3", dv, http.StatusCreated)
	if r := e.admin(http.MethodPost, rr+":rollback", reviewer, "", "", map[string]any{"to_version": 3, "reason": "thử"}); r.body["code"] != "ROLLBACK_TARGET_INVALID" {
		t.Fatalf("rollback to a draft: %d %s", r.status, r.raw)
	}
	rb := e.admin(http.MethodPost, rr+":rollback", reviewer, "", "", map[string]any{"to_version": 1, "reason": "Quay lại ngưỡng cũ"})
	wantStatus(t, "rollback", rb, http.StatusOK)
	if rb.body["from_version"] != float64(2) || rb.body["to_version"] != float64(1) {
		t.Fatalf("rollback view: %s", rb.raw)
	}
	rel := e.admin(http.MethodGet, rr+"/releases", sa, "", "", nil)
	if l := rel.body["data"].([]any); len(l) != 3 || l[0].(map[string]any)["action"] != "rollback" || l[0].(map[string]any)["reason"] != "Quay lại ngưỡng cũ" {
		t.Fatalf("releases: %s", rel.raw)
	}
	// unpublish: the runtime catalogs cannot be withdrawn, templates can.
	if r := e.admin(http.MethodPost, rr+":unpublish", reviewer, "", "", map[string]any{"reason": "x"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("unpublish ranking: %d %s", r.status, r.raw)
	}
	if r := e.admin(http.MethodPost, "/challenge-templates/"+tplQuickQuiz.String()+":unpublish", reviewer, "", "", map[string]any{"reason": "Tạm ngừng"}); r.status != http.StatusOK {
		t.Fatalf("unpublish template: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM competition.game_configs WHERE config_id = $1 AND current_version_id IS NULL`, tplQuickQuiz); n != 1 {
		t.Fatal("template still current")
	}
	// AC-04 on templates: class kind, forbidden params, paid reward, class seeding.
	tpl := map[string]any{"schema_version": 1, "code": "grade_bad", "kind": "grade_challenge", "format": "tournament", "name": "X", "description": "",
		"grade_levels": []any{4}, "mode": "battle_1v1",
		"tournament": map[string]any{"format": "single_elim", "participant_type": "individual", "team_size": nil, "max_entries": 8, "min_entries": 4,
			"seeding_policy": "random", "auto_confirm": false, "round_gap_minutes": 5, "bracket_reset": false},
		"schedule": map[string]any{"min_lead_minutes": 30, "max_lead_days": 60, "max_duration_minutes": 480}, "allowed_params": []any{"subject_id"},
		"reward_rule_refs": []any{map[string]any{"rule_id": "c", "rank_min": 1, "rank_max": 1, "points": 10, "reward_ref": map[string]any{"badge_code": "badge.c"}}},
		"reward_summary":   ""}
	ok := e.admin(http.MethodPost, "/challenge-templates", author, "", "", map[string]any{"payload": tpl})
	wantStatus(t, "template create", ok, http.StatusCreated)
	tpl["kind"] = "class_challenge"
	tpl["code"] = "grade_class"
	if r := e.admin(http.MethodPost, "/challenge-templates", author, "", "", map[string]any{"payload": tpl}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("class kind: %d %s", r.status, r.raw)
	}
	tpl["kind"] = "grade_challenge"
	tpl["code"] = "grade_bad2"
	tpl["reward_rule_refs"] = []any{map[string]any{"rule_id": "c", "points": 10, "rank_min": 1, "reward_ref": map[string]any{"coins": "100"}}}
	if r := e.admin(http.MethodPost, "/challenge-templates", author, "", "", map[string]any{"payload": tpl}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("paid reward: %d %s", r.status, r.raw)
	}
	// AC-07: every write audited with actor, type, config, version.
	for _, a := range []string{command.AuditConfigCreated, command.AuditConfigUpdated, command.AuditConfigTransitioned, command.AuditConfigPublished,
		command.AuditConfigRolledBack, command.AuditConfigUnpublished} {
		if len(e.audits(a)) == 0 {
			t.Errorf("no audit %s", a)
		}
	}
	if n := e.checkSchemasOf([]string{command.EventConfigPublished}); n[command.EventConfigPublished] != 3 {
		t.Fatalf("events checked: %v", n)
	}
	authztest.Cover(t, "getGameConfigVersion", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listGameConfigVersions", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "diffGameConfigVersions", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "unpublishGameConfig", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "listGameConfigReleases", authztest.RoleDenied, authztest.CrossTenant)
}

// T012 — F108-AC-08: the runtime applies a release at its effective_from (fake clock): the matchmaking configuration and
// Ranking Rules in effect follow the release timeline (a rollback at once), the seasons and leaderboard definitions are
// copied into the runtime tables by the consumer/job; matches already started keep their pinned version.
func TestF108ConfigApply(t *testing.T) {
	e := setupCfg(t)
	ctx := context.Background()
	sa := e.platform("system_admin")
	rev := e.perms(authz.PermGameContentReview, authz.PermGameContentPublish)
	now := e.clock.Now()
	// Matchmaking: a new version effective in 2 minutes.
	mm := seedJSON(t, "matchmaking_config.json")
	mm["ready_check_s"] = 20
	d := e.draftFrom("matchmaking-configs", cfgMatchmaking, sa, mm)
	wantStatus(t, "mm draft", d, http.StatusCreated)
	no := int(d.body["version"].(float64))
	etag := e.approve("matchmaking-configs", cfgMatchmaking.String(), no, sa, rev)
	at := now.Add(2 * time.Minute)
	wantStatus(t, "mm publish", e.admin(http.MethodPost, "/matchmaking-configs/"+cfgMatchmaking.String()+"/versions/"+strconv.Itoa(no)+":publish", rev, etag, "",
		map[string]any{"effective_from": at.UTC().Format(time.RFC3339)}), http.StatusOK)
	active := func(when time.Time) int {
		var c int
		if err := e.uow.InTenantTx(ctx, mustPlatform(t), func(ctx context.Context) error {
			cfg, err := persistence.Matchmaking{}.ActiveConfig(ctx, when)
			c = cfg.ReadyCheckS
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := active(now); c != 15 {
		t.Fatalf("before effective_from: ready_check %d", c)
	}
	if c := active(at.Add(time.Second)); c != 20 {
		t.Fatalf("after effective_from: ready_check %d", c)
	}
	// a rollback takes effect at once
	e.clock.Add(3 * time.Minute)
	wantStatus(t, "mm rollback", e.admin(http.MethodPost, "/matchmaking-configs/"+cfgMatchmaking.String()+":rollback", rev, "", "",
		map[string]any{"to_version": 2, "reason": "Ready check quá dài"}), http.StatusOK)
	if c := active(e.clock.Now()); c != 15 {
		t.Fatalf("after rollback: ready_check %d", c)
	}
	// Seasons: shorten the pre-season and add a season, effective in 2 minutes.
	start := time.Date(2026, 12, 7, 0, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	seasons := map[string]any{"schema_version": 1, "seasons": []any{
		map[string]any{"season_id": "01936a00-0000-7000-8000-000000106001", "code": "pre-season", "name": "Mùa khởi động", "theme_code": nil,
			"starts_at": "2026-01-01T00:00:00+07:00", "ends_at": start.Format(time.RFC3339)},
		map[string]any{"season_id": uuid.NewString(), "code": "2026-s1", "name": "Mùa Phép Nhân", "theme_code": "multiplication",
			"starts_at": start.Format(time.RFC3339), "ends_at": start.AddDate(0, 0, 56).Format(time.RFC3339)},
	}}
	// overlapping the active season is refused (edge case)
	overlap := map[string]any{"schema_version": 1, "seasons": []any{seasons["seasons"].([]any)[1],
		map[string]any{"season_id": "01936a00-0000-7000-8000-000000106001", "code": "pre-season", "name": "Mùa khởi động", "theme_code": nil,
			"starts_at": "2026-01-01T00:00:00+07:00", "ends_at": "2099-12-31T00:00:00+07:00"}}}
	fieldErr(t, e.draftFrom("seasons", cfgSeasons, sa, overlap), "payload/seasons/1/starts_at")
	sd := e.draftFrom("seasons", cfgSeasons, sa, seasons)
	wantStatus(t, "season draft", sd, http.StatusCreated)
	sno := int(sd.body["version"].(float64))
	etag = e.approve("seasons", cfgSeasons.String(), sno, sa, rev)
	eff := e.clock.Now().Add(2 * time.Minute)
	wantStatus(t, "season publish", e.admin(http.MethodPost, "/seasons/"+cfgSeasons.String()+"/versions/"+strconv.Itoa(sno)+":publish", rev, etag, "",
		map[string]any{"effective_from": eff.UTC().Format(time.RFC3339)}), http.StatusOK)
	// Leaderboards: retire a board, effective in 2 minutes.
	boards := seedJSON(t, "leaderboard_definitions.json")
	boards["definitions"].([]any)[3].(map[string]any)["status"] = "retired"
	bd := e.draftFrom("leaderboard-definitions", cfgBoards, sa, boards)
	wantStatus(t, "boards draft", bd, http.StatusCreated)
	etag = e.approve("leaderboard-definitions", cfgBoards.String(), 2, sa, rev)
	wantStatus(t, "boards publish", e.admin(http.MethodPost, "/leaderboard-definitions/"+cfgBoards.String()+"/versions/2:publish", rev, etag, "",
		map[string]any{"effective_from": eff.UTC().Format(time.RFC3339)}), http.StatusOK)
	// the consumer runs at publish time: nothing in effect yet
	if _, err := e.apply.Apply(jobCtx(), "seasons"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.apply.Apply(jobCtx(), "leaderboard-definitions"); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM competition.seasons WHERE code = '2026-s1'`); n != 0 {
		t.Fatal("season applied before effective_from")
	}
	if n := e.count(`SELECT count(*) FROM competition.leaderboard_definitions WHERE code = 'friends_points_weekly' AND status = 'retired'`); n != 0 {
		t.Fatal("board retired before effective_from")
	}
	// the job after effective_from applies both, once
	e.clock.Add(3 * time.Minute)
	if n, err := e.apply.ApplyDue(jobCtx()); err != nil || n < 3 {
		t.Fatalf("apply due: %d %v", n, err)
	}
	if n := e.count(`SELECT count(*) FROM competition.seasons WHERE code = '2026-s1' AND status = 'scheduled'`); n != 1 {
		t.Fatal("new season not applied")
	}
	if n := e.count(`SELECT count(*) FROM competition.seasons WHERE code = 'pre-season' AND status = 'active' AND ends_at = $1`, start); n != 1 {
		t.Fatal("pre-season end not applied")
	}
	if n := e.count(`SELECT count(*) FROM competition.leaderboard_definitions WHERE code = 'friends_points_weekly' AND status = 'retired'`); n != 1 {
		t.Fatal("board not retired")
	}
	if n, err := e.apply.ApplyDue(jobCtx()); err != nil || n != 0 {
		t.Fatalf("apply twice: %d %v", n, err)
	}
	// the consumer accepts the event of the module (owner example of the schema)
	if n := e.checkSchemasOf([]string{command.EventConfigPublished}); n[command.EventConfigPublished] != 4 {
		t.Fatalf("events: %v", n)
	}
}

func mustPlatform(t *testing.T) tenancy.PlatformScope {
	t.Helper()
	s, err := authz.SystemPlatformScope("f108-test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
