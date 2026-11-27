package wiring_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// F134: every consumer declared by the five modules records its source and event types (pkg/module.ConsumerInfos), so
// listProcessedEventIds answers the consumers of each source; and the composition exposes a reconcile peer per module.
func TestReconcileConsumersAndPeersFacade(t *testing.T) {
	r := registry(t, "worker", "reconcile")
	c, err := wiring.Compose(r)
	if err != nil {
		t.Fatal(err)
	}
	infos := r.ConsumerInfos("")
	if len(infos) < 30 {
		t.Fatalf("only %d consumers recorded", len(infos))
	}
	for _, i := range infos {
		if i.Source == "" || len(i.EventTypes) == 0 {
			t.Errorf("consumer %s of %s has no source/event types (factory needs JetStream?)", i.Name, i.Module)
		}
		if i.Source == "realtime" {
			t.Errorf("consumer %s reads a module without database", i.Name)
		}
	}
	peers := wiring.ReconcilePeers(c, "learning")
	for _, m := range wiredModules {
		if peers[m] == nil {
			t.Fatalf("no reconcile peer for %s", m)
		}
	}
	// Without the job context (platform scope + platform_job bypass) a facade refuses: the operations are not a
	// back door for any other in-process caller (F134-AC-05).
	from := time.Now().Add(-time.Hour)
	if _, _, err = peers["game"].ListOutboxEventIDs(context.Background(), from, time.Now(), nil, "", 10); err == nil {
		t.Fatal("no scope accepted")
	}
	ps, err := authz.SystemPlatformScope("reconcile")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = peers["game"].ListOutboxEventIDs(tenancy.WithScope(context.Background(), ps), from, time.Now(), nil, "", 10)
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.PermissionDenied {
		t.Fatalf("game listOutboxEventIds without job context: %v", err)
	}
	// realtime is not in x-allowed-callers: pkg/modcall refuses the call (CALLER_NOT_ALLOWED).
	_, _, _, err = wiring.ReconcilePeers(c, "realtime")["identity"].ListProcessedEventIDs(context.Background(), "game", from, time.Now(), "", 10)
	if !errors.As(err, &pe) || pe.Code != problem.CallerNotAllowed {
		t.Fatalf("realtime caller: %v", err)
	}
}
