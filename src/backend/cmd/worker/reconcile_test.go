package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// recordingAudit is a TEST DOUBLE of the module audit recorder (R17).
type recordingAudit struct{ events []audit.Event }

func (r *recordingAudit) Record(_ context.Context, e audit.Event) error {
	r.events = append(r.events, e)
	return nil
}

// emptyPeer is a TEST DOUBLE facade with nothing to compare.
type emptyPeer struct{}

func (emptyPeer) ListOutboxEventIDs(context.Context, time.Time, time.Time, []string, string, int) ([]reconcile.OutboxEvent, string, error) {
	return nil, "", nil
}

func (emptyPeer) ListProcessedEventIDs(context.Context, string, time.Time, time.Time, string, int) ([]reconcile.Consumer, []reconcile.Processed, string, error) {
	return nil, nil, "", nil
}

// F134-AC-04: a bad window stops with exit 1 before any module is composed or called.
func TestReconcileCommandBadArgs(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	composed := 0
	d := reconcileDeps{Now: func() time.Time { return now }, Peers: func(string) (map[string]reconcile.Peer, audit.Recorder, func(), error) {
		composed++
		return nil, nil, nil, nil
	}}
	for name, args := range map[string][]string{
		"unknown module":      {"--module=realtime", "--from=2026-11-26T00:00:00Z", "--restore-point=2026-11-26T01:00:00Z"},
		"missing module":      {"--from=2026-11-26T00:00:00Z", "--restore-point=2026-11-26T01:00:00Z"},
		"window > retention":  {"--module=learning", "--from=2026-10-01T00:00:00Z", "--restore-point=2026-10-02T00:00:00Z"},
		"from after to":       {"--module=learning", "--from=2026-11-26T05:00:00Z", "--to=2026-11-26T01:00:00Z", "--restore-point=2026-11-26T02:00:00Z"},
		"T outside window":    {"--module=learning", "--from=2026-11-26T00:00:00Z", "--to=2026-11-26T03:00:00Z", "--restore-point=2026-11-26T04:00:00Z"},
		"T not RFC 3339":      {"--module=learning", "--from=2026-11-26T00:00:00Z", "--restore-point=yesterday"},
		"unknown flag":        {"--module=learning", "--force"},
		"missing restore pt.": {"--module=learning", "--from=2026-11-26T00:00:00Z"},
	} {
		var out, errb bytes.Buffer
		if code := runReconcile(context.Background(), args, d, &out, &errb); code != reconcile.ExitBadArgs {
			t.Errorf("%s: exit %d, want 1 (%s)", name, code, errb.String())
		}
	}
	if composed != 0 {
		t.Fatalf("modules composed %d times on bad arguments", composed)
	}
}

// F134-AC-03, AC-06: the JSON report on stdout (schema fields only), the summary on stderr, exit 0, one audit
// ops.reconcile.completed under the platform_job bypass of the restored module.
func TestReconcileCommandReportAndAudit(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	rec := &recordingAudit{}
	tenancy.Configure(tenancy.Hooks{Recorder: rec})
	peers := map[string]reconcile.Peer{}
	for _, m := range reconcile.Modules {
		peers[m] = emptyPeer{}
	}
	closed := false
	d := reconcileDeps{Now: func() time.Time { return now }, Getenv: func(k string) string {
		if k == "EW_ACTOR" {
			return "oncall"
		}
		return ""
	}, Peers: func(caller string) (map[string]reconcile.Peer, audit.Recorder, func(), error) {
		if caller != "learning" {
			t.Fatalf("caller %s", caller)
		}
		return peers, rec, func() { closed = true }, nil
	}}
	var out, errb bytes.Buffer
	code := runReconcile(context.Background(), []string{"--module=learning", "--from=2026-11-26T00:00:00Z", "--restore-point=2026-11-26T12:00:00Z"},
		d, &out, &errb)
	if code != reconcile.ExitOK || !closed {
		t.Fatalf("exit %d closed %v: %s", code, closed, errb.String())
	}
	var r map[string]any
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("report: %v %s", err, out.String())
	}
	for _, k := range []string{"run_id", "module", "restore_point", "window_from", "window_to", "settle_seconds", "generated_at", "status", "peers",
		"counts", "mismatches", "truncated"} {
		if _, ok := r[k]; !ok {
			t.Errorf("report lacks %s", k)
		}
	}
	if r["settle_seconds"].(float64) != 600 || !strings.Contains(errb.String(), "reconcile-learning") {
		t.Fatalf("settle %v, summary %q", r["settle_seconds"], errb.String())
	}
	var bypass, done int
	for _, e := range rec.events {
		switch e.Action {
		case audit.ActionCrossTenantAccessed:
			if e.Reason == string(tenancy.ReasonPlatformJob) {
				bypass++
			}
		case "ops.reconcile.completed":
			done++
			if e.TargetID != "learning" || e.Metadata["unexplained"] != 0 || !strings.Contains(e.Reason, "oncall") {
				t.Fatalf("audit %+v", e)
			}
		}
	}
	if bypass != 1 || done != 1 {
		t.Fatalf("audits: bypass %d, completed %d", bypass, done)
	}
}
