//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// T020 — F095-AC-17 (TC-SOC-003): mute and unmute are idempotent, publish user.muted / user.unmuted once per change
// with the audit, and only affect the muter (the muted student can still befriend and sees nothing).
func TestMuteUser(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	for i := 0; i < 2; i++ {
		if r := e.call(an, http.MethodPut, "/social/mutes/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
			t.Fatalf("mute %d: %d %s", i, r.status, r.raw)
		}
	}
	if m := e.events("user.muted"); len(m) != 1 || m[0]["muter_id"] != an.id.String() || m[0]["muted_id"] != binh.id.String() {
		t.Fatalf("user.muted %v", m)
	}
	l := e.call(an, http.MethodGet, "/social/mutes", "", nil)
	if len(l.data()) != 1 || l.data()[0]["user_id"] != binh.id.String() || l.data()[0]["nickname"] != binh.nickname ||
		l.data()[0]["created_at"] == nil {
		t.Fatalf("listMutes: %s", l.raw)
	}
	pii(t, l)
	if r := e.call(binh, http.MethodGet, "/social/mutes", "", nil); len(r.data()) != 0 {
		t.Fatalf("the muted student sees the mute: %s", r.raw)
	}
	e.befriend(binh, an) // a mute does not block friendship
	for i := 0; i < 2; i++ {
		if r := e.call(an, http.MethodDelete, "/social/mutes/"+binh.id.String(), "", nil); r.status != http.StatusNoContent {
			t.Fatalf("unmute %d: %d %s", i, r.status, r.raw)
		}
	}
	if u := e.events("user.unmuted"); len(u) != 1 {
		t.Fatalf("user.unmuted %v", u)
	}
	if l := e.call(an, http.MethodGet, "/social/mutes", "", nil); len(l.data()) != 0 {
		t.Fatalf("listMutes after unmute: %s", l.raw)
	}
	if n := e.checkEventSchemas(); n["user.muted"] != 1 || n["user.unmuted"] != 1 {
		t.Fatalf("events checked %v", n)
	}
}

// T020 — F095-AC-18 (NFR-SEC-006): block/mute oneself → 422; a target that is not a student of the session tenant →
// 404. F095-AC-19: listBlocks/listMutes only return the relations the caller created; no API tells who blocked the
// caller.
func TestRelationTargetsAndLists(t *testing.T) {
	e := setup(t, envOpts{})
	an, binh, chi := e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA), e.student(e.tenantA, e.schoolA)
	dung := e.student(e.tenantB, e.schoolB)
	for _, p := range []string{"/social/blocks/", "/social/mutes/"} {
		for _, m := range []string{http.MethodPut, http.MethodDelete} {
			if r := e.call(an, m, p+an.id.String(), "", nil); r.status != http.StatusUnprocessableEntity {
				t.Fatalf("%s %s self: %d %s", m, p, r.status, r.raw)
			}
			for _, id := range []uuid.UUID{dung.id, uuid.New()} {
				if r := e.call(an, m, p+id.String(), "", nil); r.status != http.StatusNotFound {
					t.Fatalf("%s %s %s: %d %s", m, p, id, r.status, r.raw)
				}
			}
		}
	}
	// an blocks binh; chi blocks an; an mutes chi.
	e.call(an, http.MethodPut, "/social/blocks/"+binh.id.String(), "", nil)
	e.call(chi, http.MethodPut, "/social/blocks/"+an.id.String(), "", nil)
	e.call(an, http.MethodPut, "/social/mutes/"+chi.id.String(), "", nil)
	b := e.call(an, http.MethodGet, "/social/blocks", "", nil)
	if b.status != http.StatusOK || len(b.data()) != 1 || b.data()[0]["user_id"] != binh.id.String() {
		t.Fatalf("listBlocks of an: %s", b.raw)
	}
	pii(t, b)
	// Nothing tells binh it was blocked, nor an that chi blocked it.
	for _, s := range []student{binh} {
		if r := e.call(s, http.MethodGet, "/social/blocks", "", nil); len(r.data()) != 0 {
			t.Fatalf("blocked student lists a block: %s", r.raw)
		}
	}
	if r := e.call(chi, http.MethodGet, "/social/blocks", "", nil); len(r.data()) != 1 || r.data()[0]["user_id"] != an.id.String() {
		t.Fatalf("listBlocks of chi: %s", r.raw)
	}
	if r := e.call(an, http.MethodGet, "/social/mutes", "", nil); len(r.data()) != 1 || r.data()[0]["user_id"] != chi.id.String() {
		t.Fatalf("listMutes of an: %s", r.raw)
	}
}
