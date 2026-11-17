//go:build integration

// Package testkit starts real dependencies for integration tests with testcontainers (F001 T017,
// T018, T027, T028). F003 extends it (ent clients, outbox, NATS JetStream helpers).
package testkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Password of every local role (deploy/postgres/init/02-roles.sh default).
const Password = "change-me-local"

// Modules with a database.
var Modules = []string{"identity", "learning", "game", "competition", "chat"}

// AtlasImage must match tools/versions.mk.
const AtlasImage = "arigaio/atlas:0.38.0"

// RepoRoot returns the repository root (directory holding AGENTS.md; the Go module lives in src/backend).
func RepoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root (AGENTS.md) not found")
		}
		dir = parent
	}
}

// Cluster is a PostgreSQL 17 container initialised with deploy/postgres/init (databases + roles).
type Cluster struct {
	Container testcontainers.Container
	Network   *testcontainers.DockerNetwork
	Host      string
	Port      string
	root      string
}

// StartCluster starts the cluster; when migrate is true it applies every module's Atlas migrations.
func StartCluster(ctx context.Context, t testing.TB, migrate bool) *Cluster {
	t.Helper()
	root := RepoRoot(t)
	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("network: %v", err)
	}
	t.Cleanup(func() { _ = net.Remove(context.Background()) })

	initDir := filepath.Join(root, "deploy", "postgres", "init")
	entries, err := os.ReadDir(initDir)
	if err != nil {
		t.Fatalf("read init dir: %v", err)
	}
	var files []testcontainers.ContainerFile
	for _, e := range entries {
		mode := int64(0o644)
		if strings.HasSuffix(e.Name(), ".sh") {
			mode = 0o755
		}
		files = append(files, testcontainers.ContainerFile{
			HostFilePath:      filepath.Join(initDir, e.Name()),
			ContainerFilePath: "/docker-entrypoint-initdb.d/" + e.Name(),
			FileMode:          mode,
		})
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "postgres:17",
			Env:            map[string]string{"POSTGRES_PASSWORD": Password, "EW_DB_ROLE_PASSWORD": Password},
			ExposedPorts:   []string{"5432/tcp"},
			Files:          files,
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {"postgres"}},
			// The entrypoint restarts the server after init scripts: wait for the second "ready" line.
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	cl := &Cluster{Container: c, Network: net, Host: host, Port: port.Port(), root: root}
	if migrate {
		for _, m := range Modules {
			cl.Migrate(ctx, t, m)
		}
	}
	return cl
}

// DSN returns a host-side connection string for role on the module database.
func (c *Cluster) DSN(role, database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", role, Password, c.Host, c.Port, database)
}

// ModuleDSN returns the DSN of <module>_<suffix> on eduworld_<module>.
func (c *Cluster) ModuleDSN(module, suffix string) string {
	return c.DSN(module+"_"+suffix, "eduworld_"+module)
}

// Migrate applies the module's Atlas migrations with the pinned Atlas image, as <module>_migrator.
func (c *Cluster) Migrate(ctx context.Context, t testing.TB, module string) {
	t.Helper()
	dir := filepath.Join(c.root, "src", "backend", "modules", module, "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations of %s: %v", module, err)
	}
	var files []testcontainers.ContainerFile
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") || e.Name() == "atlas.sum" {
			files = append(files, testcontainers.ContainerFile{
				HostFilePath:      filepath.Join(dir, e.Name()),
				ContainerFilePath: "/migrations/" + e.Name(),
				FileMode:          0o644,
			})
		}
	}
	url := fmt.Sprintf("postgres://%s_migrator:%s@postgres:5432/eduworld_%s?sslmode=disable", module, Password, module)
	a, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      AtlasImage,
			Cmd:        []string{"migrate", "apply", "--dir", "file:///migrations", "--url", url},
			Files:      files,
			Networks:   []string{c.Network.Name},
			WaitingFor: wait.ForExit().WithExitTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("atlas migrate %s: %v", module, err)
	}
	defer func() { _ = a.Terminate(context.Background()) }()
	state, err := a.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.ExitCode != 0 {
		logs, _ := a.Logs(ctx)
		var b strings.Builder
		if logs != nil {
			buf := make([]byte, 4096)
			n, _ := logs.Read(buf)
			b.Write(buf[:n])
		}
		t.Fatalf("atlas migrate %s exit %d: %s", module, state.ExitCode, b.String())
	}
}

// StartValkey starts Valkey 8 and returns host:port.
func StartValkey(ctx context.Context, t testing.TB) (testcontainers.Container, string) {
	t.Helper()
	return start(ctx, t, "valkey/valkey:8", "6379/tcp", nil, wait.ForLog("Ready to accept connections"))
}

// StartNATS starts NATS with JetStream and returns the container and nats:// URL.
func StartNATS(ctx context.Context, t testing.TB) (testcontainers.Container, string) {
	t.Helper()
	c, addr := start(ctx, t, "nats:2.10-alpine", "4222/tcp", []string{"-js"}, wait.ForLog("Server is ready"))
	return c, "nats://" + addr
}

func start(ctx context.Context, t testing.TB, image, port string, cmd []string, w wait.Strategy) (testcontainers.Container, string) {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{Image: image, ExposedPorts: []string{port}, Cmd: cmd, WaitingFor: w},
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start %s: %v", image, err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.MappedPort(ctx, port)
	if err != nil {
		t.Fatal(err)
	}
	return c, host + ":" + p.Port()
}
