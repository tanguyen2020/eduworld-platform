//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// T013 — F100-AC-01, AC-03 (TC-PTY-001): createParty makes the caller the leader of an open party whose tenant, school
// and grade band come from the projected profile; defaults private / leader_only / party_max_members; outbox
// party.created + party history + audit in the same transaction; the same Idempotency-Key replays the same party.
func TestCreateParty(t *testing.T) {
	e := setupParty(t, popts{})
	an := e.kidA()
	body := map[string]any{"name": "  Nhóm Sao Băng  ", "party_type": "friend", "tenant_id": e.tenantB.String()}
	k := key()
	r := e.call(an, http.MethodPost, "/multiplayer/parties", k, body)
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	if r.str("leader_id") != an.id.String() || r.str("status") != "open" || r.str("visibility") != "private" ||
		r.str("invitation_policy") != "leader_only" || r.body["max_members"].(float64) != 4 || r.str("name") != "Nhóm Sao Băng" {
		t.Fatalf("party: %s", r.raw)
	}
	if ms := r.members(); len(ms) != 1 || ms[0]["role"] != "leader" || ms[0]["nickname"] != an.nickname {
		t.Fatalf("members: %s", r.raw)
	}
	noPII(t, r)
	id := r.str("party_id")
	if n := e.count(`SELECT count(*) FROM multiplayer.parties WHERE party_id = $1 AND tenant_id = $2 AND school_id = $3 AND grade_band = 'g6_7'`,
		id, e.tenantA, e.schoolA); n != 1 {
		t.Fatalf("party row from the session profile: %d", n)
	}
	// Same key → same party, nothing written twice (F100-AC-03).
	again := e.call(an, http.MethodPost, "/multiplayer/parties", k, body)
	if again.status != http.StatusCreated || again.str("party_id") != id || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s %v", again.status, again.raw, again.header)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties`); n != 1 {
		t.Fatalf("parties: %d", n)
	}
	if ev := e.events("party.created"); len(ev) != 1 || ev[0]["leader_id"] != an.id.String() || ev[0]["grade_band"] != "g6_7" {
		t.Fatalf("party.created: %v", ev)
	}
	if h := e.history(id); len(h) != 1 || h[0] != "created" {
		t.Fatalf("history: %v", h)
	}
	if a := e.audits("party.created"); len(a) != 1 {
		t.Fatalf("audit: %v", a)
	}
	// Missing Idempotency-Key → 428 (F100-AC-03; platform code IDEMPOTENCY_KEY_REQUIRED of pkg/httpx).
	if r := e.call(an, http.MethodPost, "/multiplayer/parties", "", body); r.status != http.StatusPreconditionRequired ||
		r.code() != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	if seen := e.checkEventSchemas(); seen["party.created"] != 1 {
		t.Fatalf("schemas: %v", seen)
	}
}

// T013 — F100-AC-04 (TC-TEN-024): name empty or > 40, max_members outside 2–8 or above the policy cap, a party type
// not allowed for the grade band → 422 naming the field; nothing written.
func TestCreatePartyValidation(t *testing.T) {
	e := setupParty(t, popts{})
	an := e.kidA()
	small := e.kid(e.tenantA, e.schoolA, 3) // grade 3: friend/quest only
	cases := []struct {
		who   kid
		body  map[string]any
		field string
	}{
		{an, map[string]any{"name": strings.Repeat("a", 41), "party_type": "friend"}, "name"},
		{an, map[string]any{"name": "   ", "party_type": "friend"}, "name"},
		{an, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 9}, "max_members"},
		{an, map[string]any{"name": "Nhóm", "party_type": "friend", "max_members": 1}, "max_members"},
		{small, map[string]any{"name": "Nhóm", "party_type": "study_party"}, "party_type"},
		{an, map[string]any{"name": "Nhóm", "party_type": "study_group"}, "party_type"}, // renamed study_party (ADR-0095 D7)
		{an, map[string]any{"name": "Nhóm", "party_type": "friend", "visibility": "class"}, "visibility"},
		{small, map[string]any{"name": "Nhóm", "party_type": "custom"}, "party_type"},
	}
	for _, c := range cases {
		r := e.call(c.who, http.MethodPost, "/multiplayer/parties", key(), c.body)
		if r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" || !strings.Contains(r.raw, `"`+c.field+`"`) {
			t.Fatalf("%v: %d %s", c.body, r.status, r.raw)
		}
	}
	// The cap of a school policy: max_members_cap 3 → 4 refused.
	e.exec(`INSERT INTO multiplayer.party_policies (policy_id, tenant_id, created_at, updated_at, scope_type, scope_id, school_id, rules)
		VALUES (gen_random_uuid(), $1, now(), now(), 'school', $2, $2, '{"max_members_cap":3}')`, e.tenantA, e.schoolA)
	if r := e.call(an, http.MethodPost, "/multiplayer/parties", key(), map[string]any{"name": "Nhóm", "party_type": "friend",
		"max_members": 4}); r.status != http.StatusUnprocessableEntity || !strings.Contains(r.raw, "max_members") {
		t.Fatalf("cap: %d %s", r.status, r.raw)
	}
	// Default max_members follows the cap (min(party_max_members, cap)).
	if r := e.create(an, http.StatusCreated, nil); r.body["max_members"].(float64) != 3 {
		t.Fatalf("default under cap: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties`); n != 1 {
		t.Fatalf("parties written by invalid requests: %d", n)
	}
	// A grade 3 student creates a quest party (allowed for g1_3).
	if r := e.create(small, http.StatusCreated, map[string]any{"name": "Nhóm nhỏ", "party_type": "quest"}); r.str("party_type") != "quest" {
		t.Fatalf("quest: %s", r.raw)
	}
}

// Edge case: the profile of the caller is not projected yet → 503 retryable with Retry-After, nothing written.
func TestCreatePartyUnprojected(t *testing.T) {
	e := setupParty(t, popts{})
	ghost := e.ghost(e.tenantA)
	r := e.call(ghost, http.MethodPost, "/multiplayer/parties", key(), map[string]any{"name": "Nhóm", "party_type": "friend"})
	if r.status != http.StatusServiceUnavailable || r.header.Get("Retry-After") == "" {
		t.Fatalf("unprojected: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM multiplayer.parties`); n != 0 {
		t.Fatalf("written: %d", n)
	}
}

// party_create_rate (10 creations/disbands per hour): the 11th → 429 RATE_LIMITED with Retry-After and audit.
func TestCreatePartyRate(t *testing.T) {
	cfg := partyConfig
	cfg.CreatePerHour = 2
	e := setupParty(t, popts{config: &cfg})
	an := e.kidA()
	id := e.create(an, http.StatusCreated, nil).str("party_id") // 1 created
	if r := e.call(an, http.MethodPost, "/multiplayer/parties/"+id+":disband", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("disband: %d %s", r.status, r.raw) // 2 disbanded
	}
	r := e.call(an, http.MethodPost, "/multiplayer/parties", key(), map[string]any{"name": "Nhóm", "party_type": "friend"})
	if r.status != http.StatusTooManyRequests || r.code() != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("rate: %d %s", r.status, r.raw)
	}
	if a := e.audits("party.rate_limited"); len(a) != 1 {
		t.Fatalf("audit: %v", a)
	}
	e.clock.Add(61 * time.Minute)
	e.create(an, http.StatusCreated, nil)
}
