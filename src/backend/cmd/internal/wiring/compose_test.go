package wiring_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/valkey-io/valkey-go"
	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring"
	"github.com/tanguyen2020/eduworld-platform/src/backend/cmd/internal/wiring/wiringtest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/health"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
)

// wiredModules are the modules whose Done features are wired into the binaries (OQ F010-2).
var wiredModules = []string{"identity", "learning", "game", "competition", "chat"}

// workerModules are the modules with worker entry points (relay, consumers, jobs).
var workerModules = []string{"identity", "learning", "game", "competition", "chat"}

// testEnv is a complete configuration of core-api/worker with unreachable endpoints: pools, Valkey and NATS are lazy
// or test doubles, so composition runs without any container (values are TEST FIXTURES, not secrets).
func testEnv(t *testing.T) func(string) string {
	t.Helper()
	env := wiringtest.Env(wiringtest.Options{RepoRoot: repoRoot(t), Issuer: "http://127.0.0.1:1/realms/eduworld", ValkeyAddr: "127.0.0.1:1",
		NATSURL: "nats://127.0.0.1:1", SecretsDir: t.TempDir(),
		DSN: func(module, role string) string {
			return "postgres://" + module + "_" + role + ":p@127.0.0.1:1/eduworld_" + module
		}})
	return func(k string) string { return env[k] }
}

// fakeValkey is a TEST DOUBLE (R17): composition only stores the client; any call would panic.
type fakeValkey struct{ valkey.Client }

func (fakeValkey) Close() {}

func registry(t *testing.T, binary, profile string) *module.Registry {
	t.Helper()
	return registryWith(t, binary, profile, testEnv(t))
}

func registryWith(t *testing.T, binary, profile string, getenv func(string) string) *module.Registry {
	t.Helper()
	r := module.NewRegistry(binary, profile, health.NewRegistry(buildinfo.Get(binary, profile)), getenv)
	t.Cleanup(r.Close)
	r.SetValkey(fakeValkey{})
	nc, err := nats.Connect("nats://127.0.0.1:1", nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	r.SetNATS(nc)
	return r
}

// specOperations returns the operationIds of specs/openapi/modules/<module>.v1.yaml.
func specOperations(t *testing.T, module string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "specs", "openapi", "modules", module+".v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"` // path items may carry a parameters list next to the operations
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, ops := range doc.Paths {
		for _, op := range ops {
			if m, ok := op.(map[string]any); ok {
				if id, _ := m["operationId"].(string); id != "" {
					out[id] = true
				}
			}
		}
	}
	return out
}

// implementedOperations returns the operationIds of the module spec that appear as string literals in the non-test
// Go files of modules/<module>/internal/presentation/http: the operations the module implements.
func implementedOperations(t *testing.T, module string, spec map[string]bool) map[string]bool {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "src", "backend", "modules", module, "internal", "presentation", "http")
	out := map[string]bool{}
	for _, lit := range stringLiterals(t, dir) {
		if spec[lit] {
			out[lit] = true
		}
	}
	return out
}

func stringLiterals(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if s, err := strconv.Unquote(bl.Value); err == nil {
					out = append(out, s)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// consumerNames returns the durable consumer names "<module>.<handler>" declared as constants in the consumer and
// messaging packages of module.
func consumerNames(t *testing.T, module string) []string {
	t.Helper()
	base := filepath.Join(repoRoot(t), "src", "backend", "modules", module, "internal")
	re := regexp.MustCompile(`^` + module + `\.[a-z_]+$`)
	seen := map[string]bool{}
	for _, dir := range []string{filepath.Join(base, "presentation", "consumer"), filepath.Join(base, "infrastructure", "messaging")} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		for _, s := range stringLiterals(t, dir) {
			if re.MatchString(s) {
				seen[s] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func declarations(r *module.Registry, kind string) map[string]module.Declaration {
	out := map[string]module.Declaration{}
	for _, d := range r.Declarations() {
		if d.Kind == kind {
			name, _, _ := strings.Cut(d.Name, "@")
			if prev, ok := out[name]; ok && prev.Active != d.Active {
				d.Active = false
			}
			out[name] = d
		}
	}
	return out
}

// OQ F010-2: core-api mounts every public operation implemented by identity, learning and game exactly once (a
// second Handle of an operation fails the start-up), each under its owning module and present in that module's
// spec; it runs no relay, consumer or job.
func TestCoreAPIMountsEveryImplementedOperation(t *testing.T) {
	r := registry(t, "core-api", "")
	c, err := wiring.ComposeCoreAPI(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	mounted := map[string]string{}
	for _, op := range c.Server.Operations() {
		mounted[op.ID] = op.Module
	}
	for _, m := range wiredModules {
		spec := specOperations(t, m)
		impl := implementedOperations(t, m, spec)
		if len(impl) == 0 {
			t.Fatalf("%s: no implemented operation found", m)
		}
		for id := range impl {
			if got, ok := mounted[id]; !ok {
				t.Errorf("%s: operation %s is implemented but not mounted", m, id)
			} else if got != m {
				t.Errorf("operation %s mounted under module %s, owner %s", id, got, m)
			}
		}
		for id, owner := range mounted {
			// An operation routed to several modules by a parameter (x-service-routing, e.g. /game-admin/{config_type})
			// is listed in the module spec as <operationId>_<module> (scripts/openapi/split-public.mjs, F118).
			if owner == m && !spec[id] && !spec[id+"_"+m] {
				t.Errorf("%s: mounted operation %s is not in specs/openapi/modules/%s.v1.yaml", m, id, m)
			}
		}
	}
	for _, d := range r.Declarations() {
		if d.Kind != module.KindHTTP && d.Active {
			t.Errorf("core-api runs %s %s", d.Kind, d.Name)
		}
	}
	if h := r.Public(); h == nil {
		t.Fatal("no public handler installed")
	}
	if !contains(r.Health().CheckNames(), "jwks") {
		t.Fatal("readiness check jwks not registered")
	}
	// F030-AC-23: the Activity Registry readiness of module learning.
	if !contains(r.Health().CheckNames(), "activity_registry:learning") {
		t.Fatal("readiness check activity_registry:learning not registered")
	}
	t.Logf("mounted %d operations", len(mounted))
}

// OQ F010-2: worker events runs the outbox relay of every wired module and every durable consumer the modules
// declare; worker jobs runs the scheduled jobs; neither serves HTTP.
func TestWorkerProfilesRegisterRelaysConsumersAndJobs(t *testing.T) {
	jobs := map[string][]string{
		"identity": {"partition-maintainer", "audit-retention", "student-inactive-cleanup", "saga-retry", "retention-anonymizer",
			"platform-saga", "org-membership-reconcile", "user-role-expiry", "sis-import-watch", "sis-staging-anonymizer", "sis-scheduler",
			"email-sender"},
		"learning": {"import-jobs", "content-scheduled-publish", "attempt-lifecycle", "learning-events-partitions", "regrade-runner",
			"recommendation-nightly", "assignment-scheduler", "exam-window", "exam-autosubmit", "dashboard-day-close", "read-model-refresh",
			"report-jobs", "item-analysis-nightly", "calibration-weekly", "reference-levels-nightly", "efficacy-wave-monitor", "efficacy-reports", "experiment-guardrail", "experiment-lifecycle"},
		"game": {"session-timeout", "curfew-session-closer", "quest-period-expiry", "quest-failure-timer", "curve-rollout", "xp-reconcile",
			"wallet-reconcile", "event-scheduler", "goal-recompute", "challenge-deadline", "scheduled-publish", "game-media-pipeline", "render-retry"},
		"competition": {"close-idle-hub-instances", "party-invitation-expiry", "party-presence-sweeper", "matchmaker", "matchmaking-ticket-expiry", "duel-expiry",
			"leaderboard-period-close", "leaderboard-rebuild", "leaderboard-reconcile", "season-scheduler", "rd-inflation", "tournament-scheduler"},
		"chat": {"expire-friend-requests", "moderation-switch-expirer", "moderation-sla-escalator", "school-notification-reminder",
			"sanction-expirer", "oncall-escalator", "break-glass-expirer"},
	}
	// Jobs of worker-chat (profile chat, also all): gameplay-chat-purge and chat-partition-maintainer on the chat_purger
	// pool (F117, ADR-0033), persistent-chat-retention on the chat_app pool (F120 US6, anonymisation only).
	chatJobs := map[string][]string{"chat": {"gameplay-chat-purge", "chat-partition-maintainer", "persistent-chat-retention"}}
	for _, profile := range []string{"events", "jobs", "all"} {
		t.Run(profile, func(t *testing.T) {
			r := registry(t, "worker", profile)
			if _, err := wiring.Compose(r); err != nil {
				t.Fatal(err)
			}
			events, jobsOn := profile != "jobs", profile != "events"
			relays, consumers, js := declarations(r, module.KindRelay), declarations(r, module.KindConsumer), declarations(r, module.KindJob)
			for _, m := range workerModules {
				if d, ok := relays[m+".outbox-relay"]; !ok || d.Active != events {
					t.Errorf("relay %s: %+v (want active=%v)", m, d, events)
				}
				if profile == "events" {
					t.Logf("%s consumers: %v", m, consumerNames(t, m))
				}
				for _, name := range consumerNames(t, m) {
					if d, ok := consumers[name]; !ok || d.Active != events {
						t.Errorf("consumer %s: declared=%v active=%v (want active=%v)", name, ok, d.Active, events)
					}
				}
				for _, j := range jobs[m] {
					d, ok := js[m+"."+j]
					if !ok || d.Active != jobsOn || d.Interval <= 0 {
						t.Errorf("job %s.%s: %+v (want active=%v with an interval)", m, j, d, jobsOn)
					}
				}
				for _, j := range chatJobs[m] { // worker-chat jobs (ADR-0033): active in profile all only, never in events/jobs
					if d, ok := js[m+"."+j]; !ok || d.Active != (profile == "all") {
						t.Errorf("worker-chat job %s.%s: declared=%v %+v (want active=%v)", m, j, ok, d, profile == "all")
					}
				}
			}
			for name, d := range js {
				m, j, _ := strings.Cut(name, ".")
				if !contains(jobs[m], j) && !contains(chatJobs[m], j) {
					t.Errorf("job %s (%+v) is not in the expected list of the test", name, d)
				}
			}
			for _, d := range declarations(r, module.KindHTTP) {
				if d.Active {
					t.Errorf("worker serves http for %s", d.Module)
				}
			}
			if r.Public() != nil {
				t.Error("worker installed a public handler")
			}
			want := 0
			for _, d := range r.Declarations() {
				if d.Active {
					want++
				}
			}
			if got := len(r.Workers()); got != want {
				t.Errorf("workers = %d, active declarations = %d", got, want)
			}
		})
	}
}

// Profile chat loads module chat only: no identity, learning or game entry point is declared; it runs chat-persister
// (F115 T003), persister-evt of the message changes (F118), the jobs of the role chat_purger: gameplay-chat-purge and
// chat-partition-maintainer (F117, ADR-0033), and persistent-chat-retention (F120 US6, role chat_app).
func TestWorkerChatProfileLoadsChatOnly(t *testing.T) {
	r := registry(t, "worker", "chat")
	if _, err := wiring.Compose(r); err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, d := range r.Declarations() {
		if !d.Active {
			continue
		}
		active++
		ok := d.Module == "chat" && ((d.Kind == module.KindConsumer && (d.Name == "chat.persister" || d.Name == "chat.persister_evt")) ||
			(d.Kind == module.KindJob && (d.Name == "chat.gameplay-chat-purge" || d.Name == "chat.chat-partition-maintainer" ||
				d.Name == "chat.persistent-chat-retention")))
		if !ok {
			t.Errorf("profile chat runs %s %s", d.Kind, d.Name)
		}
	}
	if active != 5 || len(r.Workers()) != 5 {
		t.Errorf("profile chat: %d active declarations, %d workers (want chat.persister, chat.persister_evt, the two purger jobs + persistent-chat-retention)", active, len(r.Workers()))
	}
}

// F115-AC-06 (NFR-SAFE-001) + F116: the real moderation pipeline (pipeline.Validator) is wired, so core-api and
// worker-chat start in every environment (local included) — never with the labelled test double, never without a
// validator (command.RequireValidator, unit test TestSendPipelineRequired).
func TestChatStartsWithRealValidator(t *testing.T) {
	for _, c := range []struct{ binary, profile string }{{"core-api", ""}, {"worker", "chat"}, {"worker", "all"}, {"worker", "events"},
		{"worker", "jobs"}} {
		for _, env := range []string{"", "local", "staging", "production"} {
			base := testEnv(t)
			getenv := func(k string) string {
				if k == "EW_ENV" {
					return env
				}
				return base(k)
			}
			if _, err := wiring.Compose(registryWith(t, c.binary, c.profile, getenv)); err != nil {
				t.Errorf("%s %s with EW_ENV=%q: %v (the real pipeline must be accepted)", c.binary, c.profile, env, err)
			}
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// repoRoot walks up from the package directory to the repository root (the directory holding specs/).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "specs", "openapi", "eduworld-api.v1.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
