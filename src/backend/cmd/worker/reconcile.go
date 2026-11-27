package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Retention parameters of CONFIG_DEFAULTS (type "code", ADR-0042): outbox_retention bounds the window (F134-AC-04),
// event_stream_retention chooses the compensation hint; reconcile_settle_minutes is the default --settle.
const (
	outboxRetention        = 30 * 24 * time.Hour
	eventStreamRetention   = 7 * 24 * time.Hour
	defaultReconcileSettle = 10 * time.Minute
	reconcileJob           = "reconcile"
)

// reconcileDeps let tests replace the composition of the five modules (the production path composes them from env).
type reconcileDeps struct {
	Getenv func(string) string
	Now    func() time.Time
	// Peers composes the modules and returns their reconcile.Peer (caller = the restored module) and the audit
	// recorder of the restored module; close releases the database pools.
	Peers func(caller string) (peers map[string]reconcile.Peer, rec audit.Recorder, closeFn func(), err error)
}

// runReconcile implements `worker reconcile --module=<module> --restore-point=<T> --from=<RFC3339> --to=<RFC3339>
// [--settle=10m] [--out=-|<file>]` (job reconcile-<module>, F134 US1, BACKUP_RECOVERY §3.5, RB-01 step 7). It returns the
// exit code: 0 no unexplained mismatch, 2 unexplained mismatches, 3 a module could not be read, 1 bad arguments.
func runReconcile(ctx context.Context, args []string, d reconcileDeps, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mod := fs.String("module", "", "restored module (identity|learning|game|competition|chat)")
	restore := fs.String("restore-point", "", "restore point T (RFC 3339)")
	from := fs.String("from", "", "window start (RFC 3339)")
	to := fs.String("to", "", "window end (RFC 3339, default now)")
	settle := fs.Duration("settle", defaultReconcileSettle, "events published within this duration are pending_delivery (reconcile_settle_minutes)")
	out := fs.String("out", "-", "report file (JSON, reconcile-report.schema.json); - = stdout")
	if err := fs.Parse(args); err != nil {
		return reconcile.ExitBadArgs
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	now := d.Now().UTC()
	if !slices.Contains(reconcile.Modules, *mod) {
		fmt.Fprintf(stderr, "worker reconcile: --module must be one of %s\n", strings.Join(reconcile.Modules, ", "))
		return reconcile.ExitBadArgs
	}
	w, err := parseWindow(*from, *to, *restore, *settle, now)
	if err == nil {
		err = w.Validate(outboxRetention, now)
	}
	if err != nil { // F134-AC-04: stop before calling any module
		fmt.Fprintln(stderr, "worker reconcile:", err)
		return reconcile.ExitBadArgs
	}
	peers, rec, closeFn, err := d.Peers(*mod)
	if err != nil {
		fmt.Fprintln(stderr, "worker reconcile: compose modules:", err)
		return reconcile.ExitIncomplete
	}
	if closeFn != nil {
		defer closeFn()
	}
	// Platform scope of the job + the audited bypass platform_job (ADR-0089): the facades refuse any other context.
	ctx = tenancy.WithModule(ctx, *mod)
	ps, err := authz.SystemPlatformScope(reconcileJob)
	if err != nil {
		fmt.Fprintln(stderr, "worker reconcile:", err)
		return reconcile.ExitBadArgs
	}
	ctx = tenancy.WithScope(ctx, ps)
	if ctx, err = tenancy.Bypass(ctx, tenancy.ReasonPlatformJob); err != nil {
		fmt.Fprintln(stderr, "worker reconcile: audit of the bypass failed:", err)
		return reconcile.ExitIncomplete
	}
	r, err := reconcile.Run(ctx, reconcile.Options{Module: *mod, Peers: peers, Window: w, StreamRetention: eventStreamRetention, Now: d.Now})
	if err != nil {
		fmt.Fprintln(stderr, "worker reconcile:", err)
		return reconcile.ExitBadArgs
	}
	if err := writeReport(r, *out, stdout); err != nil {
		fmt.Fprintln(stderr, "worker reconcile: write report:", err)
		return reconcile.ExitIncomplete
	}
	_ = r.Summary(stderr)
	if rec != nil { // F134-AC-06: one audit per run, in the outbox of the restored module
		actor := ""
		if d.Getenv != nil {
			actor = strings.TrimSpace(d.Getenv("EW_ACTOR"))
		}
		meta := map[string]any{"run_id": r.RunID.String(), "status": r.Status, "window_from": r.WindowFrom.Format(time.RFC3339),
			"window_to": r.WindowTo.Format(time.RFC3339), "restore_point": r.RestorePoint.Format(time.RFC3339), "exit_code": r.ExitCode()}
		for _, c := range reconcile.Classes {
			meta[string(c)] = r.Counts.Of(c)
		}
		if err := rec.Record(ctx, audit.Event{Action: "ops.reconcile.completed", TargetType: "module", TargetID: *mod,
			Result: audit.ResultSuccess, Reason: "reconcile-" + *mod + " run by " + valueOr(actor, "unknown"), Metadata: meta}); err != nil {
			fmt.Fprintln(stderr, "worker reconcile: audit ops.reconcile.completed:", err)
			return reconcile.ExitIncomplete
		}
	}
	return r.ExitCode()
}

func valueOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func parseWindow(from, to, restore string, settle time.Duration, now time.Time) (reconcile.Window, error) {
	var w reconcile.Window
	parse := func(name, v string) (time.Time, error) {
		if v == "" {
			return time.Time{}, nil
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: --%s: %w", reconcile.ErrWindow, name, err)
		}
		return t.UTC(), nil
	}
	var err error
	if w.From, err = parse("from", from); err != nil {
		return w, err
	}
	if w.To, err = parse("to", to); err != nil {
		return w, err
	}
	if w.To.IsZero() {
		w.To = now
	}
	if w.RestorePoint, err = parse("restore-point", restore); err != nil {
		return w, err
	}
	w.Settle = settle
	return w, nil
}

func writeReport(r reconcile.Report, path string, stdout io.Writer) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if path == "" || path == "-" {
		_, err = stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// composeReconcile builds a worker registry of profile "reconcile" (no relay, consumer, job nor HTTP runs — only the
// declarations, so each module knows its consumers) and the reconcile peers of the five modules.
func composeReconcile(getenv func(string) string) func(caller string) (map[string]reconcile.Peer, audit.Recorder, func(), error) {
	return func(caller string) (map[string]reconcile.Peer, audit.Recorder, func(), error) {
		reg := module.NewRegistry("worker", reconcileJob, health.NewRegistry(buildinfo.Get("worker", reconcileJob)), getenv)
		// Some modules take their JetStream context while composing; the connection is lazy and never used by the
		// reconciliation (nothing is published or consumed).
		nc, err := connectLazyNATS(getenv)
		if err != nil {
			reg.Close()
			return nil, nil, nil, err
		}
		reg.SetNATS(nc)
		closeAll := func() { reg.Close(); nc.Close() }
		c, err := wiring.Compose(reg)
		if err != nil {
			closeAll()
			return nil, nil, nil, err
		}
		peers := wiring.ReconcilePeers(c, caller)
		if len(peers) == 0 {
			closeAll()
			return nil, nil, nil, errors.New("no module facade composed")
		}
		return peers, reg.AuditRecorder(), closeAll, nil
	}
}

// connectLazyNATS opens the NATS connection like bootstrap (EW_NATS_URLS, EW_NATS_CREDS_FILE), retrying in the background.
func connectLazyNATS(getenv func(string) string) (*nats.Conn, error) {
	urls := strings.TrimSpace(getenv("EW_NATS_URLS"))
	if urls == "" {
		urls = nats.DefaultURL
	}
	opts := []nats.Option{nats.Name("worker-reconcile"), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1), nats.ReconnectWait(2 * time.Second)}
	if creds := strings.TrimSpace(getenv("EW_NATS_CREDS_FILE")); creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}
	return nats.Connect(urls, opts...)
}
