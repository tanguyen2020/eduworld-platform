//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/natscmd"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T018 (F090-AC-12, TC-AUTH-017): the gateway command ew.cmd.game.session.heartbeat moves last_heartbeat_at of the
// session bound to the ticket (tenant scope from Ew-Tenant-Id); a replaced session answers CONFLICT (the gateway
// closes 4409); a session of another student answers NOT_FOUND; a request without Ew-* headers is refused.
func TestNATSSessionHeartbeat(t *testing.T) {
	ctx := context.Background()
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	e := setup(ctx, t, envOpts{})
	sctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go func() {
		_ = rt.ServeCommand(sctx, nc, rt.SessionHeartbeatModule, rt.SessionHeartbeatType, natscmd.SessionHeartbeat(e.sessions), 0)
	}()
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(an)
	session := uuid.MustParse(e.start(t, an, d, newKey(), nil).body["game_session_id"].(string))
	caller := rt.Caller{TenantID: an.tenant, UserID: an.id, SchoolID: an.school, GameSessionID: session, DeviceID: d.id,
		ConnID: "c-1", NodeID: "gw-1"}
	send := func(c rt.Caller, header bool) rt.Reply {
		t.Helper()
		m := nats.NewMsg(rt.CommandSubject(rt.SessionHeartbeatModule, rt.SessionHeartbeatType))
		m.Data, _ = json.Marshal(map[string]string{"game_session_id": c.GameSessionID.String()})
		if header {
			m.Header = c.Header(ctx)
		}
		var res *nats.Msg
		for i := 0; i < 50; i++ { // the queue subscription may not be registered yet
			if res, err = nc.RequestMsg(m, 2*time.Second); err == nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		var r rt.Reply
		if err := json.Unmarshal(res.Data, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	e.clock.Add(30 * time.Second)
	if r := send(caller, true); !r.OK {
		t.Fatalf("heartbeat: %+v", r.Problem)
	}
	var last time.Time
	if err := e.platform.QueryRowContext(ctx, `SELECT last_heartbeat_at FROM game.game_sessions WHERE game_session_id = $1`, session).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if !last.Equal(e.clock.Now()) {
		t.Fatalf("last_heartbeat_at %v, want %v", last, e.clock.Now())
	}
	if r := send(caller, false); r.OK || r.Problem.Code != "UNAUTHENTICATED" {
		t.Fatalf("no headers: %+v", r)
	}
	binh := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	other := caller
	other.UserID = binh.id
	if r := send(other, true); r.OK || r.Problem.Code != "NOT_FOUND" {
		t.Fatalf("other student: %+v", r)
	}
	wrongTenant := caller
	wrongTenant.TenantID = e.tenantB
	if r := send(wrongTenant, true); r.OK || r.Problem.Code != "NOT_FOUND" {
		t.Fatalf("other tenant: %+v", r)
	}
	if r := e.takeover(t, an, e.device(an)); r.status != http.StatusCreated {
		t.Fatalf("takeover: %d %s", r.status, r.raw)
	}
	if r := send(caller, true); r.OK || r.Problem.Code != "CONFLICT" {
		t.Fatalf("replaced session: %+v", r)
	}
}
