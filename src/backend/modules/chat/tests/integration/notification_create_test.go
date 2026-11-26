//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
)

// notificationKeys are the only payload keys of notification.item.created (AsyncAPI, F123-AC-05).
var notificationKeys = []string{"notification_id", "kind", "title", "body", "message_code", "params", "ref_type", "ref_id", "action", "created_at"}

// T012 / F123-AC-01, F123-AC-03, F123-AC-04, F123-AC-05 (TC-SVC-003): friend_request.sent (F095, from the real send
// use case), party.invitation_sent, level.reached, achievement.unlocked and report.resolved (F121-AC-16) each create one
// notification for the student concerned, stored before the push of notification.item.created on ew:rt:user:<id> (not
// droppable); the same event delivered again creates nothing more (processed_events, and the unique key when the
// command runs twice); params carry a nickname only — no real name, school, class.
func TestNotificationCreate(t *testing.T) {
	n := setupNotify(t, notifyOpts{})
	ctx := context.Background()
	an := n.student(n.tenantA, n.schoolA)
	binh := n.student(n.tenantA, n.schoolA)
	box := inbox(t, n.vk, binh.id)

	// friend_request.sent of the real F095 use case (outbox of module chat), relayed as the relay does.
	n.env.send(an, binh, http.StatusCreated)
	sent := n.events("friend_request.sent")
	if len(sent) != 1 {
		t.Fatalf("friend_request.sent: %d", len(sent))
	}
	requestID := uuid.MustParse(sent[0]["request_id"].(string))
	ev := uuid.New()
	n.publish("chat", "friend_request.sent", n.tenantA, ev, sent[0])
	n.waitNotifications(binh.id, 1)
	env := next(box, "notification.item.created", 5*time.Second)
	if env == nil {
		t.Fatal("no notification.item.created on ew:rt:user:<id>")
	}
	if env.Droppable {
		t.Fatal("notification.item.created must not be droppable (resume buffer, F123-AC-04)")
	}
	p := payloadOf(env)
	for k := range p {
		if !slices.Contains(notificationKeys, k) {
			t.Errorf("unexpected key %s in %v", k, p)
		}
	}
	params, _ := p["params"].(map[string]any)
	action, _ := p["action"].(map[string]any)
	if p["kind"] != "friend_request" || p["message_code"] != "friend.request_received" || p["ref_type"] != "friend_request" ||
		p["ref_id"] != requestID.String() || params["nickname"] != an.nickname || len(params) != 1 ||
		action["kind"] != "open_friend_requests" || action["id"] != requestID.String() {
		t.Fatalf("payload %v", p)
	}
	// Stored before the push: the row exists with the pushed id.
	if n.count(`SELECT count(*) FROM social.notifications WHERE notification_id = $1 AND tenant_id = $2 AND read_at IS NULL`,
		p["notification_id"], n.tenantA) != 1 {
		t.Fatal("pushed notification not stored")
	}
	// AC-03: the same event again (a new NATS message with the same event_id) → nothing more, no second push.
	n.publish("chat", "friend_request.sent", n.tenantA, ev, sent[0])
	time.Sleep(time.Second)
	if got := n.notifications(binh.id); got != 1 {
		t.Fatalf("duplicate notification: %d", got)
	}
	if extra := next(box, "notification.item.created", 500*time.Millisecond); extra != nil {
		t.Fatalf("second push %v", payloadOf(extra))
	}
	// AC-03 below processed_events: the command run twice for the same source creates one row (unique key).
	src := notification.Source{EventID: uuid.New(), EventType: notification.EventLevelReached, TenantID: n.tenantA,
		OccurredAt: n.clock.Now(), Params: map[string]string{"level": "5"}}
	for range 2 {
		sc := eventScope(t, n.tenantA)
		if err := n.uow.InTenantTx(ctx, sc, func(ctx context.Context) error {
			_, err := n.cmd.FromEvent(ctx, src, []uuid.UUID{an.id}, nil)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got := n.notifications(an.id); got != 1 {
		t.Fatalf("command twice: %d rows", got)
	}

	// party.invitation_sent (competition, F100): the invitee gets the nickname of the inviter.
	inv, party := uuid.New(), uuid.New()
	n.publish("competition", "party.invitation_sent", n.tenantA, uuid.New(), map[string]any{"invitation_id": inv, "party_id": party,
		"tenant_id": n.tenantA, "inviter_id": an.id, "invitee_id": binh.id, "expires_at": n.clock.Now().Add(time.Minute)})
	n.waitNotifications(binh.id, 2)
	pi := next(box, "notification.item.created", 5*time.Second)
	if pi == nil || payloadOf(pi)["kind"] != "party_invitation" || payloadOf(pi)["ref_id"] != inv.String() {
		t.Fatalf("party invitation push: %v", pi)
	}

	// level.reached / achievement.unlocked (game, F074/F075: student from the payload or the envelope subject).
	n.publish("game", "level.reached", n.tenantA, uuid.New(), map[string]any{"student_id": binh.id, "from_level": 4, "to_level": 5,
		"curve_version": 1, "title_code": "explorer"})
	ach := uuid.New()
	n.publish("game", "achievement.unlocked", n.tenantA, uuid.New(), map[string]any{"student_id": binh.id, "achievement_id": ach, "tier": "gold"})
	n.waitNotifications(binh.id, 4)

	// report.resolved (F121-AC-16): moderation_notice without any detail of the sanction or of the reported student.
	report := uuid.New()
	n.publish("chat", "report.resolved", n.tenantA, uuid.New(), map[string]any{"tenant_id": n.tenantA, "report_id": report,
		"case_id": uuid.New(), "reporter_id": binh.id, "kind": "moderation_notice", "message_code": "moderation.report_handled",
		"resolved_at": n.clock.Now()})
	n.waitNotifications(binh.id, 5)

	rows, err := n.su.QueryContext(ctx, `SELECT kind, message_code, params::text, coalesce(action::text, ''), ref_type
		FROM social.notifications WHERE recipient_id = $1 ORDER BY kind`, binh.id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][4]string{}
	for rows.Next() {
		var kind, code, params, action, ref string
		if err := rows.Scan(&kind, &code, &params, &action, &ref); err != nil {
			t.Fatal(err)
		}
		got[kind] = [4]string{code, params, action, ref}
		// F123-AC-05: nothing but a nickname, a level or a tier — no real name, school, class, chat text, score, sanction.
		for _, bad := range []string{an.sub, n.schoolA.String(), "score", "sanction", "full_name", "class"} {
			if strings.Contains(params, bad) {
				t.Errorf("%s params %s contain %s", kind, params, bad)
			}
		}
	}
	want := map[string]string{"friend_request": "friend.request_received", "party_invitation": "party.invitation_received",
		"level_reached": "progression.level_reached", "achievement_unlocked": "achievement.unlocked", "moderation_notice": "moderation.report_handled"}
	for kind, code := range want {
		if got[kind][0] != code {
			t.Errorf("%s: %v", kind, got[kind])
		}
	}
	if got["moderation_notice"][1] != "{}" || got["moderation_notice"][2] != "" || got["moderation_notice"][3] != "report" {
		t.Errorf("moderation notice carries details: %v", got["moderation_notice"])
	}
	if !strings.Contains(got["level_reached"][1], `"level": "5"`) || !strings.Contains(got["achievement_unlocked"][1], `"tier": "gold"`) {
		t.Errorf("progression params: %v %v", got["level_reached"], got["achievement_unlocked"])
	}
	// Unread cache follows the inserts (5 for Bình).
	if c, ok, _ := n.unread.Get(ctx, binh.id); ok && c != 5 {
		t.Fatalf("unread cache %d", c)
	}
}

// T012 / F123-AC-06 (TC-TEN-001): an event of tenant B naming a student of tenant A, a locked student and an unknown id
// create nothing; the event is consumed (no retry, no dead letter).
func TestNotificationRecipientPolicy(t *testing.T) {
	n := setupNotify(t, notifyOpts{})
	an := n.student(n.tenantA, n.schoolA)
	dung := n.student(n.tenantB, n.schoolB)
	locked := n.student(n.tenantB, n.schoolB)
	n.exec(`UPDATE ref.students SET status = 'locked' WHERE student_id = $1`, locked.id)

	cross := uuid.New()
	n.publish("competition", "party.invitation_sent", n.tenantB, cross, map[string]any{"invitation_id": uuid.New(), "party_id": uuid.New(),
		"tenant_id": n.tenantB, "inviter_id": dung.id, "invitee_id": an.id, "expires_at": n.clock.Now().Add(time.Minute)})
	lockedEv := uuid.New()
	n.publish("game", "level.reached", n.tenantB, lockedEv, map[string]any{"student_id": locked.id, "from_level": 1, "to_level": 2})
	unknown := uuid.New()
	n.publish("game", "level.reached", n.tenantB, unknown, map[string]any{"student_id": uuid.New(), "from_level": 1, "to_level": 2})
	n.waitProcessed("chat.notify_party_invitation", cross)
	n.waitProcessed("chat.notify_progression", lockedEv)
	n.waitProcessed("chat.notify_progression", unknown)
	if got := n.count(`SELECT count(*) FROM social.notifications`); got != 0 {
		t.Fatalf("notifications across tenants or to an inactive student: %d", got)
	}
	if dl := n.count(`SELECT count(*) FROM infra.dead_letters WHERE consumer LIKE 'chat.notify_%'`); dl != 0 {
		t.Fatalf("dead letters: %d", dl)
	}
	// A payload tenant_id that differs from the envelope is refused (dead letter, nothing stored).
	bad := uuid.New()
	n.publish("chat", "friend_request.sent", n.tenantB, bad, map[string]any{"request_id": uuid.New(), "tenant_id": n.tenantA,
		"requester_id": dung.id, "addressee_id": an.id, "source": "code", "expires_at": n.clock.Now().Add(time.Hour)})
	n.waitCount(1, 10*time.Second, `SELECT count(*) FROM infra.dead_letters WHERE consumer = 'chat.notify_friend_request' AND event_id = $1`, bad)
	if got := n.notifications(an.id); got != 0 {
		t.Fatalf("mismatched tenant stored: %d", got)
	}
}
