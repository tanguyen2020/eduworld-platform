package unit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/notification"
)

// F123-AC-01, F123-AC-05 (T011): every source event of the catalog maps to one kind, message code and deep link; params
// are a whitelist (a nickname, a content title, a level/tier) — a real name, school, class or score key is refused;
// values are sanitized and bounded (≤ 255); created_at is the occurred_at of the event (idempotent row, F123-AC-03).
func TestNotificationCatalog(t *testing.T) {
	want := map[string][3]string{
		"friend_request.sent":         {"friend_request", "friend.request_received", "open_friend_requests"},
		"party.invitation_sent":       {"party_invitation", "party.invitation_received", "open_party_invitation"},
		"assignment.published":        {"assignment_new", "assignment.new", "open_assignment"},
		"assignment.results_released": {"assignment_results_released", "assignment.results_released", "open_assignment_result"},
		"level.reached":               {"level_reached", "progression.level_reached", "open_profile"},
		"achievement.unlocked":        {"achievement_unlocked", "achievement.unlocked", "open_achievements"},
		"report.resolved":             {"moderation_notice", "moderation.report_handled", ""},
	}
	if len(notification.MessageCatalog) != len(want) {
		t.Fatalf("catalog has %d entries", len(notification.MessageCatalog))
	}
	tenant, student, ref, ev := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2026, 11, 21, 8, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	for event, w := range want {
		tp := notification.MessageCatalog[event]
		if string(tp.Kind) != w[0] || tp.MessageCode != w[1] || tp.Action != w[2] {
			t.Errorf("%s: %+v", event, tp)
		}
		if !slices.Contains(notification.Kinds, tp.Kind) {
			t.Errorf("%s: kind %s outside the enum", event, tp.Kind)
		}
		for _, p := range tp.Params {
			if !slices.Contains([]string{"nickname", "title", "level", "tier"}, p) {
				t.Errorf("%s: param %s is not a nickname, a content name or an id", event, p)
			}
		}
		d, err := notification.Build(notification.Source{EventID: ev, EventType: event, TenantID: tenant, OccurredAt: at, RefID: &ref}, student, nil)
		if err != nil {
			t.Fatalf("%s: %v", event, err)
		}
		if d.CreatedAt != at.UTC() || d.SourceEventID != ev || d.RecipientID != student || d.TenantID != tenant || *d.RefID != ref {
			t.Errorf("%s: draft %+v", event, d)
		}
		if (w[2] == "") != (d.Action == nil) || (d.Action != nil && (d.Action.Kind != w[2] || *d.Action.ID != ref)) {
			t.Errorf("%s: action %+v", event, d.Action)
		}
	}
	// F123-AC-05: keys outside the whitelist are refused — never a real name, school, class, chat text or score.
	for _, bad := range []string{"full_name", "school_name", "class_name", "score", "message", "sanction"} {
		_, err := notification.Build(notification.Source{EventType: "friend_request.sent", TenantID: tenant, OccurredAt: at,
			Params: map[string]string{"nickname": "SaoBang12", bad: "x"}}, student, nil)
		if !errors.Is(err, notification.ErrParam) {
			t.Errorf("param %s accepted: %v", bad, err)
		}
	}
	d, err := notification.Build(notification.Source{EventType: "friend_request.sent", TenantID: tenant, OccurredAt: at,
		Params: map[string]string{"nickname": " Sao\x00Băng\n12 "}}, student, nil)
	if err != nil || d.Params["nickname"] != "SaoBăng12" {
		t.Fatalf("sanitize: %v %q", err, d.Params["nickname"])
	}
	long, _ := notification.Build(notification.Source{EventType: "assignment.published", TenantID: tenant, OccurredAt: at,
		Params: map[string]string{"title": strings.Repeat("ă", 400)}}, student, nil)
	if n := len([]rune(long.Params["title"])); n != notification.MaxParamLen {
		t.Fatalf("title length %d", n)
	}
	if _, err := notification.Build(notification.Source{EventType: "chat.message_created", TenantID: tenant}, student, nil); !errors.Is(err, notification.ErrUnknownSource) {
		t.Fatalf("unknown source: %v", err)
	}
}

// F123-AC-06 (T011): RecipientPolicy — only an active, not removed student of the tenant of the event.
func TestNotificationRecipientPolicy(t *testing.T) {
	tenant := uuid.New()
	ok := notification.Recipient{ID: uuid.New(), TenantID: tenant, Status: "active"}
	if !notification.Eligible(&ok, tenant) {
		t.Fatal("active student refused")
	}
	other := ok
	other.TenantID = uuid.New()
	locked := ok
	locked.Status = "locked"
	removed := ok
	removed.Deleted = true
	for name, r := range map[string]*notification.Recipient{"unknown": nil, "other tenant": &other, "locked": &locked, "cleaned up": &removed} {
		if notification.Eligible(r, tenant) {
			t.Errorf("%s accepted", name)
		}
	}
	if notification.Eligible(&ok, uuid.Nil) {
		t.Error("no tenant accepted")
	}
}

// F123-AC-09 (domain): read once; mark_read ids bounded to 100 and deduplicated.
func TestNotificationReadOnce(t *testing.T) {
	n := notification.Notification{ID: uuid.New()}
	first := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	if !n.MarkRead(first) || !n.Read() {
		t.Fatal("first read")
	}
	if n.MarkRead(first.Add(time.Hour)) || !n.ReadAt.Equal(first) {
		t.Fatalf("second read changed read_at: %v", n.ReadAt)
	}
	id := uuid.New()
	ids, err := notification.MarkReadIDs([]uuid.UUID{id, id, uuid.Nil})
	if err != nil || len(ids) != 1 {
		t.Fatalf("dedupe: %v %v", ids, err)
	}
	if _, err := notification.MarkReadIDs(make([]uuid.UUID, 101)); err == nil {
		t.Fatal("101 ids accepted")
	}
	if _, err := notification.MarkReadIDs(nil); err == nil {
		t.Fatal("no id accepted")
	}
}

// One NotificationKind enum (RB-513): the domain list equals specs/schemas/common/enums.schema.json (same order).
func TestNotificationKindEnum(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "specs", "schemas", "common", "enums.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(notification.Kinds))
	for i, k := range notification.Kinds {
		got[i] = string(k)
	}
	if !slices.Equal(got, doc.Defs["NotificationKind"].Enum) {
		t.Fatalf("kinds %v, enums.schema.json %v", got, doc.Defs["NotificationKind"].Enum)
	}
}
