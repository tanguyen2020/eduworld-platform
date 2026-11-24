package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScriptPath is the renderer script inside the Godot project (same res:// path in src/game and in the minimal project
// of the container, src/game/tools/thumbnail_renderer/renderer_project.godot).
const ScriptPath = "res://tools/thumbnail_renderer/render_thumbnails.gd"

// GodotRunner runs render_thumbnails.gd with the configured command (EW_RENDERER_GODOT, split on blanks). The command
// must give Godot a real renderer: `--headless` uses the dummy rendering server and produces empty images. The
// container sets it to /usr/local/bin/godot-render, a wrapper running Godot under `xvfb-run` with
// `--rendering-driver opengl3` (deploy/docker/game-renderer.Dockerfile).
type GodotRunner struct {
	Command []string // binary and fixed leading arguments
	Project string   // --path of the Godot project
}

// Args is the full argument list after the binary (exported for tests).
func (g GodotRunner) Args(req RenderRequest) []string {
	args := append([]string(nil), g.Command[1:]...)
	return append(args, "--path", g.Project, "--script", ScriptPath, "--",
		"--input", req.Input, "--output", req.OutputDir, "--kind", req.Kind,
		"--size", strconv.Itoa(req.Size), "--frames", strconv.Itoa(req.Frames))
}

// Render runs Godot until it exits or ctx ends (the whole process group is killed then). A non-zero exit returns the
// last lines of stderr (the script prints `render_thumbnails: <reason>`).
func (g GodotRunner) Render(ctx context.Context, req RenderRequest) error {
	if len(g.Command) == 0 || g.Command[0] == "" {
		return errors.New("no godot command configured")
	}
	cmd := exec.CommandContext(ctx, g.Command[0], g.Args(req)...)
	cmd.Dir = req.OutputDir
	stderr := &tailBuffer{max: 4096}
	cmd.Stdout = &tailBuffer{max: 1024}
	cmd.Stderr = stderr
	cmd.WaitDelay = 5 * time.Second
	isolate(cmd)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if tail := stderr.lastLines(3); tail != "" {
			return fmt.Errorf("%w: %s", err, tail)
		}
		return err
	}
	return nil
}

// tailBuffer keeps the last max bytes written.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.max; over > 0 {
		t.buf = t.buf[over:]
	}
	return len(p), nil
}

// lastLines returns the last n non-empty lines, preferring the script's own `render_thumbnails:` lines.
func (t *tailBuffer) lastLines(n int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var lines, own []string
	for _, l := range bytes.Split(t.buf, []byte("\n")) {
		s := strings.TrimSpace(string(l))
		if s == "" {
			continue
		}
		lines = append(lines, s)
		if strings.HasPrefix(s, "render_thumbnails:") {
			own = append(own, s)
		}
	}
	if len(own) > 0 {
		lines = own
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}
