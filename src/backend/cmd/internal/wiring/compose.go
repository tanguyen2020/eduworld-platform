package wiring

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Modules are the domain modules loaded by core-api and the worker profiles events/jobs/all (ADR-0074 §2).
var Modules = []string{identity.Name, learning.Name, game.Name, competition.Name, chat.Name}

// Composition is the result of Compose: the registered modules of the binary.
type Composition struct {
	Identity *identity.Module
	Learning *learning.Module
	Game     *game.Module
	Server   *httpx.Server // core-api only
}

// Compose registers the five domain modules on r and wires their facades in process (BACKEND_DESIGN §3.3, OQ
// F010-2): learning and game first, then identity, whose sagas and platform operations call them through their
// generated api packages (pkg/modcall). Each module declares its own routes, relay, consumers and jobs; the role
// of the binary decides which of them run. Competition calls the chat facade for party names once module chat
// offers it (F116).
func Compose(r *module.Registry) (*Composition, error) {
	ConfigureHooks(r)
	idm, err := identity.Register(r)
	if err != nil {
		return nil, err
	}
	lrn, err := learning.Register(r)
	if err != nil {
		return nil, err
	}
	gam, err := game.Register(r)
	if err != nil {
		return nil, err
	}
	cmp, err := competition.Register(r)
	if err != nil {
		return nil, err
	}
	if _, err := chat.Register(r); err != nil {
		return nil, err
	}
	lrn.Wire(learning.Imports{})
	gam.Wire(game.Imports{})
	// Module chat offers no moderateDisplayText yet (F116): party names fall back to the generated default name.
	cmp.Wire(competition.Imports{})
	in := identity.Imports{}
	if f := lrn.Facade(); f != nil {
		in.Learning = f
	}
	if f := gam.Facade(); f != nil {
		in.Game = f
	}
	if err := idm.Wire(in); err != nil {
		return nil, err
	}
	return &Composition{Identity: idm, Learning: lrn, Game: gam}, nil
}

// ComposeCoreAPI is Compose plus the public HTTP server of core-api.
func ComposeCoreAPI(ctx context.Context, r *module.Registry) (*Composition, error) {
	c, err := Compose(r)
	if err != nil {
		return nil, err
	}
	f := c.Identity.Facade()
	if f == nil {
		return nil, fmt.Errorf("wiring: module identity has no facade in core-api")
	}
	extra := map[string]http.Handler{}
	if h := c.Identity.SisWebhook(); h != nil {
		extra[identity.SisWebhookPath] = h
	}
	s, err := NewPublicServer(ctx, r, PublicOptions{Identity: f, Toggles: c.Identity.FeatureGateSource(), Extra: extra})
	if err != nil {
		return nil, err
	}
	c.Server = s
	return c, nil
}
