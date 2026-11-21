//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"
)

// T023 — F100-AC-10 (TC-PTY-013, AC-PTY-011): the 11th invitation within a minute → 429 PARTY_INVITE_RATE_LIMITED with
// Retry-After and an audit; a second pending invitation to the same student → 409; after a rejection the inviter waits
// party_invite_repeat_rules (5 min), and 3 rejections in a day stop the invitations to that student.
func TestInviteRateLimits(t *testing.T) {
	e := setupParty(t, popts{})
	a := e.kidA()
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Quest", "party_type": "quest", "max_members": 8}).str("party_id")
	for i := 0; i < 10; i++ {
		e.invite(a, id, e.kidA(), http.StatusCreated)
	}
	r := e.invite(a, id, e.kidA(), http.StatusTooManyRequests)
	if r.code() != "PARTY_INVITE_RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("11th: %s %v", r.raw, r.header)
	}
	if a := e.audits("party.invite_rate_limited"); len(a) != 1 || a[0]["reason"] != "party_invite_rate_minute" {
		t.Fatalf("audit: %v", a)
	}
	e.clock.Add(time.Minute)
	b := e.kidA()
	inv := e.invite(a, id, b, http.StatusCreated).str("invitation_id")
	if r := e.invite(a, id, b, http.StatusConflict); r.code() != "CONFLICT" {
		t.Fatalf("duplicate pending: %s", r.raw)
	}
	// Repeat rules: rejected → wait 5 minutes; 3 rejections a day → no more invitations to b today.
	for round := 1; round <= 3; round++ {
		if r := e.call(b, http.MethodPost, "/multiplayer/invitations/"+inv+":reject", "", nil); r.status != http.StatusNoContent {
			t.Fatalf("reject %d: %d %s", round, r.status, r.raw)
		}
		if r := e.invite(a, id, b, http.StatusTooManyRequests); r.code() != "PARTY_INVITE_RATE_LIMITED" {
			t.Fatalf("after rejection %d: %s", round, r.raw)
		}
		e.clock.Add(5 * time.Minute)
		if round < 3 {
			inv = e.invite(a, id, b, http.StatusCreated).str("invitation_id")
		}
	}
	if r := e.invite(a, id, b, http.StatusTooManyRequests); r.code() != "PARTY_INVITE_RATE_LIMITED" {
		t.Fatalf("3 rejections: %s", r.raw)
	}
	e.clock.Add(24 * time.Hour)
	e.invite(a, id, b, http.StatusCreated)
}

// T023 — party_invite_rate per day (50/day, scaled down): the day window refuses even when the minute window is free.
func TestInviteRatePerDay(t *testing.T) {
	cfg := partyConfig
	cfg.InvitePerMinute, cfg.InvitePerDay = 2, 3
	e := setupParty(t, popts{config: &cfg})
	a := e.kidA()
	id := e.create(a, http.StatusCreated, map[string]any{"name": "Quest", "party_type": "quest", "max_members": 8}).str("party_id")
	e.invite(a, id, e.kidA(), http.StatusCreated)
	e.invite(a, id, e.kidA(), http.StatusCreated)
	e.invite(a, id, e.kidA(), http.StatusTooManyRequests)
	e.clock.Add(time.Minute)
	e.invite(a, id, e.kidA(), http.StatusCreated)
	e.clock.Add(time.Minute)
	if r := e.invite(a, id, e.kidA(), http.StatusTooManyRequests); r.code() != "PARTY_INVITE_RATE_LIMITED" {
		t.Fatalf("per day: %s", r.raw)
	}
	reasons := map[any]bool{}
	for _, p := range e.audits("party.invite_rate_limited") {
		reasons[p["reason"]] = true
	}
	if !reasons["party_invite_rate_minute"] || !reasons["party_invite_rate_day"] {
		t.Fatalf("reasons: %v", reasons)
	}
}
