//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// Delta F102 (additive): PartyMember.level is the XP level of the public card ref.player_profiles, kept current by
// level.reached of module game (consumer competition.projection_game, R29: event, no synchronous call). A level only
// rises (late or replayed events, an older game_profile.updated never lower it); level.reached never creates a card;
// level.reached of another tenant never touches the card (tenant isolation, DLQ at once); getMyParty/getParty stay
// member-only (outsider of tenant B → 404).
func TestPartyMemberLevel(t *testing.T) {
	e := setupParty(t, popts{})
	an, binh := e.kidA(), e.kidA()
	id := e.party(an, binh)
	gp := consumer.GameProjection{P: command.Projection{Refs: persistence.Refs{}}}
	apply := func(tenant uuid.UUID, typ string, payload map[string]any) error {
		b, _ := json.Marshal(payload)
		env := outbox.Envelope{EventID: uuid.New(), EventType: typ, OccurredAt: time.Now(), TenantID: &tenant, Payload: b}
		sc, _ := authz.EventScope(tenant)
		return e.uow.InTenantTx(context.Background(), sc, func(ctx context.Context) error { return gp.Handle(ctx, env) })
	}
	reached := func(k kid, tenant uuid.UUID, to int) error {
		return apply(tenant, "level.reached", map[string]any{"student_id": k.id, "from_level": to - 1, "to_level": to,
			"curve_version": 1, "reached_at": time.Now().UTC().Format(time.RFC3339)})
	}
	levels := func(k kid) map[string]any {
		r := e.call(k, http.MethodGet, "/multiplayer/parties/"+id, "", nil)
		if r.status != http.StatusOK {
			t.Fatalf("getParty: %d %s", r.status, r.raw)
		}
		out := map[string]any{}
		for _, m := range r.members() {
			out[m["student_id"].(string)] = m["level"]
		}
		return out
	}
	if err := reached(an, an.tenant, 7); err != nil {
		t.Fatal(err)
	}
	if got := levels(binh)[an.id.String()]; got != float64(7) {
		t.Fatalf("level after level.reached: %v", got)
	}
	// A replayed lower level and an older-version profile event never lower it (levels never decrease).
	if err := reached(an, an.tenant, 5); err != nil {
		t.Fatal(err)
	}
	if err := apply(an.tenant, "game_profile.updated", map[string]any{"student_id": an.id, "tenant_id": an.tenant, "nickname": an.nickname,
		"level": 4, "avatar_config": nil, "source_version": 2}); err != nil {
		t.Fatal(err)
	}
	me := e.call(an, http.MethodGet, "/multiplayer/parties/me", "", nil)
	for _, m := range me.members() {
		if m["student_id"] == an.id.String() && m["level"] != float64(7) {
			t.Fatalf("level lowered: %s", me.raw)
		}
	}
	// level.reached of tenant B for a student of tenant A: refused at once (DLQ), the card is unchanged.
	if err := reached(an, e.tenantB, 9); err == nil || !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("cross-tenant level.reached: %v", err)
	}
	if e.count(`SELECT count(*) FROM ref.player_profiles WHERE student_id = $1 AND level = 7`, an.id) != 1 {
		t.Fatal("cross-tenant event changed the card")
	}
	// level.reached for a student without a card (profile not projected yet): no row is created (the card comes from
	// game_profile.updated, which carries the level too). A member without a card has no level (unit TestPartyBodyLevel).
	nobody := kid{id: uuid.New(), tenant: e.tenantA}
	if err := reached(nobody, e.tenantA, 3); err != nil {
		t.Fatal(err)
	}
	if e.count(`SELECT count(*) FROM ref.player_profiles WHERE student_id = $1`, nobody.id) != 0 {
		t.Fatal("level.reached created a card without nickname")
	}
	// Outsider of tenant B: 404, never the party nor its levels.
	if r := e.call(e.kid(e.tenantB, e.schoolB, 6), http.MethodGet, "/multiplayer/parties/"+id, "", nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B: %d %s", r.status, r.raw)
	}
}
