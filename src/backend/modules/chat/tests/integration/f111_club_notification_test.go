//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Delta F120/F123 for the client F111 (owner chốt 2026-11-28 [Confirmed]): the club events of module chat become club
// notifications through chat.notify_club — club.invitation_sent → club_invitation for the invitee; club.join_requested →
// club_join_request for the active owner and officers who are students (never the requester, a member or a staff
// owner); club.approved by school staff → club_approved for the student owner (a club created active notifies nobody).
// Every one opens the club ({kind: open_club, id: club_id}) and carries a nickname at most (R16).
func TestF111ClubNotifications(t *testing.T) {
	n := setupNotify(t, notifyOpts{})
	owner, officer, member := n.student(n.tenantA, n.schoolA), n.student(n.tenantA, n.schoolA), n.student(n.tenantA, n.schoolA)
	requester, invitee := n.student(n.tenantA, n.schoolA), n.student(n.tenantA, n.schoolA)
	clubID, staffOwned := uuid.New(), uuid.New()
	now := n.clock.Now()
	n.exec(`INSERT INTO social.clubs (club_id, tenant_id, school_id, club_type, name, owner_id, created_by_kind, status, max_members)
		VALUES ($1, $2, $3, 'interest_group', 'CLB Kham pha', $4, 'student', 'active', 50),
		       ($5, $2, $3, 'school_club', 'CLB Toan', $6, 'staff', 'active', 50)`, clubID, n.tenantA, n.schoolA, owner.id, staffOwned, uuid.New())
	for _, m := range []struct {
		club uuid.UUID
		user uuid.UUID
		role string
	}{{clubID, owner.id, "owner"}, {clubID, officer.id, "officer"}, {clubID, member.id, "member"}, {staffOwned, officer.id, "officer"}} {
		n.exec(`INSERT INTO social.club_members (club_member_id, tenant_id, club_id, user_id, school_id, role, joined_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, uuid.New(), n.tenantA, m.club, m.user, n.schoolA, m.role, now)
	}
	at := now.Format(time.RFC3339Nano)
	kindOf := func(recipient uuid.UUID, kind string) (int, string, string) {
		var c int
		var action, params string
		_ = n.su.QueryRow(`SELECT count(*), coalesce(max(action::text), ''), coalesce(max(params::text), '') FROM social.notifications
			WHERE recipient_id = $1 AND kind = $2`, recipient, kind).Scan(&c, &action, &params)
		return c, action, params
	}
	wantAction := `{"id": "` + clubID.String() + `", "kind": "open_club"}`

	n.publish("chat", "club.invitation_sent", n.tenantA, uuid.New(), map[string]any{"invitation_id": uuid.NewString(), "club_id": clubID,
		"tenant_id": n.tenantA, "inviter_id": owner.id, "invitee_id": invitee.id, "expires_at": at, "occurred_at": at})
	n.waitNotifications(invitee.id, 1)
	if c, action, params := kindOf(invitee.id, "club_invitation"); c != 1 || action != wantAction || params != `{"nickname": "`+owner.nickname+`"}` {
		t.Fatalf("club_invitation: %d %s %s", c, action, params)
	}

	n.publish("chat", "club.join_requested", n.tenantA, uuid.New(), map[string]any{"request_id": uuid.NewString(), "club_id": clubID,
		"tenant_id": n.tenantA, "requester_id": requester.id, "occurred_at": at})
	n.waitNotifications(owner.id, 1)
	n.waitNotifications(officer.id, 1)
	for name, s := range map[string]uuid.UUID{"owner": owner.id, "officer": officer.id} {
		if c, action, params := kindOf(s, "club_join_request"); c != 1 || action != wantAction || params != `{"nickname": "`+requester.nickname+`"}` {
			t.Errorf("%s club_join_request: %d %s %s", name, c, action, params)
		}
	}
	// A join request to the club of a staff owner reaches its student officer only.
	n.publish("chat", "club.join_requested", n.tenantA, uuid.New(), map[string]any{"request_id": uuid.NewString(), "club_id": staffOwned,
		"tenant_id": n.tenantA, "requester_id": requester.id, "occurred_at": at})
	n.waitNotifications(officer.id, 2)

	// club.approved: by staff → the student owner; created active (no approver) → nobody.
	n.publish("chat", "club.approved", n.tenantA, uuid.New(), map[string]any{"club_id": clubID, "tenant_id": n.tenantA,
		"club_type": "interest_group", "owner_id": owner.id, "school_id": n.schoolA, "chat_channel_id": uuid.NewString(),
		"approver_id": uuid.NewString(), "occurred_at": at})
	n.waitNotifications(owner.id, 2)
	if c, action, _ := kindOf(owner.id, "club_approved"); c != 1 || action != wantAction {
		t.Fatalf("club_approved: %d %s", c, action)
	}
	ev := uuid.New()
	n.publish("chat", "club.approved", n.tenantA, ev, map[string]any{"club_id": clubID, "tenant_id": n.tenantA, "club_type": "interest_group",
		"owner_id": member.id, "school_id": n.schoolA, "chat_channel_id": uuid.NewString(), "occurred_at": at})
	n.waitProcessed("chat.notify_club", ev)
	for name, s := range map[string]uuid.UUID{"member": member.id, "requester": requester.id} {
		if got := n.notifications(s); got != 0 {
			t.Errorf("%s got %d notifications", name, got)
		}
	}
	// Another tenant never receives the club notifications of tenant A (RecipientPolicy).
	foreign := n.student(n.tenantB, n.schoolB)
	ev = uuid.New()
	n.publish("chat", "club.invitation_sent", n.tenantA, ev, map[string]any{"invitation_id": uuid.NewString(), "club_id": clubID,
		"tenant_id": n.tenantA, "inviter_id": owner.id, "invitee_id": foreign.id, "expires_at": at, "occurred_at": at})
	n.waitProcessed("chat.notify_club", ev)
	if got := n.notifications(foreign.id); got != 0 {
		t.Fatalf("tenant B got %d", got)
	}
}
