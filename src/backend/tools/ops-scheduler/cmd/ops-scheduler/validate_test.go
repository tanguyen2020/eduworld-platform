package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const stackYAML = `services:
  core-api: { image: x }
  realtime: { image: x }
  worker-events: { image: x }
`

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// F130-AC-24: an invalid schedule (peak < baseline, missing baseline, service not in the stack, string over the
// schema length) exits 1 and every problem carries its file:line:column position.
func TestValidateRejectsInvalidScheduleWithPositions(t *testing.T) {
	dir := t.TempDir()
	stack := writeFile(t, dir, "app.stack.yml", stackYAML)
	long := strings.Repeat("a", 65)
	bad := writeFile(t, dir, "scale-schedule.yaml", `version: 1
stack: eduworld
lead_minutes: 15
windows:
  - { name: morning, start: "06:45", end: "08:00" }
services:
  - { name: core-api, baseline: 6, peak: 4 }
  - { name: realtime, peak: 6 }
  - { name: ghost-service, baseline: 1, peak: 2 }
  - { name: `+long+`, baseline: 1, peak: 2 }
`)
	var out, errb bytes.Buffer
	code := run([]string{"validate", "--stacks=" + stack, bad}, &out, &errb, func(string) string { return "" })
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %s)", code, errb.String())
	}
	msg := errb.String()
	for _, want := range []string{
		bad + ":7:", "/services/0/peak: peak 4 is lower than baseline 6",
		bad + ":8:", "/services/1/baseline: must be between 1 and 64 (missing?)",
		bad + ":9:", `service "ghost-service" is not in the stack files`,
		bad + ":10:", "/services/3/name: longer than 64 characters",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("stderr lacks %q:\n%s", want, msg)
		}
	}
}

func TestValidateRejectsOverlapAndBadTime(t *testing.T) {
	dir := t.TempDir()
	bad := writeFile(t, dir, "s.yaml", `version: 1
stack: eduworld
lead_minutes: 15
windows:
  - { name: a, start: "07:00", end: "08:00" }
  - { name: b, start: "08:10", end: "09:00" }
  - { name: c, start: "25:00", end: "26:00" }
services:
  - { name: core-api, baseline: 1, peak: 2 }
`)
	var out, errb bytes.Buffer
	if code := run([]string{"validate", bad}, &out, &errb, func(string) string { return "" }); code != 1 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{`/windows/1: overlaps window "a"`, "/windows/2/start: must be HH:MM"} {
		if !strings.Contains(errb.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, errb.String())
		}
	}
}

func TestValidateRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	bad := writeFile(t, dir, "s.yaml", "version: 1\nstack: eduworld\nlead_minutes: 15\nautoscale: true\nwindows: []\nservices: []\n")
	var out, errb bytes.Buffer
	if code := run([]string{"validate", bad}, &out, &errb, func(string) string { return "" }); code != 1 || !strings.Contains(errb.String(), "autoscale") {
		t.Fatalf("exit = %d, stderr %s", code, errb.String())
	}
}

// The committed schedules of staging and production are valid against their own stack files.
func TestCommittedSchedulesAreValid(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..", "..", "deploy", "swarm")
	for _, env := range []string{"staging", "production"} {
		dir := filepath.Join(root, env)
		stacks := []string{}
		for _, f := range []string{"infra.stack.yml", "data.stack.yml", "app.stack.yml", "ops.stack.yml"} {
			stacks = append(stacks, filepath.Join(dir, f))
		}
		var out, errb bytes.Buffer
		if code := run([]string{"validate", "--stacks=" + strings.Join(stacks, ","), filepath.Join(dir, "scale-schedule.yaml")}, &out, &errb,
			func(string) string { return "" }); code != 0 {
			t.Errorf("%s: exit %d: %s", env, code, errb.String())
		}
	}
}
