package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

type spySync struct{ ids []uuid.UUID }

func (s *spySync) Handle(_ context.Context, id uuid.UUID) error {
	s.ids = append(s.ids, id)
	return nil
}

func TestKeycloakMembershipDispatch(t *testing.T) {
	s := &spySync{}
	c := KeycloakMembership{SyncOrganization: s}
	id := uuid.New()
	for _, typ := range []string{"tenant.created", "tenant.updated"} {
		if err := c.Handle(context.Background(), outbox.Envelope{EventType: typ, Payload: []byte(`{"tenant_id":"` + id.String() + `"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.ids) != 2 || s.ids[0] != id {
		t.Fatalf("sync calls = %v", s.ids)
	}
	for _, e := range []outbox.Envelope{
		{EventType: "tenant.created", Payload: []byte(`{}`)},
		{EventType: "tenant.created", Payload: []byte(`not json`)},
		{EventType: "school.updated", Payload: []byte(`{}`)},
	} {
		if err := c.Handle(context.Background(), e); !errors.Is(err, outbox.ErrPermanent) {
			t.Fatalf("%s %s: %v (want permanent → DLQ)", e.EventType, e.Payload, err)
		}
	}
}
