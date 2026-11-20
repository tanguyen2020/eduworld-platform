//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
)

// T011 — F095-AC-06 (AC-SOC-003, SC-003): the addressee rejects silently: no event, no audit, no notification; the
// requester keeps seeing the request pending in listFriendRequests?direction=outgoing until it expires; the addressee
// no longer sees it; only the addressee may reject (others → 404).
func TestRejectIsSilent(t *testing.T) {
	e := setup(t, envOpts{})
	chi, an, binh := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	id := requestID(e.send(chi, an, http.StatusCreated))
	before := len(e.events("audit.recorded")) + len(e.events("friend_request.sent"))
	for _, s := range []student{binh, chi} {
		if r := e.call(s, http.MethodPost, "/social/friend-requests/"+id+":reject", "", nil); r.status != http.StatusNotFound {
			t.Fatalf("reject by %s: %d %s", s.nickname, r.status, r.raw)
		}
	}
	if r := e.call(an, http.MethodPost, "/social/friend-requests/"+id+":reject", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("reject: %d %s", r.status, r.raw)
	}
	if after := len(e.events("audit.recorded")) + len(e.events("friend_request.sent")); after != before {
		t.Fatalf("reject wrote %d events", after-before)
	}
	if n := e.count(`SELECT count(*) FROM infra.outbox_events WHERE event_type NOT IN ('audit.recorded', 'friend_request.sent')`); n != 0 {
		t.Fatalf("reject published %d events", n)
	}
	out := e.call(chi, http.MethodGet, "/social/friend-requests?direction=outgoing", "", nil)
	if out.status != http.StatusOK || len(out.data()) != 1 || out.data()[0]["status"] != "pending" || out.data()[0]["request_id"] != id {
		t.Fatalf("outgoing after reject: %d %s", out.status, out.raw)
	}
	pii(t, out)
	if in := e.call(an, http.MethodGet, "/social/friend-requests?direction=incoming", "", nil); len(in.data()) != 0 {
		t.Fatalf("incoming after reject: %s", in.raw)
	}
	// Rejected twice → 404 (handled request).
	if r := e.call(an, http.MethodPost, "/social/friend-requests/"+id+":reject", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("reject again: %d", r.status)
	}
	// After friend_request_ttl the request disappears for the requester too (it expired, never "rejected").
	e.clock.Add(7*24*time.Hour + time.Minute)
	if out := e.call(chi, http.MethodGet, "/social/friend-requests?direction=outgoing", "", nil); len(out.data()) != 0 {
		t.Fatalf("outgoing after expiry: %s", out.raw)
	}
}

// T011 — F095-AC-07: the job expire-friend-requests (Clock of the test) marks expired every pending request past
// friend_request_ttl, in every tenant, without any event or notification; live requests and expired friend codes are
// handled once.
func TestExpireJob(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	dung, em := e.student(e.tenantB, e.schoolB), e.student(e.tenantB, e.schoolB)
	old := requestID(e.send(an, binh, http.StatusCreated))
	oldB := requestID(e.send(dung, em, http.StatusCreated))
	if r := e.call(an, http.MethodPost, "/social/friend-codes", newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("code: %d %s", r.status, r.raw)
	}
	e.clock.Add(6 * 24 * time.Hour)
	e.send(binh, dung, http.StatusNotFound) // other tenant: nothing created
	live := requestID(e.send(em, dung, http.StatusCreated))
	e.clock.Add(24*time.Hour + time.Second)
	events := e.count(`SELECT count(*) FROM infra.outbox_events`)
	job := command.Expiry{UoW: e.uow, Repo: persistence.Expiry{}, Now: e.clock.Now}
	n, err := job.Run(context.Background())
	if err != nil || n != 3 { // two requests of two tenants + the friend code
		t.Fatalf("job: %d %v", n, err)
	}
	for id, want := range map[string]string{old: "expired", oldB: "expired", live: "pending"} {
		if c := e.count(`SELECT count(*) FROM social.friend_requests WHERE request_id = $1 AND status = $2`, id, want); c != 1 {
			t.Fatalf("request %s not %s", id, want)
		}
	}
	if c := e.count(`SELECT count(*) FROM social.friend_codes WHERE deleted_at IS NOT NULL AND delete_reason = 'expired'`); c != 1 {
		t.Fatalf("expired codes soft-deleted: %d", c)
	}
	if after := e.count(`SELECT count(*) FROM infra.outbox_events`); after != events {
		t.Fatalf("the job published %d events", after-events)
	}
	if n, err := job.Run(context.Background()); err != nil || n != 0 {
		t.Fatalf("second run: %d %v", n, err)
	}
	if r := e.call(binh, http.MethodPost, "/social/friend-requests/"+old+":accept", newKey(), nil); r.status != http.StatusNotFound {
		t.Fatalf("accept expired: %d", r.status)
	}
	// After the expiry the pair may ask again (the unique pending index no longer holds the old row).
	e.send(an, binh, http.StatusCreated)
}
