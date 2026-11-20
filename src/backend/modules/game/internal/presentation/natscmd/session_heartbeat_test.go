package natscmd_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/natscmd"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// fakeHeartbeat is a TEST DOUBLE of command.Sessions.RealtimeHeartbeat (R17).
type fakeHeartbeat struct {
	err  error
	args []uuid.UUID
}

func (f *fakeHeartbeat) RealtimeHeartbeat(_ context.Context, tenant, student, session, device uuid.UUID) error {
	f.args = []uuid.UUID{tenant, student, session, device}
	return f.err
}

// F090-AC-12: the handler uses the ticket-bound headers; an ended session answers CONFLICT (gateway closes 4409).
func TestSessionHeartbeat(t *testing.T) {
	c := rt.Caller{TenantID: uuid.New(), UserID: uuid.New(), GameSessionID: uuid.New(), DeviceID: uuid.New()}
	f := &fakeHeartbeat{}
	h := natscmd.SessionHeartbeat(f)
	body, _ := json.Marshal(map[string]string{"game_session_id": c.GameSessionID.String()})
	if r := h(context.Background(), c, body); !r.OK || f.args[0] != c.TenantID || f.args[2] != c.GameSessionID {
		t.Fatalf("ok reply %+v args %v", r, f.args)
	}
	f.err = problem.New(problem.Conflict, "ended")
	if r := h(context.Background(), c, body); r.OK || r.Problem.Code != "CONFLICT" {
		t.Fatalf("conflict reply %+v", r)
	}
	other, _ := json.Marshal(map[string]string{"game_session_id": uuid.NewString()})
	if r := h(context.Background(), c, other); r.OK || r.Problem.Code != "VALIDATION_FAILED" {
		t.Fatalf("other session %+v", r)
	}
	f.err = errors.New("db down")
	if r := h(context.Background(), c, body); r.OK || r.Problem.Code != "INTERNAL" || !r.Problem.Retryable {
		t.Fatalf("internal %+v", r)
	}
}
