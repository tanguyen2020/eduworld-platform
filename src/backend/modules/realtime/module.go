// Package realtime is the composition entry of module realtime (ADR-0074 §1): the WebSocket gateway of binary
// realtime (F090). cmd/<binary> may only call Register; everything else lives in internal/.
package realtime

import (
	"context"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/config"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/conn"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime/internal/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Name is the module name used for its database (none), env prefix and checks.
const Name = "realtime"

// Module is the registered module.
type Module struct {
	manager *conn.Manager
}

// Register wires the gateway into binary realtime (F090): readiness check, the public handler /realtime/v1 and the
// worker that keeps the Valkey Pub/Sub connection and drains on shutdown. realtime has no database and loads no
// domain module (ADR-0074 §2); in another binary it registers nothing.
func Register(r *module.Registry) (*Module, error) {
	if r.Binary() != "realtime" {
		return &Module{}, nil
	}
	cfg, err := config.From(r.Config())
	if err != nil {
		return nil, err
	}
	vk, err := r.Valkey()
	if err != nil {
		return nil, fmt.Errorf("realtime: %w", err)
	}
	deps := conn.Deps{Config: cfg, Valkey: vk, NATS: r.NATS(), Metrics: observability.New(r.Metrics(), cfg.NodeID)}
	if r.NATS() != nil {
		js, err := r.JetStream()
		if err != nil {
			return nil, fmt.Errorf("realtime: %w", err)
		}
		if deps.Audit, err = audit.NewPublisher(js); err != nil {
			return nil, fmt.Errorf("realtime: %w", err)
		}
		r.Go("realtime.audit_stream", func(ctx context.Context) error { // EVT_REALTIME; retried until NATS is up
			return audit.EnsureStream(ctx, js)
		})
	}
	m, err := conn.New(deps)
	if err != nil {
		return nil, fmt.Errorf("realtime: %w", err)
	}
	r.Health().Register("realtime:gateway", health.Readiness, m.Ready)
	r.SetPublic(m)
	r.Go("realtime.gateway", m.Run)
	return &Module{manager: m}, nil
}
