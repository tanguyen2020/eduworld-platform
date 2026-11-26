//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
)

// notify stores one notification of kind level_reached for s through the command (as a consumer would) and returns its
// id; each call is a new source event one second later than the previous one.
func (n *notifyEnv) notify(s student, level int) uuid.UUID {
	n.t.Helper()
	n.clock.Add(time.Second)
	src := notification.Source{EventID: uuid.New(), EventType: notification.EventLevelReached, TenantID: s.tenant,
		OccurredAt: n.clock.Now(), Params: map[string]string{"level": strconv.Itoa(level)}}
	if err := n.uow.InTenantTx(context.Background(), eventScope(n.t, s.tenant), func(ctx context.Context) error {
		_, err := n.cmd.FromEvent(ctx, src, []uuid.UUID{s.id}, nil)
		return err
	}); err != nil {
		n.t.Fatal(err)
	}
	var id uuid.UUID
	if err := n.su.QueryRow(`SELECT notification_id FROM social.notifications WHERE source_event_id = $1`, src.EventID).Scan(&id); err != nil {
		n.t.Fatal(err)
	}
	return id
}

func unreadCount(r resp) int {
	f, _ := r.body["unread_count"].(float64)
	return int(f)
}

// T018 / F123-AC-08, F123-AC-09, F123-AC-10: listNotifications returns the caller's notifications only, newest first,
// with unread_count and a cursor; unread=true keeps the unread ones; markNotificationRead sets read_at once (a second call
// changes nothing) and the id of another student is 404; the WS mark_read (≤ 100 ids) ignores ids of anyone else;
// markAllNotificationsRead reads every unread notification created before the call and returns unread_count 0.
func TestNotificationInbox(t *testing.T) {
	n := setupNotify(t, notifyOpts{})
	an := n.student(n.tenantA, n.schoolA)
	binh := n.student(n.tenantA, n.schoolA)
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = n.notify(an, i+2)
	}
	other := n.notify(binh, 9)

	list := n.call(an, http.MethodGet, "/notifications?limit=3", "", nil)
	if list.status != http.StatusOK || len(list.data()) != 3 || unreadCount(list) != 5 {
		t.Fatalf("first page: %d %s", list.status, list.raw)
	}
	if list.data()[0]["notification_id"] != ids[4].String() || list.data()[2]["notification_id"] != ids[2].String() {
		t.Fatalf("newest first: %s", list.raw)
	}
	page, _ := list.body["page"].(map[string]any)
	cursor, _ := page["next_cursor"].(string)
	if page["has_more"] != true || cursor == "" {
		t.Fatalf("page: %v", page)
	}
	rest := n.call(an, http.MethodGet, "/notifications?limit=3&cursor="+cursor, "", nil)
	if len(rest.data()) != 2 || rest.data()[1]["notification_id"] != ids[0].String() {
		t.Fatalf("second page: %s", rest.raw)
	}
	for _, it := range append(list.data(), rest.data()...) {
		if it["notification_id"] == other.String() {
			t.Fatal("another student's notification listed")
		}
		if it["kind"] != "level_reached" || it["read"] != false || it["message_code"] != "progression.level_reached" {
			t.Fatalf("item %v", it)
		}
	}

	// AC-09 REST: read once.
	r1 := n.call(an, http.MethodPost, "/notifications/"+ids[0].String()+":read", "", nil)
	if r1.status != http.StatusOK || r1.body["read"] != true || r1.body["notification_id"] != ids[0].String() {
		t.Fatalf("mark read: %d %s", r1.status, r1.raw)
	}
	var first time.Time
	_ = n.su.QueryRow(`SELECT read_at FROM social.notifications WHERE notification_id = $1`, ids[0]).Scan(&first)
	n.clock.Add(time.Minute)
	if r2 := n.call(an, http.MethodPost, "/notifications/"+ids[0].String()+":read", "", nil); r2.status != http.StatusOK || r2.body["read"] != true {
		t.Fatalf("mark read again: %d %s", r2.status, r2.raw)
	}
	var again time.Time
	_ = n.su.QueryRow(`SELECT read_at FROM social.notifications WHERE notification_id = $1`, ids[0]).Scan(&again)
	if !again.Equal(first) {
		t.Fatalf("read_at changed: %v → %v", first, again)
	}
	// Another student's notification, an unknown id: 404, nothing changed.
	for _, id := range []uuid.UUID{other, uuid.New()} {
		if r := n.call(an, http.MethodPost, "/notifications/"+id.String()+":read", "", nil); r.status != http.StatusNotFound || r.code() != "NOT_FOUND" {
			t.Fatalf("foreign id: %d %s", r.status, r.raw)
		}
	}
	if n.count(`SELECT count(*) FROM social.notifications WHERE notification_id = $1 AND read_at IS NULL`, other) != 1 {
		t.Fatal("foreign notification read")
	}
	if got := unreadCount(n.call(an, http.MethodGet, "/notifications?unread=true", "", nil)); got != 4 {
		t.Fatalf("unread after one read: %d", got)
	}

	// AC-09 WS: ≤ 100 ids, foreign ids ignored, result = number read now.
	reply := n.ws(an, "c-1", "notification.item.mark_read", map[string]any{"notification_ids": []uuid.UUID{ids[1], ids[2], other}})
	if reply.Problem != nil {
		t.Fatalf("mark_read: %+v", reply.Problem)
	}
	var res map[string]any
	if err := json.Unmarshal(reply.Result, &res); err != nil || res["read"] != float64(2) {
		t.Fatalf("mark_read result %s", reply.Result)
	}
	if n.count(`SELECT count(*) FROM social.notifications WHERE notification_id = $1 AND read_at IS NULL`, other) != 1 {
		t.Fatal("WS read a foreign notification")
	}
	tooMany := make([]uuid.UUID, 101)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	if code := replyCode(n.ws(an, "c-1", "notification.item.mark_read", map[string]any{"notification_ids": tooMany})); code != "VALIDATION_FAILED" {
		t.Fatalf("101 ids: %s", code)
	}
	unread := n.call(an, http.MethodGet, "/notifications?unread=true", "", nil)
	if unreadCount(unread) != 2 || len(unread.data()) != 2 {
		t.Fatalf("unread list: %s", unread.raw)
	}

	// AC-10: read all before the call; a notification created after stays unread.
	all := n.call(an, http.MethodPost, "/notifications:read-all", "", nil)
	if all.status != http.StatusOK || unreadCount(all) != 0 {
		t.Fatalf("read all: %d %s", all.status, all.raw)
	}
	if got := unreadCount(n.call(an, http.MethodGet, "/notifications", "", nil)); got != 0 {
		t.Fatalf("unread after read-all: %d", got)
	}
	n.notify(an, 12)
	if got := unreadCount(n.call(an, http.MethodGet, "/notifications", "", nil)); got != 1 {
		t.Fatalf("new notification after read-all: %d", got)
	}
	if n.count(`SELECT count(*) FROM social.notifications WHERE notification_id = $1 AND read_at IS NULL`, other) != 1 {
		t.Fatal("read-all touched another student")
	}
}

// T019 / F123-AC-11 (TC-TEN-001, TC-TEN-011): no token → 401; without X-Tenant-Id → 400 TENANT_HEADER_REQUIRED; with the
// header of another tenant → 403 TENANT_MISMATCH; ids of tenant B from tenant A → 404 and never listed; a staff member
// (portal token) sees an empty inbox (F123 only notifies students).
func TestNotificationAuthz(t *testing.T) {
	n := setupNotify(t, notifyOpts{})
	an := n.student(n.tenantA, n.schoolA)
	dung := n.student(n.tenantB, n.schoolB)
	mine := n.notify(an, 3)
	theirs := n.notify(dung, 4)
	authztest.Cover(t, "listNotifications", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "markNotificationRead", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "markAllNotificationsRead", authztest.CrossTenant, authztest.RoleDenied)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/notifications"},
		{http.MethodPost, "/notifications/" + mine.String() + ":read"},
		{http.MethodPost, "/notifications:read-all"},
	}
	for _, ro := range routes {
		if r := n.do(ro.method, ro.path, "", map[string]string{"X-Tenant-Id": an.tenant.String()}, nil); r.status != http.StatusUnauthorized {
			t.Fatalf("%s %s without token: %d %s", ro.method, ro.path, r.status, r.raw)
		}
		if r := n.do(ro.method, ro.path, an.token, nil, nil); r.status != http.StatusBadRequest || r.code() != "TENANT_HEADER_REQUIRED" {
			t.Fatalf("%s %s without X-Tenant-Id: %d %s", ro.method, ro.path, r.status, r.raw)
		}
		// RoleDenied for an own-data route: the caller acting in a tenant that is not its session tenant.
		if r := n.do(ro.method, ro.path, an.token, map[string]string{"X-Tenant-Id": n.tenantB.String()}, nil); r.status != http.StatusForbidden ||
			r.code() != "TENANT_MISMATCH" {
			t.Fatalf("%s %s with the tenant B header: %d %s", ro.method, ro.path, r.status, r.raw)
		}
	}
	// CrossTenant: the notification of tenant B from tenant A → 404, untouched, never listed.
	if r := n.call(an, http.MethodPost, "/notifications/"+theirs.String()+":read", "", nil); r.status != http.StatusNotFound {
		t.Fatalf("tenant B id: %d %s", r.status, r.raw)
	}
	if n.count(`SELECT count(*) FROM social.notifications WHERE notification_id = $1 AND read_at IS NULL`, theirs) != 1 {
		t.Fatal("tenant B notification read from tenant A")
	}
	if all := n.call(an, http.MethodPost, "/notifications:read-all", "", nil); all.status != http.StatusOK {
		t.Fatalf("read-all: %d", all.status)
	}
	if n.count(`SELECT count(*) FROM social.notifications WHERE notification_id = $1 AND read_at IS NULL`, theirs) != 1 {
		t.Fatal("read-all of tenant A touched tenant B")
	}
	for _, it := range n.call(an, http.MethodGet, "/notifications", "", nil).data() {
		if it["notification_id"] == theirs.String() {
			t.Fatal("tenant B listed in tenant A")
		}
	}
	// Staff of tenant A: authenticated, empty inbox, cannot read a student's notification.
	staff := n.staff(n.tenantA, "school_admin")
	h := map[string]string{"X-Tenant-Id": n.tenantA.String()}
	if r := n.do(http.MethodGet, "/notifications", staff, h, nil); r.status != http.StatusOK || len(r.data()) != 0 || unreadCount(r) != 0 {
		t.Fatalf("staff inbox: %d %s", r.status, r.raw)
	}
	if r := n.do(http.MethodPost, "/notifications/"+mine.String()+":read", staff, h, nil); r.status != http.StatusNotFound {
		t.Fatalf("staff marks a student's notification: %d %s", r.status, r.raw)
	}
}
