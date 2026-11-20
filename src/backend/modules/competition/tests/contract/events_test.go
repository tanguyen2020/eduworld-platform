package contract

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// fakeRefs is a TEST DOUBLE of port.ProjectionWriter (R17): it records which projection each event reached.
type fakeRefs struct{ calls []string }

func (f *fakeRefs) OwnerTenant(context.Context, string, uuid.UUID) (*uuid.UUID, error) {
	return nil, nil
}
func (f *fakeRefs) Tenant(context.Context, uuid.UUID, *string, string, string, *bool, []byte, int64) (bool, error) {
	f.calls = append(f.calls, "tenant")
	return true, nil
}
func (f *fakeRefs) Student(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *int, *string, int64) (bool, error) {
	f.calls = append(f.calls, "student")
	return true, nil
}
func (f *fakeRefs) StudentStatus(context.Context, uuid.UUID, *uuid.UUID, string) (bool, error) {
	f.calls = append(f.calls, "status")
	return true, nil
}
func (f *fakeRefs) StudentDeleted(context.Context, uuid.UUID, uuid.UUID, bool, time.Time) (bool, error) {
	f.calls = append(f.calls, "deleted")
	return true, nil
}
func (f *fakeRefs) PlayerProfile(context.Context, uuid.UUID, uuid.UUID, *string, int, []byte, int64) (bool, error) {
	f.calls = append(f.calls, "profile")
	return true, nil
}

func (f *fakeRefs) Friendship(_ context.Context, _, _, _ uuid.UUID, active bool, _ int64) (bool, error) {
	f.calls = append(f.calls, map[bool]string{true: "friend+", false: "friend-"}[active])
	return true, nil
}
func (f *fakeRefs) Block(_ context.Context, _, _, _ uuid.UUID, active bool, _ int64) (bool, error) {
	f.calls = append(f.calls, map[bool]string{true: "block+", false: "block-"}[active])
	return true, nil
}

// fakeHub is a TEST DOUBLE of the hub side of the consumers (R17).
type fakeHub struct{ removed []uuid.UUID }

func (h *fakeHub) ProfileChanged(context.Context, uuid.UUID, hub.PublicPlayer) error { return nil }
func (h *fakeHub) RemoveStudent(_ context.Context, user uuid.UUID, _ string) error {
	h.removed = append(h.removed, user)
	return nil
}

// example returns the first example of an event payload schema (owner modules) and its tenant.
func example(t *testing.T, producer, event string) (json.RawMessage, *uuid.UUID) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "specs", "events", "payloads", producer, event+".v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Examples []json.RawMessage `json:"examples"`
	}
	if err := json.Unmarshal(b, &s); err != nil || len(s.Examples) == 0 {
		t.Fatalf("%s: no example", event)
	}
	var p struct {
		TenantID *uuid.UUID `json:"tenant_id"`
	}
	_ = json.Unmarshal(s.Examples[0], &p)
	if p.TenantID == nil {
		id := uuid.New()
		p.TenantID = &id
	}
	return s.Examples[0], p.TenantID
}

// T033: the consumers of module competition accept the examples of the payload schemas owned by identity (F010,
// F011, F016) and game (F070), and refuse a payload that breaks them (DLQ at once).
func TestConsumersAcceptOwnerExamples(t *testing.T) {
	refs := &fakeRefs{}
	h := &fakeHub{}
	id := consumer.IdentityProjection{P: command.Projection{Refs: refs}}
	game := consumer.GameProjection{P: command.Projection{Refs: refs}}
	socialP := consumer.SocialProjection{P: command.Projection{Refs: refs}}
	revoked := consumer.HubAccessRevoked{Hub: h}
	cases := []struct {
		producer, event, want string
		handle                func(context.Context, outbox.Envelope) error
	}{
		{"identity", "tenant.created", "tenant", id.Handle},
		{"identity", "tenant.updated", "tenant", id.Handle},
		{"identity", "student.synced", "student", id.Handle},
		{"identity", "user.status_changed", "status", id.Handle},
		{"identity", "student.cleaned_up", "deleted", id.Handle},
		{"identity", "student.restored", "deleted", id.Handle},
		{"game", "game_profile.updated", "profile", game.Handle},
		// F095 T021: the relation events of module chat (spec F091 A-1).
		{"chat", "friend.added", "friend+", socialP.Handle},
		{"chat", "friend.removed", "friend-", socialP.Handle},
		{"chat", "user.blocked", "block+", socialP.Handle},
		{"chat", "user.unblocked", "block-", socialP.Handle},
	}
	for _, c := range cases {
		payload, tenant := example(t, c.producer, c.event)
		refs.calls = nil
		e := outbox.Envelope{EventID: uuid.New(), EventType: c.event, OccurredAt: time.Now(), TenantID: tenant, Payload: payload}
		if err := c.handle(context.Background(), e); err != nil {
			t.Errorf("%s: %v", c.event, err)
			continue
		}
		if len(refs.calls) != 1 || refs.calls[0] != c.want {
			t.Errorf("%s reached %v, want %s", c.event, refs.calls, c.want)
		}
	}
	payload, tenant := example(t, "identity", "user.access_revoked")
	if err := revoked.Handle(context.Background(), outbox.Envelope{EventType: "user.access_revoked", TenantID: tenant, Payload: payload}); err != nil ||
		len(h.removed) != 1 {
		t.Fatalf("access_revoked: %v %v", err, h.removed)
	}
	bad := outbox.Envelope{EventType: "game_profile.updated", TenantID: tenant, Payload: json.RawMessage(`{"student_id":"x"}`)}
	if err := game.Handle(context.Background(), bad); err == nil || !errors.Is(err, outbox.ErrPermanent) {
		t.Fatalf("invalid payload not refused: %v", err)
	}
}

// F095 T021: a relation event breaking the chat schema (pair not normalized, no source_version) goes to the DLQ at once.
func TestSocialProjectionRefusesInvalidPayloads(t *testing.T) {
	refs := &fakeRefs{}
	h := consumer.SocialProjection{P: command.Projection{Refs: refs}}
	tenant := uuid.New()
	lo, hi := "00000000-0000-7000-8000-000000000001", "00000000-0000-7000-8000-000000000002"
	for _, c := range []struct{ typ, payload string }{
		{"friend.added", `{"tenant_id":"` + tenant.String() + `","user_low_id":"` + hi + `","user_high_id":"` + lo + `","source_version":1}`},
		{"user.blocked", `{"tenant_id":"` + tenant.String() + `","blocker_id":"` + lo + `","blocked_id":"` + lo + `","source_version":1}`},
		{"friend.removed", `{"tenant_id":"` + tenant.String() + `","user_low_id":"` + lo + `","user_high_id":"` + hi + `"}`},
		{"user.unblocked", `{"tenant_id":"` + uuid.NewString() + `","blocker_id":"` + lo + `","blocked_id":"` + hi + `","source_version":1}`},
	} {
		e := outbox.Envelope{EventType: c.typ, TenantID: &tenant, OccurredAt: time.Now(), Payload: json.RawMessage(c.payload)}
		if err := h.Handle(context.Background(), e); err == nil || !errors.Is(err, outbox.ErrPermanent) {
			t.Errorf("%s %s: %v", c.typ, c.payload, err)
		}
	}
	if len(refs.calls) != 0 {
		t.Fatalf("invalid payloads reached the projection: %v", refs.calls)
	}
}
