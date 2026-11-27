//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

var clubEvents = []string{"club.created", "club.approved", "club.archived", "club.member_joined", "club.member_left", "club.goal_completed"}

// T035 — F120-AC-17 (TC-SOC-004): a student creates a club only as student_club_creation allows (disabled → 403
// PERMISSION_DENIED, requires_approval → pending_approval without channel); the name goes through the pipeline (451
// CHAT_CONTENT_BLOCKED, never echoed); club_type outside the four types (study_group included) and the class options
// (class_id, add_whole_class — ADR-0095) → 422; the same Idempotency-Key creates one club.
func TestClubCreate(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	an := c.student(c.tenantA, c.schoolA)
	c.school(c.tenantA, c.schoolA, nil)
	if r := c.createClub(an, map[string]any{"club_type": "interest_group", "name": "Nha tham hiem"}); r.status != 403 || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("creation disabled: %d %s", r.status, r.raw)
	}
	c.school(c.tenantA, c.schoolA, map[string]any{"student_club_creation": "requires_approval"})
	key := newKey()
	r := c.call(an, http.MethodPost, "/social/clubs", key, map[string]any{"club_type": "interest_group", "name": "Nha tham hiem",
		"description": "Kham pha the gioi"})
	if r.status != http.StatusCreated || r.body["status"] != "pending_approval" || r.body["chat_channel_id"] != nil || r.body["my_role"] != "owner" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	pii(t, r)
	again := c.call(an, http.MethodPost, "/social/clubs", key, map[string]any{"club_type": "interest_group", "name": "Nha tham hiem",
		"description": "Kham pha the gioi"})
	if again.status != http.StatusCreated || again.body["club_id"] != r.body["club_id"] || again.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", again.status, again.raw)
	}
	if n := c.count(`SELECT count(*) FROM social.clubs WHERE tenant_id = $1`, c.tenantA); n != 1 {
		t.Fatalf("clubs: %d", n)
	}
	for name, body := range map[string]map[string]any{
		"study_group":     {"club_type": "study_group", "name": "Nhom hoc"},
		"class_id":        {"club_type": "interest_group", "name": "Ca lop", "class_id": uuid.New()},
		"add_whole_class": {"club_type": "interest_group", "name": "Ca lop", "add_whole_class": true},
	} {
		if r := c.createClub(an, body); r.status != 422 || r.code() != "VALIDATION_FAILED" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if r := c.createClub(an, map[string]any{"club_type": "school_club", "name": "CLB Toan"}); r.status != 403 {
		t.Fatalf("student school_club: %d %s", r.status, r.raw)
	}
	b := c.createClub(an, map[string]any{"club_type": "interest_group", "name": "Ten " + blockedPlaceholder})
	if b.status != 451 || b.code() != "CHAT_CONTENT_BLOCKED" || strings.Contains(b.raw, blockedPlaceholder) {
		t.Fatalf("blocked name: %d %s", b.status, b.raw)
	}
	if n := c.checkEvents(clubEvents...)["club.created"]; n != 1 {
		t.Fatalf("club.created events: %d", n)
	}
}

// T036 — F120-AC-18 (TC-SOC-004): School Admin of the school of the club (social.club.approve) approves: active, the fixed
// team channel (is_gameplay false) with the owner, club.approved; acting on the club before → 409 CLUB_NOT_APPROVED;
// another school's admin → 403, another tenant → 404, a student → 403; approving twice → 409.
func TestClubApprove(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	an, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	c.school(c.tenantA, c.schoolA, map[string]any{"student_club_creation": true}) // the boolean of school.updated today
	r := c.createClub(an, map[string]any{"club_type": "competition_team", "name": "Doi thi dau"})
	if r.status != http.StatusCreated || r.body["status"] != "pending_approval" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	id := str(r.body, "club_id")
	if r := c.call(an, http.MethodPost, "/social/clubs/"+id+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{binh.id}}); r.status != 409 ||
		r.code() != "CLUB_NOT_APPROVED" {
		t.Fatalf("invite before approval: %d %s", r.status, r.raw)
	}
	if r := c.call(binh, http.MethodPost, "/social/clubs/"+id+"/join-requests", newKey(), nil); r.status != 409 || r.code() != "CLUB_NOT_APPROVED" {
		t.Fatalf("join before approval: %d %s", r.status, r.raw)
	}
	approve := func(token string, tenant uuid.UUID, key string) resp {
		return c.portal(token, tenant, http.MethodPost, "/social/clubs/"+id+":approve", map[string]string{"Idempotency-Key": key}, nil)
	}
	if r := approve(c.staffAt(c.tenantA, "school_admin", c.schoolA2), c.tenantA, newKey()); r.status != 403 || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("other school admin: %d %s", r.status, r.raw)
	}
	if r := approve(c.staffAt(c.tenantB, "school_admin", c.schoolB), c.tenantB, newKey()); r.status != 404 {
		t.Fatalf("other tenant admin: %d %s", r.status, r.raw)
	}
	if r := c.call(an, http.MethodPost, "/social/clubs/"+id+":approve", newKey(), nil); r.status != 403 || r.code() != "PORTAL_ACCESS_STAFF_ONLY" {
		t.Fatalf("student approves: %d %s", r.status, r.raw)
	}
	admin := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	pending := c.portal(admin, c.tenantA, http.MethodGet, "/social/clubs?status=pending_approval", nil, nil)
	if pending.status != http.StatusOK || len(pending.data()) != 1 || pending.data()[0]["school_id"] != c.schoolA.String() {
		t.Fatalf("staff list pending: %d %s", pending.status, pending.raw)
	}
	key := newKey()
	a := approve(admin, c.tenantA, key)
	if a.status != http.StatusOK || a.body["status"] != "active" || a.body["chat_channel_id"] == nil {
		t.Fatalf("approve: %d %s", a.status, a.raw)
	}
	ch := uuid.MustParse(str(a.body, "chat_channel_id"))
	if ch != channel.ClubID(c.tenantA, uuid.MustParse(id)) {
		t.Fatal("not the club channel id")
	}
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND channel_type = 'team' AND NOT is_gameplay
		AND scope_ref->>'club_id' = $2`, ch, id); n != 1 {
		t.Fatal("team channel")
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2 AND left_at IS NULL`, ch, an.id); n != 1 {
		t.Fatal("owner not in the channel")
	}
	if r := approve(admin, c.tenantA, key); r.status != http.StatusOK || r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("approve replay: %d %s", r.status, r.raw)
	}
	if r := approve(admin, c.tenantA, newKey()); r.status != 409 {
		t.Fatalf("approve twice: %d %s", r.status, r.raw)
	}
	if n := c.checkEvents(clubEvents...)["club.approved"]; n != 1 {
		t.Fatalf("club.approved events: %d", n)
	}
	if countOf(c.audits(), "club.approved") != 1 {
		t.Fatal("approve audit")
	}
}

// T037 — F120-AC-19 (TC-SOC-005, TC-SOC-006): the last seat of a club taken by two acceptances at once → one member, the
// other 409 CLUB_FULL; a sixth club of a student → 409 CLUB_LIMIT_REACHED; inviting a student of another tenant, of
// another school (no cross-school) or in a block with the inviter → 403 SOCIAL_SCOPE_VIOLATION; staff is never a member.
func TestClubLimits(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	owner := c.student(c.tenantA, c.schoolA)
	clubID, _ := c.mustClub(owner, "Hoi sach")
	c.exec(`UPDATE social.clubs SET max_members = 3 WHERE club_id = $1`, clubID)
	x, y, z := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	c.joinClub(owner, x, clubID) // 2 of 3
	invs := map[student]string{}
	for _, s := range []student{y, z} {
		r := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{s.id}})
		if r.status != http.StatusCreated {
			t.Fatalf("invite: %d %s", r.status, r.raw)
		}
		invs[s] = str(r.data()[0], "invitation_id")
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for s, inv := range invs {
		wg.Add(1)
		go func(s student, inv string) {
			defer wg.Done()
			codes <- c.call(s, http.MethodPost, "/social/club-invitations/"+inv+":accept", newKey(), nil).status
		}(s, inv)
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for code := range codes {
		got[code]++
	}
	if got[200] != 1 || got[409] != 1 {
		t.Fatalf("concurrent last seat: %v", got)
	}
	if n := c.count(`SELECT member_count FROM social.clubs WHERE club_id = $1`, clubID); n != 3 {
		t.Fatalf("member_count: %d", n)
	}
	// clubs_per_student_max = 5: a student in five clubs cannot join a sixth.
	w := c.student(c.tenantA, c.schoolA)
	for i := range 5 {
		o := c.student(c.tenantA, c.schoolA)
		id, _ := c.mustClub(o, "Club "+string(rune('A'+i)))
		c.joinClub(o, w, id)
	}
	o6 := c.student(c.tenantA, c.schoolA)
	sixth, _ := c.mustClub(o6, "Club sau")
	inv := c.call(o6, http.MethodPost, "/social/clubs/"+sixth.String()+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{w.id}})
	if r := c.call(w, http.MethodPost, "/social/club-invitations/"+str(inv.data()[0], "invitation_id")+":accept", newKey(), nil); r.status != 409 ||
		r.code() != "CLUB_LIMIT_REACHED" {
		t.Fatalf("sixth club: %d %s", r.status, r.raw)
	}
	// Scope: another tenant, another school without cross-school social, a block with the inviter, a staff id.
	other, foreign := c.student(c.tenantA, c.schoolA2), c.student(c.tenantB, c.schoolB)
	blocked := c.student(c.tenantA, c.schoolA)
	if r := c.call(blocked, http.MethodPut, "/social/blocks/"+owner.id.String(), "", nil); r.status != http.StatusNoContent {
		t.Fatalf("block: %d %s", r.status, r.raw)
	}
	staffID := uuid.New()
	for name, id := range map[string]uuid.UUID{"other tenant": foreign.id, "other school": other.id, "blocked": blocked.id, "staff": staffID} {
		r := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{id}})
		if r.status != 403 || r.code() != "SOCIAL_SCOPE_VIOLATION" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE user_id = $1`, staffID); n != 0 {
		t.Fatal("staff became a channel member")
	}
}

// T012, T013, T014, T015, T038 — F120-AC-04 (TC-SVC-003), F120-AC-05 (TC-CHAT-009), F120-AC-20 (TC-SOC-008): joining a club
// adds the student to the club channel in the same transaction (club.member_joined); the same Idempotency-Key joins once;
// leaving or being removed sets left_at (history 404, club.member_left); joining again shows only the messages from the new
// joined_at; a non-member of the school, staff and another tenant read nothing (404/403), RLS hides the channel from a
// repository query without the tenant condition; archiving makes the channel read_only (send CHAT_CHANNEL_CLOSED, history
// kept).
func TestClubChannelMembership(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	c.pace = 0
	owner, binh, chi := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	outsider, foreign := c.student(c.tenantA, c.schoolA), c.student(c.tenantB, c.schoolB)
	clubID, ch := c.mustClub(owner, "CLB Toan")
	inv := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/invitations", newKey(), map[string]any{"invitee_ids": []uuid.UUID{binh.id}})
	key := newKey()
	for range 2 {
		if r := c.call(binh, http.MethodPost, "/social/club-invitations/"+str(inv.data()[0], "invitation_id")+":accept", key, nil); r.status != http.StatusOK {
			t.Fatalf("accept: %d %s", r.status, r.raw)
		}
	}
	if n := c.count(`SELECT count(*) FROM social.club_members WHERE club_id = $1 AND user_id = $2`, clubID, binh.id); n != 1 {
		t.Fatalf("member rows after a replay: %d", n)
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2 AND left_at IS NULL`, ch, binh.id); n != 1 {
		t.Fatal("not in the club channel")
	}
	if channelOf(c.call(binh, http.MethodGet, "/chat/channels", "", nil), "team") == nil {
		t.Fatal("club channel not listed")
	}
	c.joinClub(owner, chi, clubID)
	if r := c.wsText(owner, ch, "chao ca club"); !r.OK {
		t.Fatalf("send: %+v", r.Problem)
	}
	c.waitCount(1, 5*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch)
	if r := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusOK || len(r.data()) != 1 {
		t.Fatalf("member history: %d %s", r.status, r.raw)
	}
	// Isolation (F120-AC-05): non-member of the school, another tenant → 404; staff → 403 (player route), never content.
	for _, s := range []student{outsider, foreign} {
		if r := c.call(s, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusNotFound {
			t.Fatalf("non-member history: %d %s", r.status, r.raw)
		}
		if r := c.wsText(s, ch, "cho minh vao"); replyCode(r) != "NOT_FOUND" {
			t.Fatalf("non-member send: %+v", r)
		}
	}
	staff := c.staffAt(c.tenantA, "school_admin", c.schoolA)
	if r := c.portal(staff, c.tenantA, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", nil, nil); r.status != 403 ||
		strings.Contains(r.raw, "chao ca club") {
		t.Fatalf("staff history: %d %s", r.status, r.raw)
	}
	scopeB, _ := authz.EventScope(c.tenantB)
	_ = c.read.InTenantRead(context.Background(), scopeB, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		var n int
		// A repository query that forgot the tenant condition: RLS of the session tenant (B) hides tenant A's rows.
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM chat.channels WHERE channel_id = $1`, ch).Scan(&n); err != nil || n != 0 {
			t.Errorf("RLS: %d %v", n, err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM social.clubs WHERE club_id = $1`, clubID).Scan(&n); err != nil || n != 0 {
			t.Errorf("RLS clubs: %d %v", n, err)
		}
		return nil
	})
	// binh leaves: left_at, history 404, club.member_left.
	if r := c.call(binh, http.MethodPost, "/social/clubs/"+clubID.String()+":leave", "", nil); r.status != http.StatusNoContent {
		t.Fatalf("leave: %d %s", r.status, r.raw)
	}
	if r := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("history after leaving: %d %s", r.status, r.raw)
	}
	c.clock.Add(time.Minute)
	if r := c.wsText(owner, ch, "tin khi binh vang mat"); !r.OK {
		t.Fatalf("send: %+v", r.Problem)
	}
	c.waitCount(2, 5*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch)
	c.clock.Add(time.Minute)
	c.joinClub(owner, binh, clubID) // a new effective membership from the new joined_at
	c.clock.Add(time.Minute)
	if r := c.wsText(owner, ch, "chao mung tro lai"); !r.OK {
		t.Fatalf("send: %+v", r.Problem)
	}
	c.waitCount(3, 5*time.Second, `SELECT count(*) FROM chat.messages WHERE channel_id = $1`, ch)
	h := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil)
	if h.status != http.StatusOK || len(h.data()) != 1 || h.data()[0]["body"] != "chao mung tro lai" {
		t.Fatalf("history after joining again: %d %s", h.status, h.raw)
	}
	// Delta F112-2: the owner (a student) no longer removes chi — school staff do, with a reason (club.member_left reason removed).
	if r := c.call(owner, http.MethodDelete, "/social/clubs/"+clubID.String()+"/members/"+chi.id.String(), "", nil); r.status != 403 ||
		r.code() != "PERMISSION_DENIED" {
		t.Fatalf("student owner removes: %d %s", r.status, r.raw)
	}
	if r := c.portal(staff, c.tenantA, http.MethodPost, "/tenant/social/clubs/"+clubID.String()+"/members/"+chi.id.String()+":remove",
		map[string]string{"Idempotency-Key": newKey()}, map[string]any{"reason": "Nhà trường gỡ thành viên"}); r.status != http.StatusNoContent {
		t.Fatalf("staff remove: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.channel_members WHERE channel_id = $1 AND user_id = $2 AND left_at IS NOT NULL`, ch, chi.id); n != 1 {
		t.Fatal("removed member still active in the channel")
	}
	left := c.events("club.member_left")
	if len(left) != 2 || left[1]["reason"] != "removed" {
		t.Fatalf("club.member_left: %v", left)
	}
	// Archive (delta F112-2: staff only, the student owner → 403): channel read_only, history kept.
	if r := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+":archive", newKey(), nil); r.status != 403 ||
		r.code() != "PERMISSION_DENIED" {
		t.Fatalf("student owner archives: %d %s", r.status, r.raw)
	}
	if r := c.portal(staff, c.tenantA, http.MethodPost, "/social/clubs/"+clubID.String()+":archive", map[string]string{"Idempotency-Key": newKey()},
		nil); r.status != http.StatusNoContent {
		t.Fatalf("archive: %d %s", r.status, r.raw)
	}
	if n := c.count(`SELECT count(*) FROM chat.channels WHERE channel_id = $1 AND status = 'read_only'`, ch); n != 1 {
		t.Fatal("channel not read_only")
	}
	if r := c.wsText(owner, ch, "con ai khong"); replyCode(r) != "CHAT_CHANNEL_CLOSED" {
		t.Fatalf("send to an archived club: %+v", r)
	}
	if r := c.call(binh, http.MethodGet, "/chat/channels/"+ch.String()+"/messages", "", nil); r.status != http.StatusOK {
		t.Fatalf("history of an archived club: %d %s", r.status, r.raw)
	}
	seen := c.checkEvents(clubEvents...)
	if seen["club.member_joined"] < 4 || seen["club.archived"] != 1 {
		t.Fatalf("events: %v", seen)
	}
}

// T039 — F120-AC-21 (TC-SOC-007): a goal of the week counts verified events of the members (activity.graded, quest.completed,
// world.discovery_found by metric), each event once (a redelivered event_id adds nothing), never a score or mastery value;
// reaching the target completes the goal and publishes club.goal_completed; a metric without a verified source yet → 422.
func TestClubGoals(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	owner, binh := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	clubID, _ := c.mustClub(owner, "Hoc nhom")
	c.joinClub(owner, binh, clubID)
	c.clock.Set(at(10, 0))
	if r := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/goals", newKey(), map[string]any{"metric": "minutes_learning",
		"target": 30}); r.status != 422 {
		t.Fatalf("minutes_learning: %d %s", r.status, r.raw)
	}
	g := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/goals", newKey(), map[string]any{"metric": "activities_completed",
		"target": 3, "title": "Ba bai tap"})
	if g.status != http.StatusCreated || g.body["week_start"] != "2026-11-23" {
		t.Fatalf("goal: %d %s", g.status, g.raw)
	}
	goals := consumer.ClubGoals{H: c.clubs, Loc: ict}
	graded := func(s student) map[string]any {
		return map[string]any{"attempt_id": uuid.New(), "student_id": s.id, "evaluation_id": uuid.New(), "activity_type": "quiz",
			"mode": "practice", "source": "learning", "score": 9, "max_score": 10, "result_release": "immediate", "objective_ids": []string{}}
	}
	e1 := uuid.New()
	for range 2 {
		if err := c.event(goals.Handle, e1, "activity.graded", c.tenantA, graded(owner)); err != nil {
			t.Fatal(err)
		}
	}
	progress := func() int { return c.count(`SELECT progress FROM social.club_goals WHERE club_id = $1`, clubID) }
	if p := progress(); p != 1 {
		t.Fatalf("progress after a redelivery: %d", p)
	}
	if err := c.event(goals.Handle, uuid.New(), "quest.completed", c.tenantA, map[string]any{"student_id": binh.id}); err != nil {
		t.Fatal(err)
	}
	if err := c.event(goals.Handle, uuid.New(), "activity.graded", c.tenantA, graded(binh)); err != nil {
		t.Fatal(err)
	}
	if p := progress(); p != 2 {
		t.Fatalf("progress: %d", p)
	}
	if err := c.event(goals.Handle, uuid.New(), "activity.graded", c.tenantA, graded(owner)); err != nil {
		t.Fatal(err)
	}
	if n := c.count(`SELECT count(*) FROM social.club_goals WHERE club_id = $1 AND status = 'completed' AND progress = 3`, clubID); n != 1 {
		t.Fatal("goal not completed")
	}
	if n := c.checkEvents(clubEvents...)["club.goal_completed"]; n != 1 {
		t.Fatalf("club.goal_completed: %d", n)
	}
	l := c.call(owner, http.MethodGet, "/social/clubs/"+clubID.String()+"/goals", "", nil)
	if l.status != http.StatusOK || len(l.data()) != 1 || l.data()[0]["my_contribution"] != float64(2) || strings.Contains(l.raw, "score") {
		t.Fatalf("list goals: %d %s", l.status, l.raw)
	}
	if n := c.count(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'social' AND table_name LIKE 'club%'
		AND (column_name LIKE '%score%' OR column_name LIKE '%mastery%')`); n != 0 {
		t.Fatal("a club table stores a score or mastery")
	}
	ev := c.call(owner, http.MethodPost, "/social/clubs/"+clubID.String()+"/events", newKey(), map[string]any{"title": "Luyen tap thu bay",
		"starts_at": at(15, 0).Format(time.RFC3339), "event_kind": "practice_session"})
	if ev.status != http.StatusCreated {
		t.Fatalf("event: %d %s", ev.status, ev.raw)
	}
	if r := c.call(binh, http.MethodGet, "/social/clubs/"+clubID.String()+"/events", "", nil); r.status != http.StatusOK || len(r.data()) != 1 {
		t.Fatalf("list events: %d %s", r.status, r.raw)
	}
}

// T040 — authorization and tenant isolation of every /social/clubs* operation (TC-TEN-001): staff on a player route → 403
// GAME_ACCESS_STUDENT_ONLY; a student of another tenant → 404 (the club does not exist for it); X-Tenant-Id of another
// tenant → 403 TENANT_MISMATCH; a student of another school (no cross-school social) neither sees nor joins the club;
// members-only reads refuse non-members (404); a member without the role → 403.
func TestClubAuthz(t *testing.T) {
	c := setupPersistent(t, chatOpts{})
	owner, member := c.student(c.tenantA, c.schoolA), c.student(c.tenantA, c.schoolA)
	other, foreign := c.student(c.tenantA, c.schoolA2), c.student(c.tenantB, c.schoolB)
	clubID, _ := c.mustClub(owner, "CLB Ve")
	c.joinClub(owner, member, clubID)
	id := clubID.String()
	req := c.call(member, http.MethodGet, "/social/clubs/"+id+"/join-requests", "", nil)
	if req.status != 403 {
		t.Fatalf("member lists requests: %d %s", req.status, req.raw)
	}
	reqID, invID := uuid.NewString(), uuid.NewString()
	type call struct {
		op, method, path string
		key              bool
		body             any
	}
	calls := []call{
		{"getClub", http.MethodGet, "/social/clubs/" + id, false, nil},
		{"updateClub", http.MethodPatch, "/social/clubs/" + id, false, map[string]any{"name": "Ten moi"}},
		{"leaveClub", http.MethodPost, "/social/clubs/" + id + ":leave", false, nil},
		{"archiveClub", http.MethodPost, "/social/clubs/" + id + ":archive", true, nil},
		{"listClubMembers", http.MethodGet, "/social/clubs/" + id + "/members", false, nil},
		{"updateClubMemberRole", http.MethodPatch, "/social/clubs/" + id + "/members/" + member.id.String(), false, map[string]any{"role": "officer"}},
		{"removeClubMember", http.MethodDelete, "/social/clubs/" + id + "/members/" + member.id.String(), false, nil},
		{"listClubJoinRequests", http.MethodGet, "/social/clubs/" + id + "/join-requests", false, nil},
		{"requestJoinClub", http.MethodPost, "/social/clubs/" + id + "/join-requests", true, nil},
		{"acceptClubJoinRequest", http.MethodPost, "/social/club-join-requests/" + reqID + ":accept", true, nil},
		{"rejectClubJoinRequest", http.MethodPost, "/social/club-join-requests/" + reqID + ":reject", false, nil},
		{"inviteToClub", http.MethodPost, "/social/clubs/" + id + "/invitations", true, map[string]any{"invitee_ids": []string{owner.id.String()}}},
		{"acceptClubInvitation", http.MethodPost, "/social/club-invitations/" + invID + ":accept", true, nil},
		{"declineClubInvitation", http.MethodPost, "/social/club-invitations/" + invID + ":decline", false, nil},
		{"listClubGoals", http.MethodGet, "/social/clubs/" + id + "/goals", false, nil},
		{"createClubGoal", http.MethodPost, "/social/clubs/" + id + "/goals", true, map[string]any{"metric": "quests_completed", "target": 2}},
		{"listClubEvents", http.MethodGet, "/social/clubs/" + id + "/events", false, nil},
		{"createClubEvent", http.MethodPost, "/social/clubs/" + id + "/events", true, map[string]any{"title": "Su kien", "starts_at": "2026-12-01T08:00:00Z"}},
	}
	headers := func(tenant uuid.UUID, key bool) map[string]string {
		h := map[string]string{"X-Tenant-Id": tenant.String(), "If-Match": "1"}
		if key {
			h["Idempotency-Key"] = newKey()
		}
		return h
	}
	staff := c.staff(c.tenantA, "tenant_admin")
	for _, x := range calls {
		if x.op != "getClub" && x.op != "archiveClub" {
			if r := c.do(x.method, x.path, staff, headers(c.tenantA, x.key), x.body); r.status != 403 || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
				t.Errorf("%s staff: %d %s", x.op, r.status, r.raw)
			}
		}
		if r := c.do(x.method, x.path, owner.token, headers(c.tenantB, x.key), x.body); r.status != 403 || r.code() != "TENANT_MISMATCH" {
			t.Errorf("%s header mismatch: %d %s", x.op, r.status, r.raw)
		}
		if r := c.do(x.method, x.path, foreign.token, headers(foreign.tenant, x.key), x.body); r.status != 404 {
			t.Errorf("%s other tenant: %d %s", x.op, r.status, r.raw)
		}
	}
	// Another school without cross-school social: the club does not exist for it.
	if r := c.call(other, http.MethodGet, "/social/clubs/"+id, "", nil); r.status != 404 {
		t.Fatalf("other school get: %d %s", r.status, r.raw)
	}
	if r := c.call(other, http.MethodGet, "/social/clubs?scope=discoverable", "", nil); r.status != 200 || len(r.data()) != 0 {
		t.Fatalf("other school discoverable: %d %s", r.status, r.raw)
	}
	if r := c.call(other, http.MethodPost, "/social/clubs/"+id+"/join-requests", newKey(), nil); r.status != 404 {
		t.Fatalf("other school join: %d %s", r.status, r.raw)
	}
	same := c.student(c.tenantA, c.schoolA)
	if r := c.call(same, http.MethodGet, "/social/clubs?scope=discoverable", "", nil); r.status != 200 || len(r.data()) != 1 {
		t.Fatalf("same school discoverable: %d %s", r.status, r.raw)
	}
	if r := c.call(same, http.MethodGet, "/social/clubs/"+id+"/members", "", nil); r.status != 404 {
		t.Fatalf("non-member members: %d %s", r.status, r.raw)
	}
	mine := c.call(member, http.MethodGet, "/social/clubs?scope=mine", "", nil)
	if mine.status != 200 || len(mine.data()) != 1 || mine.data()[0]["my_role"] != "member" {
		t.Fatalf("mine: %d %s", mine.status, mine.raw)
	}
	pii(t, c.call(member, http.MethodGet, "/social/clubs/"+id+"/members", "", nil))
	// Staff of another school of the tenant: 403 on getClub/archiveClub; staff list stays inside the grant.
	other2 := c.staffAt(c.tenantA, "school_admin", c.schoolA2)
	if r := c.portal(other2, c.tenantA, http.MethodGet, "/social/clubs/"+id, nil, nil); r.status != 403 {
		t.Fatalf("other school staff get: %d %s", r.status, r.raw)
	}
	if r := c.portal(other2, c.tenantA, http.MethodGet, "/social/clubs", nil, nil); r.status != 200 || len(r.data()) != 0 {
		t.Fatalf("other school staff list: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listClubs", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "approveClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "leaveClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "archiveClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listClubMembers", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateClubMemberRole", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "removeClubMember", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listClubJoinRequests", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "requestJoinClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "acceptClubJoinRequest", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "rejectClubJoinRequest", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "inviteToClub", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "acceptClubInvitation", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "declineClubInvitation", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listClubGoals", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createClubGoal", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listClubEvents", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createClubEvent", authztest.CrossTenant, authztest.RoleDenied)
}
