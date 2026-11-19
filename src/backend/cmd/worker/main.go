// Command worker runs background work by profile (ADR-0074 §2): events (outbox relay, consumers),
// chat (chat persister, async moderation, purge), jobs (scheduled jobs), all (local: everything).
// Staging/production run each profile as its own Swarm service.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/bootstrap"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// Profiles accepted by --profile / EW_WORKER_PROFILE.
var profiles = map[string]bool{"events": true, "chat": true, "jobs": true, "all": true}

func main() {
	bootstrap.MaybeHealthcheck(":9200")
	authz.MustValidateAtStartup() // F002-AC-14: exit 1 if a role breaks the permission catalog
	if len(os.Args) > 1 && os.Args[1] == "projection" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := runProjection(ctx, os.Args[2:], &projections, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "worker projection:", err)
			stop()
			os.Exit(1)
		}
		return
	}
	fs := flag.NewFlagSet("worker", flag.ExitOnError)
	profileFlag := fs.String("profile", "", "events | chat | jobs | all (default: $EW_WORKER_PROFILE)")
	_ = fs.Parse(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *profileFlag, os.Getenv, nil); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, profile string, getenv func(string) string, ready func(admin, http string)) error {
	p, err := resolveProfile(profile, getenv)
	if err != nil {
		return err
	}
	return bootstrap.Run(ctx, bootstrap.Options{
		Binary:           "worker",
		Profile:          p,
		DefaultAdminAddr: ":9200",
		Modules:          modulesFor(p),
		Register:         func(r *module.Registry) error { return registerFor(p, r) },
		Getenv:           getenv,
		Stdout:           os.Stdout,
		Ready:            ready,
	})
}

func resolveProfile(flagValue string, getenv func(string) string) (string, error) {
	p := strings.TrimSpace(flagValue)
	if p == "" {
		p = strings.TrimSpace(getenv("EW_WORKER_PROFILE"))
	}
	if p == "" {
		return "", fmt.Errorf("profile is required: --profile or EW_WORKER_PROFILE (events|chat|jobs|all)")
	}
	if !profiles[p] {
		return "", fmt.Errorf("unknown profile %q (events|chat|jobs|all)", p)
	}
	return p, nil
}

// modulesFor: profile chat only needs eduworld_chat; events/jobs/all touch every domain module.
func modulesFor(profile string) []string {
	if profile == "chat" {
		return []string{chat.Name}
	}
	return wiring.Modules
}

// registerFor composes the modules of the profile (OQ F010-2): events runs the outbox relays and consumers, jobs
// the scheduled jobs, all both (local). Chat consumers and the gameplay purge come with the chat features.
func registerFor(profile string, r *module.Registry) error {
	if profile == "chat" {
		_, err := chat.Register(r)
		return err
	}
	_, err := wiring.Compose(r)
	return err
}
