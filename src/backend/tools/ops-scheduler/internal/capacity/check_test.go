package capacity_test

import (
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/capacity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/dockerapi"
)

const cpu = int64(1e9)
const gib = int64(1 << 30)

func appNodes() []dockerapi.Node {
	node := func(id, role, avail, state string) dockerapi.Node {
		return dockerapi.Node{ID: id, Labels: map[string]string{"role": role}, Availability: avail, State: state,
			Capacity: dockerapi.Resources{NanoCPUs: 16 * cpu, MemoryBytes: 32 * gib}}
	}
	return []dockerapi.Node{
		node("a1", "app", "active", "ready"),
		node("a2", "app", "active", "ready"),
		node("a3", "app", "drain", "ready"), // drained: no capacity
		node("a4", "app", "active", "down"), // down: no capacity
		node("r1", "realtime", "active", "ready"),
	}
}

// F130-AC-23: a scale-up whose reservations exceed the free capacity of the role is refused; scale-down passes.
func TestReserveRejectsBeyondRoleCapacity(t *testing.T) {
	tasks := []dockerapi.Task{
		{NodeID: "a1", Reservations: dockerapi.Resources{NanoCPUs: 12 * cpu, MemoryBytes: 8 * gib}},
		{NodeID: "a3", Reservations: dockerapi.Resources{NanoCPUs: 16 * cpu}}, // on a drained node: ignored
	}
	free := capacity.Free(appNodes(), tasks)
	if free["app"].NanoCPUs != 20*cpu || free["app"].MemoryBytes != 56*gib {
		t.Fatalf("free app = %+v, want 20 CPU / 56 GiB", free["app"])
	}
	coreAPI := dockerapi.Service{Name: "eduworld_core-api", Constraints: []string{"node.labels.role == app"},
		Reservations: dockerapi.Resources{NanoCPUs: 2 * cpu, MemoryBytes: 2 * gib}}
	l := capacity.NewLedger(free)
	if err := l.Reserve(coreAPI, 6, 16); err != nil { // +10 × 2 CPU = 20 CPU: exactly fits
		t.Fatalf("fitting scale refused: %v", err)
	}
	if err := l.Reserve(coreAPI, 16, 17); err == nil || !strings.Contains(err.Error(), "role app") {
		t.Fatalf("scale beyond capacity accepted: %v", err)
	}
	if err := l.Reserve(coreAPI, 16, 6); err != nil {
		t.Fatalf("scale-down refused: %v", err)
	}
	noRole := dockerapi.Service{Name: "eduworld_x", Reservations: coreAPI.Reservations}
	if err := l.Reserve(noRole, 1, 2); err == nil {
		t.Fatal("service without a role constraint accepted")
	}
}

func TestRole(t *testing.T) {
	if r := capacity.Role([]string{"node.labels.name == pg-1", "node.labels.role==data-pg"}); r != "data-pg" {
		t.Fatalf("Role = %q", r)
	}
}
