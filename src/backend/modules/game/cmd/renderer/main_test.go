package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/render"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{
		"EW_NATS_URL": "nats://nats:4222", "EW_S3_ENDPOINT": "http://floci:4566", "EW_S3_ACCESS_KEY": "test",
		"EW_S3_SECRET_KEY": "test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NATSURL != "nats://nats:4222" || !slices.Equal(cfg.Godot, []string{"godot"}) || cfg.Project != "/opt/eduworld/game" ||
		cfg.Timeout != 120*time.Second || cfg.AdminAddr != ":9090" || cfg.WorkDir == "" {
		t.Fatalf("defaults %+v", cfg)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{
		"EW_NATS_URLS": "nats://a:4222,nats://b:4222", "EW_S3_ENDPOINT": "http://s3:9000", "EW_S3_REGION": "eu-1",
		"EW_S3_ACCESS_KEY": "k", "EW_S3_SECRET_KEY": "s",
		"EW_RENDERER_GODOT":   "/usr/local/bin/godot-render  --verbose",
		"EW_RENDERER_PROJECT": "/srv/game", "EW_RENDERER_TIMEOUT": "45s", "EW_RENDERER_WORKDIR": "/tmp/r",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NATSURL != "nats://a:4222,nats://b:4222" || cfg.S3.Region != "eu-1" || cfg.Project != "/srv/game" ||
		cfg.Timeout != 45*time.Second || cfg.WorkDir != "/tmp/r" ||
		!slices.Equal(cfg.Godot, []string{"/usr/local/bin/godot-render", "--verbose"}) {
		t.Fatalf("overrides %+v", cfg)
	}
}

func TestLoadConfigReportsMissingVariablesByNameOnly(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{"EW_S3_ACCESS_KEY": "super-secret-access"}))
	if err == nil {
		t.Fatal("missing variables accepted")
	}
	for _, name := range []string{"EW_NATS_URL", "EW_S3_ENDPOINT", "EW_S3_SECRET_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("%q does not name %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "super-secret-access") || strings.Contains(err.Error(), "EW_S3_ACCESS_KEY") {
		t.Fatalf("%q leaks a value or names a present variable", err)
	}
	_, err = LoadConfig(env(map[string]string{"EW_NATS_URL": "nats://n", "EW_S3_ENDPOINT": "http://s",
		"EW_S3_ACCESS_KEY": "a", "EW_S3_SECRET_KEY": "b", "EW_RENDERER_TIMEOUT": "0s"}))
	if err == nil {
		t.Fatal("zero timeout accepted")
	}
}

func TestLoadConfigReadsSecretFiles(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s3_secret")
	if err := os.WriteFile(f, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(env(map[string]string{"EW_NATS_URL": "nats://n", "EW_S3_ENDPOINT": "http://s",
		"EW_S3_ACCESS_KEY": "a", "EW_S3_SECRET_KEY_FILE": f}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.S3.SecretKey != "from-file" {
		t.Fatal("EW_S3_SECRET_KEY_FILE not read")
	}
}

func TestGodotArgs(t *testing.T) {
	g := GodotRunner{Command: []string{"xvfb-run", "-a", "godot"}, Project: "/opt/eduworld/game"}
	got := g.Args(RenderRequest{Input: "/w/render-1/source.glb", OutputDir: "/w/render-1/out", Kind: render.KindTurntable,
		Size: 512, Frames: 24})
	want := []string{"-a", "godot", "--path", "/opt/eduworld/game", "--script",
		"res://tools/thumbnail_renderer/render_thumbnails.gd", "--", "--input", "/w/render-1/source.glb",
		"--output", "/w/render-1/out", "--kind", "render_turntable", "--size", "512", "--frames", "24"}
	if !slices.Equal(got, want) {
		t.Fatalf("args\n got %q\nwant %q", got, want)
	}
	if !slices.Equal(g.Command, []string{"xvfb-run", "-a", "godot"}) {
		t.Fatal("Args modified the configured command")
	}
}

// TestHelperGodot is not a test: GodotRunner runs the test binary as a stand-in for Godot (TEST-ONLY helper process).
func TestHelperGodot(t *testing.T) {
	mode := os.Getenv("RENDERER_TEST_HELPER")
	if mode == "" {
		t.Skip("helper process only")
	}
	switch mode {
	case "fail":
		fmt.Fprintln(os.Stderr, "Godot Engine v4.7.2.stable")
		fmt.Fprintln(os.Stderr, "render_thumbnails: empty scene: no visible mesh in source.glb")
		os.Exit(5)
	case "sleep":
		time.Sleep(10 * time.Second)
	}
	os.Exit(0)
}

func helperRunner() GodotRunner {
	return GodotRunner{Command: []string{os.Args[0], "-test.run=^TestHelperGodot$", "--"}, Project: "/opt/eduworld/game"}
}

func TestGodotRunnerReportsTheScriptError(t *testing.T) {
	t.Setenv("RENDERER_TEST_HELPER", "fail")
	err := helperRunner().Render(context.Background(), RenderRequest{OutputDir: t.TempDir(), Kind: render.KindThumbnail,
		Size: 512, Frames: 1})
	if err == nil || !strings.Contains(err.Error(), "render_thumbnails: empty scene") ||
		strings.Contains(err.Error(), "Godot Engine") {
		t.Fatalf("error %v", err)
	}
}

func TestGodotRunnerStopsAtTheDeadline(t *testing.T) {
	t.Setenv("RENDERER_TEST_HELPER", "sleep")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := helperRunner().Render(ctx, RenderRequest{OutputDir: t.TempDir(), Kind: render.KindThumbnail, Size: 512, Frames: 1})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 8*time.Second {
		t.Fatalf("error %v after %s", err, time.Since(start))
	}
}

func TestQueueConfiguration(t *testing.T) {
	s := StreamConfig()
	if s.Name != "GAME_RENDER_JOBS" || !slices.Equal(s.Subjects, []string{"ew.job.game.render.>"}) ||
		s.Retention != jetstream.WorkQueuePolicy || s.MaxAge != render.MaxAge {
		t.Fatalf("stream %+v", s)
	}
	c := ConsumerConfig(120 * time.Second)
	if c.Durable != "game_renderer" || c.FilterSubject != "ew.job.game.render.request" ||
		c.AckPolicy != jetstream.AckExplicitPolicy || c.AckWait < 120*time.Second || c.MaxDeliver <= 0 {
		t.Fatalf("consumer %+v", c)
	}
	id := uuid.MustParse("0b9c5f5e-8f2a-4c1e-9d3b-5a6e7f8091a2")
	if got := ResultMsgID(render.Result{RenderJobID: id, Attempt: 3}); got != "result:"+id.String()+":3" {
		t.Fatalf("msg id %q", got)
	}
}

func TestTailBufferKeepsTheEnd(t *testing.T) {
	b := &tailBuffer{max: 16}
	_, _ = b.Write([]byte("first line\nsecond line\nthird\n"))
	if got := b.lastLines(1); got != "third" {
		t.Fatalf("last line %q", got)
	}
	if len(b.buf) > 16 {
		t.Fatalf("buffer grew to %d", len(b.buf))
	}
}
