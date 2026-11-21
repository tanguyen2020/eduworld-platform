package facade

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// fakeRooms is a TEST DOUBLE of the room host registry (R17): one room of tenant A with one seated student.
type fakeRooms struct {
	port.Registry
	room    string
	tenant  uuid.UUID
	student uuid.UUID
	pos     *hub.Vec3
}

func (f fakeRooms) UserRoom(_ context.Context, user uuid.UUID) (*port.UserRoom, error) {
	if user != f.student {
		return nil, nil
	}
	return &port.UserRoom{RoomID: f.room, AreaID: "main_hub"}, nil
}

func (f fakeRooms) Meta(_ context.Context, room string) (*port.RoomMeta, error) {
	if room != f.room {
		return nil, nil
	}
	return &port.RoomMeta{RoomID: room, AreaID: "main_hub", Partition: hub.Partition{TenantID: f.tenant, SchoolID: uuid.New(), GradeBand: "g1_3"}}, nil
}

func (f fakeRooms) Position(_ context.Context, room string, user uuid.UUID) (*hub.Vec3, error) {
	if room != f.room || user != f.student {
		return nil, nil
	}
	return f.pos, nil
}

func scoped(t *testing.T, tenant uuid.UUID) context.Context {
	t.Helper()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	return tenancy.WithScope(context.Background(), sc)
}

// Callee-side facade test of getHubPosition (F071 T049, spec example of HubPosition): the seated student of the tenant
// gets the last position of the room host; another tenant, a student in no room or an expired position → 404 NOT_FOUND
// (never another tenant's room); no tenant scope → refused; no registry in the binary → 503 UNAVAILABLE.
func TestGetHubPositionFacade(t *testing.T) {
	tenant, other, student := uuid.New(), uuid.New(), uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-000000000101")
	rooms := fakeRooms{room: "0193a5f4-6c1e-7b2a-9d1f-000000000201", tenant: tenant, student: student, pos: &hub.Vec3{1.5, 0, -4}}
	f := Facade{Hub: &query.HubPosition{Rooms: rooms, Positions: rooms}}
	resp, err := f.GetHubPosition(scoped(t, tenant), api.GetHubPositionRequestObject{StudentId: student})
	v, ok := resp.(api.GetHubPosition200JSONResponse)
	if err != nil || !ok || v.AreaId != "main_hub" || v.RoomId != rooms.room || len(v.Position) != 3 || v.Position[2] != -4 {
		t.Fatalf("seated student: %+v %v", resp, err)
	}
	code := func(err error) problem.Code {
		var pe *problem.Error
		if !errors.As(err, &pe) {
			return ""
		}
		return pe.Code
	}
	if _, err := f.GetHubPosition(scoped(t, other), api.GetHubPositionRequestObject{StudentId: student}); code(err) != problem.NotFound {
		t.Fatalf("other tenant: %v", err)
	}
	if _, err := f.GetHubPosition(scoped(t, tenant), api.GetHubPositionRequestObject{StudentId: uuid.New()}); code(err) != problem.NotFound {
		t.Fatalf("not seated: %v", err)
	}
	expired := Facade{Hub: &query.HubPosition{Rooms: rooms, Positions: fakeRooms{}}}
	if _, err := expired.GetHubPosition(scoped(t, tenant), api.GetHubPositionRequestObject{StudentId: student}); code(err) != problem.NotFound {
		t.Fatalf("expired position: %v", err)
	}
	if _, err := f.GetHubPosition(context.Background(), api.GetHubPositionRequestObject{StudentId: student}); code(err) != problem.TenantNotSelected {
		t.Fatalf("no scope: %v", err)
	}
	if _, err := (Facade{}).GetHubPosition(scoped(t, tenant), api.GetHubPositionRequestObject{StudentId: student}); code(err) != problem.Unavailable {
		t.Fatalf("no registry: %v", err)
	}
}
