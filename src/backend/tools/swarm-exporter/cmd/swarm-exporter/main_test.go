package main

import (
	"bytes"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestRunRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  map[string]string
		code int
		msg  string
	}{
		{"unknown command", []string{"nope"}, nil, 2, "unknown command"},
		{"missing docker api", []string{"run"}, nil, 1, "EW_SWARM_EXPORTER_DOCKER_API"},
		{"bad interval", []string{"run"}, map[string]string{"EW_SWARM_EXPORTER_DOCKER_API": "http://proxy:2375", "EW_SWARM_EXPORTER_INTERVAL": "10ms"}, 1, "EW_SWARM_EXPORTER_INTERVAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			if got := run(c.args, &out, env(c.env)); got != c.code {
				t.Fatalf("exit %d, want %d (%s)", got, c.code, out.String())
			}
			if !strings.Contains(out.String(), c.msg) {
				t.Errorf("output %q does not name %q", out.String(), c.msg)
			}
		})
	}
}

func TestHealthcheckFailsWithoutServer(t *testing.T) {
	if got := healthcheck(env(map[string]string{"EW_ADMIN_ADDR": "127.0.0.1:1"})); got != 1 {
		t.Fatalf("healthcheck = %d, want 1", got)
	}
}
