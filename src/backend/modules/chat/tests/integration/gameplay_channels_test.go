//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/persistence"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/chatid"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// gameplay returns the consumer chat.gameplay_channels on the real adapters of the chat environment.
func (c *chatEnv) gameplay() consumer.Gameplay {
	return consumer.Gameplay{H: command.Gameplay{Channels: c.channels, Gameplay: persistence.Gameplay{},
		Events: persistence.Publisher{W: c.writer}, Metrics: observability.Gameplay{}, Seq: cache.Seq{C: c.vk},
		Config: command.GameplayConfig{DefaultRetentionDays: 7}}}
}

// competition delivers one competition event to the consumer in the transaction the outbox consumer opens.
func (c *chatEnv) competition(typ string, tenant uuid.UUID, schemaVersion int, payload any) error {
	c.t.Helper()
	g := c.gameplay()
	return c.handle(c.t, func(ctx context.Context, e outbox.Envelope) error {
		e.SchemaVersion = schemaVersion
		return g.Handle(ctx, e)
	}, typ, tenant, payload)
}

func (c *chatEnv) mustCompetition(typ string, tenant uuid.UUID, payload any) {
	c.t.Helper()
	v := 1
	if typ == "party.created" {
		v = 2
	}
	if err := c.competition(typ, tenant, v, payload); err != nil {
		c.t.Fatalf("%s: %v", typ, err)
	}
}

func (c *chatEnv) activeMembers(ch uuid.UUID) int {
	return c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND left_at IS NULL AND deleted_at IS NULL`, ch)
}

func partyCreated(party, tenant, school, leader uuid.UUID, v int, at time.Time) map[string]any {
	return map[string]any{"party_id": party, "tenant_id": tenant, "school_id": school, "leader_id": leader, "party_type": "friend",
		"visibility": "private", "invitation_policy": "leader_only", "max_members": 4, "grade_band": "g1_3", "state_version": v,
		"created_at": at}
}

func memberJoined(party, tenant, student uuid.UUID, v int) map[string]any {
	return map[string]any{"party_id": party, "tenant_id": tenant, "student_id": student, "role": "member", "via": "invitation",
		"state_version": v, "joined_at": time.Now().UTC()}
}

// T010, T012, T013 — F117-AC-01, AC-03, AC-04, AC-05, AC-06, AC-15 (TC-CHAT-017, TC-SVC-003, TC-CHAT-020, TC-CHAT-024): the
// party channel (deterministic id) is opened and filled from party.* whatever the order or the redeliveries, publishes
// chat.channel_opened with the members, closes read-only on party.disbanded with purge_after = closed_at + retention
// (tenant override over platform, floor 3 days), and its history stays readable; a member who left reads and sends
// nothing new.
func TestGameplayPartyChannel(t *testing.T) {
	c := setupChat(t, chatOpts{})
	an, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA2)
	party := uuid.New()
	ch := chatid.Party(party)
	created := c.clock.Now()

	// Out of order: Chi's join (v2) arrives before party.created (v0) — the channel exists and Chi is a member.
	c.mustCompetition("party.member_joined", c.tenantA, memberJoined(party, c.tenantA, chi.id, 2))
	c.mustCompetition("party.created", c.tenantA, partyCreated(party, c.tenantA, c.schoolA, an.id, 0, created))
	c.mustCompetition("party.member_joined", c.tenantA, memberJoined(party, c.tenantA, binh.id, 1))
	// Redeliveries change nothing (processed twice: no duplicate channel or member).
	c.mustCompetition("party.member_joined", c.tenantA, memberJoined(party, c.tenantA, binh.id, 1))
	c.mustCompetition("party.created", c.tenantA, partyCreated(party, c.tenantA, c.schoolA, an.id, 0, created))
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND channel_type = 'party' AND is_gameplay AND status = 'open'`, ch); n != 1 {
		t.Fatalf("party channel: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND deleted_at IS NULL`, ch); n != 3 || c.activeMembers(ch) != 3 {
		t.Fatalf("members: %d (want 3, no duplicate)", n)
	}
	opened := c.events(command.EventChannelOpened)
	if len(opened) < 3 {
		t.Fatalf("chat.channel_opened: %d", len(opened))
	}
	last := opened[len(opened)-1]
	if ids, _ := last["member_ids"].([]any); last["channel_id"] != ch.String() || last["channel_type"] != "party" || len(ids) != 3 {
		t.Fatalf("chat.channel_opened payload: %v", last)
	}
	// Everyone chats at once: no "turn chat on" step.
	for _, s := range []student{an, binh, chi} {
		if r := c.sendWS(s, "conn-"+s.nickname, ch, "xin chao"); !r.OK {
			t.Fatalf("send of a member: %+v", r.Problem)
		}
	}
	// Chi leaves (v3): no sending nor reading anymore; a late redelivery of her join (v2) does not bring her back.
	c.mustCompetition("party.member_left", c.tenantA, map[string]any{"party_id": party, "tenant_id": c.tenantA, "student_id": chi.id,
		"reason": "left", "state_version": 3})
	c.mustCompetition("party.member_joined", c.tenantA, memberJoined(party, c.tenantA, chi.id, 2))
	if c.activeMembers(ch) != 2 {
		t.Fatalf("after leave: %d active", c.activeMembers(ch))
	}
	if r := c.sendWS(chi, "conn-chi", ch, "con o day"); r.OK || replyCode(r) != "NOT_FOUND" {
		t.Fatalf("send after leave: %+v", r)
	}
	if r := c.call(chi, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("history after leave: %d", r.status)
	}
	// Reconnect / other device: membership kept, chat.channel.sync returns the missed messages (F117-AC-06).
	r := c.ws(an, "conn-an-tablet", "chat.channel.sync", map[string]any{"channels": map[string]int64{ch.String(): 0}})
	if !r.OK {
		t.Fatalf("sync: %+v", r.Problem)
	}
	res := map[string]any{}
	_ = jsonUnmarshal(r.Result, &res)
	if chans, _ := res["channels"].([]any); len(chans) != 1 || chans[0].(map[string]any)["count"] != float64(3) {
		t.Fatalf("sync result: %v", res)
	}
	// Retention: tenant override 2 days (below the minimum) → 3 days with a warning (F117-AC-15).
	c.exec(`UPDATE ref.tenants SET gameplay_chat_retention_days = 2 WHERE tenant_id = $1`, c.tenantA)
	disbanded := c.clock.Now().Add(time.Minute)
	c.mustCompetition("party.disbanded", c.tenantA, map[string]any{"party_id": party, "tenant_id": c.tenantA, "reason": "leader_disbanded",
		"actor_id": an.id, "disbanded_at": disbanded, "state_version": 4})
	var purgeAfter time.Time
	if err := c.su.QueryRow(`SELECT purge_after FROM chat.channels WHERE channel_id = $1 AND status = 'closed' AND closed_at = $2`,
		ch, disbanded).Scan(&purgeAfter); err != nil {
		t.Fatalf("closed channel: %v", err)
	}
	if !purgeAfter.Equal(disbanded.AddDate(0, 0, 3)) {
		t.Fatalf("purge_after %v, want closed_at + 3 days (floor)", purgeAfter)
	}
	if ev := c.events(command.EventChannelClosed); len(ev) != 1 || ev[0]["channel_id"] != ch.String() || ev[0]["purge_after"] == nil {
		t.Fatalf("chat.channel_closed: %v", ev)
	}
	// Read-only: CHAT_CHANNEL_CLOSED (409) on WS and REST; the history stays readable until the purge.
	if r := c.sendWS(an, "conn-an", ch, "con ai khong"); r.OK || replyCode(r) != "CHAT_CHANNEL_CLOSED" {
		t.Fatalf("send to a closed channel: %+v", r)
	}
	if r := c.sendREST(an, ch, map[string]any{"client_message_id": uuid.New(), "kind": "text", "body": "hello"}); r.status != http.StatusConflict ||
		r.code() != "CHAT_CHANNEL_CLOSED" {
		t.Fatalf("REST send to a closed channel: %d %s", r.status, r.raw)
	}
	c.waitCount(3, 5*time.Second, `SELECT count(*) FROM chat.gameplay_messages WHERE channel_id = $1`, ch)
	if r := c.call(an, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusOK || len(r.data()) != 3 {
		t.Fatalf("history of a closed channel: %d %s", r.status, r.raw)
	}
	// A join after party.disbanded (lagging event) is ignored; a closed channel never gets members again.
	late := c.student(c.tenantA, c.schoolA)
	c.mustCompetition("party.member_joined", c.tenantA, memberJoined(party, c.tenantA, late.id, 5))
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2`, ch, late.id); n != 0 {
		t.Fatal("member added to a disbanded party channel")
	}
	// listMyChannels shows the closed channel with status and purge_after (F117-AC-14, FR-CHAT-011).
	lr := c.call(an, http.MethodGet, "/chat/channels", "", nil)
	found := false
	for _, d := range lr.data() {
		if d["channel_id"] == ch.String() {
			found = d["status"] == "closed" && d["purge_after"] != nil
		}
	}
	if lr.status != http.StatusOK || !found {
		t.Fatalf("listMyChannels: %d %s", lr.status, lr.raw)
	}
	if seen := c.checkEvents(command.EventChannelOpened, command.EventChannelClosed); seen[command.EventChannelOpened] == 0 ||
		seen[command.EventChannelClosed] != 1 {
		t.Fatalf("schemas checked: %v", seen)
	}
}

func participants(team map[int][]student) []map[string]any {
	var out []map[string]any
	for no, ss := range team {
		for _, s := range ss {
			out = append(out, map[string]any{"student_id": s.id, "tenant_id": s.tenant, "school_id": s.school, "team_no": no})
		}
	}
	return out
}

func matchStarted(match uuid.UUID, mode string, ps []map[string]any) map[string]any {
	return map[string]any{"match_id": match, "room_id": match, "mode": mode, "matchmaking_type": "skill", "ranked": true, "is_bot": false,
		"subject_id": uuid.New(), "grade_level": 3, "question_count": 7, "started_at": time.Now().UTC(), "participants": ps}
}

// T011, T014 — F117-AC-02, AC-03, AC-07 (TC-CHAT-017, TC-TEN-012): match.started opens one match channel of every player and,
// for a team battle, one team channel per team; the other team gets 404 on a team channel; a participant of another tenant
// or who is not an active student (staff, bot) is never added; a student of another tenant cannot read the channel; an end
// event arriving first creates the channels closed so that a late match.started adds nobody.
func TestGameplayMatchChannels(t *testing.T) {
	c := setupChat(t, chatOpts{})
	a1, a2 := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	b1, b2 := c.student(c.tenantA, c.schoolA2), c.student(c.tenantA, c.schoolA2)
	other := c.student(c.tenantB, c.schoolB)
	staff := student{id: uuid.New(), tenant: c.tenantA, school: c.schoolA} // not in ref.students (staff or bot)
	match := uuid.New()
	ps := participants(map[int][]student{0: {a1, a2}, 1: {b1, b2, staff}})
	ps = append(ps, map[string]any{"student_id": other.id, "tenant_id": other.tenant, "school_id": other.school, "team_no": 1})
	c.mustCompetition("match.started", c.tenantA, matchStarted(match, "team_battle", ps))
	c.mustCompetition("match.started", c.tenantA, matchStarted(match, "team_battle", ps)) // redelivery
	mch, t0, t1 := chatid.Match(match), chatid.Team(match, 0), chatid.Team(match, 1)
	if c.activeMembers(mch) != 4 || c.activeMembers(t0) != 2 || c.activeMembers(t1) != 2 {
		t.Fatalf("members: match %d team0 %d team1 %d", c.activeMembers(mch), c.activeMembers(t0), c.activeMembers(t1))
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE user_id IN ($1, $2)`, staff.id, other.id); n != 0 {
		t.Fatal("a non-student or another tenant's student was added")
	}
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_type = 'team' AND is_gameplay AND scope_ref->>'match_id' = $1`,
		match.String()); n != 2 {
		t.Fatalf("team channels: %d", n)
	}
	if r := c.call(b1, http.MethodGet, "/chat/channels/"+t0.String()+"/messages", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("other team reads team 0: %d", r.status)
	}
	if r := c.call(a1, http.MethodGet, "/chat/channels/"+t0.String()+"/messages", "", nil); r.status != http.StatusOK {
		t.Fatalf("own team: %d %s", r.status, r.raw)
	}
	if r := c.sendWS(b1, "conn-b1", t0, "doi ban"); r.OK || replyCode(r) != "NOT_FOUND" {
		t.Fatalf("other team sends to team 0: %+v", r)
	}
	if r := c.call(other, http.MethodGet, "/chat/channels/"+mch.String()+"/messages", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B reads the match of tenant A: %d", r.status)
	}
	if r := c.sendWS(a1, "conn-a1", mch, "chuc may man"); !r.OK {
		t.Fatalf("match send: %+v", r.Problem)
	}
	// An event of tenant B naming the match of tenant A never reaches tenant A's channel (RLS + tenant of the envelope).
	if err := c.competition("match.completed", c.tenantB, 1, map[string]any{"match_id": match, "results": participants(map[int][]student{0: {other}}),
		"ended_at": time.Now().UTC()}); err == nil {
		t.Fatal("an event of tenant B on a channel id of tenant A must be refused (DLQ)")
	}
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND status = 'closed'`, mch); n != 0 {
		t.Fatal("tenant B closed the channel of tenant A")
	}
	// 1v1: one match channel, no team channel.
	duel := uuid.New()
	c.mustCompetition("match.started", c.tenantA, matchStarted(duel, "battle_1v1", participants(map[int][]student{0: {a1}, 1: {b1}})))
	if c.activeMembers(chatid.Match(duel)) != 2 || c.count(`SELECT count(*) FROM chat.channels WHERE scope_ref->>'match_id' = $1 AND channel_type = 'team'`,
		duel.String()) != 0 {
		t.Fatal("1v1 channels")
	}
	// match.completed closes the match and team channels; a redelivered match.started adds nobody.
	c.mustCompetition("match.completed", c.tenantA, map[string]any{"match_id": match, "results": participants(map[int][]student{0: {a1, a2}, 1: {b1, b2}}), "ended_at": time.Now().UTC()})
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id IN ($1, $2, $3) AND status = 'closed' AND purge_after IS NOT NULL`,
		mch, t0, t1); n != 3 {
		t.Fatalf("closed: %d", n)
	}
	// match.aborted before match.started (reordered): created closed; the late start adds nobody.
	aborted := uuid.New()
	c.mustCompetition("match.aborted", c.tenantA, map[string]any{"match_id": aborted, "reason": "plan_failure",
		"aborted_at": time.Now().UTC(), "participants": participants(map[int][]student{0: {a1}, 1: {b1}})})
	c.mustCompetition("match.started", c.tenantA, matchStarted(aborted, "battle_1v1", participants(map[int][]student{0: {a1}, 1: {b1}})))
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND status = 'closed'`, chatid.Match(aborted)); n != 1 ||
		c.activeMembers(chatid.Match(aborted)) != 0 {
		t.Fatal("aborted match channel")
	}
	// An invalid payload is permanent (DLQ at once).
	if err := c.competition("match.started", c.tenantA, 1, map[string]any{"match_id": uuid.New(), "participants": []any{}}); err == nil {
		t.Fatal("empty participants accepted")
	}
	// The scope of the event is the tenant of the envelope only (never derived from the payload).
	if _, err := authz.EventScope(c.tenantA); err != nil {
		t.Fatal(err)
	}
}

// T032 / F117-AC-15 (research R4): chat_retention of the platform and the tenant override are projected from
// platform_settings.changed; purge_after of a closing channel uses the tenant override when the platform allows it, the
// platform value otherwise; an invalid payload goes to the DLQ.
func TestGameplayRetentionProjection(t *testing.T) {
	c := setupChat(t, chatOpts{})
	id := consumer.IdentityProjection{P: command.Projection{Refs: persistence.Refs{}, Social: persistence.Social{}}}
	platform := func(after map[string]any) error {
		b, _ := json.Marshal(map[string]any{"key": "chat_retention", "scope": "platform", "tenant_id": nil, "school_id": nil, "flag_key": nil,
			"before": nil, "after": after, "reason": "test", "actor_id": uuid.New(), "curfew_window": nil})
		ps, err := authz.SystemPlatformScope("consumer-chat-projection-identity")
		if err != nil {
			t.Fatal(err)
		}
		env := outbox.Envelope{EventID: uuid.New(), EventType: "platform_settings.changed", OccurredAt: time.Now(), Payload: b}
		return c.uow.InTenantTx(context.Background(), ps, func(ctx context.Context) error { return id.Handle(ctx, env) })
	}
	tenantSetting := func(days any) error {
		after := map[string]any{"inactive_student_cleanup_days": 180}
		if days != nil {
			after["gameplay_chat_retention_days"] = days
		}
		time.Sleep(2 * time.Millisecond) // versions are the occurred_at in µs
		return c.handle(t, id.Handle, "platform_settings.changed", c.tenantA, map[string]any{"key": "tenant_settings", "scope": "tenant",
			"tenant_id": c.tenantA, "school_id": nil, "flag_key": nil, "before": nil, "after": after, "reason": nil, "actor_id": uuid.New(),
			"curfew_window": nil})
	}
	closeMatch := func() time.Duration {
		a, b := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
		m := uuid.New()
		ps := participants(map[int][]student{0: {a}, 1: {b}})
		c.mustCompetition("match.started", c.tenantA, matchStarted(m, "battle_1v1", ps))
		end := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
		c.mustCompetition("match.completed", c.tenantA, map[string]any{"match_id": m, "results": ps, "ended_at": end})
		var pa time.Time
		if err := c.su.QueryRow(`SELECT purge_after FROM chat.channels WHERE channel_id = $1`, chatid.Match(m)).Scan(&pa); err != nil {
			t.Fatal(err)
		}
		return pa.Sub(end)
	}
	day := 24 * time.Hour
	if d := closeMatch(); d != 7*day {
		t.Fatalf("default retention: %v", d)
	}
	if err := platform(map[string]any{"gameplay_chat_retention_days": 10, "ephemeral_retention_hours": 24, "allow_tenant_override": true}); err != nil {
		t.Fatal(err)
	}
	if d := closeMatch(); d != 10*day {
		t.Fatalf("platform retention: %v", d)
	}
	if err := tenantSetting(5); err != nil {
		t.Fatal(err)
	}
	if d := closeMatch(); d != 5*day {
		t.Fatalf("tenant override: %v", d)
	}
	if err := platform(map[string]any{"gameplay_chat_retention_days": 10, "ephemeral_retention_hours": 24, "allow_tenant_override": false}); err != nil {
		t.Fatal(err)
	}
	if d := closeMatch(); d != 10*day {
		t.Fatalf("override not allowed: %v", d)
	}
	if err := tenantSetting(nil); err != nil {
		t.Fatal(err)
	}
	if n := c.count(`SELECT count(*) FROM ref.tenants WHERE tenant_id = $1 AND gameplay_chat_retention_days IS NULL`, c.tenantA); n != 1 {
		t.Fatal("tenant override not cleared")
	}
	if err := platform(map[string]any{"gameplay_chat_retention_days": 0}); err == nil {
		t.Fatal("invalid chat_retention accepted")
	}
	if err := tenantSetting(400); err == nil {
		t.Fatal("invalid tenant retention accepted")
	}
}
