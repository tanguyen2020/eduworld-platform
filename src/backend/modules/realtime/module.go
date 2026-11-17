// Package realtime is the composition entry of module realtime (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package realtime

import (
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Name is the module name used for its database (none), env prefix and checks.
const Name = "realtime"

// Module is the registered module. Facade() and Wire(Imports) are added when the module exposes or
// consumes facade operations (specs/openapi/modules/*.internal.v1.yaml).
type Module struct{}

// Register wires the module into the running binary. F001: health checks only; routes, consumers,
// jobs and facades are added by the features that own them.
func Register(r *module.Registry) (*Module, error) {
	_ = r // realtime has no database; Valkey/NATS checks are registered by cmd/realtime.
	return &Module{}, nil
}
