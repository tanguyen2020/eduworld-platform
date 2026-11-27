package contract

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/contract"
)

func publicSpec(t *testing.T) *openapi3.T {
	t.Helper()
	root, err := contract.Root()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(root, "specs", "openapi", "eduworld-api.v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func enumOf(t *testing.T, doc *openapi3.T, name string) []string {
	t.Helper()
	s := doc.Components.Schemas[name]
	if s == nil || s.Value == nil {
		t.Fatalf("schema %s not in the spec", name)
	}
	var out []string
	for _, v := range s.Value.Enum {
		out = append(out, v.(string))
	}
	return out
}

// Delta F112-4, F112-9: the enums published in the contract are the very lists the server checks.
func TestF112EnumsMatchServer(t *testing.T) {
	doc := publicSpec(t)
	if got := enumOf(t, doc, "ClubGoalAcceptedMetric"); !slices.Equal(got, club.AcceptedMetrics) {
		t.Fatalf("ClubGoalAcceptedMetric %v, server %v", got, club.AcceptedMetrics)
	}
	create := doc.Components.Schemas["ClubGoalCreate"].Value.Properties["metric"].Value
	for _, m := range club.AcceptedMetrics {
		if !slices.Contains(create.Enum, any(m)) {
			t.Errorf("accepted metric %s outside ClubGoalCreate.metric", m)
		}
	}
	if got := enumOf(t, doc, "ChannelNotificationLevel"); !slices.Equal(got, command.NotificationLevels) {
		t.Fatalf("ChannelNotificationLevel %v, server %v", got, command.NotificationLevels)
	}
	kinds := doc.Components.Schemas["ClubEventCreate"].Value.Properties["event_kind"].Value.Enum
	for _, k := range kinds {
		_, err := club.StudentEventKind(k.(string), nil)
		if (k == club.EventTournament) != (err != nil) {
			t.Errorf("event_kind %v: %v", k, err)
		}
	}
}

// Delta F112-8: the searchMessages example of the contract is what the server computes (plain text, code point ranges).
func TestF112SearchHighlightExample(t *testing.T) {
	doc := publicSpec(t)
	op := doc.Paths.Find("/chat/search").Get
	ex := op.Responses.Value("200").Value.Content["application/json"].Example.(map[string]any)
	hit := ex["data"].([]any)[0].(map[string]any)
	text, ranges := message.Highlight(hit["message"].(map[string]any)["body"].(string), "nhan vat", 8)
	if text != hit["highlight"] {
		t.Fatalf("highlight %q, server %q", hit["highlight"], text)
	}
	var want []message.Range
	for _, r := range hit["highlight_ranges"].([]any) {
		m := r.(map[string]any)
		want = append(want, message.Range{Start: int(m["start"].(float64)), Length: int(m["length"].(float64))})
	}
	if !slices.Equal(ranges, want) {
		t.Fatalf("ranges %v, example %v", ranges, want)
	}
}

// Delta F112-7: the payloads module chat now publishes validate against their JSON Schemas (chat_policy.updated with
// direct_message_enabled, the new chat_policy.common_tenant_social_updated).
func TestF112PolicyEventPayloads(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, c := range []struct {
		event   string
		payload map[string]any
	}{
		{"chat_policy.updated", map[string]any{"policy_id": uuid.NewString(), "tenant_id": uuid.NewString(), "scope_type": "school",
			"scope_id": uuid.NewString(), "policy_version": 2, "changed_fields": []string{"rules"}, "direct_message_enabled": false,
			"actor_id": uuid.NewString(), "occurred_at": now}},
		{"chat_policy.updated", map[string]any{"policy_id": uuid.NewString(), "scope_type": "platform", "policy_version": 3,
			"changed_fields": []string{"rules"}, "direct_message_enabled": true, "actor_id": uuid.NewString(), "occurred_at": now}},
		{command.EventCommonOverrideUpdated, map[string]any{"tenant_id": uuid.NewString(), "direct_enabled": true, "world_enabled": false,
			"area_free_text_enabled": false, "version": 1, "actor_id": uuid.NewString(), "occurred_at": now}},
	} {
		raw, _ := json.Marshal(c.payload)
		var v any
		_ = json.Unmarshal(raw, &v)
		if err := contract.CheckPayload("chat", c.event, 1, v); err != nil {
			t.Errorf("%s: %v", c.event, err)
		}
	}
	bad := map[string]any{"tenant_id": uuid.NewString(), "direct_enabled": true, "version": 1, "actor_id": uuid.NewString(), "occurred_at": now,
		"reason": "never in the event"}
	if err := contract.CheckPayload("chat", command.EventCommonOverrideUpdated, 1, bad); err == nil {
		t.Fatal("a payload with a reason (and without the other flags) passed the schema")
	}
}
