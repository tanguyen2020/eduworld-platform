// Package capacity is the application rule of ops-scheduler that refuses a scale-up the nodes of the service's
// role cannot hold (F130-AC-23, DEPLOYMENT §4.4: "không scale vượt năng lực node vai trò"). It is pure: inputs
// come from dockerapi, the decision is a value.
package capacity

import (
	"fmt"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/dockerapi"
)

// Role returns the node role a service is pinned to (constraint "node.labels.role == <role>"), "" when none.
func Role(constraints []string) string {
	for _, c := range constraints {
		k, v, ok := strings.Cut(c, "==")
		if ok && strings.TrimSpace(k) == "node.labels.role" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Free returns the unreserved capacity of every role: capacity of ready+active nodes labelled role=<r> minus
// the reservations of running tasks placed on them.
func Free(nodes []dockerapi.Node, tasks []dockerapi.Task) map[string]dockerapi.Resources {
	roleOf := map[string]string{}
	free := map[string]dockerapi.Resources{}
	for _, n := range nodes {
		r := n.Labels["role"]
		if r == "" || n.State != "ready" || n.Availability != "active" {
			continue
		}
		roleOf[n.ID] = r
		f := free[r]
		f.NanoCPUs += n.Capacity.NanoCPUs
		f.MemoryBytes += n.Capacity.MemoryBytes
		free[r] = f
	}
	for _, t := range tasks {
		r, ok := roleOf[t.NodeID]
		if !ok {
			continue
		}
		f := free[r]
		f.NanoCPUs -= t.Reservations.NanoCPUs
		f.MemoryBytes -= t.Reservations.MemoryBytes
		free[r] = f
	}
	return free
}

// Ledger tracks free capacity while several services of one tick scale up.
type Ledger struct {
	free map[string]dockerapi.Resources
}

// NewLedger starts from Free(nodes, tasks).
func NewLedger(free map[string]dockerapi.Resources) *Ledger { return &Ledger{free: free} }

// Reserve accepts a scale of s from current to target replicas when the role still has room, and books the extra
// reservation. Scale-downs always pass. The error explains the refusal (logged, result="rejected").
func (l *Ledger) Reserve(s dockerapi.Service, current, target uint64) error {
	if target <= current {
		return nil
	}
	role := Role(s.Constraints)
	if role == "" {
		return fmt.Errorf("%s has no node.labels.role constraint: cannot check capacity", s.Name)
	}
	extra := int64(target - current)
	need := dockerapi.Resources{NanoCPUs: extra * s.Reservations.NanoCPUs, MemoryBytes: extra * s.Reservations.MemoryBytes}
	f := l.free[role]
	if need.NanoCPUs > f.NanoCPUs || need.MemoryBytes > f.MemoryBytes {
		return fmt.Errorf("%s: +%d replicas need %.2f CPU / %d MiB, role %s has %.2f CPU / %d MiB free",
			s.Name, extra, float64(need.NanoCPUs)/1e9, need.MemoryBytes>>20, role, float64(f.NanoCPUs)/1e9, f.MemoryBytes>>20)
	}
	f.NanoCPUs -= need.NanoCPUs
	f.MemoryBytes -= need.MemoryBytes
	l.free[role] = f
	return nil
}
