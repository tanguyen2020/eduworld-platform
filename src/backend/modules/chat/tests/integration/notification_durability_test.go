//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// T020 / F123-AC-12 (R9, R21, ADR-0070): the application roles cannot DELETE or TRUNCATE social.notifications (soft-delete
// only); the read state lives in PostgreSQL — when Valkey loses the unread counter, listNotifications counts it again
// from PostgreSQL and caches it.
func TestNotificationDurability(t *testing.T) {
	n := setupNotify(t, notifyOpts{})
	ctx := context.Background()
	an := n.student(n.tenantA, n.schoolA)
	first := n.notify(an, 2)
	n.notify(an, 3)
	n.notify(an, 4)

	for role, pool := range map[string]*struct{ q func(string) error }{
		"chat_app":      {q: func(q string) error { _, err := n.app.ExecContext(ctx, q); return err }},
		"chat_platform": {q: func(q string) error { _, err := n.platform.ExecContext(ctx, q); return err }},
	} {
		for _, q := range []string{"DELETE FROM social.notifications", "TRUNCATE social.notifications", "DELETE FROM social.notifications_default"} {
			if err := pool.q(q); err == nil || !strings.Contains(err.Error(), "permission denied") {
				t.Fatalf("%s: %q → %v", role, q, err)
			}
		}
	}
	if got := n.notifications(an.id); got != 3 {
		t.Fatalf("rows after refused deletes: %d", got)
	}

	// Unread counter: cached after the first list, lost with Valkey, recounted from PostgreSQL.
	if got := unreadCount(n.call(an, http.MethodGet, "/notifications", "", nil)); got != 3 {
		t.Fatalf("unread: %d", got)
	}
	if c, ok, err := n.unread.Get(ctx, an.id); err != nil || !ok || c != 3 {
		t.Fatalf("cached unread: %d %v %v", c, ok, err)
	}
	if err := n.vk.Do(ctx, n.vk.B().Flushall().Build()).Error(); err != nil {
		t.Fatal(err)
	}
	if r := n.call(an, http.MethodPost, "/notifications/"+first.String()+":read", "", nil); r.status != http.StatusOK {
		t.Fatalf("mark read without cache: %d %s", r.status, r.raw)
	}
	if got := unreadCount(n.call(an, http.MethodGet, "/notifications?unread=true", "", nil)); got != 2 {
		t.Fatalf("unread recounted after the Valkey loss: %d", got)
	}
	if n.count(`SELECT count(*) FROM social.notifications WHERE recipient_id = $1 AND read_at IS NOT NULL`, an.id) != 1 {
		t.Fatal("read state not in PostgreSQL")
	}
}
