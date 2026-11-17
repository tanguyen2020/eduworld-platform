// Package chat is the composition entry of module chat (ADR-0074 §1).
// cmd/<binary> may only call Register (and later Facade/Wire); everything else lives in internal/.
package chat

import (
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/migrations"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Name is the module name used for its database (eduworld_chat), env prefix and checks.
const Name = "chat"

// Module is the registered module. Facade() and Wire(Imports) are added when the module exposes or
// consumes facade operations (specs/openapi/modules/*.internal.v1.yaml).
type Module struct{}

// Register wires the module into the running binary. F001: health checks only; routes, consumers,
// jobs and facades are added by the features that own them.
func Register(r *module.Registry) (*Module, error) {
	db, err := r.Database(Name)
	if err != nil {
		return nil, fmt.Errorf("chat: open database: %w", err)
	}
	r.Health().Register("db:"+Name, health.Readiness, health.PostgresCheck(db))
	r.Health().Register("atlas:"+Name, health.Readiness, health.AtlasRevisionCheck(db, migrations.Latest()))
	return &Module{}, nil
}
