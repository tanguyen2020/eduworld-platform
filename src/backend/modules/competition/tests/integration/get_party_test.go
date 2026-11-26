//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// T016 — F100-AC-06 (TC-TEN-001, R16): getMyParty / getParty return the members with student_id, nickname, avatar,
// role, joined_at, level only (no real name, school, class); a non-member, a party of another school or tenant → 404
// PARTY_NOT_FOUND.
func TestGetParty(t *testing.T) {
	e := setupParty(t, popts{})
	an, binh := e.kidA(), e.kidA()
	id := e.party(an, binh)
	e.online.set(binh.id, true)
	for _, k := range []kid{an, binh} {
		for _, path := range []string{"/multiplayer/parties/me", "/multiplayer/parties/" + id} {
			r := e.call(k, http.MethodGet, path, "", nil)
			if r.status != http.StatusOK || r.str("party_id") != id {
				t.Fatalf("%s by %s: %d %s", path, k.nickname, r.status, r.raw)
			}
			noPII(t, r)
			ms := r.members()
			if len(ms) != 2 {
				t.Fatalf("members: %s", r.raw)
			}
			for _, m := range ms {
				for k := range m {
					switch k {
					case "student_id", "nickname", "avatar_config", "role", "joined_at", "presence", "level": // level: delta F102
					default:
						t.Fatalf("member field %s: %s", k, r.raw)
					}
				}
				if m["level"] != float64(4) { // projected level of the public card (delta F102)
					t.Fatalf("level: %v", m)
				}
				if m["student_id"] == binh.id.String() && m["presence"] != "online" {
					t.Fatalf("presence: %v", m)
				}
			}
		}
	}
	// Outsiders of the same school, another school and another tenant: 404, never the party.
	other := e.kidA()
	school2 := e.kid(e.tenantA, e.schoolA2, 6)
	dung := e.kid(e.tenantB, e.schoolB, 7)
	for _, k := range []kid{other, school2, dung} {
		r := e.call(k, http.MethodGet, "/multiplayer/parties/"+id, "", nil)
		if r.status != http.StatusNotFound || r.code() != "PARTY_NOT_FOUND" {
			t.Fatalf("getParty by outsider %s: %d %s", k.nickname, r.status, r.raw)
		}
	}
	if r := e.call(other, http.MethodGet, "/multiplayer/parties/me", "", nil); r.status != http.StatusNotFound || r.code() != "PARTY_NOT_FOUND" {
		t.Fatalf("getMyParty without party: %d %s", r.status, r.raw)
	}
	if r := e.call(an, http.MethodGet, "/multiplayer/parties/"+uuid.NewString(), "", nil); r.status != http.StatusNotFound {
		t.Fatalf("unknown: %d", r.status)
	}
}
